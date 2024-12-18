package viewmodels

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"

	"synctools/internal/config"
	"synctools/internal/model"
	"synctools/internal/service"
)

// ConfigViewModel 配置视图模型
type ConfigViewModel struct {
	configManager *config.Manager
	syncService   *service.SyncService
	logger        model.Logger

	// UI 组件
	window        *walk.MainWindow
	configTable   *walk.TableView
	configList    *ConfigListModel
	redirectTable *walk.TableView
	redirectList  *RedirectListModel
	statusBar     *walk.StatusBarItem

	// 编辑字段
	nameEdit    *walk.LineEdit
	versionEdit *walk.LineEdit
	hostEdit    *walk.LineEdit
	portEdit    *walk.NumberEdit
	syncDirEdit *walk.LineEdit
	ignoreEdit  *walk.TextEdit
}

// NewConfigViewModel 创建新的配置视图模型
func NewConfigViewModel(configManager *config.Manager, syncService *service.SyncService, logger model.Logger) *ConfigViewModel {
	return &ConfigViewModel{
		configManager: configManager,
		syncService:   syncService,
		logger:        logger,
	}
}

// Initialize 初始化视图模型
func (vm *ConfigViewModel) Initialize(window *walk.MainWindow) error {
	vm.window = window

	// 初始化配置列表模型
	vm.configList = NewConfigListModel(vm.configManager)
	vm.redirectList = NewRedirectListModel(vm.configManager)

	// 设置配置变更回调
	vm.configManager.SetOnChanged(vm.onConfigChanged)

	return nil
}

// SetupUI 设置UI组件
func (vm *ConfigViewModel) SetupUI(
	configTable *walk.TableView,
	redirectTable *walk.TableView,
	statusBar *walk.StatusBarItem,
	nameEdit *walk.LineEdit,
	versionEdit *walk.LineEdit,
	hostEdit *walk.LineEdit,
	portEdit *walk.NumberEdit,
	syncDirEdit *walk.LineEdit,
	ignoreEdit *walk.TextEdit,
) {
	vm.configTable = configTable
	vm.redirectTable = redirectTable
	vm.statusBar = statusBar
	vm.nameEdit = nameEdit
	vm.versionEdit = versionEdit
	vm.hostEdit = hostEdit
	vm.portEdit = portEdit
	vm.syncDirEdit = syncDirEdit
	vm.ignoreEdit = ignoreEdit

	// 更新UI显示
	vm.updateUI()
}

// onConfigChanged 配置变更回调
func (vm *ConfigViewModel) onConfigChanged() {
	vm.updateUI()
}

// updateUI 更新UI显示
func (vm *ConfigViewModel) updateUI() {
	config := vm.configManager.GetCurrentConfig()
	if config == nil {
		return
	}

	// 更新基本信息
	vm.nameEdit.SetText(config.Name)
	vm.versionEdit.SetText(config.Version)
	vm.hostEdit.SetText(config.Host)
	vm.portEdit.SetValue(float64(config.Port))
	vm.syncDirEdit.SetText(config.SyncDir)

	// 更新忽略列表
	vm.ignoreEdit.SetText(strings.Join(config.IgnoreList, "\n"))

	// 刷新表格
	vm.configList.PublishRowsReset()
	vm.redirectList.PublishRowsReset()

	// 更新状态栏
	if vm.syncService.IsRunning() {
		vm.statusBar.SetText("状态: 运行中")
	} else {
		vm.statusBar.SetText("状态: 已停止")
	}
}

// SaveConfig 保存当前配置
func (vm *ConfigViewModel) SaveConfig() error {
	config := vm.configManager.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 更新配置值
	config.Name = vm.nameEdit.Text()
	config.Version = vm.versionEdit.Text()
	config.Host = vm.hostEdit.Text()
	config.Port = int(vm.portEdit.Value())
	config.SyncDir = vm.syncDirEdit.Text()
	config.IgnoreList = strings.Split(vm.ignoreEdit.Text(), "\n")

	// 验证配置
	if err := vm.configManager.ValidateConfig(config); err != nil {
		return err
	}

	// 保存配置
	return vm.configManager.SaveCurrentConfig()
}

// BrowseSyncDir 浏览同步目录
func (vm *ConfigViewModel) BrowseSyncDir() error {
	dlg := walk.FileDialog{
		Title:          "选择同步目录",
		FilePath:       vm.syncDirEdit.Text(),
		InitialDirPath: filepath.Dir(vm.syncDirEdit.Text()),
	}

	if ok, err := dlg.ShowBrowseFolder(vm.window); err != nil {
		return err
	} else if !ok {
		return nil
	}

	vm.syncDirEdit.SetText(dlg.FilePath)
	return nil
}

// StartServer 启动服务器
func (vm *ConfigViewModel) StartServer() error {
	if err := vm.SaveConfig(); err != nil {
		return err
	}

	// 设置进度回调
	vm.syncService.SetProgressCallback(func(progress *service.SyncProgress) {
		// 更新状态栏
		if vm.statusBar != nil {
			vm.statusBar.SetText(progress.Status)
		}

		// 记录日志
		vm.logger.Log("同步进度",
			"total", progress.TotalFiles,
			"processed", progress.ProcessedFiles,
			"current", progress.CurrentFile,
			"status", progress.Status,
		)
	})

	if err := vm.syncService.Start(); err != nil {
		return err
	}

	vm.updateUI()
	return nil
}

// StopServer 停止服务器
func (vm *ConfigViewModel) StopServer() error {
	if err := vm.syncService.Stop(); err != nil {
		return err
	}

	vm.updateUI()
	return nil
}

// ConfigListModel 配置列表模型
type ConfigListModel struct {
	walk.TableModelBase
	configManager *config.Manager
}

// NewConfigListModel 创建新的配置列表模型
func NewConfigListModel(configManager *config.Manager) *ConfigListModel {
	return &ConfigListModel{
		configManager: configManager,
	}
}

// RowCount 返回行数
func (m *ConfigListModel) RowCount() int {
	configs, _ := m.configManager.ListConfigs()
	return len(configs)
}

// Value 返回单元格值
func (m *ConfigListModel) Value(row, col int) interface{} {
	configs, _ := m.configManager.ListConfigs()
	if row < 0 || row >= len(configs) {
		return nil
	}

	config := configs[row]
	switch col {
	case 0:
		return config.Name
	case 1:
		return config.Version
	case 2:
		return config.SyncDir
	}

	return nil
}

// RedirectListModel 重定向列表模型
type RedirectListModel struct {
	walk.TableModelBase
	configManager *config.Manager
}

// NewRedirectListModel 创建新的重定向列表模型
func NewRedirectListModel(configManager *config.Manager) *RedirectListModel {
	return &RedirectListModel{
		configManager: configManager,
	}
}

// RowCount 返回行数
func (m *RedirectListModel) RowCount() int {
	config := m.configManager.GetCurrentConfig()
	if config == nil {
		return 0
	}
	return len(config.FolderRedirects)
}

// Value 返回单元格值
func (m *RedirectListModel) Value(row, col int) interface{} {
	config := m.configManager.GetCurrentConfig()
	if config == nil || row < 0 || row >= len(config.FolderRedirects) {
		return nil
	}

	redirect := config.FolderRedirects[row]
	switch col {
	case 0:
		return redirect.ServerPath
	case 1:
		return redirect.ClientPath
	}

	return nil
}

// Save 保存配置
func (vm *ConfigViewModel) Save(config *model.Config) error {
	return vm.configManager.Save(config)
}

// CreateConfig 创建新的配置
func (vm *ConfigViewModel) CreateConfig(name, version string) error {
	uuid, err := model.NewUUID()
	if err != nil {
		return fmt.Errorf("生成UUID失败: %v", err)
	}

	config := &model.Config{
		UUID:    uuid,
		Name:    name,
		Version: version,
		Host:    "0.0.0.0",
		Port:    6666,
		IgnoreList: []string{
			".clientconfig",
			".DS_Store",
			"thumbs.db",
		},
		FolderRedirects: []model.FolderRedirect{
			{ServerPath: "clientmods", ClientPath: "mods"},
		},
	}

	if err := vm.configManager.ValidateConfig(config); err != nil {
		return err
	}

	if err := vm.configManager.SaveCurrentConfig(); err != nil {
		return err
	}

	if err := vm.configManager.LoadConfig(config.UUID); err != nil {
		return err
	}

	return nil
}

// AddRedirect 添加重定向配置
func (vm *ConfigViewModel) AddRedirect(serverPath, clientPath string) error {
	config := vm.configManager.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 检查路径是否已存在
	for _, redirect := range config.FolderRedirects {
		if redirect.ServerPath == serverPath || redirect.ClientPath == clientPath {
			return fmt.Errorf("路径已存在")
		}
	}

	// 添加新的重定向
	config.FolderRedirects = append(config.FolderRedirects, model.FolderRedirect{
		ServerPath: serverPath,
		ClientPath: clientPath,
	})

	// 保存配置
	if err := vm.configManager.SaveCurrentConfig(); err != nil {
		return err
	}

	vm.updateUI()
	return nil
}

// DeleteRedirect 删除重定向配置
func (vm *ConfigViewModel) DeleteRedirect(index int) error {
	config := vm.configManager.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.FolderRedirects) {
		return fmt.Errorf("无效的索引")
	}

	// 删除重定向
	config.FolderRedirects = append(
		config.FolderRedirects[:index],
		config.FolderRedirects[index+1:]...,
	)

	// 保存配置
	if err := vm.configManager.SaveCurrentConfig(); err != nil {
		return err
	}

	vm.updateUI()
	return nil
}

// ListConfigs 获取配置列表
func (vm *ConfigViewModel) ListConfigs() ([]*model.Config, error) {
	return vm.configManager.ListConfigs()
}

// LoadConfig 加载配置
func (vm *ConfigViewModel) LoadConfig(uuid string) error {
	return vm.configManager.LoadConfig(uuid)
}

// DeleteConfig 删除配置
func (vm *ConfigViewModel) DeleteConfig(uuid string) error {
	return vm.configManager.DeleteConfig(uuid)
}

// GetCurrentConfig 获取当前配置
func (vm *ConfigViewModel) GetCurrentConfig() *model.Config {
	return vm.configManager.GetCurrentConfig()
}
