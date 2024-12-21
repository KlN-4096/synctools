package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"synctools/internal/model"
	"synctools/pkg/common"
)

// Server 网络服务器
type Server struct {
	config     *model.Config
	listener   net.Listener
	clients    map[string]*Client
	clientsMux sync.RWMutex
	logger     model.Logger
	running    bool
}

// NewServer 创建新的网络服务器
func NewServer(config *model.Config, logger model.Logger) *Server {
	return &Server{
		config:  config,
		clients: make(map[string]*Client),
		logger:  logger,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	if s.running {
		return fmt.Errorf("服务器已在运行中")
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("启动服务器失败: %v", err)
	}

	s.listener = listener
	s.running = true
	s.logger.Info("服务器已启动", "addr", addr)

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

	s.logger.Info("服务器已停止")
	return nil
}

// acceptLoop 接受新的客户端连接
func (s *Server) acceptLoop() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				s.logger.Info("临时接受连接错误", "error", err)
				continue
			}
			if s.running {
				s.logger.Error("接受连接失败", "error", err)
			}
			return
		}

		client := NewClient(conn, s)
		s.addClient(client)
		go client.Start()
	}
}

// addClient 添加客户端
func (s *Server) addClient(client *Client) {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()
	s.clients[client.ID] = client
	s.logger.Info("客户端已连接", "id", client.ID, "addr", client.conn.RemoteAddr())
}

// removeClient 移除客户端
func (s *Server) removeClient(client *Client) {
	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()
	delete(s.clients, client.ID)
	s.logger.Info("客户端已断开", "id", client.ID)
}

// ClientMessage 客户端消息
type ClientMessage struct {
	Type    string          `json:"type"`    // 消息类型
	UUID    string          `json:"uuid"`    // 客户端UUID
	Payload json.RawMessage `json:"payload"` // 消息内容
}

// SyncRequest 同步请求
type SyncRequest struct {
	FolderPath string `json:"folder_path"` // 文件夹路径
	SyncMode   string `json:"sync_mode"`   // 同步模式
	PackMD5    string `json:"pack_md5"`    // pack模式下的MD5
}

// SyncResponse 同步响应
type SyncResponse struct {
	Success  bool   `json:"success"`   // 是否成功
	Message  string `json:"message"`   // 消息
	PackMD5  string `json:"pack_md5"`  // 新的MD5值
	NeedPack bool   `json:"need_pack"` // 是否需要传输压缩包
}

// PackTransferRequest 压缩包传输请求
type PackTransferRequest struct {
	FolderPath string `json:"folder_path"` // 文件夹路径
	PackMD5    string `json:"pack_md5"`    // 压缩包MD5
	Offset     int64  `json:"offset"`      // 传输偏移量
	ChunkSize  int    `json:"chunk_size"`  // 分块大小
}

// PackTransferResponse 压缩包传输响应
type PackTransferResponse struct {
	Success   bool   `json:"success"`   // 是否成功
	Message   string `json:"message"`   // 消息
	Data      []byte `json:"data"`      // 数据块
	Offset    int64  `json:"offset"`    // 当前偏移量
	Size      int64  `json:"size"`      // 总大小
	Completed bool   `json:"completed"` // 是否传输完成
}

// Client 客户端连接
type Client struct {
	ID           string
	UUID         string
	conn         net.Conn
	server       *Server
	state        *model.ClientState
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
		var msg ClientMessage
		if err := decoder.Decode(&msg); err != nil {
			if err != io.EOF {
				c.server.logger.Error("读取客户端消息失败", "error", err, "client", c.ID)
			}
			return
		}

		c.lastActivity = time.Now()
		c.handleMessage(&msg)
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(msg *ClientMessage) {
	switch msg.Type {
	case "register":
		c.handleRegister(msg)
	case "sync_request":
		c.handleSyncRequest(msg)
	case "pack_transfer_request":
		c.handlePackTransfer(msg)
	default:
		c.server.logger.Error("未知的消息类型", "type", msg.Type, "client", c.ID)
	}
}

// handleRegister 处理客户端注册
func (c *Client) handleRegister(msg *ClientMessage) {
	c.UUID = msg.UUID
	c.state = &model.ClientState{
		UUID:         msg.UUID,
		LastSyncTime: time.Now().Unix(),
		FolderStates: make(map[string]model.PackState),
		IsOnline:     true,
		Version:      c.server.config.Version,
	}

	response := map[string]interface{}{
		"success": true,
		"message": "注册成功",
	}
	c.sendResponse("register_response", response)
}

// handleSyncRequest 处理同步请求
func (c *Client) handleSyncRequest(msg *ClientMessage) {
	var req SyncRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.server.logger.Error("解析同步请求失败", "error", err, "client", c.ID)
		return
	}

	// 处理不同的同步模式
	switch req.SyncMode {
	case model.SyncModePack:
		c.handlePackSync(req)
	default:
		// 其他模式按原有逻辑处理
		c.handleLegacySync(req)
	}
}

// handlePackSync 处理pack模式的同步
func (c *Client) handlePackSync(req SyncRequest) {
	folderState, exists := c.state.FolderStates[req.FolderPath]

	// 检查是否需要更新
	if exists && folderState.MD5 == req.PackMD5 {
		c.sendSyncResponse(true, "无需更新", req.PackMD5, false)
		return
	}

	// 计算文件夹哈希作为压缩包名称
	folderHash, err := common.CalculateFileHash(req.FolderPath)
	if err != nil {
		c.server.logger.Error("计算文件夹哈希失败", "error", err, "client", c.ID)
		c.sendSyncResponse(false, fmt.Sprintf("计算文件夹哈希失败: %v", err), "", false)
		return
	}

	// 创建压缩包
	packPath := common.JoinPath(c.server.config.SyncDir, "packs", fmt.Sprintf("%s.zip", folderHash))
	if !common.IsFileExists(packPath) {
		err := common.CreateZipPackage(req.FolderPath, packPath, &common.ZipOptions{
			TempDir: common.JoinPath(c.server.config.SyncDir, "temp"),
		})
		if err != nil {
			c.server.logger.Error("创建压缩包失败", "error", err, "client", c.ID)
			c.sendSyncResponse(false, fmt.Sprintf("创建压缩包失败: %v", err), "", false)
			return
		}
	}

	// 计算新的MD5
	md5, err := common.CalculateFileMD5(packPath)
	if err != nil {
		c.server.logger.Error("计算MD5失败", "error", err, "client", c.ID)
		c.sendSyncResponse(false, fmt.Sprintf("计算MD5失败: %v", err), "", false)
		return
	}

	// 更新状态
	c.state.FolderStates[req.FolderPath] = model.PackState{
		PackPath:   packPath,
		MD5:        md5,
		CreateTime: time.Now().Unix(),
		Size:       0, // TODO: 获取文件大小
	}

	// 发送响应
	c.sendSyncResponse(true, "需要更新", md5, true)
}

// handleLegacySync 处理原有的同步模式
func (c *Client) handleLegacySync(req SyncRequest) {
	// 保持原有的同步逻辑
	c.server.logger.Log("使用原有模式同步", "mode", req.SyncMode, "client", c.ID)
}

// handlePackTransfer 处理压缩包传输
func (c *Client) handlePackTransfer(msg *ClientMessage) {
	var req PackTransferRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.server.logger.Error("解析传输请求失败", "error", err, "client", c.ID)
		return
	}

	// 获取文件夹状态
	folderState, exists := c.state.FolderStates[req.FolderPath]
	if !exists {
		c.sendPackTransferResponse(false, "未找到文件夹状态", nil, 0, 0, true)
		return
	}

	// 验证MD5
	if folderState.MD5 != req.PackMD5 {
		c.sendPackTransferResponse(false, "MD5不匹配", nil, 0, 0, true)
		return
	}

	// 打开压缩包文件
	file, err := os.Open(folderState.PackPath)
	if err != nil {
		c.server.logger.Error("打开压缩包失败", "error", err, "client", c.ID)
		c.sendPackTransferResponse(false, fmt.Sprintf("打开压缩包失败: %v", err), nil, 0, 0, true)
		return
	}
	defer file.Close()

	// 获取文件大小
	fileInfo, err := file.Stat()
	if err != nil {
		c.server.logger.Error("获取文件信息失败", "error", err, "client", c.ID)
		c.sendPackTransferResponse(false, fmt.Sprintf("获取文件信息失败: %v", err), nil, 0, 0, true)
		return
	}

	// 设置读取位置
	if _, err := file.Seek(req.Offset, 0); err != nil {
		c.server.logger.Error("设置文件偏移量失败", "error", err, "client", c.ID)
		c.sendPackTransferResponse(false, fmt.Sprintf("设置文件偏移量失败: %v", err), nil, 0, 0, true)
		return
	}

	// 读取数据块
	chunkSize := req.ChunkSize
	if chunkSize <= 0 || chunkSize > 1024*1024 { // 限制最大块大小为1MB
		chunkSize = 1024 * 1024
	}

	data := make([]byte, chunkSize)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		c.server.logger.Error("读取文件失败", "error", err, "client", c.ID)
		c.sendPackTransferResponse(false, fmt.Sprintf("读取文件失败: %v", err), nil, 0, 0, true)
		return
	}

	// 发送响应
	completed := req.Offset+int64(n) >= fileInfo.Size()
	c.sendPackTransferResponse(true, "", data[:n], req.Offset+int64(n), fileInfo.Size(), completed)

	// 更新状态
	if completed {
		c.server.logger.Log("压缩包传输完成",
			"client", c.ID,
			"folder", req.FolderPath,
			"size", fileInfo.Size())
	}
}

// sendResponse 发送响应
func (c *Client) sendResponse(msgType string, data interface{}) {
	response := struct {
		Type    string      `json:"type"`
		Payload interface{} `json:"payload"`
	}{
		Type:    msgType,
		Payload: data,
	}

	if err := json.NewEncoder(c.conn).Encode(response); err != nil {
		c.server.logger.Error("发送响应失败", "error", err, "client", c.ID)
	}
}

// sendSyncResponse 发送同步响应
func (c *Client) sendSyncResponse(success bool, message string, md5 string, needPack bool) {
	c.sendResponse("sync_response", SyncResponse{
		Success:  success,
		Message:  message,
		PackMD5:  md5,
		NeedPack: needPack,
	})
}

// sendPackTransferResponse 发送压缩包传输响应
func (c *Client) sendPackTransferResponse(success bool, message string, data []byte, offset int64, size int64, completed bool) {
	c.sendResponse("pack_transfer_response", PackTransferResponse{
		Success:   success,
		Message:   message,
		Data:      data,
		Offset:    offset,
		Size:      size,
		Completed: completed,
	})
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.conn.Close()
}

// IsRunning 检查服务器是否正在运行
func (s *Server) IsRunning() bool {
	return s.running
}
