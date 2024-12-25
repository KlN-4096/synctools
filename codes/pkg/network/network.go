package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/errors"
)

// Server 网络服务器实现
type Server struct {
	config      *interfaces.Config
	syncService interfaces.SyncService
	listener    net.Listener
	clients     map[string]*Client
	clientsMux  sync.RWMutex
	logger      interfaces.Logger
	running     bool
	status      string
	networkOps  interfaces.NetworkOperations
}

// NewServer 创建新的网络服务器
func NewServer(config *interfaces.Config, syncService interfaces.SyncService, logger interfaces.Logger) *Server {
	return &Server{
		config:      config,
		syncService: syncService,
		clients:     make(map[string]*Client),
		logger:      logger,
		status:      "初始化",
		networkOps:  NewOperations(logger),
	}
}

// Start 实现 interfaces.NetworkServer 接口
func (s *Server) Start() error {
	if s.running {
		return errors.ErrNetworkServerStart
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.status = fmt.Sprintf("启动失败: %v", err)
		return errors.NewError("NETWORK_START", "启动服务器失败", err)
	}

	s.listener = listener
	s.running = true
	s.status = "运行中"

	s.logger.Info("服务状态变更", interfaces.Fields{
		"status":  "started",
		"type":    "network",
		"address": s.listener.Addr().String(),
	})

	// 在新的 goroutine 中处理客户端连接
	go s.acceptLoop()

	return nil
}

// Stop 实现 interfaces.NetworkServer 接口
func (s *Server) Stop() error {
	if !s.running {
		return nil
	}
	s.running = false

	if s.listener != nil {
		s.listener.Close()
	}

	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()

	for _, client := range s.clients {
		client.Close()
	}
	s.clients = make(map[string]*Client)
	s.status = "已停止"

	s.logger.Info("服务状态变更", interfaces.Fields{
		"status": "stopped",
		"type":   "network",
	})
	return nil
}

// HandleClient 实现 interfaces.NetworkServer 接口
func (s *Server) HandleClient(conn net.Conn) {
	// 设置连接超时
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	defer func() {
		conn.Close()
		s.logger.Info("网络连接", interfaces.Fields{
			"action":  "client_disconnected",
			"address": conn.RemoteAddr().String(),
		})
	}()

	client := NewClient(conn, s)
	s.addClient(client)
	client.Start() // 直接调用，不使用 goroutine
}

// GetStatus 实现 interfaces.NetworkServer 接口
func (s *Server) GetStatus() string {
	return s.status
}

// IsRunning 实现 interfaces.NetworkServer 接口
func (s *Server) IsRunning() bool {
	return s.running
}

// acceptLoop 接受新的客户端连接
func (s *Server) acceptLoop() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				s.logger.Warn("网络操作", interfaces.Fields{
					"action": "accept_temporary_error",
					"error":  err,
				})
				continue
			}
			if s.running {
				s.logger.Error("网络操作失败", interfaces.Fields{
					"operation": "accept",
					"error":     err,
				})
			}
			return
		}
		s.logger.Info("网络连接", interfaces.Fields{
			"action":  "client_connected",
			"address": conn.RemoteAddr().String(),
		})
		go s.HandleClient(conn)
	}
}

// addClient 添加客户端
func (s *Server) addClient(client *Client) {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()
	s.clients[client.ID] = client
	s.logger.Info("客户端已连接", interfaces.Fields{
		"id":   client.ID,
		"addr": client.conn.RemoteAddr(),
	})
}

// removeClient 移除客户端
func (s *Server) removeClient(client *Client) {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()
	delete(s.clients, client.ID)
	s.logger.Info("客户端已断开", interfaces.Fields{
		"id": client.ID,
	})
}

// Client 客户端连接
type Client struct {
	ID           string
	UUID         string
	conn         net.Conn
	server       *Server
	lastActivity time.Time
}

// NewClient 创建新的客户端
func NewClient(conn net.Conn, server *Server) *Client {
	client := &Client{
		ID:           fmt.Sprintf("client-%s", conn.RemoteAddr()),
		conn:         conn,
		server:       server,
		lastActivity: time.Now(),
	}

	// 启动超时检查
	go client.checkTimeout()

	return client
}

// Start 开始处理客户端消息
func (c *Client) Start() {
	defer c.Close()

	for {
		var msg interfaces.Message
		if err := c.server.networkOps.ReadJSON(c.conn, &msg); err != nil {
			if err != io.EOF {
				c.server.logger.Error("读取消息失败", interfaces.Fields{
					"client": c.ID,
					"error":  err,
				})
			}
			return
		}

		c.lastActivity = time.Now()
		c.handleMessage(&msg)
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(msg *interfaces.Message) {
	c.server.logger.Debug("收到消息", interfaces.Fields{
		"client": c.ID,
		"type":   msg.Type,
	})

	// 处理消息
	if err := c.server.syncService.HandleSyncRequest(msg); err != nil {
		c.server.logger.Error("处理消息失败", interfaces.Fields{
			"client": c.ID,
			"type":   msg.Type,
			"error":  err,
		})
	}
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.conn.Close()
	c.server.removeClient(c)
}

// checkTimeout 检查客户端是否超时
func (c *Client) checkTimeout() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if time.Since(c.lastActivity) > time.Duration(c.server.config.ConnTimeout)*time.Second {
			c.server.logger.Info("客户端超时", interfaces.Fields{
				"id":            c.ID,
				"last_activity": c.lastActivity,
			})
			c.Close()
			return
		}
	}
}

// Operations 实现网络操作接口
type Operations struct {
	logger interfaces.Logger
}

// NewOperations 创建网络操作实例
func NewOperations(logger interfaces.Logger) interfaces.NetworkOperations {
	return &Operations{
		logger: logger,
	}
}

// WriteJSON 写入JSON数据
func (o *Operations) WriteJSON(conn net.Conn, data interface{}) error {
	if conn == nil {
		return errors.NewNetworkError("WriteJSON", "连接为空", nil)
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(data); err != nil {
		return errors.NewNetworkError("WriteJSON", "编码JSON数据失败", err)
	}
	return nil
}

// ReadJSON 读取JSON数据
func (o *Operations) ReadJSON(conn net.Conn, data interface{}) error {
	if conn == nil {
		return errors.NewNetworkError("ReadJSON", "连接为空", nil)
	}

	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(data); err != nil {
		return errors.NewNetworkError("ReadJSON", "解码JSON数据失败", err)
	}
	return nil
}

// SendFile 发送文件
func (o *Operations) SendFile(conn net.Conn, path string, progress chan<- interfaces.Progress) error {
	if conn == nil {
		return errors.NewNetworkError("SendFile", "连接为空", nil)
	}

	file, err := os.Open(path)
	if err != nil {
		return errors.NewNetworkError("SendFile", "打开文件失败", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return errors.NewNetworkError("SendFile", "获取文件信息失败", err)
	}

	buffer := make([]byte, 32*1024)
	totalWritten := int64(0)
	startTime := time.Now()

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.NewNetworkError("SendFile", "读取文件失败", err)
		}

		written, err := conn.Write(buffer[:n])
		if err != nil {
			return errors.NewNetworkError("SendFile", "写入数据失败", err)
		}

		totalWritten += int64(written)

		if progress != nil {
			elapsed := time.Since(startTime).Seconds()
			speed := float64(totalWritten) / elapsed
			remaining := int64((float64(info.Size()-totalWritten) / speed))

			progress <- interfaces.Progress{
				Total:     info.Size(),
				Current:   totalWritten,
				Speed:     speed,
				Remaining: remaining,
				FileName:  filepath.Base(path),
				Status:    "sending",
			}
		}
	}

	return nil
}

// ReceiveFile 接收文件
func (o *Operations) ReceiveFile(conn net.Conn, path string, progress chan<- interfaces.Progress) error {
	if conn == nil {
		return errors.NewNetworkError("ReceiveFile", "连接为空", nil)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return errors.NewNetworkError("ReceiveFile", "创建目录失败", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return errors.NewNetworkError("ReceiveFile", "创建文件失败", err)
	}
	defer file.Close()

	buffer := make([]byte, 32*1024)
	totalRead := int64(0)
	startTime := time.Now()

	for {
		n, err := conn.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.NewNetworkError("ReceiveFile", "读取数据失败", err)
		}

		written, err := file.Write(buffer[:n])
		if err != nil {
			return errors.NewNetworkError("ReceiveFile", "写入文件失败", err)
		}

		totalRead += int64(written)

		if progress != nil {
			elapsed := time.Since(startTime).Seconds()
			speed := float64(totalRead) / elapsed

			progress <- interfaces.Progress{
				Total:     -1, // 未知总大小
				Current:   totalRead,
				Speed:     speed,
				Remaining: -1, // 未知剩余时间
				FileName:  filepath.Base(path),
				Status:    "receiving",
			}
		}
	}

	return nil
}

// SendFiles 发送多个文件
func (o *Operations) SendFiles(conn net.Conn, files []string, progress chan<- interfaces.Progress) error {
	for _, file := range files {
		if err := o.SendFile(conn, file, progress); err != nil {
			return err
		}
	}
	return nil
}

// ReceiveFiles 接收多个文件
func (o *Operations) ReceiveFiles(conn net.Conn, destDir string, progress chan<- interfaces.Progress) error {
	for {
		var fileInfo interfaces.FileInfo
		if err := o.ReadJSON(conn, &fileInfo); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		destPath := filepath.Join(destDir, fileInfo.Path)
		if err := o.ReceiveFile(conn, destPath, progress); err != nil {
			return err
		}
	}
	return nil
}
