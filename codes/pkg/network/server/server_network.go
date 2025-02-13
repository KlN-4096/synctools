package network

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"

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
	ID        string
	UUID      string
	conn      net.Conn
	server    *Server
	msgSender *message.MessageSender
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
	// 创建客户端实例
	client := &Client{
		ID:        fmt.Sprintf("client-%s", conn.RemoteAddr()),
		conn:      conn,
		server:    s,
		msgSender: message.NewMessageSender(s.logger),
	}

	// 添加到客户端列表
	s.clientsMux.Lock()
	s.clients[client.ID] = client
	s.clientsMux.Unlock()

	s.logger.Info("客户端已连接", interfaces.Fields{
		"id":   client.ID,
		"addr": conn.RemoteAddr(),
	})

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

		// 记录接收到的消息，使用格式化的payload
		s.logger.Debug("接收到客户端消息", interfaces.Fields{
			"client":  client.ID,
			"type":    msg.Type,
			"uuid":    msg.UUID,
			"payload": client.msgSender.FormatPayload(msg.Payload),
		})

		switch msg.Type {
		case "init":
			var initRequest struct {
				UUID   string                       `json:"uuid"`
				MD5Map map[string]map[string]string `json:"md5_map"`
			}
			if err := json.Unmarshal(msg.Payload, &initRequest); err != nil {
				s.logger.Error("解析初始化请求失败", interfaces.Fields{
					"error": err,
					"uuid":  msg.UUID,
				})
				return
			}

			client.UUID = initRequest.UUID

			// 获取所有同步文件夹的MD5列表
			serverMD5Map := make(map[string]map[string]string)
			for _, folder := range s.config.SyncFolders {
				files, err := s.syncService.GetLocalFilesWithMD5(folder.Path)
				if err != nil {
					s.logger.Error("获取服务端文件MD5失败", interfaces.Fields{
						"folder": folder.Path,
						"error":  err,
					})
					continue
				}
				serverMD5Map[folder.Path] = files
			}

			response := struct {
				Success bool                         `json:"success"`
				Message string                       `json:"message"`
				Config  *interfaces.Config           `json:"config"`
				MD5Map  map[string]map[string]string `json:"md5_map"`
			}{
				Success: true,
				Message: "初始化成功",
				Config:  s.config,
				MD5Map:  serverMD5Map,
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

		case "data":
			var syncRequest interfaces.SyncRequest
			if err := json.Unmarshal(msg.Payload, &syncRequest); err != nil {
				s.logger.Error("解析同步请求失败", interfaces.Fields{
					"error": err,
					"uuid":  msg.UUID,
				})
				continue
			}

			// 处理其他同步请求
			if err := s.syncService.HandleSyncRequest(&syncRequest); err != nil {
				s.logger.Error("处理同步请求失败", interfaces.Fields{
					"error": err,
					"uuid":  msg.UUID,
				})
				client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
					"success": false,
					"message": err.Error(),
				})
				continue
			}

			client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
				"success": true,
				"message": "同步成功",
			})

		case "file_request":
			go func() {
				var syncRequest interfaces.SyncRequest
				if err := json.Unmarshal(msg.Payload, &syncRequest); err != nil {
					s.logger.Error("解析同步请求失败", interfaces.Fields{
						"error": err,
						"uuid":  msg.UUID,
					})
					client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
						"success": false,
						"message": fmt.Sprintf("解析同步请求失败: %v", err),
					})
					return
				}

				// 处理文件下载请求
				filePath := filepath.Join(s.config.SyncDir, syncRequest.Path)
				s.logger.Debug("处理文件下载请求", interfaces.Fields{
					"file":         filePath,
					"request_path": syncRequest.Path,
					"server_path":  filePath,
				})

				// 检查文件是否存在
				fileInfo, err := os.Stat(filePath)
				if err != nil {
					s.logger.Error("文件不存在", interfaces.Fields{
						"file":  filePath,
						"error": err,
					})
					client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
						"success": false,
						"message": fmt.Sprintf("文件不存在: %v", err),
					})
					return
				}

				// 读取文件内容并计算MD5
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					s.logger.Error("读取文件失败", interfaces.Fields{
						"file":  filePath,
						"error": err,
					})
					client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
						"success": false,
						"message": fmt.Sprintf("读取文件失败: %v", err),
					})
					return
				}

				hash := md5.New()
				hash.Write(fileContent)
				md5sum := hex.EncodeToString(hash.Sum(nil))

				// 统一使用斜杠作为路径分隔符
				normalizedPath := filepath.ToSlash(syncRequest.Path)

				// 发送文件信息
				client.msgSender.SendMessage(conn, "file", msg.UUID, map[string]interface{}{
					"name": filepath.Base(normalizedPath),
					"size": fileInfo.Size(),
					"md5":  md5sum,
					"path": normalizedPath,
				})

				// 发送文件内容
				chunk := struct {
					Data []byte `json:"data"`
				}{
					Data: fileContent,
				}
				if err := client.msgSender.SendMessage(conn, "file_data", msg.UUID, chunk); err != nil {
					s.logger.Error("发送文件数据失败", interfaces.Fields{
						"file":  filePath,
						"error": err,
					})
					client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
						"success": false,
						"message": fmt.Sprintf("发送文件数据失败: %v", err),
					})
					return
				}

				s.logger.Debug("文件发送成功", interfaces.Fields{
					"file": filePath,
					"md5":  md5sum,
					"size": client.msgSender.FormatFileSize(fileInfo.Size()),
				})
			}()

		case "list_request":
			go func() {
				var syncRequest interfaces.SyncRequest
				if err := json.Unmarshal(msg.Payload, &syncRequest); err != nil {
					s.logger.Error("解析同步请求失败", interfaces.Fields{
						"error": err,
						"uuid":  msg.UUID,
					})
					return
				}

				// 获取同步目录
				syncDir := filepath.Join(s.config.SyncDir, syncRequest.Path)
				var files []string
				var dirs []string

				err := filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						if os.IsNotExist(err) {
							return nil
						}
						return err
					}

					// 获取相对路径
					relPath, err := filepath.Rel(syncDir, path)
					if err != nil {
						return err
					}

					// 如果是根目录，跳过
					if relPath == "." {
						return nil
					}

					if info.IsDir() {
						dirs = append(dirs, relPath)
						s.logger.Debug("发现目录", interfaces.Fields{
							"folder": syncRequest.Path,
							"dir":    relPath,
						})
					} else {
						files = append(files, relPath)
						s.logger.Debug("发现文件", interfaces.Fields{
							"folder": syncRequest.Path,
							"file":   relPath,
						})
					}
					return nil
				})

				if err != nil {
					s.logger.Error("获取文件列表失败", interfaces.Fields{
						"error": err,
					})
					client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
						"success": false,
						"message": fmt.Sprintf("获取文件列表失败: %v", err),
					})
					return
				}

				s.logger.Info("返回文件列表", interfaces.Fields{
					"folder":     syncRequest.Path,
					"file_count": len(files),
					"dir_count":  len(dirs),
					"files":      files,
					"dirs":       dirs,
				})

				client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
					"success": true,
					"files":   files,
					"dirs":    dirs,
				})
			}()

		case "delete_request":
			go func() {
				var syncRequest interfaces.SyncRequest
				if err := json.Unmarshal(msg.Payload, &syncRequest); err != nil {
					s.logger.Error("解析同步请求失败", interfaces.Fields{
						"error": err,
						"uuid":  msg.UUID,
					})
					return
				}

				// 处理文件删除请求
				filePath := filepath.Join(s.config.SyncDir, syncRequest.Path)
				s.logger.Debug("处理文件删除请求", interfaces.Fields{
					"file": filePath,
				})

				if err := os.Remove(filePath); err != nil {
					if !os.IsNotExist(err) {
						s.logger.Error("删除文件失败", interfaces.Fields{
							"file":  filePath,
							"error": err,
						})
						client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
							"success": false,
							"message": fmt.Sprintf("删除文件失败: %v", err),
						})
						return
					}
				}

				s.logger.Info("文件删除成功", interfaces.Fields{
					"file": filePath,
				})

				client.msgSender.SendMessage(conn, "data", msg.UUID, map[string]interface{}{
					"success": true,
					"message": "文件删除成功",
				})
			}()

		default:
			s.logger.Error("未知的消息类型", interfaces.Fields{
				"type": msg.Type,
			})
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
