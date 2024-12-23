/*
文件作用:
- 实现网络服务器
- 管理客户端连接
- 处理网络消息和数据传输
- 提供服务器状态管理

主要方法:
- NewServer: 创建新的网络服务器
- Start: 启动服务器
- Stop: 停止服务器
- HandleClient: 处理客户端连接
- GetStatus: 获取服务器状态
- IsRunning: 检查服务器是否运行中
- acceptLoop: 接受新的客户端连接
- addClient/removeClient: 管理客户端连接

Client结构体方法:
- NewClient: 创建新的客户端连接
- Start: 开始处理客户端消息
- handleMessage: 处理客户端消息
- Close: 关闭客户端连接
*/

package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"synctools/internal/interfaces"
	"synctools/pkg/errors"
)

// Server 网络服务器实现
type Server struct {
	config     *interfaces.Config
	listener   net.Listener
	clients    map[string]*Client
	clientsMux sync.RWMutex
	logger     interfaces.Logger
	running    bool
	status     string
}

// NewServer 创建新的网络服务器
func NewServer(config *interfaces.Config, logger interfaces.Logger) *Server {
	return &Server{
		config:  config,
		clients: make(map[string]*Client),
		logger:  logger,
		status:  "初始化",
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
	defer func() {
		conn.Close()
		s.logger.Info("网络连接", interfaces.Fields{
			"action":  "client_disconnected",
			"address": conn.RemoteAddr().String(),
		})
	}()

	client := NewClient(conn, s)
	s.addClient(client)
	go client.Start()
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
		s.HandleClient(conn)
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
	return &Client{
		ID:           fmt.Sprintf("client-%s", conn.RemoteAddr()),
		conn:         conn,
		server:       server,
		lastActivity: time.Now(),
	}
}

// Start 开始处理客户端连接
func (c *Client) Start() {
	defer func() {
		c.Close()
		c.server.removeClient(c)
	}()

	decoder := json.NewDecoder(c.conn)
	for {
		var msg interfaces.Message
		if err := decoder.Decode(&msg); err != nil {
			if err != io.EOF {
				c.server.logger.Error("读取客户端消息失败", interfaces.Fields{
					"error":  err,
					"client": c.ID,
				})
			}
			return
		}

		c.lastActivity = time.Now()
		if err := c.handleMessage(&msg); err != nil {
			c.server.logger.Error("处理消息失败", interfaces.Fields{
				"error":  err,
				"client": c.ID,
				"type":   msg.Type,
			})
		}
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(msg *interfaces.Message) error {
	// 具体消息处理逻辑将在后续实现
	return nil
}

// Close 关闭客户端连接
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
