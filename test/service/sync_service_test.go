package service_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"synctools/internal/config"
	"synctools/internal/model"
	"synctools/internal/service"
)

// mockLogger 用于测试的日志记录器
type mockLogger struct {
	entries []string
	mu      sync.Mutex
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		entries: make([]string, 0),
	}
}

func (l *mockLogger) Log(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprintf(format, v...))
}

func (l *mockLogger) Info(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprintf("[INFO] "+msg, args...))
}

func (l *mockLogger) Error(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprintf("[ERROR] "+msg, args...))
}

func (l *mockLogger) DebugLog(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprintf("[DEBUG] "+format, v...))
}

func (l *mockLogger) SetDebugMode(enabled bool) {}

func (l *mockLogger) GetDebugMode() bool { return false }

// setupTestConfig 设置测试配置环境
func setupTestConfig(t *testing.T) (*config.Manager, string) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "synctools_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	// 创建配置管理器
	logger := newMockLogger()
	configManager, err := config.NewManager(tempDir, logger)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("创建配置管理器失败: %v", err)
	}

	return configManager, tempDir
}

// cleanupTestConfig 清理测试配置环境
func cleanupTestConfig(tempDir string) {
	os.RemoveAll(tempDir)
}

// TestSyncService_ServiceStateManagement 测试服务状态管理
func TestSyncService_ServiceStateManagement(t *testing.T) {
	logger := newMockLogger()
	configManager, tempDir := setupTestConfig(t)
	defer cleanupTestConfig(tempDir)

	// 创建一个有效的配置
	validConfig := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0",
		Host:    "localhost",
		Port:    8080,
		SyncDir: filepath.Join(tempDir, "sync"),
	}
	if err := configManager.Save(validConfig); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	if err := configManager.LoadConfig(validConfig.UUID); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	syncService := service.NewSyncService(configManager, logger)

	// 测试启动服务
	t.Run("启动服务", func(t *testing.T) {
		err := syncService.Start()
		if err != nil {
			t.Errorf("启动服务失败: %v", err)
		}
		if !syncService.IsRunning() {
			t.Error("服务应该处于运行状态")
		}
	})

	// 测试重复启动服务
	t.Run("重复启动服务", func(t *testing.T) {
		err := syncService.Start()
		if err == nil {
			t.Error("重复启动服务应该返回错误")
		}
	})

	// 测试停止服务
	t.Run("停止服务", func(t *testing.T) {
		err := syncService.Stop()
		if err != nil {
			t.Errorf("停止服务失败: %v", err)
		}
		if syncService.IsRunning() {
			t.Error("服务应该处于停止状态")
		}
	})

	// 测试重启服务
	t.Run("重启服务", func(t *testing.T) {
		// 先启动
		err := syncService.Start()
		if err != nil {
			t.Errorf("启动服务失败: %v", err)
		}

		// 然后停止
		err = syncService.Stop()
		if err != nil {
			t.Errorf("停止服务失败: %v", err)
		}

		// 最后重新启动
		err = syncService.Start()
		if err != nil {
			t.Errorf("重新启动服务失败: %v", err)
		}
		if !syncService.IsRunning() {
			t.Error("服务应该处于运行状态")
		}
	})

	// 测试无配置时启动服务
	t.Run("无配置启动服务", func(t *testing.T) {
		configManager, tempDir := setupTestConfig(t)
		defer cleanupTestConfig(tempDir)
		syncService := service.NewSyncService(configManager, logger)
		err := syncService.Start()
		if err == nil {
			t.Error("无配置时启动服务应该返回错误")
		}
	})
}

// TestSyncService_ProgressTracking 测试同步进度
func TestSyncService_ProgressTracking(t *testing.T) {
	logger := newMockLogger()
	configManager, tempDir := setupTestConfig(t)
	defer cleanupTestConfig(tempDir)

	// 创建并设置有效配置
	validConfig := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0",
		Host:    "localhost",
		Port:    8080,
		SyncDir: filepath.Join(tempDir, "sync"),
	}
	if err := configManager.Save(validConfig); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	if err := configManager.LoadConfig(validConfig.UUID); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	syncService := service.NewSyncService(configManager, logger)

	var progressUpdates []*service.SyncProgress
	var mu sync.Mutex

	// 设置进度回调
	syncService.SetProgressCallback(func(progress *service.SyncProgress) {
		mu.Lock()
		defer mu.Unlock()
		progressUpdates = append(progressUpdates, progress)
	})

	// 测试进度回调正常工作
	t.Run("进度回调正常工作", func(t *testing.T) {
		// 清空进度更新列表
		mu.Lock()
		progressUpdates = nil
		mu.Unlock()

		// 启动服务
		if err := syncService.Start(); err != nil {
			t.Fatalf("启动服务失败: %v", err)
		}

		// 等待一段时间以接收进度更新
		time.Sleep(500 * time.Millisecond)

		// 检查是否收到进度更新
		mu.Lock()
		updateCount := len(progressUpdates)
		mu.Unlock()

		if updateCount == 0 {
			t.Log("当前日志记录:", logger.entries)
			t.Error("应该收到至少一个进度更新")
		}

		// 停止服务
		if err := syncService.Stop(); err != nil {
			t.Fatalf("停止服务失败: %v", err)
		}

		// 等待一段时间以确保收到停止状态
		time.Sleep(500 * time.Millisecond)
	})

	// 测试进度计算准确性
	t.Run("进度计算准确性", func(t *testing.T) {
		// 清空进度更新列表
		mu.Lock()
		progressUpdates = nil
		mu.Unlock()

		// 启动服务
		if err := syncService.Start(); err != nil {
			t.Fatalf("启动服务失败: %v", err)
		}

		// 等待一段时间以接收进度更新
		time.Sleep(500 * time.Millisecond)

		mu.Lock()
		updates := make([]*service.SyncProgress, len(progressUpdates))
		copy(updates, progressUpdates)
		mu.Unlock()

		for _, progress := range updates {
			if progress.ProcessedFiles > progress.TotalFiles {
				t.Errorf("处理文件数(%d)不应超过总文件数(%d)",
					progress.ProcessedFiles, progress.TotalFiles)
			}
		}

		// 停止服务
		if err := syncService.Stop(); err != nil {
			t.Fatalf("停止服务失败: %v", err)
		}

		// 等待一段时间以确保收到停止状态
		time.Sleep(500 * time.Millisecond)
	})

	// 测试取消同步操作
	t.Run("取消同步操作", func(t *testing.T) {
		// 清空进度更新列表
		mu.Lock()
		progressUpdates = nil
		mu.Unlock()

		// 启动服务
		if err := syncService.Start(); err != nil {
			t.Fatalf("启动服务失败: %v", err)
		}

		// 等待一段时间以确保服务已启动
		time.Sleep(500 * time.Millisecond)

		// 停止服务
		if err := syncService.Stop(); err != nil {
			t.Errorf("取消同步操作失败: %v", err)
		}

		// 等待一段时间以确保收到最终状态
		time.Sleep(500 * time.Millisecond)

		mu.Lock()
		updates := make([]*service.SyncProgress, len(progressUpdates))
		copy(updates, progressUpdates)
		mu.Unlock()

		if len(updates) == 0 {
			t.Error("应该收到至少一个进度更新")
			return
		}

		lastProgress := updates[len(updates)-1]
		if lastProgress.Status != "服务已停止" {
			t.Errorf("取消同步后状态应为'服务已停止'，实际为'%s'", lastProgress.Status)
		}
	})
}

// TestSyncService_ConfigManagement 测试配置管理
func TestSyncService_ConfigManagement(t *testing.T) {
	logger := newMockLogger()
	configManager, tempDir := setupTestConfig(t)
	defer cleanupTestConfig(tempDir)

	syncService := service.NewSyncService(configManager, logger)

	// 测试加载配置
	t.Run("加载配置", func(t *testing.T) {
		config := &model.Config{
			UUID:    "test-uuid",
			Name:    "test-config",
			Version: "1.0",
			Host:    "localhost",
			Port:    8080,
			SyncDir: filepath.Join(tempDir, "sync"),
		}
		if err := configManager.Save(config); err != nil {
			t.Fatalf("保存配置失败: %v", err)
		}
		if err := configManager.LoadConfig(config.UUID); err != nil {
			t.Fatalf("加载配置失败: %v", err)
		}

		loadedConfig := configManager.GetCurrentConfig()
		if loadedConfig == nil {
			t.Error("加载配置失败")
			return
		}
		if loadedConfig.UUID != config.UUID {
			t.Error("加载的配置与原配置不匹配")
		}
	})

	// 测试配置变更通知
	t.Run("配置变更通知", func(t *testing.T) {
		// 先启动服务
		config := &model.Config{
			UUID:    "test-uuid-3",
			Name:    "test-config-3",
			Version: "3.0",
			Host:    "localhost",
			Port:    8082,
			SyncDir: filepath.Join(tempDir, "sync3"),
		}
		if err := configManager.Save(config); err != nil {
			t.Fatalf("保存配置失败: %v", err)
		}
		if err := configManager.LoadConfig(config.UUID); err != nil {
			t.Fatalf("加载配置失败: %v", err)
		}

		err := syncService.Start()
		if err != nil {
			t.Errorf("启动服务失败: %v", err)
		}

		// 更改配置
		newConfig := &model.Config{
			UUID:    "test-uuid-3",
			Name:    "test-config-3-updated",
			Version: "3.1",
			Host:    "localhost",
			Port:    8082,
			SyncDir: filepath.Join(tempDir, "sync3"),
		}
		if err := configManager.Save(newConfig); err != nil {
			t.Fatalf("保存新配置失败: %v", err)
		}
		if err := configManager.LoadConfig(newConfig.UUID); err != nil {
			t.Fatalf("加载新配置失败: %v", err)
		}

		// 重启服务以应用新配置
		err = syncService.Stop()
		if err != nil {
			t.Errorf("停止服务失败: %v", err)
		}
		err = syncService.Start()
		if err != nil {
			t.Errorf("重启服务失败: %v", err)
		}

		// 验证新配置是否生效
		currentConfig := configManager.GetCurrentConfig()
		if currentConfig == nil {
			t.Error("当前配置为空")
			return
		}
		if currentConfig.Version != "3.1" {
			t.Error("配置更新未生效")
		}
	})

	// 测试配置验证
	t.Run("配置验证", func(t *testing.T) {
		invalidConfig := &model.Config{
			UUID:    "test-uuid-4",
			Name:    "", // 无效：名称为空
			Version: "4.0",
			Host:    "localhost",
			Port:    8083,
			SyncDir: filepath.Join(tempDir, "sync4"),
		}
		if err := configManager.Save(invalidConfig); err == nil {
			t.Error("保存无效配置应该返回错误")
		}
	})
}
