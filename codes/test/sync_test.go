package service_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/logger"
	"synctools/codes/pkg/network"
	"synctools/codes/pkg/service"
	"synctools/codes/pkg/storage"
)

// mockServer 模拟服务器
type mockServer struct {
	t      *testing.T
	ln     net.Listener
	logger interfaces.Logger
}

func newMockServer(t *testing.T) (*mockServer, error) {
	// 创建日志器
	log, err := logger.NewDefaultLogger("./logs")
	if err != nil {
		return nil, err
	}

	// 监听端口
	ln, err := net.Listen("tcp", "127.0.0.1:25000")
	if err != nil {
		return nil, err
	}

	return &mockServer{
		t:      t,
		ln:     ln,
		logger: log,
	}, nil
}

func (s *mockServer) serve() {
	// 接受一个连接
	conn, err := s.ln.Accept()
	if err != nil {
		if !strings.Contains(err.Error(), "use of closed network connection") {
			s.t.Errorf("接受连接失败: %v", err)
		}
		return
	}

	// 使用 defer 确保连接最后关闭
	defer conn.Close()

	// 设置超时
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// 处理连接
	s.handleConnection(conn)
}

func (s *mockServer) handleConnection(conn net.Conn) {
	ops := network.NewOperations(s.logger)

	// 读取客户端请求
	var msg interfaces.Message
	if err := ops.ReadJSON(conn, &msg); err != nil {
		if !strings.Contains(err.Error(), "use of closed network connection") {
			s.t.Errorf("读取请求失败: %v", err)
		}
		return
	}

	// 验证请求类型和内容
	if msg.Type != "sync_request" {
		s.t.Errorf("预期请求类型为 sync_request, 实际为 %s", msg.Type)
		return
	}

	// 解析同步请求
	var syncReq struct {
		Path      string   `json:"path"`
		Mode      string   `json:"mode"`
		Direction string   `json:"direction"`
		Files     []string `json:"files,omitempty"`
	}
	if err := json.Unmarshal(msg.Payload, &syncReq); err != nil {
		s.t.Errorf("解析同步请求失败: %v", err)
		return
	}

	// 验证请求参数
	if syncReq.Mode != "mirror" {
		s.t.Errorf("预期同步模式为 mirror, 实际为 %s", syncReq.Mode)
	}
	if syncReq.Direction != "upload" {
		s.t.Errorf("预期同步方向为 upload, 实际为 %s", syncReq.Direction)
	}

	// 发送响应
	response := interfaces.Message{
		Type: "sync_response",
		UUID: msg.UUID,
		Payload: json.RawMessage(`{
			"success": true,
			"message": "同步完成"
		}`),
	}
	if err := ops.WriteJSON(conn, &response); err != nil {
		if !strings.Contains(err.Error(), "use of closed network connection") {
			s.t.Errorf("发送响应失败: %v", err)
		}
		return
	}
}

func TestClientSync(t *testing.T) {
	// 创建测试目录
	testDir := "test_sync"
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	defer os.RemoveAll(testDir)

	// 创建测试文件
	testFile := filepath.Join(testDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 创建模拟服务器
	server, err := newMockServer(t)
	if err != nil {
		t.Fatalf("创建模拟服务器失败: %v", err)
	}
	defer server.ln.Close()

	// 在后台运行服务器
	go server.serve()

	// 创建日志器
	log, err := logger.NewDefaultLogger("./logs")
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 创建存储服务
	store, err := storage.NewFileStorage("./test_storage", log)
	if err != nil {
		t.Fatalf("创建存储服务失败: %v", err)
	}

	// 创建配置
	config := &interfaces.Config{
		UUID:    "test-uuid",
		Type:    interfaces.ConfigTypeClient,
		Name:    "Test Client",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    25000,
		SyncDir: testDir,
	}

	// 创建并启动同步服务
	syncService := service.NewSyncService(config, log, store)
	if err := syncService.Start(); err != nil {
		t.Fatalf("启动同步服务失败: %v", err)
	}
	defer syncService.Stop()

	// 模拟点击同步按钮后的操作
	t.Run("同步操作", func(t *testing.T) {
		// 执行同步操作
		if err := syncService.SyncFiles(testDir); err != nil {
			t.Fatalf("同步文件失败: %v", err)
		}

		// 等待同步完成
		time.Sleep(2 * time.Second) // 增加等待时间

		// 检查文件是否存在和内容是否正确
		syncedFile := filepath.Join(testDir, "test.txt")
		if _, err := os.Stat(syncedFile); os.IsNotExist(err) {
			t.Errorf("同步后文件不存在: %s", syncedFile)
		}

		content, err := os.ReadFile(syncedFile)
		if err != nil {
			t.Errorf("读取同步文件失败: %v", err)
		} else if string(content) != "test content" {
			t.Errorf("文件内容不匹配，期望: test content，实际: %s", string(content))
		}

		// 验证同步状态
		status := syncService.GetSyncStatus()
		t.Logf("最终同步状态: %s", status)
		if status != "运行中" && status != "同步完成" {
			t.Errorf("同步状态异常: %s", status)
		}
	})
}
