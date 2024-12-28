package client

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/network/message"
)

// NetworkClient 网络客户端结构体
type NetworkClient struct {
	logger      interfaces.Logger
	conn        net.Conn
	serverAddr  string
	serverPort  string
	connected   bool
	onConnLost  func()
	syncService interfaces.SyncService
	msgSender   *message.MessageSender
}

// NewNetworkClient 创建新的网络客户端
func NewNetworkClient(logger interfaces.Logger, syncService interfaces.SyncService) *NetworkClient {
	return &NetworkClient{
		logger:      logger,
		connected:   false,
		syncService: syncService,
		msgSender:   message.NewMessageSender(logger),
	}
}

// Connect 连接到服务器
func (c *NetworkClient) Connect(addr, port string) error {
	c.logger.Debug("开始连接服务器", interfaces.Fields{
		"serverAddr": addr,
		"serverPort": port,
	})

	if addr == "" || port == "" {
		return fmt.Errorf("服务器地址或端口不能为空")
	}

	c.serverAddr = addr
	c.serverPort = port

	// 建立连接
	serverAddr := fmt.Sprintf("%s:%s", addr, port)
	conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
	if err != nil {
		c.logger.Error("连接服务器失败", interfaces.Fields{"error": err})
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	c.conn = conn
	c.connected = true

	// 发送初始化消息
	config := c.syncService.GetCurrentConfig()
	if err := c.msgSender.SendMessage(conn, "init", config.UUID, nil); err != nil {
		c.Disconnect()
		return fmt.Errorf("发送初始化消息失败: %v", err)
	}

	// 等待初始化响应
	msg, err := c.msgSender.ReceiveMessage(conn)
	if err != nil {
		c.Disconnect()
		return fmt.Errorf("接收初始化响应失败: %v", err)
	}

	if msg.Type != "init_response" {
		c.Disconnect()
		return fmt.Errorf("收到意外的响应类型: %s", msg.Type)
	}

	var response struct {
		Success bool               `json:"success"`
		Message string             `json:"message"`
		Config  *interfaces.Config `json:"config"`
	}

	if err := json.Unmarshal(msg.Payload, &response); err != nil {
		c.Disconnect()
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if !response.Success {
		c.Disconnect()
		return fmt.Errorf("服务器拒绝连接: %s", response.Message)
	}

	// 更新本地配置
	if err := c.syncService.SaveConfig(response.Config); err != nil {
		c.logger.Error("保存服务器配置失败", interfaces.Fields{
			"error": err,
		})
	}

	c.logger.Debug("连接初始化成功", interfaces.Fields{
		"config": response.Config,
	})

	go c.monitorConnection()
	return nil
}

// Disconnect 断开连接
func (c *NetworkClient) Disconnect() error {
	if !c.IsConnected() {
		return nil
	}

	c.logger.Debug("断开服务器连接", interfaces.Fields{})

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Error("关闭连接失败", interfaces.Fields{"error": err})
		}
		c.conn = nil
	}

	c.connected = false
	return nil
}

// IsConnected 检查是否已连接
func (c *NetworkClient) IsConnected() bool {
	return c.connected && c.conn != nil
}

// SendData 发送数据
func (c *NetworkClient) SendData(data interface{}) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}
	config := c.syncService.GetCurrentConfig()
	return c.msgSender.SendMessage(c.conn, "data", config.UUID, data)
}

// ReceiveData 接收数据
func (c *NetworkClient) ReceiveData(v interface{}) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}
	msg, err := c.msgSender.ReceiveMessage(c.conn)
	if err != nil {
		return err
	}
	return json.Unmarshal(msg.Payload, v)
}

// SendFile 发送文件
func (c *NetworkClient) SendFile(path string, progress chan<- interfaces.Progress) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}
	config := c.syncService.GetCurrentConfig()
	return c.msgSender.SendFile(c.conn, config.UUID, path, progress)
}

// ReceiveFile 接收文件
func (c *NetworkClient) ReceiveFile(destDir string, progress chan<- interfaces.Progress) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}
	return c.msgSender.ReceiveFile(c.conn, destDir, progress)
}

// SetConnectionLostCallback 设置连接丢失回调
func (c *NetworkClient) SetConnectionLostCallback(callback func()) {
	c.onConnLost = callback
}

// monitorConnection 监控连接状态
func (c *NetworkClient) monitorConnection() {
	buffer := make([]byte, 1)
	for c.IsConnected() {
		c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, err := c.conn.Read(buffer)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			c.logger.Error("连接监控检测到错误", interfaces.Fields{"error": err})
			if c.onConnLost != nil {
				c.onConnLost()
			}
			c.Disconnect()
			break
		}
	}
}

// isTimeout 判断是否为超时错误
func isTimeout(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
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

// SyncFiles 同步文件到服务器
func (c *NetworkClient) SyncFiles(sourcePath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}

	c.logger.Info("开始同步文件", interfaces.Fields{
		"sourcePath": sourcePath,
	})

	// 1. 扫描本地文件
	files, err := c.scanLocalFiles(sourcePath)
	if err != nil {
		return fmt.Errorf("扫描本地文件失败: %v", err)
	}

	// 2. 获取服务器文件列表
	serverFiles, err := c.getServerFileList()
	if err != nil {
		return fmt.Errorf("获取服务器文件列表失败: %v", err)
	}

	// 3. 对比文件差异
	diffFiles := c.compareFiles(files, serverFiles)

	// 4. 同步差异文件
	for _, file := range diffFiles {
		if err := c.syncFile(file); err != nil {
			c.logger.Error("同步文件失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}
	}

	return nil
}

// scanLocalFiles 扫描本地文件
func (c *NetworkClient) scanLocalFiles(sourcePath string) ([]string, error) {
	var files []string
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(sourcePath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		return nil
	})
	return files, err
}

// getServerFileList 获取服务器文件列表
func (c *NetworkClient) getServerFileList() ([]string, error) {
	request := &SyncRequest{
		Operation: "get_file_list",
	}

	if err := c.SendData(request); err != nil {
		return nil, err
	}

	var response SyncResponse
	if err := c.ReceiveData(&response); err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, fmt.Errorf("获取服务器文件列表失败: %s", response.Message)
	}

	files, ok := response.Data.([]string)
	if !ok {
		return nil, fmt.Errorf("服务器返回的文件列表格式无效")
	}

	return files, nil
}

// compareFiles 对比文件差异
func (c *NetworkClient) compareFiles(localFiles, serverFiles []string) []string {
	var diffFiles []string
	localMap := make(map[string]bool)

	// 将本地文件列表转换为map
	for _, file := range localFiles {
		localMap[file] = true
	}

	// 检查服务器上没有的文件
	for _, file := range localFiles {
		found := false
		for _, serverFile := range serverFiles {
			if file == serverFile {
				found = true
				break
			}
		}
		if !found {
			diffFiles = append(diffFiles, file)
		}
	}

	return diffFiles
}

// syncFile 同步单个文件
func (c *NetworkClient) syncFile(file string) error {
	// 创建进度通道
	progress := make(chan interfaces.Progress, 1)
	defer close(progress)

	// 启动goroutine监控进度
	go func() {
		for p := range progress {
			c.logger.Debug("同步进度", interfaces.Fields{
				"file":      p.FileName,
				"progress":  fmt.Sprintf("%.2f%%", float64(p.Current)/float64(p.Total)*100),
				"speed":     fmt.Sprintf("%.2f MB/s", p.Speed/1024/1024),
				"remaining": fmt.Sprintf("%ds", p.Remaining),
			})
		}
	}()

	// 发送文件
	return c.SendFile(file, progress)
}
