package viewmodels_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"synctools/internal/config"
	"synctools/internal/model"
	"synctools/internal/service"
	"synctools/internal/ui/viewmodels"

	"github.com/lxn/walk"
)

// mockMainWindow 模拟主窗口
type mockMainWindow struct {
	messageShown string
	messageTitle string
	messageStyle walk.MsgBoxStyle
}

func (w *mockMainWindow) MsgBox(title, message string, style walk.MsgBoxStyle) int {
	w.messageTitle = title
	w.messageShown = message
	w.messageStyle = style
	return walk.DlgCmdOK
}

// LineEditIface 定义 LineEdit 接口
type LineEditIface interface {
	Text() string
	SetText(text string) error
}

// TableViewIface 定义 TableView 接口
type TableViewIface interface {
	Model() interface{}
	SetModel(model interface{}) error
	CurrentIndex() int
	Width() int
	Columns() *walk.TableViewColumnList
}

// mockLineEdit 模拟文本输入框
type mockLineEdit struct {
	text string
}

func NewMockLineEdit() *mockLineEdit {
	return &mockLineEdit{text: ""}
}

func (e *mockLineEdit) Text() string {
	return e.text
}

func (e *mockLineEdit) SetText(text string) error {
	e.text = text
	return nil
}

// mockNumberEdit 模拟数字输入框
type mockNumberEdit struct {
	value float64
}

func (e *mockNumberEdit) Value() float64 {
	return e.value
}

func (e *mockNumberEdit) SetValue(value float64) error {
	e.value = value
	return nil
}

// mockTableView 模拟表格视图
type mockTableView struct {
	model      interface{}
	currentRow int
	width      int
	columns    *walk.TableViewColumnList
}

func NewMockTableView() *mockTableView {
	return &mockTableView{
		width:   800,
		columns: new(walk.TableViewColumnList),
	}
}

func (v *mockTableView) Model() interface{} {
	return v.model
}

func (v *mockTableView) SetModel(model interface{}) error {
	v.model = model
	return nil
}

func (v *mockTableView) CurrentIndex() int {
	return v.currentRow
}

func (v *mockTableView) Width() int {
	return v.width
}

func (v *mockTableView) Columns() *walk.TableViewColumnList {
	return v.columns
}

// mockLogger 模拟日志记录器
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

// setupTest 设置测试环境
func setupTest(t *testing.T) (*viewmodels.ConfigViewModel, *mockMainWindow, *mockLogger, string) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "synctools_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	// 创建日志记录器
	logger := newMockLogger()

	// 创建配置管理器
	configManager, err := config.NewManager(tempDir, logger)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("创建配置管理器失败: %v", err)
	}

	// 创建主窗口
	mainWindow := &mockMainWindow{}

	// 创建同步服务
	syncService := service.NewSyncService(configManager, logger)

	// 创建视图模型
	viewModel := viewmodels.NewConfigViewModel(configManager, syncService, logger)

	// 初始化视图模型
	if err := viewModel.Initialize(nil); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("初始化视图模型失败: %v", err)
	}

	return viewModel, mainWindow, logger, tempDir
}

// cleanupTest 清理测试环境
func cleanupTest(tempDir string) {
	os.RemoveAll(tempDir)
}

// setupUIControls 设置 UI 控件
func setupUIControls(viewModel *viewmodels.ConfigViewModel) (LineEditIface, LineEditIface, LineEditIface, LineEditIface, LineEditIface, TableViewIface, TableViewIface) {
	nameEdit := NewMockLineEdit()
	versionEdit := NewMockLineEdit()
	hostEdit := NewMockLineEdit()
	portEdit := NewMockLineEdit()
	syncDirEdit := NewMockLineEdit()
	configTable := NewMockTableView()
	redirectTable := NewMockTableView()
	syncFolderTable := NewMockTableView()

	viewModel.SetupUI(
		configTable,
		redirectTable,
		nil, // StatusBar
		nameEdit,
		versionEdit,
		hostEdit,
		portEdit,
		syncDirEdit,
		nil, // ignoreEdit
		syncFolderTable,
	)

	return nameEdit, versionEdit, hostEdit, portEdit, syncDirEdit, configTable, redirectTable
}

// TestConfigViewModel_UIBinding 测试 UI 绑定
func TestConfigViewModel_UIBinding(t *testing.T) {
	viewModel, _, _, tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 测试初始化视图模型
	t.Run("初始化视图模型", func(t *testing.T) {
		nameEdit, versionEdit, hostEdit, portEdit, _, _, _ := setupUIControls(viewModel)

		// 验证初始状态
		if nameEdit.Text() != "" {
			t.Error("名称应为空")
		}
		if versionEdit.Text() != "" {
			t.Error("版本应为空")
		}
		if hostEdit.Text() != "0.0.0.0" {
			t.Error("主机地址应为 0.0.0.0")
		}
		if portEdit.Text() != "6666" {
			t.Error("端口应为 6666")
		}
	})

	// 测试更新 UI 控件
	t.Run("更新 UI 控件", func(t *testing.T) {
		nameEdit, _, _, _, _, _, _ := setupUIControls(viewModel)

		// 设置配置
		config := &model.Config{
			Name: "test-config",
		}
		viewModel.SetName(config.Name)

		// 验证 UI 更新
		if nameEdit.Text() != config.Name {
			t.Errorf("名称未更新，期望 %s，实际 %s", config.Name, nameEdit.Text())
		}
	})

	// 测试数据双向绑定
	t.Run("数据双向绑定", func(t *testing.T) {
		nameEdit, _, _, _, _, _, _ := setupUIControls(viewModel)
		viewModel.SetName("new-name")
		if nameEdit.Text() != "new-name" {
			t.Error("UI 未随数据更新")
		}

		nameEdit.SetText("another-name")
		if viewModel.GetName() != "another-name" {
			t.Error("数据未随 UI 更新")
		}
	})
}

// TestConfigViewModel_UserOperations 测试用户操作
func TestConfigViewModel_UserOperations(t *testing.T) {
	viewModel, mainWindow, _, tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 测试保存配置操作
	t.Run("保存配置操作", func(t *testing.T) {
		setupUIControls(viewModel)

		// 创建初始配置
		uuid, err := model.NewUUID()
		if err != nil {
			t.Fatalf("生成UUID失败: %v", err)
		}
		config := &model.Config{
			UUID:    uuid,
			Name:    "test-config",
			Version: "1.0",
			Host:    "localhost",
			Port:    8080,
			SyncDir: filepath.Join(tempDir, "sync"),
		}
		if err := viewModel.Save(config); err != nil {
			t.Fatalf("保存初始配置失败: %v", err)
		}
		if err := viewModel.LoadConfig(config.UUID); err != nil {
			t.Fatalf("加载配置失败: %v", err)
		}

		// 设置配置数据
		viewModel.SetName("test-config-updated")
		viewModel.SetVersion("1.1")
		viewModel.SetHost("127.0.0.1")
		viewModel.SetPort(8081)
		viewModel.SetSyncDir(filepath.Join(tempDir, "sync-new"))

		// 保存配置
		err = viewModel.SaveConfig()
		if err != nil {
			t.Errorf("保存配置失败: %v", err)
		}

		// 验证配置已保存
		configs := viewModel.GetConfigList()
		if len(configs) != 1 {
			t.Error("配置未保存成功")
		}

		// 验证配置内容
		currentConfig := viewModel.GetCurrentConfig()
		if currentConfig == nil {
			t.Error("当前配置为空")
		} else {
			if currentConfig.Name != "test-config-updated" {
				t.Error("配置名称未更新")
			}
			if currentConfig.Version != "1.1" {
				t.Error("配置版本未更新")
			}
			if currentConfig.Host != "127.0.0.1" {
				t.Error("配置主机地址未更新")
			}
			if currentConfig.Port != 8081 {
				t.Error("配置端口未更新")
			}
			if currentConfig.SyncDir != filepath.Join(tempDir, "sync-new") {
				t.Error("配置同步目录未更新")
			}
		}
	})

	// 测试浏览目录操作
	t.Run("浏览目录操作", func(t *testing.T) {
		_, _, _, _, syncDirEdit, _, _ := setupUIControls(viewModel)

		// 创建初始配置
		uuid, err := model.NewUUID()
		if err != nil {
			t.Fatalf("生成UUID失败: %v", err)
		}
		config := &model.Config{
			UUID:    uuid,
			Name:    "test-config",
			Version: "1.0",
			Host:    "localhost",
			Port:    8080,
			SyncDir: filepath.Join(tempDir, "sync"),
		}
		if err := viewModel.Save(config); err != nil {
			t.Fatalf("保存初始配置失败: %v", err)
		}
		if err := viewModel.LoadConfig(config.UUID); err != nil {
			t.Fatalf("加载配置失败: %v", err)
		}

		newPath := filepath.Join(tempDir, "new-sync")

		// 模拟用户选择目录
		err = viewModel.SetSyncDir(newPath)
		if err != nil {
			t.Errorf("设置同步目录失败: %v", err)
		}

		// 验证目录已更新
		if syncDirEdit.Text() != newPath {
			t.Error("同步目录未更新")
		}
	})

	// 测试启动服务操作
	t.Run("启动服务操作", func(t *testing.T) {
		setupUIControls(viewModel)

		// 创建初始配置
		uuid, err := model.NewUUID()
		if err != nil {
			t.Fatalf("生成UUID失败: %v", err)
		}
		config := &model.Config{
			UUID:    uuid,
			Name:    "test-config",
			Version: "1.0",
			Host:    "localhost",
			Port:    8080,
			SyncDir: filepath.Join(tempDir, "sync"),
		}
		if err := viewModel.Save(config); err != nil {
			t.Fatalf("保存初始配置失败: %v", err)
		}
		if err := viewModel.LoadConfig(config.UUID); err != nil {
			t.Fatalf("加载配置失败: %v", err)
		}

		// 启动服务
		err = viewModel.StartServer()
		if err != nil {
			t.Errorf("启动服务失败: %v", err)
		}

		// 验证服务状态
		if !viewModel.IsServerRunning() {
			t.Error("服务应该处于运行状态")
		}
	})

	// 测试停止服务操作
	t.Run("停止服务操作", func(t *testing.T) {
		setupUIControls(viewModel)

		// 确保服务正在运行
		if !viewModel.IsServerRunning() {
			// 创建并加载有效配置
			uuid, err := model.NewUUID()
			if err != nil {
				t.Fatalf("生成UUID失败: %v", err)
			}
			config := &model.Config{
				UUID:    uuid,
				Name:    "test-config",
				Version: "1.0",
				Host:    "localhost",
				Port:    8080,
				SyncDir: filepath.Join(tempDir, "sync"),
			}
			if err := viewModel.Save(config); err != nil {
				t.Fatalf("保存配置失败: %v", err)
			}
			if err := viewModel.LoadConfig(config.UUID); err != nil {
				t.Fatalf("加载配置失败: %v", err)
			}

			if err := viewModel.StartServer(); err != nil {
				t.Fatalf("启动服务失败: %v", err)
			}
		}

		// 停止服务
		err := viewModel.StopServer()
		if err != nil {
			t.Errorf("停止服务失败: %v", err)
		}

		// 验证服务状态
		if viewModel.IsServerRunning() {
			t.Error("服务应该处于停止状态")
		}
	})

	// 测试错误提示显示
	t.Run("错误提示显示", func(t *testing.T) {
		setupUIControls(viewModel)

		// 创建无效配置
		uuid, err := model.NewUUID()
		if err != nil {
			t.Fatalf("生成UUID失败: %v", err)
		}
		config := &model.Config{
			UUID:    uuid,
			Name:    "", // 无效：名称为空
			Version: "1.0",
			Host:    "localhost",
			Port:    8080,
			SyncDir: filepath.Join(tempDir, "sync"),
		}

		// 尝试保存无效配置
		err = viewModel.Save(config)
		if err == nil {
			t.Error("保存无效配置应该返回错误")
		}

		// 显示错误消息
		mainWindow.MsgBox("错误", err.Error(), walk.MsgBoxIconError)

		// 验证错误提示
		if mainWindow.messageShown == "" {
			t.Error("应该显示错误消息")
		}
		if mainWindow.messageStyle&walk.MsgBoxIconError == 0 {
			t.Error("应该显示错误图标")
		}
	})
}

// TestConfigViewModel_ListModel 测试列表模型
func TestConfigViewModel_ListModel(t *testing.T) {
	// 测试配置列表显示
	t.Run("配置列表显示", func(t *testing.T) {
		viewModel, _, _, tempDir := setupTest(t)
		defer cleanupTest(tempDir)
		setupUIControls(viewModel)

		// 创建测试配置
		configs := []*model.Config{
			{
				UUID:    "test-uuid-1",
				Name:    "test-config-1",
				Version: "1.0",
				Host:    "localhost",
				Port:    8080,
				SyncDir: filepath.Join(tempDir, "sync1"),
			},
			{
				UUID:    "test-uuid-2",
				Name:    "test-config-2",
				Version: "2.0",
				Host:    "localhost",
				Port:    8081,
				SyncDir: filepath.Join(tempDir, "sync2"),
			},
		}

		// 保存配置
		for _, cfg := range configs {
			if err := viewModel.Save(cfg); err != nil {
				t.Fatalf("保存配置失败: %v", err)
			}
		}

		// 获取列表数据
		listModel := viewModel.GetConfigListModel()
		if listModel.RowCount() != len(configs) {
			t.Errorf("配置列表行数不匹配，期望 %d，实际 %d", len(configs), listModel.RowCount())
		}

		// 验证列表内容
		for i, cfg := range configs {
			name := listModel.Value(i, 0)
			version := listModel.Value(i, 1)
			if name != cfg.Name {
				t.Errorf("配置名称不匹配，期望 %s，实际 %s", cfg.Name, name)
			}
			if version != cfg.Version {
				t.Errorf("配置版本不匹配，期望 %s，实际 %s", cfg.Version, version)
			}
		}
	})

	// 测试重定向列表显示
	t.Run("重定向列表显示", func(t *testing.T) {
		viewModel, _, _, tempDir := setupTest(t)
		defer cleanupTest(tempDir)
		setupUIControls(viewModel)

		// 创建测试配置
		config := &model.Config{
			UUID:    "test-uuid",
			Name:    "test-config",
			Version: "1.0",
			Host:    "localhost",
			Port:    8080,
			SyncDir: filepath.Join(tempDir, "sync"),
			FolderRedirects: []model.FolderRedirect{
				{
					ServerPath: "/server/path1",
					ClientPath: "/client/path1",
				},
				{
					ServerPath: "/server/path2",
					ClientPath: "/client/path2",
				},
			},
		}

		// 保存并加载配置
		if err := viewModel.Save(config); err != nil {
			t.Fatalf("保存配置失败: %v", err)
		}
		if err := viewModel.LoadConfig(config.UUID); err != nil {
			t.Fatalf("加载配置失败: %v", err)
		}

		// 获取重定向列表数据
		redirectModel := viewModel.GetRedirectListModel()
		if redirectModel.RowCount() != len(config.FolderRedirects) {
			t.Errorf("重定向列表行数不匹配，期望 %d，实际 %d",
				len(config.FolderRedirects), redirectModel.RowCount())
		}

		// 验证列表内容
		for i, redirect := range config.FolderRedirects {
			serverPath := redirectModel.Value(i, 0)
			clientPath := redirectModel.Value(i, 1)
			if serverPath != redirect.ServerPath {
				t.Errorf("服务器路径不匹配，期望 %s，实际 %s",
					redirect.ServerPath, serverPath)
			}
			if clientPath != redirect.ClientPath {
				t.Errorf("客户端路径不匹配，期望 %s，实际 %s",
					redirect.ClientPath, clientPath)
			}
		}
	})

	// 测试列表更新
	t.Run("列表更新", func(t *testing.T) {
		viewModel, _, _, tempDir := setupTest(t)
		defer cleanupTest(tempDir)
		setupUIControls(viewModel)

		// 添加新配置
		newConfig := &model.Config{
			UUID:    "test-uuid-new",
			Name:    "new-config",
			Version: "1.0",
			Host:    "localhost",
			Port:    8080,
			SyncDir: filepath.Join(tempDir, "sync-new"),
		}
		if err := viewModel.Save(newConfig); err != nil {
			t.Fatalf("保存配置失败: %v", err)
		}

		// 验证列表是否更新
		listModel := viewModel.GetConfigListModel()
		found := false
		for i := 0; i < listModel.RowCount(); i++ {
			if listModel.Value(i, 0) == newConfig.Name {
				found = true
				break
			}
		}
		if !found {
			t.Error("新配置未显示在列表中")
		}
	})

	// 测试选择项变更
	t.Run("选择项变更", func(t *testing.T) {
		viewModel, _, logger, tempDir := setupTest(t)
		defer cleanupTest(tempDir)
		setupUIControls(viewModel)

		// 创建并保存配置
		config := &model.Config{
			UUID:    "test-uuid",
			Name:    "test-config",
			Version: "1.0",
			Host:    "localhost",
			Port:    8080,
			SyncDir: filepath.Join(tempDir, "sync"),
		}
		if err := viewModel.Save(config); err != nil {
			t.Fatalf("保存配置失败: %v", err)
		}

		logger.DebugLog("配置已保存，准备选择配置")

		// 模拟选择配置
		if err := viewModel.OnConfigSelected(0); err != nil {
			t.Errorf("选择配置失败: %v", err)
		}

		logger.DebugLog("配置已选择，准备验证")

		// 验证当前配置
		currentConfig := viewModel.GetCurrentConfig()
		if currentConfig == nil {
			t.Error("当前配置为空")
		} else if currentConfig.UUID != config.UUID {
			t.Errorf("选择的配置不正确，期望 UUID=%s，实际 UUID=%s", config.UUID, currentConfig.UUID)
		}
	})

	// 测试排序和过滤
	t.Run("排序和过滤", func(t *testing.T) {
		viewModel, _, _, tempDir := setupTest(t)
		defer cleanupTest(tempDir)
		setupUIControls(viewModel)

		// 创建测试配置
		configs := []*model.Config{
			{
				UUID:    "test-uuid-1",
				Name:    "config-b",
				Version: "1.0",
				Host:    "localhost",
				Port:    8080,
				SyncDir: filepath.Join(tempDir, "sync1"),
			},
			{
				UUID:    "test-uuid-2",
				Name:    "config-a",
				Version: "2.0",
				Host:    "localhost",
				Port:    8081,
				SyncDir: filepath.Join(tempDir, "sync2"),
			},
		}

		// 保存配置
		for _, cfg := range configs {
			if err := viewModel.Save(cfg); err != nil {
				t.Fatalf("保存配置失败: %v", err)
			}
		}

		// 设置排序
		listModel := viewModel.GetConfigListModel()
		listModel.Sort(0, walk.SortAscending)

		// 验证排序结果
		if listModel.Value(0, 0) != "config-a" {
			t.Error("配置列表排序不正确")
		}

		// 测试过滤
		viewModel.SetFilter("config-a")
		if listModel.RowCount() != 1 {
			t.Error("配置列表过滤不正确")
		}
	})
}
