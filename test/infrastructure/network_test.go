package infrastructure

import (
	"fmt"
	"net"
	"testing"
	"time"

	"synctools/internal/model"
	"synctools/internal/network"
)

// MockLogger 模拟日志记录器
type MockLogger struct{}

func (m *MockLogger) Log(format string, v ...interface{})      {}
func (m *MockLogger) Info(msg string, args ...interface{})     {}
func (m *MockLogger) Error(msg string, args ...interface{})    {}
func (m *MockLogger) DebugLog(format string, v ...interface{}) {}
func (m *MockLogger) SetDebugMode(enabled bool)                {}
func (m *MockLogger) GetDebugMode() bool                       { return false }

func getServerAddr(server *network.Server) (string, error) {
	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
	return "127.0.0.1:8080", nil
}

func TestServer_StartStop(t *testing.T) {
	// 准备测试数据
	config := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    8080,
		SyncDir: "/test/dir",
	}

	// 创建服务器
	server := network.NewServer(config, &MockLogger{})

	// 测试启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// 验证服务器状态
	if !server.IsRunning() {
		t.Error("IsRunning() = false, want true")
	}

	// 测试停止服务器
	if err := server.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// 验证服务器状态
	if server.IsRunning() {
		t.Error("IsRunning() = true, want false")
	}
}

func TestServer_ClientConnection(t *testing.T) {
	// 准备测试数据
	config := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    8080,
		SyncDir: "/test/dir",
	}

	// 创建服务器
	server := network.NewServer(config, &MockLogger{})

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端连接
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port))
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// 发送测试数据
	testData := []byte("Hello, Server!")
	if _, err := conn.Write(testData); err != nil {
		t.Errorf("Failed to write to server: %v", err)
	}

	// 等待一段时间确保数据处理
	time.Sleep(100 * time.Millisecond)
}

func TestServer_MultipleClients(t *testing.T) {
	// 准备测试数据
	config := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    8080,
		SyncDir: "/test/dir",
	}

	// 创建服务器
	server := network.NewServer(config, &MockLogger{})

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建多个客户端连接
	numClients := 5
	for i := 0; i < numClients; i++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port))
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		defer conn.Close()

		// 发送测试数据
		testData := []byte(fmt.Sprintf("Hello from client %d", i))
		if _, err := conn.Write(testData); err != nil {
			t.Errorf("Failed to write from client %d: %v", i, err)
		}
	}

	// 等待一段时间确保数据处理
	time.Sleep(100 * time.Millisecond)
}
