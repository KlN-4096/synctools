package client

import (
	"fmt"
	"net"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/network"
)

// NetworkClient 网络客户端结构体
type NetworkClient struct {
	logger     interfaces.Logger
	conn       net.Conn
	networkOps interfaces.NetworkOperations
	serverAddr string
	serverPort string
	connected  bool
	onConnLost func()
}

// NewNetworkClient 创建新的网络客户端
func NewNetworkClient(logger interfaces.Logger) *NetworkClient {
	return &NetworkClient{
		logger:     logger,
		networkOps: network.NewOperations(logger),
		connected:  false,
	}
}

// Connect 连接到服务器
func (c *NetworkClient) Connect(addr, port string) error {
	c.logger.Debug("开始连接服务器", interfaces.Fields{
		"serverAddr": addr,
		"serverPort": port,
	})

	if c.IsConnected() {
		return fmt.Errorf("已经连接到服务器")
	}

	// 验证地址和端口
	if addr == "" || port == "" {
		return fmt.Errorf("服务器地址或端口不能为空")
	}

	c.serverAddr = addr
	c.serverPort = port

	// 建立连接
	serverAddr := fmt.Sprintf("%s:%s", addr, port)
	conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
	if err != nil {
		c.logger.Error("连接服务器失败", interfaces.Fields{
			"error": err,
		})
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	c.conn = conn
	c.connected = true

	// 启动连接监控
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
			c.logger.Error("关闭连接失败", interfaces.Fields{
				"error": err,
			})
			return fmt.Errorf("关闭连接失败: %v", err)
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

	return c.networkOps.WriteJSON(c.conn, data)
}

// ReceiveData 接收数据
func (c *NetworkClient) ReceiveData(v interface{}) error {
	if !c.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}

	return c.networkOps.ReadJSON(c.conn, v)
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

			c.logger.Error("连接监控检测到错误", interfaces.Fields{
				"error": err,
			})

			c.handleConnectionLost()
			break
		}
	}
}

// handleConnectionLost 处理连接丢失
func (c *NetworkClient) handleConnectionLost() {
	c.logger.Debug("处理连接丢失", interfaces.Fields{})

	_ = c.Disconnect()

	if c.onConnLost != nil {
		c.onConnLost()
	}
}

// isTimeout 判断是否为超时错误
func isTimeout(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}
