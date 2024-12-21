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
	"synctools/internal/network"
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

	// 缓存
	currentConfig *model.Config

	// 移除进度回调
	lastLoggedProgress int // 上次记录的进度百分比
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
	vm.configList = NewConfigListModel(vm.syncService, vm.logger)
	vm.redirectList = NewRedirectListModel(vm.syncService)
	vm.syncFolderList = NewSyncFolderListModel(vm.syncService)

	// 设置配置变更回调
	vm.syncService.SetOnConfigChanged(func() {
		vm.logger.DebugLog("触发配置变更回调")
		// 清除缓存
		vm.currentConfig = nil
		// 更新所有列表模型的缓存
		vm.configList.refreshCache()
		vm.redirectList.refreshCache()
		vm.syncFolderList.refreshCache()
		// 更新UI
		vm.UpdateUI()
	})

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
	// 获取当前配置（使用缓存）
	config := vm.GetCurrentConfig()

	// 强制所有表格重新加载数据
	if vm.configTable != nil {
		vm.configTable.SetModel(nil)
		vm.configTable.SetModel(vm.configList)
	}
	if vm.redirectTable != nil {
		vm.redirectTable.SetModel(nil)
		vm.redirectTable.SetModel(vm.redirectList)
	}
	if vm.syncFolderTable != nil {
		vm.syncFolderTable.SetModel(nil)
		vm.syncFolderTable.SetModel(vm.syncFolderList)
	}

	// 更新UI组件
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
	} else {
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
	}

	// 刷新表格（使用缓存的配置）
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
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	vm.logger.DebugLog("SaveConfig - UI values: name=%s, version=%s, host=%s, port=%s, syncDir=%s",
		vm.nameEdit.Text(), vm.versionEdit.Text(), vm.hostEdit.Text(), vm.portEdit.Text(), vm.syncDirEdit.Text())

	// 创建一个新的配置对象，以避免引用问题
	newConfig := &model.Config{
		UUID:            config.UUID,
		Type:            model.ConfigTypeServer, // 设置为服务器配置
		Name:            vm.nameEdit.Text(),
		Version:         vm.versionEdit.Text(),
		Host:            vm.hostEdit.Text(),
		SyncDir:         vm.syncDirEdit.Text(),
		SyncFolders:     make([]model.SyncFolder, len(config.SyncFolders)),
		IgnoreList:      make([]string, 0),
		FolderRedirects: make([]model.FolderRedirect, len(config.FolderRedirects)),
	}

	// 解析端口号
	port, err := strconv.Atoi(vm.portEdit.Text())
	if err != nil {
		return fmt.Errorf("端口号无效: %v", err)
	}
	newConfig.Port = port

	vm.logger.DebugLog("SaveConfig - New config: %+v", newConfig)

	// 复制切片内容
	copy(newConfig.SyncFolders, config.SyncFolders)
	copy(newConfig.FolderRedirects, config.FolderRedirects)

	// 处理忽略列表
	if vm.ignoreEdit != nil {
		ignoreList := strings.Split(vm.ignoreEdit.Text(), "\n")
		newConfig.IgnoreList = make([]string, len(ignoreList))
		copy(newConfig.IgnoreList, ignoreList)
	}

	// 验证配置
	if err := vm.syncService.ValidateConfig(newConfig); err != nil {
		return err
	}

	// 清除缓存
	vm.currentConfig = nil
	return vm.syncService.Save(newConfig)
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
	// 先保存当前配置
	if err := vm.SaveConfig(); err != nil {
		return err
	}

	// 确保服务器已初始化
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 重新加载配置以确保使用最新的设置
	if err := vm.syncService.LoadConfig(config.UUID); err != nil {
		return fmt.Errorf("重新加载配置失败: %v", err)
	}

	// 获取最新的配置
	config = vm.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("无法获取配置")
	}

	vm.logger.DebugLog("启动服务器 - 使用配置: %+v", config)

	// 重新创建网络服务器
	server := network.NewServer(config, vm.logger)
	vm.syncService.SetServer(server)

	// 设置进度回调
	vm.syncService.SetProgressCallback(func(progress *service.SyncProgress) {
		// 计算当前进度百分比
		var currentProgress int
		if progress.PackMode {
			if progress.BytesTotal > 0 {
				currentProgress = int(float64(progress.BytesProcessed) / float64(progress.BytesTotal) * 100)
			}
		} else {
			if progress.TotalFiles > 0 {
				currentProgress = int(float64(progress.ProcessedFiles) / float64(progress.TotalFiles) * 100)
			}
		}

		// 每10%记录一次日志
		if currentProgress/10 > vm.lastLoggedProgress/10 {
			vm.logger.Log("同步进度: %d%% - %s", currentProgress, progress.Status)
			vm.lastLoggedProgress = currentProgress
		}

		// 更新状态栏
		if vm.statusBar != nil {
			vm.statusBar.SetText(progress.Status)
		}
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

	// 重置进度记录
	vm.lastLoggedProgress = 0
	vm.logger.Log("服务已停止")

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
	syncService   *service.SyncService
	logger        Logger
	sortColumn    int
	sortOrder     walk.SortOrder
	filter        string
	cachedConfigs []*model.Config
}

// NewConfigListModel 创建新的配置列表模型
func NewConfigListModel(syncService *service.SyncService, logger Logger) *ConfigListModel {
	model := &ConfigListModel{
		syncService: syncService,
		logger:      logger,
		sortColumn:  -1,
	}
	// 初始加载配置
	model.refreshCache()
	return model
}

// refreshCache 刷新配置缓存
func (m *ConfigListModel) refreshCache() {
	configs, err := m.syncService.ListConfigs()
	if err != nil {
		m.cachedConfigs = nil
		m.logger.Error("刷新配置缓存失败: %v", err)
		return
	}

	m.logger.DebugLog("获取到 %d 个配置", len(configs))
	for _, cfg := range configs {
		m.logger.DebugLog("配置: UUID=%s, Name=%s, Type=%s", cfg.UUID, cfg.Name, cfg.Type)
	}

	// 只保留服务器配置
	serverConfigs := make([]*model.Config, 0)
	for _, config := range configs {
		if config.Type == model.ConfigTypeServer {
			serverConfigs = append(serverConfigs, config)
			m.logger.DebugLog("添加服务器配置: UUID=%s, Name=%s", config.UUID, config.Name)
		} else {
			m.logger.DebugLog("跳过非服务器配置: UUID=%s, Name=%s, Type=%s", config.UUID, config.Name, config.Type)
		}
	}
	m.cachedConfigs = serverConfigs
	m.logger.DebugLog("最终保留 %d 个服务器配置", len(serverConfigs))
}

// RowCount 返回行数
func (m *ConfigListModel) RowCount() int {
	return len(m.cachedConfigs)
}

// Value 获取单元格值
func (m *ConfigListModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.cachedConfigs) {
		m.logger.DebugLog("Value: 无效的行索引 %d (总行数: %d)", row, len(m.cachedConfigs))
		return nil
	}

	config := m.cachedConfigs[row]

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

	sort.Slice(m.cachedConfigs, func(i, j int) bool {
		var less bool
		switch col {
		case 0:
			less = m.cachedConfigs[i].Name < m.cachedConfigs[j].Name
		case 1:
			less = m.cachedConfigs[i].Version < m.cachedConfigs[j].Version
		case 2:
			less = m.cachedConfigs[i].SyncDir < m.cachedConfigs[j].SyncDir
		}

		if order == walk.SortDescending {
			return !less
		}
		return less
	})

	m.PublishRowsReset()
	return nil
}

// PublishRowsReset 重置行并刷新缓存
func (m *ConfigListModel) PublishRowsReset() {
	m.logger.DebugLog("开始刷新配置列表")
	m.refreshCache()
	m.TableModelBase.PublishRowsReset()
	m.logger.DebugLog("配置列表刷新完成，共 %d 个配置", len(m.cachedConfigs))
}

// RedirectListModel 重定向列表模型
type RedirectListModel struct {
	walk.TableModelBase
	syncService   *service.SyncService
	currentConfig *model.Config
}

// NewRedirectListModel 创建新的重定向列表模型
func NewRedirectListModel(syncService *service.SyncService) *RedirectListModel {
	return &RedirectListModel{
		syncService: syncService,
	}
}

// refreshCache 刷新缓存
func (m *RedirectListModel) refreshCache() {
	m.currentConfig = m.syncService.GetCurrentConfig()
}

// RowCount 返回行数
func (m *RedirectListModel) RowCount() int {
	if m.currentConfig == nil {
		m.refreshCache()
	}
	if m.currentConfig == nil {
		return 0
	}
	return len(m.currentConfig.FolderRedirects)
}

// Value 获取单元格值
func (m *RedirectListModel) Value(row, col int) interface{} {
	if m.currentConfig == nil {
		m.refreshCache()
	}
	if m.currentConfig == nil || row < 0 || row >= len(m.currentConfig.FolderRedirects) {
		return nil
	}

	redirect := m.currentConfig.FolderRedirects[row]
	switch col {
	case 0:
		return redirect.ServerPath
	case 1:
		return redirect.ClientPath
	}
	return nil
}

// PublishRowsReset 重置行并刷新缓存
func (m *RedirectListModel) PublishRowsReset() {
	m.refreshCache()
	m.TableModelBase.PublishRowsReset()
}

// SyncFolderListModel 同步文件夹列表模型
type SyncFolderListModel struct {
	walk.TableModelBase
	syncService   *service.SyncService
	currentConfig *model.Config
}

// NewSyncFolderListModel 创建新的同步文件夹列表模型
func NewSyncFolderListModel(syncService *service.SyncService) *SyncFolderListModel {
	return &SyncFolderListModel{
		syncService: syncService,
	}
}

// refreshCache 刷新缓存
func (m *SyncFolderListModel) refreshCache() {
	m.currentConfig = m.syncService.GetCurrentConfig()
}

// RowCount 返回行数
func (m *SyncFolderListModel) RowCount() int {
	if m.currentConfig == nil {
		m.refreshCache()
	}
	if m.currentConfig == nil {
		return 0
	}
	return len(m.currentConfig.SyncFolders)
}

// Value 获取单元格值
func (m *SyncFolderListModel) Value(row, col int) interface{} {
	if m.currentConfig == nil {
		m.refreshCache()
	}
	if m.currentConfig == nil || row < 0 || row >= len(m.currentConfig.SyncFolders) {
		return nil
	}

	folder := m.currentConfig.SyncFolders[row]
	switch col {
	case 0:
		return folder.Path
	case 1:
		return folder.SyncMode
	case 2:
		// 检查文件夹是否存在
		if _, err := os.Stat(filepath.Join(m.currentConfig.SyncDir, folder.Path)); os.IsNotExist(err) {
			return "×"
		}
		return "√"
	}
	return nil
}

// PublishRowsReset 重置行并刷新缓存
func (m *SyncFolderListModel) PublishRowsReset() {
	m.refreshCache()
	m.TableModelBase.PublishRowsReset()
}

// GetCurrentConfig 获取当前配置（使用缓存）
func (vm *ConfigViewModel) GetCurrentConfig() *model.Config {
	if vm.currentConfig == nil {
		// 只在缓存为空时记录日志
		vm.currentConfig = vm.syncService.GetCurrentConfig()
		if vm.currentConfig == nil {
			vm.logger.DebugLog("当前没有选中的配置")
		}
	}
	return vm.currentConfig
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
	// 先清除缓存，确保加载新配置
	vm.currentConfig = nil

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
		Type:    model.ConfigTypeServer, // 设置为服务器配置
		IgnoreList: []string{
			".clientconfig",
			".DS_Store",
			"thumbs.db",
		},
		FolderRedirects: []model.FolderRedirect{
			{ServerPath: "clientmods", ClientPath: "mods"},
		},
		SyncFolders: []model.SyncFolder{
			{
				Path:     "mods",
				SyncMode: model.SyncModePack,
			},
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
			return fmt.Errorf("路径已存在")
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
