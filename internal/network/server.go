package network

import (
	"fmt"
	"io"
	"net"
	"sync"

	"synctools/internal/model"
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

// Client 客户端连接
type Client struct {
	ID     string
	conn   net.Conn
	server *Server
}

// NewClient 创建新的客户端
func NewClient(conn net.Conn, server *Server) *Client {
	return &Client{
		ID:     fmt.Sprintf("client-%s", conn.RemoteAddr()),
		conn:   conn,
		server: server,
	}
}

// Start 开始处理客户端连接
func (c *Client) Start() {
	defer func() {
		c.Close()
		c.server.removeClient(c)
	}()

	buffer := make([]byte, 4096)
	for {
		n, err := c.conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				c.server.logger.Error("读取客户端数据失败", "error", err, "client", c.ID)
			}
			return
		}

		c.server.logger.DebugLog("收到客户端数据", "bytes", n, "client", c.ID)
	}
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.conn.Close()
}

// IsRunning 检查服务器是否正在运行
func (s *Server) IsRunning() bool {
	return s.running
}
