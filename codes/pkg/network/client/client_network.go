package client

import (
	"encoding/json"
	"fmt"
	"net"
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
	syncService interfaces.ClientSyncService
	msgSender   *message.MessageSender
	lastActive  time.Time // 添加最后活动时间
	isSyncing   bool      // 添加同步状态标志
}

// NewNetworkClient 创建新的网络客户端
func NewNetworkClient(logger interfaces.Logger, syncService interfaces.ClientSyncService) *NetworkClient {
	return &NetworkClient{
		logger:      logger,
		connected:   false,
		syncService: syncService,
		msgSender:   message.NewMessageSender(logger),
		lastActive:  time.Now(),
		isSyncing:   false,
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

	// 建立连接，保留5秒的初始连接超时
	serverAddr := fmt.Sprintf("%s:%s", addr, port)
	conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
	if err != nil {
		c.logger.Error("连接服务器失败", interfaces.Fields{"error": err})
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	c.conn = conn
	c.connected = true
	c.lastActive = time.Now()

	// 启动无操作检测
	go c.monitorInactivity()
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
func (c *NetworkClient) SendData(msgType string, data interface{}) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}
	c.UpdateActivity()
	config := c.syncService.GetCurrentConfig()
	return c.msgSender.SendMessage(c.conn, msgType, config.UUID, data)
}

// ReceiveData 接收数据
func (c *NetworkClient) ReceiveData(v interface{}) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}
	c.UpdateActivity()
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
	c.UpdateActivity()
	config := c.syncService.GetCurrentConfig()
	return c.msgSender.SendFile(c.conn, config.UUID, path, progress)
}

// ReceiveFile 接收文件
func (c *NetworkClient) ReceiveFile(destDir string, progress chan<- interfaces.Progress) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}
	c.UpdateActivity()
	return c.msgSender.ReceiveFile(c.conn, destDir, progress)
}

// SetConnectionLostCallback 设置连接丢失回调
func (c *NetworkClient) SetConnectionLostCallback(callback func()) {
	c.onConnLost = callback
}

// UpdateActivity 更新最后活动时间
func (c *NetworkClient) UpdateActivity() {
	c.lastActive = time.Now()
}

// SetSyncing 设置同步状态
func (c *NetworkClient) SetSyncing(syncing bool) {
	c.isSyncing = syncing
	if syncing {
		c.UpdateActivity()
	}
}

// monitorInactivity 监控无操作状态
func (c *NetworkClient) monitorInactivity() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	for range ticker.C {
		if !c.IsConnected() {
			return
		}

		// 如果正在同步，跳过检查
		if c.isSyncing {
			continue
		}

		// 如果超过3分钟没有活动，自动断开连接
		if time.Since(c.lastActive) > 3*time.Minute {
			c.logger.Info("检测到3分钟无操作，自动断开连接", interfaces.Fields{})
			if c.onConnLost != nil {
				c.onConnLost()
			}
			c.Disconnect()
			return
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

// SendInitMessage 发送初始化消息并接收响应
func (c *NetworkClient) SendInitMessage(initData interface{}) (*interfaces.Config, map[string]map[string]string, error) {
	// 发送初始化消息
	if err := c.SendData("init", initData); err != nil {
		return nil, nil, fmt.Errorf("发送初始化消息失败: %v", err)
	}

	// 接收服务器响应
	var response struct {
		Success bool                         `json:"success"`
		Message string                       `json:"message"`
		Config  *interfaces.Config           `json:"config"`
		MD5Map  map[string]map[string]string `json:"md5_map"`
	}

	if err := c.ReceiveData(&response); err != nil {
		return nil, nil, fmt.Errorf("接收初始化响应失败: %v", err)
	}

	if !response.Success {
		return nil, nil, fmt.Errorf("服务器拒绝连接: %s", response.Message)
	}

	return response.Config, response.MD5Map, nil
}
