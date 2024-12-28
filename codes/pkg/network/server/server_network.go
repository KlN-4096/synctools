package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/errors"
	"synctools/codes/pkg/network/message"
)

// Server 网络服务器实现
type Server struct {
	config      *interfaces.Config
	syncService interfaces.ServerSyncService
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

// SyncRequest 同步请求结构体
type SyncRequest struct {
	Operation string      `json:"operation"`
	Path      string      `json:"path"`
	Data      interface{} `json:"data"`
}

// SyncResponse 同步响应结构体
type SyncResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// NewServer 创建新的网络服务器
func NewServer(config *interfaces.Config, syncService interfaces.ServerSyncService, logger interfaces.Logger) *Server {
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

// HandleSyncRequest 处理同步请求
func (s *Server) HandleSyncRequest(client *Client, request *SyncRequest) error {
	s.logger.Info("处理同步请求", interfaces.Fields{
		"operation": request.Operation,
		"path":      request.Path,
		"clientID":  client.ID,
	})

	// 验证请求
	if err := s.validateSyncRequest(request); err != nil {
		return s.sendErrorResponse(client, err.Error())
	}

	switch request.Operation {
	case "get_file_list":
		return s.handleGetFileList(client, request)
	case "upload_file":
		return s.handleFileUpload(client, request)
	case "delete_file":
		return s.handleFileDelete(client, request)
	default:
		return s.sendErrorResponse(client, fmt.Sprintf("未知的操作类型: %s", request.Operation))
	}
}

// handleGetFileList 处理获取文件列表请求
func (s *Server) handleGetFileList(client *Client, request *SyncRequest) error {
	config := s.syncService.GetCurrentConfig()
	if config == nil {
		return s.sendErrorResponse(client, "服务器配置未初始化")
	}

	// 获取同步目录下的所有文件
	var files []string
	err := filepath.Walk(config.SyncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(config.SyncDir, path)
			if err != nil {
				return err
			}
			// 检查是否被忽略
			if !s.isFileIgnored(relPath) {
				files = append(files, relPath)
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("扫描文件失败", interfaces.Fields{
			"error": err,
			"path":  config.SyncDir,
		})
		return s.sendErrorResponse(client, fmt.Sprintf("扫描文件失败: %v", err))
	}

	// 发送文件列表响应
	response := &SyncResponse{
		Success: true,
		Data:    files,
	}

	return s.sendResponse(client, response)
}

// handleFileUpload 处理文件上传请求
func (s *Server) handleFileUpload(client *Client, request *SyncRequest) error {
	config := s.syncService.GetCurrentConfig()
	if config == nil {
		return s.sendErrorResponse(client, "服务器配置未初始化")
	}

	// 创建进度通道
	progress := make(chan interfaces.Progress, 1)
	defer close(progress)

	// 启动goroutine监控进度
	go func() {
		for p := range progress {
			s.logger.Debug("接收文件进度", interfaces.Fields{
				"file":      p.FileName,
				"progress":  fmt.Sprintf("%.2f%%", float64(p.Current)/float64(p.Total)*100),
				"speed":     fmt.Sprintf("%.2f MB/s", p.Speed/1024/1024),
				"remaining": fmt.Sprintf("%ds", p.Remaining),
				"clientID":  client.ID,
			})
		}
	}()

	// 接收文件
	if err := client.msgSender.ReceiveFile(client.conn, config.SyncDir, progress); err != nil {
		s.logger.Error("接收文件失败", interfaces.Fields{
			"error":    err,
			"path":     request.Path,
			"clientID": client.ID,
		})
		return s.sendErrorResponse(client, fmt.Sprintf("接收文件失败: %v", err))
	}

	return s.sendResponse(client, &SyncResponse{
		Success: true,
		Message: "文件上传成功",
	})
}

// handleFileDelete 处理文件删除请求
func (s *Server) handleFileDelete(client *Client, request *SyncRequest) error {
	config := s.syncService.GetCurrentConfig()
	if config == nil {
		return s.sendErrorResponse(client, "服务器配置未初始化")
	}

	filePath := filepath.Join(config.SyncDir, request.Path)
	if err := os.Remove(filePath); err != nil {
		s.logger.Error("删除文件失败", interfaces.Fields{
			"error":    err,
			"path":     filePath,
			"clientID": client.ID,
		})
		return s.sendErrorResponse(client, fmt.Sprintf("删除文件失败: %v", err))
	}

	return s.sendResponse(client, &SyncResponse{
		Success: true,
		Message: "文件删除成功",
	})
}

// sendResponse 发送响应
func (s *Server) sendResponse(client *Client, response *SyncResponse) error {
	return client.msgSender.SendMessage(client.conn, "sync_response", client.UUID, response)
}

// sendErrorResponse 发送错误响应
func (s *Server) sendErrorResponse(client *Client, message string) error {
	return s.sendResponse(client, &SyncResponse{
		Success: false,
		Message: message,
	})
}

// validateSyncRequest 验证同步请求
func (s *Server) validateSyncRequest(request *SyncRequest) error {
	if request == nil {
		return fmt.Errorf("请求不能为空")
	}

	config := s.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("服务器配置未初始化")
	}

	// 验证路径安全性
	if request.Path != "" {
		fullPath := filepath.Join(config.SyncDir, request.Path)
		if !strings.HasPrefix(fullPath, config.SyncDir) {
			return fmt.Errorf("非法的文件路径")
		}
	}

	return nil
}

// isFileIgnored 检查文件是否被忽略
func (s *Server) isFileIgnored(path string) bool {
	config := s.syncService.GetCurrentConfig()
	if config == nil {
		return false
	}

	// 检查忽略列表
	for _, pattern := range config.IgnoreList {
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}

	return false
}
