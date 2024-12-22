/*
文件作用:
- 实现配置界面的视图模型
- 管理配置数据绑定
- 处理配置界面交互
- 提供配置操作接口

主要方法:
- NewConfigViewModel: 创建配置视图模型
- LoadConfig: 加载配置
- SaveConfig: 保存配置
- UpdateUI: 更新界面
- AddSyncFolder: 添加同步文件夹
- RemoveSyncFolder: 移除同步文件夹
*/

package viewmodels

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lxn/walk"

	"synctools/internal/interfaces"
	"synctools/pkg/network"
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

// ViewModelLogger 日志接口
type ViewModelLogger interface {
	Debug(msg string, fields interfaces.Fields)
	Info(msg string, fields interfaces.Fields)
	Warn(msg string, fields interfaces.Fields)
	Error(msg string, fields interfaces.Fields)
	Fatal(msg string, fields interfaces.Fields)
	WithFields(fields interfaces.Fields) interfaces.Logger
	SetLevel(level interfaces.LogLevel)

	// UI特定的日志方法
	Log(format string, v ...interface{})
	DebugLog(format string, v ...interface{})
	SetDebugMode(enabled bool)
	GetDebugMode() bool
}

// ConfigViewModel 配置视图模型
type ConfigViewModel struct {
	syncService interfaces.SyncService
	logger      ViewModelLogger

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
	currentConfig *interfaces.Config

	// 移除进度回调
	lastLoggedProgress int // 上次记录的进度百分比
}

// NewConfigViewModel 创建新的配置视图模型
func NewConfigViewModel(syncService interfaces.SyncService, logger ViewModelLogger) *ConfigViewModel {
	return &ConfigViewModel{
		syncService: syncService,
		logger:      logger,
	}
}

// Initialize 初始化视图模型
func (vm *ConfigViewModel) Initialize() error {
	// 加载默认配置
	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		vm.logger.Info("没有默认配置", interfaces.Fields{})
		return nil
	}

	vm.currentConfig = config
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
	config := vm.GetCurrentConfig()
	if config == nil {
		return
	}

	// 更新基本信息
	vm.nameEdit.SetText(config.Name)
	vm.versionEdit.SetText(config.Version)
	vm.hostEdit.SetText(config.Host)
	vm.portEdit.SetText(strconv.Itoa(config.Port))
	vm.syncDirEdit.SetText(config.SyncDir)

	// 更新忽略列表
	vm.ignoreEdit.SetText(strings.Join(config.IgnoreList, "\n"))
}

// SaveConfig 保存当前配置
func (vm *ConfigViewModel) SaveConfig() error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	vm.logger.Debug("保存配置", interfaces.Fields{
		"name":    vm.nameEdit.Text(),
		"version": vm.versionEdit.Text(),
		"host":    vm.hostEdit.Text(),
		"port":    vm.portEdit.Text(),
		"syncDir": vm.syncDirEdit.Text(),
	})

	// 创建一个新的配置对象，以避免引用问题
	newConfig := &interfaces.Config{
		UUID:            config.UUID,
		Type:            interfaces.ConfigTypeServer, // 设置为服务器配置
		Name:            vm.nameEdit.Text(),
		Version:         vm.versionEdit.Text(),
		Host:            vm.hostEdit.Text(),
		SyncDir:         vm.syncDirEdit.Text(),
		SyncFolders:     make([]interfaces.SyncFolder, len(config.SyncFolders)),
		IgnoreList:      strings.Split(vm.ignoreEdit.Text(), "\n"),
		FolderRedirects: make([]interfaces.FolderRedirect, len(config.FolderRedirects)),
	}

	// 解析端口号
	if port, err := strconv.Atoi(vm.portEdit.Text()); err == nil {
		newConfig.Port = port
	} else {
		vm.logger.Error("解析端口号失败", interfaces.Fields{
			"error": err.Error(),
			"port":  vm.portEdit.Text(),
		})
		return fmt.Errorf("无效的端口号: %v", err)
	}

	// 复制同步文件夹列表
	copy(newConfig.SyncFolders, config.SyncFolders)

	// 复制重定向列表
	copy(newConfig.FolderRedirects, config.FolderRedirects)

	// 验证配置
	if err := vm.syncService.ValidateConfig(newConfig); err != nil {
		vm.logger.Error("配置验证失败", interfaces.Fields{
			"error": err.Error(),
		})
		return fmt.Errorf("配置验证失败: %v", err)
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(newConfig); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err.Error(),
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	vm.logger.Info("配置已保存", interfaces.Fields{
		"uuid": newConfig.UUID,
		"name": newConfig.Name,
	})

	return nil
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

	vm.logger.Debug("启动服务器", interfaces.Fields{
		"config": fmt.Sprintf("%+v", config),
	})

	// 重新创建网络服务器
	server := network.NewServer(config, vm.logger)
	vm.syncService.SetServer(server)

	// 设置进度回调
	vm.syncService.SetProgressCallback(func(progress *interfaces.Progress) {
		// 计算当前进度百分比
		var currentProgress int
		if progress.Total > 0 {
			currentProgress = int(float64(progress.Current) / float64(progress.Total) * 100)
		}

		// 每10%记录一次日志
		if currentProgress/10 > vm.lastLoggedProgress/10 {
			vm.logger.Info("同步进度", interfaces.Fields{
				"progress": currentProgress,
				"status":   progress.Status,
			})
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
	syncService   interfaces.SyncService
	logger        ViewModelLogger
	sortColumn    int
	sortOrder     walk.SortOrder
	filter        string
	cachedConfigs []*interfaces.Config
}

// NewConfigListModel 创建新的配置列表模型
func NewConfigListModel(syncService interfaces.SyncService, logger ViewModelLogger) *ConfigListModel {
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
		m.logger.Error("刷新配置缓存失败", interfaces.Fields{
			"error": err.Error(),
		})
		return
	}

	m.logger.Debug("获取配置列表", interfaces.Fields{
		"count": len(configs),
	})

	for _, cfg := range configs {
		m.logger.Debug("配置信息", interfaces.Fields{
			"uuid": cfg.UUID,
			"name": cfg.Name,
			"type": cfg.Type,
		})
	}

	// 只保留服务器配置
	serverConfigs := make([]*interfaces.Config, 0)
	for _, config := range configs {
		if config.Type == interfaces.ConfigTypeServer {
			serverConfigs = append(serverConfigs, config)
			m.logger.Debug("添加服务器配置", interfaces.Fields{
				"uuid": config.UUID,
				"name": config.Name,
			})
		} else {
			m.logger.Debug("跳过非服务器配置", interfaces.Fields{
				"uuid": config.UUID,
				"name": config.Name,
				"type": config.Type,
			})
		}
	}
	m.cachedConfigs = serverConfigs
	m.logger.Debug("最终配置数量", interfaces.Fields{
		"count": len(serverConfigs),
	})
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
	syncService   interfaces.SyncService
	currentConfig *interfaces.Config
}

// NewRedirectListModel 创建新的重定向列表模型
func NewRedirectListModel(syncService interfaces.SyncService) *RedirectListModel {
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
	syncService   interfaces.SyncService
	currentConfig *interfaces.Config
}

// NewSyncFolderListModel 创建新的同步文件夹列表模型
func NewSyncFolderListModel(syncService interfaces.SyncService) *SyncFolderListModel {
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

// GetCurrentConfig 获取当前配置
func (vm *ConfigViewModel) GetCurrentConfig() *interfaces.Config {
	return vm.currentConfig
}

// UpdateSyncFolder 更新同步文件夹
func (vm *ConfigViewModel) UpdateSyncFolder(index int, path string, mode interfaces.SyncMode) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.SyncFolders) {
		return fmt.Errorf("无效的索引")
	}

	// 更新同步文件夹
	config.SyncFolders[index].Path = path
	config.SyncFolders[index].SyncMode = mode

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err.Error(),
			"path":  path,
			"mode":  mode,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// UpdateRedirect 更新重定向配置
func (vm *ConfigViewModel) UpdateRedirect(index int, serverPath, clientPath string) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.FolderRedirects) {
		return fmt.Errorf("无效的索引")
	}

	// 更新重定向配置
	config.FolderRedirects[index].ServerPath = serverPath
	config.FolderRedirects[index].ClientPath = clientPath

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":      err.Error(),
			"serverPath": serverPath,
			"clientPath": clientPath,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// ListConfigs 获取配置列表
func (vm *ConfigViewModel) ListConfigs() ([]*interfaces.Config, error) {
	return vm.syncService.ListConfigs()
}

// LoadConfig 加载配置
func (vm *ConfigViewModel) LoadConfig(uuid string) error {
	if err := vm.syncService.LoadConfig(uuid); err != nil {
		vm.logger.Error("加载配置失败", interfaces.Fields{
			"error": err.Error(),
			"uuid":  uuid,
		})
		return fmt.Errorf("加载配置失败: %v", err)
	}

	// 获取当前配置
	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("加载配置失败: 配置为空")
	}

	vm.currentConfig = config
	vm.UpdateUI()
	return nil
}

// CreateConfig 创建新配置
func (vm *ConfigViewModel) CreateConfig(name, version string) error {
	// 创建新的配置
	config := &interfaces.Config{
		UUID:            fmt.Sprintf("cfg-%s", name),
		Type:            interfaces.ConfigTypeServer,
		Name:            name,
		Version:         version,
		Host:            "0.0.0.0",
		Port:            8080,
		SyncDir:         filepath.Join(filepath.Dir(os.Args[0]), "sync"),
		SyncFolders:     make([]interfaces.SyncFolder, 0),
		IgnoreList:      make([]string, 0),
		FolderRedirects: make([]interfaces.FolderRedirect, 0),
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":   err.Error(),
			"name":    name,
			"version": version,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// DeleteConfig 删除配置
func (vm *ConfigViewModel) DeleteConfig(uuid string) error {
	return vm.syncService.DeleteConfig(uuid)
}

// AddRedirect 添加重定向配置
func (vm *ConfigViewModel) AddRedirect(serverPath, clientPath string) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 创建新的重定向配置
	redirect := interfaces.FolderRedirect{
		ServerPath: serverPath,
		ClientPath: clientPath,
	}

	// 添加到列表
	config.FolderRedirects = append(config.FolderRedirects, redirect)

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":      err.Error(),
			"serverPath": serverPath,
			"clientPath": clientPath,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

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
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err.Error(),
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	vm.UpdateUI()
	return nil
}

// AddSyncFolder 添加同步文件夹
func (vm *ConfigViewModel) AddSyncFolder(path string, mode interfaces.SyncMode) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 创建新的同步文件夹
	folder := interfaces.SyncFolder{
		Path:     path,
		SyncMode: mode,
	}

	// 添加到列表
	config.SyncFolders = append(config.SyncFolders, folder)

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err.Error(),
			"path":  path,
			"mode":  mode,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

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
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err.Error(),
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	vm.UpdateUI()
	return nil
}
