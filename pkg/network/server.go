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
	"synctools/pkg/storage"
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
	go func() {
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
	}()

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

// checkTimeout 检查客户端是否超时
func (c *Client) checkTimeout() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Since(c.lastActivity) > 40*time.Minute {
				c.server.logger.Warn("客户端超时", interfaces.Fields{
					"client":       c.ID,
					"lastActivity": c.lastActivity,
				})
				c.Close()
				return
			}
		}
	}
}

// Start 开始处理客户端连接
func (c *Client) Start() {
	defer func() {
		c.Close()
		c.server.removeClient(c)
	}()

	for {
		var msg interfaces.Message
		if err := c.server.networkOps.ReadJSON(c.conn, &msg); err != nil {
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
	c.server.logger.Debug("收到客户端消息", interfaces.Fields{
		"client": c.ID,
		"type":   msg.Type,
		"uuid":   msg.UUID,
	})

	switch msg.Type {
	case "init":
		// 处理初始化消息
		c.UUID = msg.UUID
		c.server.logger.Info("客户端初始化", interfaces.Fields{
			"client": c.ID,
			"uuid":   c.UUID,
		})

		// 准备配置响应
		configResponse := struct {
			Success bool               `json:"success"`
			Config  *interfaces.Config `json:"config"`
		}{
			Success: true,
			Config:  c.server.config,
		}

		// 序列化配置响应
		payload, err := json.Marshal(configResponse)
		if err != nil {
			c.server.logger.Error("序列化配置响应失败", interfaces.Fields{
				"error":  err,
				"client": c.ID,
			})
			return err
		}

		// 发送初始化响应
		response := &interfaces.Message{
			Type:    "init_response",
			UUID:    c.UUID,
			Payload: payload,
		}

		if err := c.server.networkOps.WriteJSON(c.conn, response); err != nil {
			c.server.logger.Error("发送初始化响应失败", interfaces.Fields{
				"error":  err,
				"client": c.ID,
			})
			return err
		}

		return nil

	case "sync_request":
		// 处理同步请求
		c.server.logger.Info("收到同步请求", interfaces.Fields{
			"client": c.ID,
		})

		// 解析同步请求数据
		var syncRequestData struct {
			Path      string   `json:"path"`
			Mode      string   `json:"mode"`
			Direction string   `json:"direction"`
			Files     []string `json:"files,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &syncRequestData); err != nil {
			c.server.logger.Error("解析同步请求失败", interfaces.Fields{
				"error":  err,
				"client": c.ID,
			})

			// 准备错误响应
			syncResponse := struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
			}{
				Success: false,
				Error:   fmt.Sprintf("解析同步请求失败: %v", err),
			}

			// 序列化响应
			payload, err := json.Marshal(syncResponse)
			if err != nil {
				return err
			}

			// 发送错误响应
			response := &interfaces.Message{
				Type:    "sync_response",
				UUID:    c.UUID,
				Payload: payload,
			}

			return c.server.networkOps.WriteJSON(c.conn, response)
		}

		// 为目标路径创建存储实例
		targetStorage, err := storage.NewFileStorage(syncRequestData.Path, c.server.logger)
		if err != nil {
			c.server.logger.Error("创建存储实例失败", interfaces.Fields{
				"error":  err,
				"client": c.ID,
				"path":   syncRequestData.Path,
			})

			// 准备错误响应
			syncResponse := struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
			}{
				Success: false,
				Error:   fmt.Sprintf("创建存储实例失败: %v", err),
			}

			// 序列化响应
			payload, err := json.Marshal(syncResponse)
			if err != nil {
				return err
			}

			// 发送错误响应
			response := &interfaces.Message{
				Type:    "sync_response",
				UUID:    c.UUID,
				Payload: payload,
			}

			return c.server.networkOps.WriteJSON(c.conn, response)
		}

		// 创建完整的同步请求
		fullSyncRequest := &interfaces.SyncRequest{
			Path:      syncRequestData.Path,
			Mode:      interfaces.SyncMode(syncRequestData.Mode),
			Direction: interfaces.SyncDirection(syncRequestData.Direction),
			Files:     syncRequestData.Files,
			Storage:   targetStorage,
		}

		// 处理同步请求
		if err := c.server.syncService.HandleSyncRequest(fullSyncRequest); err != nil {
			c.server.logger.Error("处理同步请求失败", interfaces.Fields{
				"error":  err,
				"client": c.ID,
				"path":   syncRequestData.Path,
			})

			// 准备错误响应
			syncResponse := struct {
				Success bool   `json:"success"`
				Error   string `json:"error"`
			}{
				Success: false,
				Error:   err.Error(),
			}

			// 序列化响应
			payload, err := json.Marshal(syncResponse)
			if err != nil {
				return err
			}

			// 发送错误响应
			response := &interfaces.Message{
				Type:    "sync_response",
				UUID:    c.UUID,
				Payload: payload,
			}

			return c.server.networkOps.WriteJSON(c.conn, response)
		}

		// 准备成功响应
		syncResponse := struct {
			Success bool `json:"success"`
		}{
			Success: true,
		}

		// 序列化响应
		payload, err := json.Marshal(syncResponse)
		if err != nil {
			return err
		}

		// 发送成功响应
		response := &interfaces.Message{
			Type:    "sync_response",
			UUID:    c.UUID,
			Payload: payload,
		}

		if err := c.server.networkOps.WriteJSON(c.conn, response); err != nil {
			c.server.logger.Error("发送同步响应失败", interfaces.Fields{
				"error":  err,
				"client": c.ID,
			})
			return err
		}

		c.server.logger.Info("同步请求处理完成", interfaces.Fields{
			"client": c.ID,
			"path":   syncRequestData.Path,
		})

		return nil

	case "heartbeat":
		// 处理心跳消息
		c.lastActivity = time.Now()

		// 发送心跳响应
		response := &interfaces.Message{
			Type: "heartbeat_response",
			UUID: c.UUID,
		}

		if err := c.server.networkOps.WriteJSON(c.conn, response); err != nil {
			c.server.logger.Error("发送心跳响应失败", interfaces.Fields{
				"error":  err,
				"client": c.ID,
			})
			return err
		}

		c.server.logger.Debug("心跳响应已发送", interfaces.Fields{
			"client": c.ID,
		})
		return nil

	default:
		return fmt.Errorf("未知的消息类型: %s", msg.Type)
	}
}

// Close 关闭客户端连接
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
