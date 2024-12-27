package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/internal/ui/client/viewmodels"
	"synctools/codes/pkg/logger"
	"synctools/codes/pkg/service"
	"synctools/codes/pkg/storage"
)

func TestSyncProcess(t *testing.T) {
	// 创建测试目录结构
	tmpDir := t.TempDir()
	testClientDir := filepath.Join(tmpDir, "client")
	testServerDir := filepath.Join(tmpDir, "server")

	// 创建客户端和服务器目录
	if err := os.MkdirAll(testClientDir, 0755); err != nil {
		t.Fatalf("创建客户端目录失败: %v", err)
	}
	if err := os.MkdirAll(testServerDir, 0755); err != nil {
		t.Fatalf("创建服务器目录失败: %v", err)
	}

	// 在服务器目录中创建测试文件
	testContent := []byte("test content")
	testFile := filepath.Join(testServerDir, "test.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 创建日志器
	log, err := logger.NewDefaultLogger(filepath.Join(tmpDir, "logs"))
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 创建配置和存储服务
	storageDir := filepath.Join(tmpDir, "configs")
	store, err := storage.NewFileStorage(storageDir, log)
	if err != nil {
		t.Fatalf("创建存储服务失败: %v", err)
	}

	// 创建服务器配置
	serverConfig := &interfaces.Config{
		UUID:       "test-server",
		Type:       interfaces.ConfigTypeServer,
		Name:       "Test Server",
		Version:    "1.0.0",
		Host:       "127.0.0.1",
		Port:       25000,
		SyncDir:    testServerDir,
		IgnoreList: []string{".DS_Store", "thumbs.db"},
		SyncFolders: []interfaces.SyncFolder{
			{
				Path:     ".",
				SyncMode: interfaces.MirrorSync,
			},
		},
	}

	// 创建并启动服务器端同步服务
	serverService := service.NewSyncService(serverConfig, log, store)
	if err := serverService.Start(); err != nil {
		t.Fatalf("启动服务器同步服务失败: %v", err)
	}
	if err := serverService.StartServer(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}
	defer serverService.Stop()

	// 创建客户端配置
	clientConfig := &interfaces.Config{
		UUID:    "test-client",
		Type:    interfaces.ConfigTypeClient,
		Name:    "Test Client",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    25000,
		SyncDir: testClientDir,
	}

	// 创建客户端同步服务
	clientService := service.NewSyncService(clientConfig, log, store)
	if err := clientService.Start(); err != nil {
		t.Fatalf("启动客户端同步服务失败: %v", err)
	}
	defer clientService.Stop()

	// 创建客户端 ViewModel
	viewModel := viewmodels.NewMainViewModel(clientService, log)

	// 测试连接到服务器
	t.Run("连接服务器", func(t *testing.T) {
		if err := viewModel.Connect(); err != nil {
			t.Fatalf("连接服务器失败: %v", err)
		}
		defer viewModel.Disconnect()

		// 验证连接状态
		if !viewModel.IsConnected() {
			t.Fatal("连接状态检查失败")
		}

		// 等待连接稳定
		time.Sleep(time.Second)

		// 执行同步
		if err := viewModel.SyncFiles(testClientDir); err != nil {
			t.Fatalf("同步文件失败: %v", err)
		}

		// 等待同步完成
		time.Sleep(time.Second * 2)

		// 验证同步结果
		synced := false
		for i := 0; i < 5; i++ {
			if viewModel.GetCurrentConfig() != nil &&
				clientService.GetSyncStatus() != "同步中" {
				synced = true
				break
			}
			time.Sleep(time.Second)
		}
		if !synced {
			t.Fatal("同步未完成")
		}

		// 检查客户端文件
		syncedFile := filepath.Join(testClientDir, "test.txt")
		content, err := os.ReadFile(syncedFile)
		if err != nil {
			t.Fatalf("读取同步后的文件失败: %v", err)
		}
		if string(content) != string(testContent) {
			t.Errorf("文件内容不匹配，期望: %s，实际: %s",
				string(testContent), string(content))
		}
	})
}
