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
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !c.IsConnected() {
			return
		}

		// 发送心跳
		if err := c.msgSender.SendMessage(c.conn, "heartbeat", "", nil); err != nil {
			c.logger.Error("发送心跳失败", interfaces.Fields{"error": err})
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
