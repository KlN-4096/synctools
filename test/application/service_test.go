package application

import (
	"testing"

	"synctools/internal/model"
	"synctools/internal/service"
)

// MockConfigManager 模拟配置管理器
type MockConfigManager struct {
	currentConfig *model.Config
	onChanged     func()
}

func (m *MockConfigManager) GetCurrentConfig() *model.Config {
	return m.currentConfig
}

func (m *MockConfigManager) LoadConfig(uuid string) error {
	return nil
}

func (m *MockConfigManager) SaveCurrentConfig() error {
	return nil
}

func (m *MockConfigManager) ListConfigs() ([]*model.Config, error) {
	return []*model.Config{m.currentConfig}, nil
}

func (m *MockConfigManager) Save(config *model.Config) error {
	return nil
}

func (m *MockConfigManager) DeleteConfig(uuid string) error {
	return nil
}

func (m *MockConfigManager) ValidateConfig(config *model.Config) error {
	return config.Validate()
}

func (m *MockConfigManager) SetOnChanged(callback func()) {
	m.onChanged = callback
}

// MockLogger 模拟日志记录器
type MockLogger struct{}

func (m *MockLogger) Log(format string, v ...interface{})      {}
func (m *MockLogger) Info(msg string, args ...interface{})     {}
func (m *MockLogger) Error(msg string, args ...interface{})    {}
func (m *MockLogger) DebugLog(format string, v ...interface{}) {}
func (m *MockLogger) SetDebugMode(enabled bool)                {}
func (m *MockLogger) GetDebugMode() bool                       { return false }

// MockServer 模拟服务器
type MockServer struct {
	running bool
}

func (m *MockServer) Start() error {
	m.running = true
	return nil
}

func (m *MockServer) Stop() error {
	m.running = false
	return nil
}

func (m *MockServer) IsRunning() bool {
	return m.running
}

func TestSyncService_StartStop(t *testing.T) {
	// 准备测试数据
	config := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    8080,
		SyncDir: "/test/dir",
	}

	// 创建模拟对象
	configManager := &MockConfigManager{currentConfig: config}
	logger := &MockLogger{}
	server := &MockServer{}

	// 创建同步服务
	syncService := service.NewSyncService(configManager, logger)
	syncService.SetServer(server)

	// 测试启动服务
	if err := syncService.Start(); err != nil {
		t.Errorf("Start() error = %v", err)
	}
	if !syncService.IsRunning() {
		t.Error("Start() failed to set running state")
	}

	// 测试停止服务
	if err := syncService.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
	if syncService.IsRunning() {
		t.Error("Stop() failed to clear running state")
	}
}

func TestSyncService_ConfigOperations(t *testing.T) {
	// 准备测试数据
	config := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    8080,
		SyncDir: "/test/dir",
	}

	// 创建模拟对象
	configManager := &MockConfigManager{currentConfig: config}
	logger := &MockLogger{}

	// 创建同步服务
	syncService := service.NewSyncService(configManager, logger)

	// 测试获取当前配置
	if got := syncService.GetCurrentConfig(); got != config {
		t.Errorf("GetCurrentConfig() = %v, want %v", got, config)
	}

	// 测试获取配置列表
	configs, err := syncService.ListConfigs()
	if err != nil {
		t.Errorf("ListConfigs() error = %v", err)
	}
	if len(configs) != 1 || configs[0] != config {
		t.Errorf("ListConfigs() = %v, want [%v]", configs, config)
	}

	// 测试保存配置
	if err := syncService.SaveConfig(); err != nil {
		t.Errorf("SaveConfig() error = %v", err)
	}

	// 测试验证配置
	if err := syncService.ValidateConfig(config); err != nil {
		t.Errorf("ValidateConfig() error = %v", err)
	}
}
