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

	go s.acceptLoop()
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

// HandleClient 处理客户端连接
func (s *Server) HandleClient(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(60 * time.Second))

	defer func() {
		conn.Close()
		s.logger.Info("网络连接", interfaces.Fields{
			"action":  "client_disconnected",
			"address": conn.RemoteAddr().String(),
		})
	}()

	client := NewClient(conn, s)
	s.addClient(client)
	client.Start()
}

// GetStatus 获取服务器状态
func (s *Server) GetStatus() string {
	return s.status
}

// IsRunning 检查服务器是否运行中
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
	msgSender    *message.MessageSender
}

// NewClient 创建新的客户端
func NewClient(conn net.Conn, server *Server) *Client {
	client := &Client{
		ID:           fmt.Sprintf("client-%s", conn.RemoteAddr()),
		conn:         conn,
		server:       server,
		lastActivity: time.Now(),
		msgSender:    message.NewMessageSender(server.logger),
	}

	go client.checkTimeout()
	return client
}

// Start 开始处理客户端消息
func (c *Client) Start() {
	defer c.Close()

	for {
		msg, err := c.msgSender.ReceiveMessage(c.conn)
		if err != nil {
			if err != io.EOF {
				c.server.logger.Error("读取消息失败", interfaces.Fields{
					"client": c.ID,
					"error":  err,
				})
			}
			return
		}

		c.lastActivity = time.Now()
		c.handleMessage(msg)
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(msg *interfaces.Message) {
	c.server.logger.Debug("收到消息", interfaces.Fields{
		"client": c.ID,
		"type":   msg.Type,
	})

	switch msg.Type {
	case "init":
		if err := c.handleInitMessage(msg); err != nil {
			c.server.logger.Error("处理初始化消息失败", interfaces.Fields{
				"error": err,
				"uuid":  msg.UUID,
			})
		}
	case "sync_request":
		if err := c.handleSyncRequest(msg); err != nil {
			c.server.logger.Error("处理同步请求失败", interfaces.Fields{
				"error": err,
				"uuid":  msg.UUID,
			})
		}
	case "heartbeat":
		c.lastActivity = time.Now()
	default:
		c.server.logger.Error("未知的消息类型", interfaces.Fields{
			"type": msg.Type,
		})
	}
}

// handleInitMessage 处理初始化消息
func (c *Client) handleInitMessage(msg *interfaces.Message) error {
	c.UUID = msg.UUID
	return c.msgSender.SendSyncResponse(c.conn, msg.UUID, true, "初始化成功")
}

// handleSyncRequest 处理同步请求
func (c *Client) handleSyncRequest(msg *interfaces.Message) error {
	c.conn.SetReadDeadline(time.Time{})
	c.conn.SetWriteDeadline(time.Time{})
	defer func() {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	}()

	var syncRequest interfaces.SyncRequest
	syncMsg, err := c.msgSender.ReceiveMessage(c.conn)
	if err != nil {
		return c.msgSender.SendSyncResponse(c.conn, msg.UUID, false, fmt.Sprintf("接收同步请求失败: %v", err))
	}

	if err := json.Unmarshal(syncMsg.Payload, &syncRequest); err != nil {
		return c.msgSender.SendSyncResponse(c.conn, msg.UUID, false, fmt.Sprintf("解析同步请求失败: %v", err))
	}

	if err := c.server.syncService.HandleSyncRequest(&syncRequest); err != nil {
		return c.msgSender.SendSyncResponse(c.conn, msg.UUID, false, err.Error())
	}

	return c.msgSender.SendSyncResponse(c.conn, msg.UUID, true, "同步成功")
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.conn.Close()
	c.server.removeClient(c)
}

// checkTimeout 检查客户端是否超时
func (c *Client) checkTimeout() {
	ticker := time.NewTicker(60 * time.Second)
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
