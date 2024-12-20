package frontend

import (
	"testing"

	"synctools/internal/model"
	"synctools/internal/service"
	"synctools/internal/ui/viewmodels"
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
	m.currentConfig = config
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

// MockLogger 模拟日志记录器
type MockLogger struct{}

func (m *MockLogger) Log(format string, v ...interface{})      {}
func (m *MockLogger) Info(msg string, args ...interface{})     {}
func (m *MockLogger) Error(msg string, args ...interface{})    {}
func (m *MockLogger) DebugLog(format string, v ...interface{}) {}
func (m *MockLogger) SetDebugMode(enabled bool)                {}
func (m *MockLogger) GetDebugMode() bool                       { return false }

// MockLineEdit 模拟文本输入框
type MockLineEdit struct {
	text string
}

func (m *MockLineEdit) Text() string {
	return m.text
}

func (m *MockLineEdit) SetText(text string) error {
	m.text = text
	return nil
}

func TestConfigViewModel_UpdateUI(t *testing.T) {
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

	nameEdit := &MockLineEdit{}
	versionEdit := &MockLineEdit{}
	hostEdit := &MockLineEdit{}
	portEdit := &MockLineEdit{}
	syncDirEdit := &MockLineEdit{}

	// 创建视图模型
	viewModel := viewmodels.NewConfigViewModel(syncService, logger)

	// 设置UI组件
	viewModel.SetupUI(
		nil, // configTable
		nil, // redirectTable
		nil, // statusBar
		nameEdit,
		versionEdit,
		hostEdit,
		portEdit,
		syncDirEdit,
		nil, // ignoreEdit
		nil, // syncFolderTable
	)

	// 验证UI更新
	if nameEdit.Text() != config.Name {
		t.Errorf("Name = %v, want %v", nameEdit.Text(), config.Name)
	}
	if versionEdit.Text() != config.Version {
		t.Errorf("Version = %v, want %v", versionEdit.Text(), config.Version)
	}
	if hostEdit.Text() != config.Host {
		t.Errorf("Host = %v, want %v", hostEdit.Text(), config.Host)
	}
	if portEdit.Text() != "8080" {
		t.Errorf("Port = %v, want 8080", portEdit.Text())
	}
	if syncDirEdit.Text() != config.SyncDir {
		t.Errorf("SyncDir = %v, want %v", syncDirEdit.Text(), config.SyncDir)
	}
}

func TestConfigViewModel_SaveConfig(t *testing.T) {
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

	nameEdit := &MockLineEdit{text: "new-name"}
	versionEdit := &MockLineEdit{text: "2.0.0"}
	hostEdit := &MockLineEdit{text: "0.0.0.0"}
	portEdit := &MockLineEdit{text: "9090"}
	syncDirEdit := &MockLineEdit{text: "/new/dir"}

	// 创建视图模型
	viewModel := viewmodels.NewConfigViewModel(syncService, logger)

	// 设置UI组件
	viewModel.SetupUI(
		nil, // configTable
		nil, // redirectTable
		nil, // statusBar
		nameEdit,
		versionEdit,
		hostEdit,
		portEdit,
		syncDirEdit,
		nil, // ignoreEdit
		nil, // syncFolderTable
	)

	// 测试保存配置
	if err := viewModel.SaveConfig(); err != nil {
		t.Errorf("SaveConfig() error = %v", err)
	}

	// 验证配置更新
	config = configManager.GetCurrentConfig()
	if config.Name != nameEdit.Text() {
		t.Errorf("Name = %v, want %v", config.Name, nameEdit.Text())
	}
	if config.Version != versionEdit.Text() {
		t.Errorf("Version = %v, want %v", config.Version, versionEdit.Text())
	}
	if config.Host != hostEdit.Text() {
		t.Errorf("Host = %v, want %v", config.Host, hostEdit.Text())
	}
	if config.Port != 9090 {
		t.Errorf("Port = %v, want 9090", config.Port)
	}
	if config.SyncDir != syncDirEdit.Text() {
		t.Errorf("SyncDir = %v, want %v", config.SyncDir, syncDirEdit.Text())
	}
}
