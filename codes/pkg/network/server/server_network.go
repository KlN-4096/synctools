package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/errors"
	"synctools/codes/pkg/network/message"
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
}

// Client 客户端连接
type Client struct {
	ID           string
	UUID         string
	conn         net.Conn
	server       *Server
	lastActivity time.Time
	msgSender    *message.MessageSender
}

// NewServer 创建新的网络服务器
func NewServer(config *interfaces.Config, syncService interfaces.SyncService, logger interfaces.Logger) *Server {
	return &Server{
		config:      config,
		syncService: syncService,
		clients:     make(map[string]*Client),
		logger:      logger,
		status:      "初始化",
	}
}

// Start 启动服务器
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

	go s.acceptClients()
	return nil
}

// Stop 停止服务器
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
		client.conn.Close()
		delete(s.clients, client.ID)
	}
	s.status = "已停止"

	s.logger.Info("服务状态变更", interfaces.Fields{
		"status": "stopped",
		"type":   "network",
	})
	return nil
}

// acceptClients 接受新的客户端连接
func (s *Server) acceptClients() {
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
		go s.HandleClient(conn)
	}
}

// HandleClient 处理客户端连接
func (s *Server) HandleClient(conn net.Conn) {
	// 设置连接超时
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(60 * time.Second))

	// 创建客户端实例
	client := &Client{
		ID:           fmt.Sprintf("client-%s", conn.RemoteAddr()),
		conn:         conn,
		server:       s,
		lastActivity: time.Now(),
		msgSender:    message.NewMessageSender(s.logger),
	}

	// 添加到客户端列表
	s.clientsMux.Lock()
	s.clients[client.ID] = client
	s.clientsMux.Unlock()

	s.logger.Info("客户端已连接", interfaces.Fields{
		"id":   client.ID,
		"addr": conn.RemoteAddr(),
	})

	// 启动超时检查
	go s.monitorClient(client)

	defer func() {
		conn.Close()
		s.clientsMux.Lock()
		delete(s.clients, client.ID)
		s.clientsMux.Unlock()
		s.logger.Info("客户端已断开", interfaces.Fields{
			"id": client.ID,
		})
	}()

	// 处理客户端消息
	for {
		msg, err := client.msgSender.ReceiveMessage(conn)
		if err != nil {
			if err != io.EOF {
				s.logger.Error("读取消息失败", interfaces.Fields{
					"client": client.ID,
					"error":  err,
				})
			}
			return
		}

		client.lastActivity = time.Now()

		switch msg.Type {
		case "init":
			client.UUID = msg.UUID
			response := struct {
				Success bool               `json:"success"`
				Message string             `json:"message"`
				Config  *interfaces.Config `json:"config"`
			}{
				Success: true,
				Message: "初始化成功",
				Config:  s.config,
			}
			if err := client.msgSender.SendMessage(conn, "init_response", msg.UUID, response); err != nil {
				s.logger.Error("发送初始化响应失败", interfaces.Fields{
					"error": err,
					"uuid":  msg.UUID,
				})
				return
			}

		case "sync_request":
			var syncRequest interfaces.SyncRequest
			if err := json.Unmarshal(msg.Payload, &syncRequest); err != nil {
				client.msgSender.SendMessage(conn, "sync_response", msg.UUID, map[string]interface{}{
					"success": false,
					"message": fmt.Sprintf("解析同步请求失败: %v", err),
				})
				continue
			}

			if err := s.syncService.HandleSyncRequest(&syncRequest); err != nil {
				client.msgSender.SendMessage(conn, "sync_response", msg.UUID, map[string]interface{}{
					"success": false,
					"message": err.Error(),
				})
				continue
			}

			client.msgSender.SendMessage(conn, "sync_response", msg.UUID, map[string]interface{}{
				"success": true,
				"message": "同步成功",
			})

		case "file_info":
			// 处理文件传输请求
			var fileInfo interfaces.FileInfo
			if err := json.Unmarshal(msg.Payload, &fileInfo); err != nil {
				s.logger.Error("解析文件信息失败", interfaces.Fields{
					"error": err,
					"uuid":  msg.UUID,
				})
				continue
			}
			// 处理文件接收...

		case "heartbeat":
			client.lastActivity = time.Now()

		default:
			s.logger.Error("未知的消息类型", interfaces.Fields{
				"type": msg.Type,
			})
		}
	}
}

// monitorClient 监控客户端连接状态
func (s *Server) monitorClient(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !s.running {
			return
		}

		if time.Since(client.lastActivity) > time.Duration(s.config.ConnTimeout)*time.Second {
			s.logger.Info("客户端超时", interfaces.Fields{
				"id":            client.ID,
				"last_activity": client.lastActivity,
			})
			client.conn.Close()
			return
		}
	}
}

// GetStatus 获取服务器状态
func (s *Server) GetStatus() string {
	return s.status
}

// IsRunning 检查服务器是否运行中
func (s *Server) IsRunning() bool {
	return s.running
}
