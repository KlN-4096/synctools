package network_test

import (
	"net"
	"sync"
	"testing"
	"time"

	"synctools/internal/model"
	"synctools/internal/network"
)

// mockLogger 实现 model.Logger 接口用于测试
type mockLogger struct{}

func (m *mockLogger) Log(format string, v ...interface{})      {}
func (m *mockLogger) Info(msg string, args ...interface{})     {}
func (m *mockLogger) Error(msg string, args ...interface{})    {}
func (m *mockLogger) Debug(msg string, args ...interface{})    {}
func (m *mockLogger) Warn(msg string, args ...interface{})     {}
func (m *mockLogger) DebugLog(format string, v ...interface{}) {}
func (m *mockLogger) SetDebugMode(enabled bool)                {}
func (m *mockLogger) GetDebugMode() bool                       { return false }

// TestServerStart 测试服务器启动相关功能
func TestServerStart(t *testing.T) {
	tests := []struct {
		name    string
		config  *model.Config
		wantErr bool
	}{
		{
			name: "正常启动",
			config: &model.Config{
				Host: "localhost",
				Port: 8080,
			},
			wantErr: false,
		},
		{
			name: "无效端口",
			config: &model.Config{
				Host: "localhost",
				Port: -1,
			},
			wantErr: true,
		},
		{
			name: "无效地址",
			config: &model.Config{
				Host: "invalid-host-name",
				Port: 8080,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := network.NewServer(tt.config, &mockLogger{})
			err := server.Start()

			if (err != nil) != tt.wantErr {
				t.Errorf("Server.Start() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				if err := server.Stop(); err != nil {
					t.Errorf("Server.Stop() error = %v", err)
				}
			}
		})
	}
}

// TestServerRepeatedStart 测试重复启动服务器
func TestServerRepeatedStart(t *testing.T) {
	config := &model.Config{
		Host: "localhost",
		Port: 8081,
	}

	server := network.NewServer(config, &mockLogger{})

	// 第一次启动
	if err := server.Start(); err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	// 尝试重复启动
	if err := server.Start(); err == nil {
		t.Error("Expected error on repeated start, got nil")
	}

	server.Stop()
}

// TestClientConnections 测试客户端连接
func TestClientConnections(t *testing.T) {
	config := &model.Config{
		Host: "localhost",
		Port: 8082,
	}

	server := network.NewServer(config, &mockLogger{})
	if err := server.Start(); err != nil {
		t.Fatalf("Server start failed: %v", err)
	}
	defer server.Stop()

	// 测试多个客户端同时连接
	var wg sync.WaitGroup
	clientCount := 5

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", "localhost:8082")
			if err != nil {
				t.Errorf("Client connection failed: %v", err)
				return
			}
			defer conn.Close()

			// 保持连接一段时间
			time.Sleep(100 * time.Millisecond)
		}()
	}

	wg.Wait()
}

// TestDataTransfer 测试数据传输
func TestDataTransfer(t *testing.T) {
	config := &model.Config{
		Host: "localhost",
		Port: 8083,
	}

	server := network.NewServer(config, &mockLogger{})
	if err := server.Start(); err != nil {
		t.Fatalf("Server start failed: %v", err)
	}
	defer server.Stop()

	// 连接客户端
	conn, err := net.Dial("tcp", "localhost:8083")
	if err != nil {
		t.Fatalf("Client connection failed: %v", err)
	}
	defer conn.Close()

	// 发送测试数据
	testData := []byte("Hello, Server!")
	_, err = conn.Write(testData)
	if err != nil {
		t.Errorf("Failed to send data: %v", err)
	}

	// 等待服务器处理数据
	time.Sleep(100 * time.Millisecond)
}

// TestServerStop 测试服务器停止
func TestServerStop(t *testing.T) {
	config := &model.Config{
		Host: "localhost",
		Port: 8084,
	}

	server := network.NewServer(config, &mockLogger{})

	// 测试停止未启动的服务器
	if err := server.Stop(); err != nil {
		t.Errorf("Stop non-running server failed: %v", err)
	}

	// 启动并停止服务器
	if err := server.Start(); err != nil {
		t.Fatalf("Server start failed: %v", err)
	}

	if err := server.Stop(); err != nil {
		t.Errorf("Server stop failed: %v", err)
	}

	// 确保所有连接都已关闭
	conn, err := net.Dial("tcp", "localhost:8084")
	if err == nil {
		conn.Close()
		t.Error("Server still accepting connections after stop")
	}
}
