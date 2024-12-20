package viewmodels

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lxn/walk"

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
	syncService *service.SyncService
	logger      Logger

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
func NewConfigViewModel(syncService *service.SyncService, logger Logger) *ConfigViewModel {
	return &ConfigViewModel{
		syncService: syncService,
		logger:      logger,
	}
}

// Initialize 初始化视图模型
func (vm *ConfigViewModel) Initialize(window *walk.MainWindow) error {
	vm.window = window

	// 初始化配置列表模型
	vm.configList = NewConfigListModel(vm.syncService)
	vm.redirectList = NewRedirectListModel(vm.syncService)
	vm.syncFolderList = NewSyncFolderListModel(vm.syncService)

	// 设置配置变更回调
	vm.syncService.SetOnConfigChanged(vm.UpdateUI)

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

// UpdateUI 更新UI显示
func (vm *ConfigViewModel) UpdateUI() {
	config := vm.syncService.GetCurrentConfig()
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
	config := vm.syncService.GetCurrentConfig()
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
	if err := vm.syncService.ValidateConfig(config); err != nil {
		return err
	}

	// 保存配置
	return vm.syncService.SaveConfig()
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
	syncService *service.SyncService
	sortColumn  int
	sortOrder   walk.SortOrder
	filter      string
}

// NewConfigListModel 创建新的配置列表模型
func NewConfigListModel(syncService *service.SyncService) *ConfigListModel {
	return &ConfigListModel{
		syncService: syncService,
		sortColumn:  -1,
	}
}

// RowCount 返回行数
func (m *ConfigListModel) RowCount() int {
	configs, err := m.syncService.ListConfigs()
	if err != nil {
		// TODO: 考虑添加错误日志记录
		return 0
	}
	return len(configs)
}

// Value 获取单元格值
func (m *ConfigListModel) Value(row, col int) interface{} {
	configs, err := m.syncService.ListConfigs()
	if err != nil {
		// TODO: 考虑添加错误日志记录
		return nil
	}
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

// Sort 排序
func (m *ConfigListModel) Sort(col int, order walk.SortOrder) error {
	m.sortColumn = col
	m.sortOrder = order

	configs, err := m.syncService.ListConfigs()
	if err != nil {
		// TODO: 考虑添加错误日志记录
		return err
	}

	sort.Slice(configs, func(i, j int) bool {
		var less bool
		switch col {
		case 0:
			less = configs[i].Name < configs[j].Name
		case 1:
			less = configs[i].Version < configs[j].Version
		case 2:
			less = configs[i].SyncDir < configs[j].SyncDir
		}

		if order == walk.SortDescending {
			return !less
		}
		return less
	})

	m.PublishRowsReset()
	return nil
}

// RedirectListModel 重定向列表模型
type RedirectListModel struct {
	walk.TableModelBase
	syncService *service.SyncService
}

// NewRedirectListModel 创建新的重定向列表模型
func NewRedirectListModel(syncService *service.SyncService) *RedirectListModel {
	return &RedirectListModel{
		syncService: syncService,
	}
}

// RowCount 返回行数
func (m *RedirectListModel) RowCount() int {
	config := m.syncService.GetCurrentConfig()
	if config == nil {
		return 0
	}
	return len(config.FolderRedirects)
}

// Value 获取单元格值
func (m *RedirectListModel) Value(row, col int) interface{} {
	config := m.syncService.GetCurrentConfig()
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

// SyncFolderListModel 同步文件夹列表模型
type SyncFolderListModel struct {
	walk.TableModelBase
	syncService *service.SyncService
}

// NewSyncFolderListModel 创建新的同步文件夹列表模型
func NewSyncFolderListModel(syncService *service.SyncService) *SyncFolderListModel {
	return &SyncFolderListModel{
		syncService: syncService,
	}
}

// RowCount 返回行数
func (m *SyncFolderListModel) RowCount() int {
	config := m.syncService.GetCurrentConfig()
	if config == nil {
		return 0
	}
	return len(config.SyncFolders)
}

// Value 获取单元格值
func (m *SyncFolderListModel) Value(row, col int) interface{} {
	config := m.syncService.GetCurrentConfig()
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

// GetCurrentConfig 获取当前配置
func (vm *ConfigViewModel) GetCurrentConfig() *model.Config {
	return vm.syncService.GetCurrentConfig()
}

// UpdateSyncFolder 更新同步文件夹
func (vm *ConfigViewModel) UpdateSyncFolder(index int, path, mode string) error {
	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.SyncFolders) {
		return fmt.Errorf("无效的索引")
	}

	// 验证路径
	if path == "" {
		return fmt.Errorf("路径不能为空")
	}

	// 验证模式
	if mode != "mirror" && mode != "push" {
		return fmt.Errorf("无效的同步模式")
	}

	// 更新数据
	config.SyncFolders[index].Path = path
	config.SyncFolders[index].SyncMode = mode

	// 保存配置
	if err := vm.syncService.SaveConfig(); err != nil {
		return err
	}

	// 刷新表格
	vm.syncFolderList.PublishRowsReset()

	return nil
}

// UpdateRedirect 更新文件夹重定向
func (vm *ConfigViewModel) UpdateRedirect(index int, serverPath, clientPath string) error {
	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.FolderRedirects) {
		return fmt.Errorf("无效的索引")
	}

	// 验证路径
	if serverPath == "" || clientPath == "" {
		return fmt.Errorf("路径不能为空")
	}

	// 更新数据
	config.FolderRedirects[index].ServerPath = serverPath
	config.FolderRedirects[index].ClientPath = clientPath

	// 保存配置
	if err := vm.syncService.SaveConfig(); err != nil {
		return err
	}

	// 刷新表格
	vm.redirectList.PublishRowsReset()

	return nil
}

// ListConfigs 获取配置列表
func (vm *ConfigViewModel) ListConfigs() ([]*model.Config, error) {
	return vm.syncService.ListConfigs()
}

// LoadConfig 加载配置
func (vm *ConfigViewModel) LoadConfig(uuid string) error {
	if err := vm.syncService.LoadConfig(uuid); err != nil {
		return err
	}
	vm.UpdateUI()
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

	if err := vm.syncService.Save(config); err != nil {
		return err
	}

	return vm.LoadConfig(config.UUID)
}

// DeleteConfig 删除配置
func (vm *ConfigViewModel) DeleteConfig(uuid string) error {
	return vm.syncService.DeleteConfig(uuid)
}

// AddRedirect 添加重定向配置
func (vm *ConfigViewModel) AddRedirect(serverPath, clientPath string) error {
	config := vm.syncService.GetCurrentConfig()
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
	if err := vm.syncService.SaveConfig(); err != nil {
		return err
	}

	vm.UpdateUI()
	return nil
}

// DeleteRedirect 删除重定向配置
func (vm *ConfigViewModel) DeleteRedirect(index int) error {
	config := vm.syncService.GetCurrentConfig()
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
	if err := vm.syncService.SaveConfig(); err != nil {
		return err
	}

	vm.UpdateUI()
	return nil
}

// AddSyncFolder 添加同步文件夹
func (vm *ConfigViewModel) AddSyncFolder(path, mode string) error {
	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 检查路径是否已存在
	for _, folder := range config.SyncFolders {
		if folder.Path == path {
			return fmt.Errorf("路径已���在")
		}
	}

	// 添加新的同步文件夹
	config.SyncFolders = append(config.SyncFolders, model.SyncFolder{
		Path:     path,
		SyncMode: mode,
	})

	// 保存配置
	if err := vm.syncService.SaveConfig(); err != nil {
		return err
	}

	vm.UpdateUI()
	return nil
}

// DeleteSyncFolder 删除同步文件夹
func (vm *ConfigViewModel) DeleteSyncFolder(index int) error {
	config := vm.syncService.GetCurrentConfig()
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
	if err := vm.syncService.SaveConfig(); err != nil {
		return err
	}

	vm.UpdateUI()
	return nil
}
