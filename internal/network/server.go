/*
文件作用:
- 实现网络服务器功能
- 处理客户端连接
- 管理文件同步请求
- 提供文件传输服务

主要方法:
- NewServer: 创建服务器实例
- Start: 启动服务器
- Stop: 停止服务器
- HandleClient: 处理客户端连接
- SendFile: 发送文件到客户端
*/

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

	pkgcommon "synctools/pkg/common"
)

// Server 网络服务器
type Server struct {
	config     *pkgcommon.Config
	listener   net.Listener
	clients    map[string]*Client
	clientsMux sync.RWMutex
	logger     pkgcommon.Logger
	running    bool
}

// NewServer 创建新的网络服务器
func NewServer(config *pkgcommon.Config, logger pkgcommon.Logger) *Server {
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
	FolderPath string          `json:"folder_path"` // 文件夹路径
	SyncMode   string          `json:"sync_mode"`   // 同步模式
	PackMD5    string          `json:"pack_md5"`    // pack模式下的MD5
	Payload    json.RawMessage `json:"payload"`     // 额外的请求数据
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
	state        *pkgcommon.ClientState
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
	var err error

	handlers := map[string]MessageHandler{
		"register":              c.handleRegister,
		"sync_request":          c.handleSyncRequest,
		"pack_transfer_request": c.handlePackTransfer,
		"file_transfer_request": c.handleFileTransfer,
	}

	handler, exists := handlers[msg.Type]
	if !exists {
		c.server.logger.Error("未知的消息类型", "type", msg.Type, "client", c.ID)
		return
	}

	if err = handler(c, msg); err != nil {
		c.server.logger.Error("处理消息失败", "type", msg.Type, "error", err, "client", c.ID)
		c.sendErrorResponse(msg.Type+"_response", err)
	}
}

// handleFileTransfer 处理文件传输请求
func (c *Client) handleFileTransfer(client *Client, msg *ClientMessage) error {
	var req struct {
		FilePath  string `json:"file_path"`
		ChunkSize int    `json:"chunk_size"`
		Offset    int64  `json:"offset"`
	}

	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return fmt.Errorf("解析请求失败: %v", err)
	}

	op := &FileTransferOperation{
		Path:      filepath.Join(c.server.config.SyncDir, req.FilePath),
		ChunkSize: req.ChunkSize,
		Offset:    req.Offset,
		Client:    c,
	}

	return op.Execute()
}

// handleSyncRequest 处理同步请求
func (c *Client) handleSyncRequest(client *Client, msg *ClientMessage) error {
	var syncReq struct {
		FolderPath string            `json:"folder_path"`
		SyncMode   string            `json:"sync_mode"`
		FileHashes map[string]string `json:"file_hashes"`
	}

	if err := json.Unmarshal(msg.Payload, &syncReq); err != nil {
		return fmt.Errorf("解析同步请求失败: %v", err)
	}

	op := &FileSyncOperation{
		SrcPath:  filepath.Join(c.server.config.SyncDir, syncReq.FolderPath),
		DstPath:  syncReq.FolderPath,
		Mode:     syncReq.SyncMode,
		Client:   c,
		FileHash: syncReq.FileHashes,
	}

	return op.Execute()
}

// handleRegister 处理客户端注册
func (c *Client) handleRegister(client *Client, msg *ClientMessage) error {
	c.UUID = msg.UUID
	c.state = &pkgcommon.ClientState{
		UUID:         msg.UUID,
		LastSyncTime: time.Now().Unix(),
		FolderStates: make(map[string]pkgcommon.PackState),
		IsOnline:     true,
		Version:      c.server.config.Version,
	}

	c.sendResponse("register_response", Response{
		Success: true,
		Message: "注册成功",
	})
	return nil
}

// sendResponse 发送统一响应
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
func (c *Client) sendSyncResponse(success bool, message string, manifestHash string, complete bool) {
	response := struct {
		Type    string `json:"type"`
		Payload struct {
			Success      bool   `json:"success"`
			Message      string `json:"message"`
			ManifestHash string `json:"manifest_hash,omitempty"`
			Complete     bool   `json:"complete"`
		} `json:"payload"`
	}{
		Type: "sync_response",
		Payload: struct {
			Success      bool   `json:"success"`
			Message      string `json:"message"`
			ManifestHash string `json:"manifest_hash,omitempty"`
			Complete     bool   `json:"complete"`
		}{
			Success:      success,
			Message:      message,
			ManifestHash: manifestHash,
			Complete:     complete,
		},
	}

	if err := json.NewEncoder(c.conn).Encode(response); err != nil {
		c.server.logger.Error("发送同步响应失败", "error", err)
	}
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

// handlePackTransfer 处理压缩包传输请求
func (c *Client) handlePackTransfer(client *Client, msg *ClientMessage) error {
	var req PackTransferRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return fmt.Errorf("解析传输请求失败: %v", err)
	}

	// 获取文件夹状态
	folderState, exists := c.state.FolderStates[req.FolderPath]
	if !exists {
		c.sendPackTransferResponse(false, "未找到文件夹状态", nil, 0, 0, true)
		return nil
	}

	// 验证MD5
	if folderState.MD5 != req.PackMD5 {
		c.sendPackTransferResponse(false, "MD5不匹配", nil, 0, 0, true)
		return nil
	}

	op := &FileTransferOperation{
		Path:      folderState.PackPath,
		ChunkSize: req.ChunkSize,
		Offset:    req.Offset,
		Client:    c,
	}

	return op.Execute()
}

// sendErrorResponse 发送错误响应
func (c *Client) sendErrorResponse(msgType string, err error) {
	c.sendResponse(msgType, map[string]interface{}{
		"success": false,
		"message": err.Error(),
	})
}

// MessageHandler 消息处理器
type MessageHandler func(*Client, *ClientMessage) error

// ResponseType 响应类型
type ResponseType string

const (
	ResponseTypeRegister     ResponseType = "register_response"
	ResponseTypeSync         ResponseType = "sync_response"
	ResponseTypePackTransfer ResponseType = "pack_transfer_response"
	ResponseTypeFileTransfer ResponseType = "file_transfer_response"
)

// Response 统一响应结构
type Response struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Completed bool        `json:"completed,omitempty"`
}

// FileOperation 文件操作接口
type FileOperation interface {
	Execute() error
}

// FileTransferOperation 文件传输操作
type FileTransferOperation struct {
	Path      string
	ChunkSize int
	Offset    int64
	Client    *Client
}

// Execute 执行文件传输
func (op *FileTransferOperation) Execute() error {
	result, err := op.Client.transferFile(FileTransferOptions{
		FilePath:   op.Path,
		ChunkSize:  op.ChunkSize,
		Offset:     op.Offset,
		RetryCount: 3,
	})
	if err != nil {
		return err
	}

	op.Client.sendResponse("file_transfer_response", TransferResult{
		Success:   result.Success,
		Message:   result.Message,
		Data:      result.Data,
		Offset:    result.Offset,
		Size:      result.Size,
		Completed: result.Completed,
	})
	return nil
}

// FileSyncOperation 文件同步操作
type FileSyncOperation struct {
	SrcPath  string
	DstPath  string
	Mode     string
	Client   *Client
	FileHash map[string]string
}

// Execute 执行文件同步
func (op *FileSyncOperation) Execute() error {
	// 确保目标目录存在
	if err := os.MkdirAll(op.DstPath, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 获取源目录文件列表
	srcFiles, err := op.Client.getFileList(op.SrcPath)
	if err != nil {
		return err
	}

	// 转换文件哈希列表
	hashFiles := make(map[string]FileInfo)
	for path, hash := range op.FileHash {
		hashFiles[path] = FileInfo{
			Path: path,
			Hash: hash,
		}
	}

	// 根据同步模式处理
	switch op.Mode {
	case pkgcommon.SyncModeMirror:
		return op.executeMirrorSync(srcFiles, hashFiles)
	case pkgcommon.SyncModePush:
		return op.executePushSync(srcFiles, hashFiles)
	case pkgcommon.SyncModePack:
		return op.executePackSync(srcFiles, hashFiles)
	default:
		return fmt.Errorf("不支持的同步模式: %s", op.Mode)
	}
}

// executePackSync 执行打包同步
func (op *FileSyncOperation) executePackSync(srcFiles, hashFiles map[string]FileInfo) error {
	// 创建临时目录用于打包
	tempDir, err := os.MkdirTemp("", "sync_pack_*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建文件清单
	manifestPath := filepath.Join(tempDir, "manifest.json")
	manifestData := make(map[string]string)
	for path, info := range srcFiles {
		manifestData[path] = info.Hash
	}

	manifestBytes, err := json.MarshalIndent(manifestData, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化文件清单失败: %v", err)
	}

	if err := os.WriteFile(manifestPath, manifestBytes, 0644); err != nil {
		return fmt.Errorf("写入文件清单失败: %v", err)
	}

	// 计算文件清单的哈希值
	manifestHash, err := pkgcommon.CalculateFileHash(manifestPath)
	if err != nil {
		return fmt.Errorf("计算文件清单哈希值失败: %v", err)
	}

	// 发送响应
	response := struct {
		Success      bool              `json:"success"`
		Message      string            `json:"message"`
		ManifestHash string            `json:"manifest_hash"`
		FileHashes   map[string]string `json:"file_hashes"`
	}{
		Success:      true,
		Message:      "同步请求处理成功",
		ManifestHash: manifestHash,
		FileHashes:   manifestData,
	}

	op.Client.sendResponse("sync_response", response)
	return nil
}

// executeMirrorSync 执行镜像同步
func (op *FileSyncOperation) executeMirrorSync(srcFiles, hashFiles map[string]FileInfo) error {
	filesToAdd, filesToDel := op.Client.compareFiles(srcFiles, hashFiles)

	response := struct {
		Success    bool     `json:"success"`
		Message    string   `json:"message"`
		DiffCount  int      `json:"diff_count"`
		FilesToAdd []string `json:"files_to_add"`
		FilesToDel []string `json:"files_to_del"`
	}{
		Success:    true,
		Message:    "同步请求处理成功",
		DiffCount:  len(filesToAdd) + len(filesToDel),
		FilesToAdd: filesToAdd,
		FilesToDel: filesToDel,
	}

	op.Client.sendResponse("sync_response", response)
	return nil
}

// executePushSync 执行推送同步
func (op *FileSyncOperation) executePushSync(srcFiles, hashFiles map[string]FileInfo) error {
	filesToAdd, _ := op.Client.compareFiles(hashFiles, srcFiles)

	response := struct {
		Success    bool     `json:"success"`
		Message    string   `json:"message"`
		DiffCount  int      `json:"diff_count"`
		FilesToAdd []string `json:"files_to_add"`
	}{
		Success:    true,
		Message:    "同步请求处理��功",
		DiffCount:  len(filesToAdd),
		FilesToAdd: filesToAdd,
	}

	op.Client.sendResponse("sync_response", response)
	return nil
}

// FileTransferOptions 文件传输选项
type FileTransferOptions struct {
	FilePath   string // 文件路径
	ChunkSize  int    // 分块大小
	Offset     int64  // 起始偏移量
	RetryCount int    // 重试次数
}

// TransferResult 传输结果
type TransferResult struct {
	Success   bool   // 是否成功
	Message   string // 消息
	Data      []byte // 数据
	Offset    int64  // 当前偏移量
	Size      int64  // 总大小
	Completed bool   // 是否完成
}

// FileInfo 文件信息
type FileInfo struct {
	Path string // 相对路径
	Hash string // 文件哈希
	Size int64  // 文件大小
}

// transferFile 通用文件传输方法
func (c *Client) transferFile(opts FileTransferOptions) (*TransferResult, error) {
	if opts.ChunkSize <= 0 || opts.ChunkSize > 1024*1024 {
		opts.ChunkSize = 32 * 1024 // 默认32KB
	}

	file, err := os.Open(opts.FilePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}

	if _, err := file.Seek(opts.Offset, 0); err != nil {
		return nil, fmt.Errorf("设置文件偏移量失败: %v", err)
	}

	data := make([]byte, opts.ChunkSize)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	completed := opts.Offset+int64(n) >= fileInfo.Size()
	return &TransferResult{
		Success:   true,
		Data:      data[:n],
		Offset:    opts.Offset + int64(n),
		Size:      fileInfo.Size(),
		Completed: completed,
	}, nil
}

// getFileList 获取目录下的文件列表
func (c *Client) getFileList(dirPath string) (map[string]FileInfo, error) {
	files := make(map[string]FileInfo)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}

			hash, err := pkgcommon.CalculateFileHash(path)
			if err != nil {
				return err
			}

			files[relPath] = FileInfo{
				Path: relPath,
				Hash: hash,
				Size: info.Size(),
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("获取文件列表失败: %v", err)
	}

	return files, nil
}

// compareFiles 比较文件差异
func (c *Client) compareFiles(serverFiles, clientFiles map[string]FileInfo) (toAdd, toDel []string) {
	for path, serverFile := range serverFiles {
		clientFile, exists := clientFiles[path]
		if !exists || clientFile.Hash != serverFile.Hash {
			toAdd = append(toAdd, path)
		}
	}

	for path := range clientFiles {
		if _, exists := serverFiles[path]; !exists {
			toDel = append(toDel, path)
		}
	}

	return toAdd, toDel
}
