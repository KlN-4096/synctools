package viewmodels

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lxn/walk"

	"synctools/internal/config"
	"synctools/internal/model"
	"synctools/internal/service"
)

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

// ConfigViewModel 配置视图模型
type ConfigViewModel struct {
	configManager *config.Manager
	syncService   *service.SyncService
	logger        Logger

	// UI 组件
	window          *walk.MainWindow
	configTable     TableViewIface
	configList      *ConfigListModel
	redirectTable   TableViewIface
	redirectList    *RedirectListModel
	syncFolderTable TableViewIface
	syncFolderList  *SyncFolderListModel
	statusBar       *walk.StatusBarItem

	// 编辑字段
	nameEdit    LineEditIface
	versionEdit LineEditIface
	hostEdit    LineEditIface
	portEdit    LineEditIface
	syncDirEdit LineEditIface
	ignoreEdit  *walk.TextEdit
}

// NewConfigViewModel 创建新的配置视图模型
func NewConfigViewModel(configManager *config.Manager, syncService *service.SyncService, logger Logger) *ConfigViewModel {
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
	vm.syncFolderList = NewSyncFolderListModel(vm.configManager)

	// 设置配置变更回调
	vm.configManager.SetOnChanged(vm.onConfigChanged)

	return nil
}

// SetupUI 设置UI组件
func (vm *ConfigViewModel) SetupUI(
	configTable TableViewIface,
	redirectTable TableViewIface,
	statusBar *walk.StatusBarItem,
	nameEdit LineEditIface,
	versionEdit LineEditIface,
	hostEdit LineEditIface,
	portEdit LineEditIface,
	syncDirEdit LineEditIface,
	ignoreEdit *walk.TextEdit,
	syncFolderTable TableViewIface,
) {
	// 检查必要的 UI 控件
	if nameEdit == nil || versionEdit == nil || hostEdit == nil || portEdit == nil || syncDirEdit == nil {
		panic("必要的 UI 控件不能为空")
	}

	vm.configTable = configTable
	vm.redirectTable = redirectTable
	vm.syncFolderTable = syncFolderTable
	vm.statusBar = statusBar
	vm.nameEdit = nameEdit
	vm.versionEdit = versionEdit
	vm.hostEdit = hostEdit
	vm.portEdit = portEdit
	vm.syncDirEdit = syncDirEdit
	vm.ignoreEdit = ignoreEdit

	// 设置默认值
	hostEdit.SetText("0.0.0.0")
	portEdit.SetText("6666")

	// 设置表格模型
	if configTable != nil {
		configTable.SetModel(vm.configList)
	}
	if redirectTable != nil {
		redirectTable.SetModel(vm.redirectList)
	}
	if syncFolderTable != nil {
		syncFolderTable.SetModel(vm.syncFolderList)
	}

	// 更新UI显示
	vm.UpdateUI()
}

// onConfigChanged 配置变更回调
func (vm *ConfigViewModel) onConfigChanged() {
	vm.UpdateUI()
}

// UpdateUI 更新UI显示
func (vm *ConfigViewModel) UpdateUI() {
	config := vm.configManager.GetCurrentConfig()
	if config == nil {
		// 设置默认值
		vm.nameEdit.SetText("")
		vm.versionEdit.SetText("")
		vm.hostEdit.SetText("0.0.0.0")
		vm.portEdit.SetText("6666")
		vm.syncDirEdit.SetText("")
		if vm.ignoreEdit != nil {
			vm.ignoreEdit.SetText("")
		}
		return
	}

	// 更新基本信息
	vm.nameEdit.SetText(config.Name)
	vm.versionEdit.SetText(config.Version)
	vm.hostEdit.SetText(config.Host)
	vm.portEdit.SetText(fmt.Sprintf("%d", config.Port))
	vm.syncDirEdit.SetText(config.SyncDir)

	// 更新忽略列表
	if vm.ignoreEdit != nil {
		vm.ignoreEdit.SetText(strings.Join(config.IgnoreList, "\n"))
	}

	// 刷新表格
	if vm.configList != nil {
		vm.configList.PublishRowsReset()
	}
	if vm.redirectList != nil {
		vm.redirectList.PublishRowsReset()
	}
	if vm.syncFolderList != nil {
		vm.syncFolderList.PublishRowsReset()
	}

	// 更新状态栏
	if vm.statusBar != nil {
		if vm.syncService.IsRunning() {
			vm.statusBar.SetText("状态: 运行中")
		} else {
			vm.statusBar.SetText("状态: 已停止")
		}
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
	port, err := strconv.Atoi(vm.portEdit.Text())
	if err != nil {
		return fmt.Errorf("端口号无效: %v", err)
	}
	config.Port = port
	config.SyncDir = vm.syncDirEdit.Text()
	if vm.ignoreEdit != nil {
		config.IgnoreList = strings.Split(vm.ignoreEdit.Text(), "\n")
	}

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

	vm.UpdateUI()
	return nil
}

// StopServer 停止服务器
func (vm *ConfigViewModel) StopServer() error {
	if err := vm.syncService.Stop(); err != nil {
		return err
	}

	vm.UpdateUI()
	return nil
}

// IsServerRunning 返回服务器是否正在运行
func (vm *ConfigViewModel) IsServerRunning() bool {
	return vm.syncService.IsRunning()
}

// ConfigListModel 配置列表模型
type ConfigListModel struct {
	walk.TableModelBase
	configManager *config.Manager
	sortColumn    int
	sortOrder     walk.SortOrder
	filter        string
}

// NewConfigListModel 创建新的配置列表模型
func NewConfigListModel(configManager *config.Manager) *ConfigListModel {
	return &ConfigListModel{
		configManager: configManager,
		sortColumn:    -1,
	}
}

// RowCount 返回行数
func (m *ConfigListModel) RowCount() int {
	if m.configManager == nil {
		return 0
	}
	configs, _ := m.configManager.ListConfigs()

	// 应用过滤
	if m.filter != "" {
		filteredConfigs := make([]*model.Config, 0)
		for _, cfg := range configs {
			if strings.Contains(strings.ToLower(cfg.Name), strings.ToLower(m.filter)) {
				filteredConfigs = append(filteredConfigs, cfg)
			}
		}
		configs = filteredConfigs
	}

	return len(configs)
}

// Value 返回单元格值
func (m *ConfigListModel) Value(row, col int) interface{} {
	if m.configManager == nil {
		return nil
	}
	configs, _ := m.configManager.ListConfigs()

	// 应用过滤
	if m.filter != "" {
		filteredConfigs := make([]*model.Config, 0)
		for _, cfg := range configs {
			if strings.Contains(strings.ToLower(cfg.Name), strings.ToLower(m.filter)) {
				filteredConfigs = append(filteredConfigs, cfg)
			}
		}
		configs = filteredConfigs
	}

	if row < 0 || row >= len(configs) {
		return nil
	}

	// 如果需要排序，先对配置列表进行排序
	if m.sortColumn >= 0 {
		sort.Slice(configs, func(i, j int) bool {
			var result bool
			switch m.sortColumn {
			case 0:
				result = configs[i].Name < configs[j].Name
			case 1:
				result = configs[i].Version < configs[j].Version
			case 2:
				result = configs[i].SyncDir < configs[j].SyncDir
			default:
				return false
			}
			if m.sortOrder == walk.SortDescending {
				return !result
			}
			return result
		})
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

// Sort 设置排序
func (m *ConfigListModel) Sort(col int, order walk.SortOrder) error {
	m.sortColumn = col
	m.sortOrder = order
	m.PublishRowsReset()
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
	vm.logger.DebugLog("开始保存配置: UUID=%s, Name=%s", config.UUID, config.Name)

	// 保存配置
	if err := vm.configManager.Save(config); err != nil {
		vm.logger.Error("保存配置失败", "error", err)
		return err
	}
	vm.logger.DebugLog("配置已保存到存储")

	// 加载配置
	if err := vm.configManager.LoadConfig(config.UUID); err != nil {
		vm.logger.Error("加载配置失败", "error", err)
		return fmt.Errorf("加载配置失败: %v", err)
	}
	vm.logger.DebugLog("配置已重新加载")

	// 更新UI显示
	vm.UpdateUI()
	vm.logger.DebugLog("UI已更新")
	return nil
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

	return vm.configManager.Save(config)
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

// SetName 设置名称
func (vm *ConfigViewModel) SetName(name string) {
	vm.nameEdit.SetText(name)
}

// GetName 获取名称
func (vm *ConfigViewModel) GetName() string {
	return vm.nameEdit.Text()
}

// SetVersion 设置版本
func (vm *ConfigViewModel) SetVersion(version string) {
	vm.versionEdit.SetText(version)
}

// GetVersion 获取版本
func (vm *ConfigViewModel) GetVersion() string {
	return vm.versionEdit.Text()
}

// SetHost 设置主机地址
func (vm *ConfigViewModel) SetHost(host string) {
	vm.hostEdit.SetText(host)
}

// GetHost 获取主机地址
func (vm *ConfigViewModel) GetHost() string {
	return vm.hostEdit.Text()
}

// SetPort 设置端口
func (vm *ConfigViewModel) SetPort(port int) {
	vm.portEdit.SetText(fmt.Sprintf("%d", port))
}

// GetPort 获取端口
func (vm *ConfigViewModel) GetPort() int {
	port, err := strconv.Atoi(vm.portEdit.Text())
	if err != nil {
		return 0
	}
	return port
}

// SetSyncDir 设置同步目录
func (vm *ConfigViewModel) SetSyncDir(dir string) error {
	return vm.syncDirEdit.SetText(dir)
}

// GetSyncDir 获取同步目录
func (vm *ConfigViewModel) GetSyncDir() string {
	return vm.syncDirEdit.Text()
}

// OnConfigSelected 处理配置选择事件
func (vm *ConfigViewModel) OnConfigSelected(index int) error {
	vm.logger.DebugLog("开始处理配置选择事件: index=%d", index)

	configs, err := vm.configManager.ListConfigs()
	if err != nil {
		vm.logger.Error("获取配置列表失败", "error", err)
		return err
	}
	vm.logger.DebugLog("获取到配置列表，共 %d 个配置", len(configs))

	// 应用过滤和排序
	if vm.configList != nil {
		if vm.configList.filter != "" {
			vm.logger.DebugLog("应用过滤器: %s", vm.configList.filter)
			filteredConfigs := make([]*model.Config, 0)
			for _, cfg := range configs {
				if strings.Contains(strings.ToLower(cfg.Name), strings.ToLower(vm.configList.filter)) {
					filteredConfigs = append(filteredConfigs, cfg)
				}
			}
			configs = filteredConfigs
			vm.logger.DebugLog("过滤后剩余 %d 个配置", len(configs))
		}

		if vm.configList.sortColumn >= 0 {
			vm.logger.DebugLog("应用排序: column=%d, order=%v", vm.configList.sortColumn, vm.configList.sortOrder)
			sort.Slice(configs, func(i, j int) bool {
				var result bool
				switch vm.configList.sortColumn {
				case 0:
					result = configs[i].Name < configs[j].Name
				case 1:
					result = configs[i].Version < configs[j].Version
				case 2:
					result = configs[i].SyncDir < configs[j].SyncDir
				default:
					return false
				}
				if vm.configList.sortOrder == walk.SortDescending {
					return !result
				}
				return result
			})
		}
	}

	if index < 0 || index >= len(configs) {
		vm.logger.Error("无效的选��索引", "index", index, "total", len(configs))
		return fmt.Errorf("无效的选择索引")
	}

	// 加载选中的配置
	selectedConfig := configs[index]
	vm.logger.DebugLog("选中的配置: UUID=%s, Name=%s", selectedConfig.UUID, selectedConfig.Name)

	// 加载配置
	if err := vm.configManager.LoadConfig(selectedConfig.UUID); err != nil {
		vm.logger.Error("加载配置失败", "error", err)
		return fmt.Errorf("加载配置失败: %v", err)
	}

	// 获取当前配置
	currentConfig := vm.configManager.GetCurrentConfig()
	if currentConfig == nil {
		vm.logger.Error("加载配置后当前配置为空")
		return fmt.Errorf("加载配置后当前配置为空")
	}
	vm.logger.DebugLog("当前配置: UUID=%s, Name=%s", currentConfig.UUID, currentConfig.Name)

	// 更新UI控件
	vm.nameEdit.SetText(currentConfig.Name)
	vm.versionEdit.SetText(currentConfig.Version)
	vm.hostEdit.SetText(currentConfig.Host)
	vm.portEdit.SetText(fmt.Sprintf("%d", currentConfig.Port))
	vm.syncDirEdit.SetText(currentConfig.SyncDir)

	// 更新UI显示
	vm.UpdateUI()
	vm.logger.DebugLog("UI已更新")
	return nil
}

// SetFilter 设置过滤条件
func (vm *ConfigViewModel) SetFilter(filter string) {
	if vm.configList != nil {
		vm.configList.filter = filter
		vm.configList.PublishRowsReset()
	}
}

// GetConfigList 获取配置列表
func (vm *ConfigViewModel) GetConfigList() []*model.Config {
	configs, _ := vm.configManager.ListConfigs()
	return configs
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

	vm.UpdateUI()
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

	vm.UpdateUI()
	return nil
}

// GetConfigListModel 获取配置列表模型
func (vm *ConfigViewModel) GetConfigListModel() *ConfigListModel {
	return vm.configList
}

// GetRedirectListModel 获取重定向列表模型
func (vm *ConfigViewModel) GetRedirectListModel() *RedirectListModel {
	return vm.redirectList
}

// SyncFolderListModel 同步文件夹列表模型
type SyncFolderListModel struct {
	walk.TableModelBase
	configManager *config.Manager
}

// NewSyncFolderListModel 创建新的同步文件夹列表模型
func NewSyncFolderListModel(configManager *config.Manager) *SyncFolderListModel {
	m := &SyncFolderListModel{
		configManager: configManager,
	}
	return m
}

// RowCount 获取行数
func (m *SyncFolderListModel) RowCount() int {
	config := m.configManager.GetCurrentConfig()
	if config == nil {
		return 0
	}
	return len(config.SyncFolders)
}

// ColumnCount 返回列数
func (m *SyncFolderListModel) ColumnCount() int {
	return 3 // 路径、同步模式、是否有效
}

// Value 获取单元格值
func (m *SyncFolderListModel) Value(row, col int) interface{} {
	config := m.configManager.GetCurrentConfig()
	if config == nil || row < 0 || row >= len(config.SyncFolders) {
		return nil
	}

	folder := config.SyncFolders[row]
	switch col {
	case 0:
		return folder.Path
	case 1:
		return folder.SyncMode
	case 2:
		// 检查文件夹是否存在
		if _, err := os.Stat(filepath.Join(config.SyncDir, folder.Path)); os.IsNotExist(err) {
			return "×"
		}
		return "√"
	}
	return nil
}

// AddSyncFolder 添加同步文件夹
func (vm *ConfigViewModel) AddSyncFolder(path, mode string) error {
	config := vm.configManager.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 检查路径是否已存在
	for _, folder := range config.SyncFolders {
		if folder.Path == path {
			return fmt.Errorf("路径已存在")
		}
	}

	// 添加新的同步文件夹
	config.SyncFolders = append(config.SyncFolders, model.SyncFolder{
		Path:     path,
		SyncMode: mode,
	})

	// 保存配置
	if err := vm.configManager.SaveCurrentConfig(); err != nil {
		return err
	}

	vm.UpdateUI()
	return nil
}

// DeleteSyncFolder 删除同步文件夹
func (vm *ConfigViewModel) DeleteSyncFolder(index int) error {
	config := vm.configManager.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.SyncFolders) {
		return fmt.Errorf("无效的索引")
	}

	// 删除同步文件夹
	config.SyncFolders = append(
		config.SyncFolders[:index],
		config.SyncFolders[index+1:]...,
	)

	// 保存配置
	if err := vm.configManager.SaveCurrentConfig(); err != nil {
		return err
	}

	vm.UpdateUI()
	return nil
}

// GetSyncFolderListModel 获取同步文件夹列表模型
func (vm *ConfigViewModel) GetSyncFolderListModel() *SyncFolderListModel {
	return vm.syncFolderList
}

// RefreshSyncFolders 刷新同步文件夹列表
func (vm *ConfigViewModel) RefreshSyncFolders() error {
	if vm.syncFolderList != nil {
		vm.syncFolderList.PublishRowsReset()
	}
	return nil
}
