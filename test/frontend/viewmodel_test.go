package frontend

import (
	"fmt"
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
	fmt.Printf("[DEBUG] MockConfigManager.GetCurrentConfig() -> %+v\n", m.currentConfig)
	return m.currentConfig
}

func (m *MockConfigManager) LoadConfig(uuid string) error {
	return nil
}

func (m *MockConfigManager) SaveCurrentConfig() error {
	fmt.Printf("[DEBUG] MockConfigManager.SaveCurrentConfig() -> %+v\n", m.currentConfig)
	if m.currentConfig == nil {
		return nil
	}
	return m.Save(m.currentConfig)
}

func (m *MockConfigManager) ListConfigs() ([]*model.Config, error) {
	return []*model.Config{m.currentConfig}, nil
}

func (m *MockConfigManager) Save(config *model.Config) error {
	fmt.Printf("[DEBUG] MockConfigManager.Save() input -> %+v\n", config)
	// 直接使用传入的配置对象
	m.currentConfig = config
	fmt.Printf("[DEBUG] MockConfigManager.Save() updated -> %+v\n", m.currentConfig)

	if m.onChanged != nil {
		m.onChanged()
	}
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

func (m *MockLogger) Log(format string, v ...interface{}) {
	fmt.Printf("[LOG] "+format+"\n", v...)
}
func (m *MockLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO] "+msg+"\n", args...)
}
func (m *MockLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}
func (m *MockLogger) DebugLog(format string, v ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", v...)
}
func (m *MockLogger) SetDebugMode(enabled bool) {}
func (m *MockLogger) GetDebugMode() bool        { return true }

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
	fmt.Printf("[TEST] Initial config -> %+v\n", config)

	// 创建模拟对象
	configManager := &MockConfigManager{currentConfig: config}
	logger := &MockLogger{}
	server := &MockServer{}

	// 创建同步服务
	syncService := service.NewSyncService(configManager, logger)
	syncService.SetServer(server)

	// 创建视图模型
	viewModel := viewmodels.NewConfigViewModel(syncService, logger)

	// 创建UI组件
	nameEdit := &MockLineEdit{text: "new-name"}
	versionEdit := &MockLineEdit{text: "2.0.0"}
	hostEdit := &MockLineEdit{text: "0.0.0.0"}
	portEdit := &MockLineEdit{text: "9090"}
	syncDirEdit := &MockLineEdit{text: "/new/dir"}

	fmt.Printf("[TEST] Before SetupUI - UI components values: name=%s, version=%s, host=%s, port=%s, syncDir=%s\n",
		nameEdit.Text(), versionEdit.Text(), hostEdit.Text(), portEdit.Text(), syncDirEdit.Text())

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

	fmt.Printf("[TEST] After SetupUI - UI components values: name=%s, version=%s, host=%s, port=%s, syncDir=%s\n",
		nameEdit.Text(), versionEdit.Text(), hostEdit.Text(), portEdit.Text(), syncDirEdit.Text())

	// 重新设置UI组件的值
	nameEdit.SetText("new-name")
	versionEdit.SetText("2.0.0")
	hostEdit.SetText("0.0.0.0")
	portEdit.SetText("9090")
	syncDirEdit.SetText("/new/dir")

	fmt.Printf("[TEST] After Reset - UI components values: name=%s, version=%s, host=%s, port=%s, syncDir=%s\n",
		nameEdit.Text(), versionEdit.Text(), hostEdit.Text(), portEdit.Text(), syncDirEdit.Text())

	// 测试保存配置
	if err := viewModel.SaveConfig(); err != nil {
		t.Errorf("SaveConfig() error = %v", err)
	}

	// 验证配置更新
	updatedConfig := configManager.GetCurrentConfig()
	fmt.Printf("[TEST] Final config -> %+v\n", updatedConfig)

	if updatedConfig.Name != "new-name" {
		t.Errorf("Name = %v, want %v", updatedConfig.Name, "new-name")
	}
	if updatedConfig.Version != "2.0.0" {
		t.Errorf("Version = %v, want %v", updatedConfig.Version, "2.0.0")
	}
	if updatedConfig.Host != "0.0.0.0" {
		t.Errorf("Host = %v, want %v", updatedConfig.Host, "0.0.0.0")
	}
	if updatedConfig.Port != 9090 {
		t.Errorf("Port = %v, want %v", updatedConfig.Port, 9090)
	}
	if updatedConfig.SyncDir != "/new/dir" {
		t.Errorf("SyncDir = %v, want %v", updatedConfig.SyncDir, "/new/dir")
	}
}
