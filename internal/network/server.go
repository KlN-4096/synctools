package network

import (
	"fmt"
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
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("启动服务器失败: %v", err)
	}

	s.listener = listener
	s.logger.Info("服务器已启动", "addr", addr)

	go s.acceptLoop()
	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if s.listener != nil {
		s.listener.Close()
	}

	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()

	for _, client := range s.clients {
		client.Close()
	}
	s.clients = make(map[string]*Client)

	return nil
}

// acceptLoop 接受新的客户端连接
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return
		}

		client := NewClient(conn)
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
	ID   string
	conn net.Conn
}

// NewClient 创建新的客户端
func NewClient(conn net.Conn) *Client {
	return &Client{
		ID:   fmt.Sprintf("client-%s", conn.RemoteAddr()),
		conn: conn,
	}
}

// Start 开始处理客户端连接
func (c *Client) Start() {
	defer c.Close()

	buffer := make([]byte, 4096)
	for {
		_, err := c.conn.Read(buffer)
		if err != nil {
			return
		}
	}
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.conn.Close()
}
