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

	// UI 状态
	isEditing bool

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

	// 按钮
	startServerButton *walk.PushButton
	saveButton        *walk.PushButton
}

// NewConfigViewModel 创建新的配置视图模型
func NewConfigViewModel(syncService interfaces.SyncService, logger ViewModelLogger) *ConfigViewModel {
	vm := &ConfigViewModel{
		syncService: syncService,
		logger:      logger,
	}

	// 创建列表模型
	vm.configList = NewConfigListModel(syncService, logger)
	vm.redirectList = NewRedirectListModel(syncService, logger)
	vm.syncFolderList = NewSyncFolderListModel(syncService, logger)

	return vm
}

// Initialize 初始化视图模型
func (vm *ConfigViewModel) Initialize() error {
	vm.logger.Debug("开始初始化配置视图模型", nil)

	// 初始化配置列表模型
	vm.configList = NewConfigListModel(vm.syncService, vm.logger)
	vm.redirectList = NewRedirectListModel(vm.syncService, vm.logger)
	vm.syncFolderList = NewSyncFolderListModel(vm.syncService, vm.logger)

	// 加载默认配置
	config := vm.syncService.GetCurrentConfig()
	vm.logger.Debug("获取当前配置", interfaces.Fields{
		"config": config,
	})

	if config == nil {
		vm.logger.Info("没有默认配置", interfaces.Fields{})
		return nil
	}

	// 更新UI
	vm.logger.Debug("开始更新UI", nil)
	vm.UpdateUI()
	vm.logger.Debug("UI更新完成", nil)

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
	startServerButton *walk.PushButton,
	saveButton *walk.PushButton,
) {
	vm.logger.Debug("开始设置UI组件", nil)

	// 检查必要的 UI 控件
	if nameEdit == nil || versionEdit == nil || hostEdit == nil || portEdit == nil || syncDirEdit == nil {
		vm.logger.Error("必要的UI控件为空", nil)
		panic("必要的 UI 控件不能为空")
	}

	// 设置 UI 组件
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
	vm.startServerButton = startServerButton
	vm.saveButton = saveButton

	// 检查服务器初始状态
	if vm.syncService != nil {
		isRunning := vm.syncService.IsRunning()
		vm.logger.Debug("初始化时检查服务器状态", interfaces.Fields{
			"isRunning": isRunning,
		})
	} else {
		vm.logger.Error("同步服务未初始化", nil)
	}

	vm.logger.Debug("开始设置表格模型", nil)
	// 设置表格模型
	if configTable != nil {
		vm.logger.Debug("设置配置列表模型", interfaces.Fields{
			"model": vm.configList != nil,
			"rows":  vm.configList.RowCount(),
		})
		// 先刷新缓存
		vm.configList.refreshCache()
		vm.logger.Debug("刷新缓存后的配置列表", interfaces.Fields{
			"rows": vm.configList.RowCount(),
		})
		// 设置模型
		if err := configTable.SetModel(vm.configList); err != nil {
			vm.logger.Error("设置配置列表模型失败", interfaces.Fields{
				"error": err.Error(),
			})
		}
		// 通知UI更新
		vm.configList.PublishRowsReset()
		vm.logger.Debug("配置列表模型设置完成", nil)
	} else {
		vm.logger.Warn("配置表格为空", nil)
	}

	// 更新UI显示
	vm.UpdateUI()
	vm.logger.Debug("UI组件设置完成", nil)
}

// UpdateUI 更新 UI 显示
func (vm *ConfigViewModel) UpdateUI() {
	vm.logger.Debug("开始更新UI组件", nil)

	config := vm.syncService.GetCurrentConfig()
	vm.logger.Debug("获取当前配置用于更新UI", interfaces.Fields{
		"config": config,
	})

	if config == nil {
		vm.logger.Warn("更新UI时配置为空", nil)
		return
	}

	// 强制所有表格重新加载数据
	if vm.configTable != nil {
		vm.logger.Debug("更新配置表格", interfaces.Fields{
			"before_rows": vm.configList.RowCount(),
		})
		vm.configTable.SetModel(nil)
		vm.configTable.SetModel(vm.configList)
		vm.configList.PublishRowsReset()
		vm.logger.Debug("配置表格更新完成", interfaces.Fields{
			"after_rows": vm.configList.RowCount(),
		})
	} else {
		vm.logger.Warn("配置表格为空", nil)
	}

	// 更新同步文件夹表格
	if vm.syncFolderTable != nil {
		vm.logger.Debug("更新同步文件夹表格", interfaces.Fields{
			"before_rows": vm.syncFolderList.RowCount(),
		})
		vm.syncFolderTable.SetModel(nil)
		vm.syncFolderTable.SetModel(vm.syncFolderList)
		vm.syncFolderList.PublishRowsReset()
		vm.logger.Debug("同步文件夹表格更新完成", interfaces.Fields{
			"after_rows": vm.syncFolderList.RowCount(),
		})
	} else {
		vm.logger.Warn("同步文件夹表格为空", nil)
	}

	// 更新基本信息
	vm.logger.Debug("更新基本信息", interfaces.Fields{
		"name":     config.Name,
		"version":  config.Version,
		"host":     config.Host,
		"port":     config.Port,
		"sync_dir": config.SyncDir,
	})

	vm.nameEdit.SetText(config.Name)
	vm.versionEdit.SetText(config.Version)
	vm.hostEdit.SetText(config.Host)
	vm.portEdit.SetText(strconv.Itoa(config.Port))
	vm.syncDirEdit.SetText(config.SyncDir)
	vm.ignoreEdit.SetText(strings.Join(config.IgnoreList, "\n"))

	// 更新按钮状态
	vm.updateButtonStates()
	vm.logger.Debug("UI组件更新完成", nil)
}

// SaveConfig 处理保存配置的 UI 操作
func (vm *ConfigViewModel) SaveConfig() error {
	// 安全检查
	if vm == nil || vm.syncService == nil {
		return fmt.Errorf("视图模型或同步服务未初始化")
	}

	// 检查是否有选中的配置
	if vm.syncService.GetCurrentConfig() == nil {
		if vm.statusBar != nil {
			vm.setStatus("没有选中的配置")
		}
		return fmt.Errorf("没有选中的配置")
	}

	// 从 UI 收集配置数据
	config := vm.collectConfigFromUI()

	// 调用服务层保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		if vm.statusBar != nil {
			vm.setStatus("保存配置失败")
		}
		if vm.saveButton != nil {
			vm.saveButton.SetEnabled(true)
		}
		return err
	}

	// 更新 UI 状态
	vm.isEditing = false
	if vm.saveButton != nil {
		vm.saveButton.SetEnabled(true)
	}
	if vm.statusBar != nil {
		vm.setStatus("配置已保存")
	}
	return nil
}

// collectConfigFromUI 从 UI 控件收集配置数据
func (vm *ConfigViewModel) collectConfigFromUI() *interfaces.Config {
	// 安全检查
	if vm == nil || vm.syncService == nil {
		return nil
	}

	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		config = &interfaces.Config{}
	}

	// 创建新的配置对象
	newConfig := &interfaces.Config{
		UUID:            config.UUID,
		Type:            interfaces.ConfigTypeServer,
		SyncFolders:     config.SyncFolders,
		FolderRedirects: config.FolderRedirects,
	}

	// 安全地获取 UI 值
	if vm.nameEdit != nil {
		newConfig.Name = vm.nameEdit.Text()
	} else {
		newConfig.Name = config.Name
	}

	if vm.versionEdit != nil {
		newConfig.Version = vm.versionEdit.Text()
	} else {
		newConfig.Version = config.Version
	}

	if vm.hostEdit != nil {
		newConfig.Host = vm.hostEdit.Text()
	} else {
		newConfig.Host = config.Host
	}

	if vm.portEdit != nil {
		newConfig.Port = vm.getPortFromUI()
	} else {
		newConfig.Port = config.Port
	}

	if vm.syncDirEdit != nil {
		newConfig.SyncDir = vm.syncDirEdit.Text()
	} else {
		newConfig.SyncDir = config.SyncDir
	}

	if vm.ignoreEdit != nil {
		newConfig.IgnoreList = vm.getIgnoreListFromUI()
	} else {
		newConfig.IgnoreList = config.IgnoreList
	}

	return newConfig
}

// getPortFromUI 从 UI 获取端口号
func (vm *ConfigViewModel) getPortFromUI() int {
	port, err := strconv.Atoi(vm.portEdit.Text())
	if err != nil {
		vm.logger.Error("解析端口号失败", interfaces.Fields{
			"error": err.Error(),
			"port":  vm.portEdit.Text(),
		})
		return 8080 // 默认端口
	}
	return port
}

// getIgnoreListFromUI 从 UI 获取忽略列表
func (vm *ConfigViewModel) getIgnoreListFromUI() []string {
	text := vm.ignoreEdit.Text()
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// updateButtonStates 更新按钮状态
func (vm *ConfigViewModel) updateButtonStates() {
	// 服务器按钮
	if vm.startServerButton != nil {
		isRunning := vm.syncService.IsRunning()
		vm.logger.Debug("检查服务器运行状态", interfaces.Fields{
			"isRunning": isRunning,
		})

		if isRunning {
			vm.startServerButton.SetText("停止服务器")
			vm.logger.Debug("设置按钮文本为: 停止服务器", nil)
		} else {
			vm.startServerButton.SetText("启动服务器")
			vm.logger.Debug("设置按钮文本为: 启动服务器", nil)
		}
		vm.startServerButton.SetEnabled(true)
	} else {
		vm.logger.Warn("服务器按钮为空", nil)
	}

	// 保存按钮
	if vm.saveButton != nil {
		vm.saveButton.SetEnabled(!vm.isEditing)
	}
}

// setStatus 设置状态栏文本
func (vm *ConfigViewModel) setStatus(status string) {
	if vm == nil || vm.logger == nil {
		return
	}

	if vm.statusBar != nil {
		vm.statusBar.SetText(status)
	}
	vm.logger.Debug("UI状态更新", interfaces.Fields{
		"status": status,
	})
}

// StartServer 处理启动服务器的 UI 操作
func (vm *ConfigViewModel) StartServer() error {
	// 安全检查
	if vm == nil || vm.syncService == nil {
		return fmt.Errorf("视图模型或同步服务未初始化")
	}

	// 检查是否有选中的配置
	if vm.syncService.GetCurrentConfig() == nil {
		vm.setStatus("没有选中的配置")
		return fmt.Errorf("没有选中的配置")
	}

	// 更新 UI 状态
	if vm.startServerButton != nil {
		vm.startServerButton.SetEnabled(false)
	}
	vm.setStatus("正在启动服务器...")

	// 调用服务层启动服务器
	if err := vm.syncService.StartServer(); err != nil {
		vm.setStatus("启动服务器失败")
		if vm.startServerButton != nil {
			vm.startServerButton.SetEnabled(true)
		}
		return err
	}

	// 更新 UI 状态
	if vm.startServerButton != nil {
		vm.startServerButton.SetText("停止服务器")
		vm.startServerButton.SetEnabled(true)
	}
	vm.setStatus("服务器已启动")
	return nil
}

// StopServer 处理停止服务器的 UI 操作
func (vm *ConfigViewModel) StopServer() error {
	// 安全检查
	if vm == nil || vm.syncService == nil {
		return fmt.Errorf("视图模型或同步服务未初始化")
	}

	// 更新 UI 状态
	if vm.startServerButton != nil {
		vm.startServerButton.SetEnabled(false)
	}
	vm.setStatus("正在停止服务器...")

	// 调用服务层停止服务器
	if err := vm.syncService.StopServer(); err != nil {
		vm.setStatus("停止服务器失败")
		if vm.startServerButton != nil {
			vm.startServerButton.SetEnabled(true)
		}
		return err
	}

	// 更新 UI 状态
	if vm.startServerButton != nil {
		vm.startServerButton.SetText("启动服务器")
		vm.startServerButton.SetEnabled(true)
	}
	vm.setStatus("服务器已停止")
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
	vm.isEditing = true
	vm.saveButton.SetEnabled(true)
	return nil
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
	m.logger.Debug("开始刷新配置缓存", nil)
	configs, err := m.syncService.ListConfigs()
	if err != nil {
		m.cachedConfigs = nil
		m.logger.Error("刷新配置缓存失败", interfaces.Fields{
			"error": err.Error(),
		})
		return
	}

	m.logger.Debug("获取配置列表", interfaces.Fields{
		"total_count": len(configs),
	})

	// 只保留服务器配置
	serverConfigs := make([]*interfaces.Config, 0)
	for _, config := range configs {
		if config.Type == interfaces.ConfigTypeServer {
			serverConfigs = append(serverConfigs, config)
			// m.logger.Debug("找到服务器配置", interfaces.Fields{
			// 	"uuid":     config.UUID,
			// 	"name":     config.Name,
			// 	"version":  config.Version,
			// 	"host":     config.Host,
			// 	"port":     config.Port,
			// 	"sync_dir": config.SyncDir,
			// })
		} else {
			// m.logger.Debug("跳过非服务器配置", interfaces.Fields{
			// 	"uuid": config.UUID,
			// 	"name": config.Name,
			// 	"type": config.Type,
			// })
		}
	}
	m.cachedConfigs = serverConfigs
	// m.logger.Debug("服务器配置统计", interfaces.Fields{
	// 	"total_configs":  len(configs),
	// 	"server_configs": len(serverConfigs),
	// })
}

// RowCount 返回行数
func (m *ConfigListModel) RowCount() int {
	count := len(m.cachedConfigs)
	m.logger.Debug("获取行数", interfaces.Fields{
		"count": count,
	})
	return count
}

// ColumnCount 返回列数
func (m *ConfigListModel) ColumnCount() int {
	m.logger.Debug("获取列数", interfaces.Fields{
		"count": 3,
	})
	return 3 // 名称、版本、同步目录
}

// ColumnTitle 返回列标题
func (m *ConfigListModel) ColumnTitle(col int) string {
	var title string
	switch col {
	case 0:
		title = "名称"
	case 1:
		title = "版本"
	case 2:
		title = "同步目录"
	}
	m.logger.Debug("获取列标题", interfaces.Fields{
		"col":   col,
		"title": title,
	})
	return title
}

// Value 获取单元格值
func (m *ConfigListModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.cachedConfigs) {
		m.logger.Debug("Value: 无效的行索引", interfaces.Fields{
			"row":       row,
			"total_row": len(m.cachedConfigs),
		})
		return nil
	}

	config := m.cachedConfigs[row]
	var value interface{}
	switch col {
	case 0:
		value = config.Name
	case 1:
		value = config.Version
	case 2:
		value = config.SyncDir
	default:
		m.logger.Debug("Value: 无效的列索引", interfaces.Fields{
			"col": col,
		})
		return nil
	}

	// m.logger.Debug("获取单元格值", interfaces.Fields{
	// 	"row":      row,
	// 	"col":      col,
	// 	"value":    value,
	// 	"config":   config.UUID,
	// 	"name":     config.Name,
	// 	"version":  config.Version,
	// 	"sync_dir": config.SyncDir,
	// })
	return value
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
	m.logger.Debug("开始刷新配置列表", interfaces.Fields{
		"before_rows": len(m.cachedConfigs),
	})
	m.refreshCache()
	m.TableModelBase.PublishRowsReset()
	m.logger.Debug("配置列表刷新完成", interfaces.Fields{
		"after_rows": len(m.cachedConfigs),
		"configs":    m.cachedConfigs,
	})
}

// RedirectListModel 重定向列表模型
type RedirectListModel struct {
	walk.TableModelBase
	syncService   interfaces.SyncService
	logger        ViewModelLogger
	currentConfig *interfaces.Config
}

// NewRedirectListModel 创建新的重定向列表模型
func NewRedirectListModel(syncService interfaces.SyncService, logger ViewModelLogger) *RedirectListModel {
	return &RedirectListModel{
		syncService: syncService,
		logger:      logger,
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
	logger        ViewModelLogger
	currentConfig *interfaces.Config
}

// NewSyncFolderListModel 创建新的同步文件夹列表模型
func NewSyncFolderListModel(syncService interfaces.SyncService, logger ViewModelLogger) *SyncFolderListModel {
	return &SyncFolderListModel{
		syncService: syncService,
		logger:      logger,
	}
}

// refreshCache 刷新缓��
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

// ColumnCount 返回列数
func (m *SyncFolderListModel) ColumnCount() int {
	return 4 // 文件夹名称、同步模式、重定向路径、是否有效
}

// ColumnTitle 返回列标题
func (m *SyncFolderListModel) ColumnTitle(col int) string {
	switch col {
	case 0:
		return "文件夹名称"
	case 1:
		return "同步模式"
	case 2:
		return "重定向路径"
	case 3:
		return "是否有效"
	}
	return ""
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
		// 查找对应的重定向配置
		for _, redirect := range m.currentConfig.FolderRedirects {
			if redirect.ServerPath == folder.Path {
				return redirect.ClientPath
			}
		}
		return ""
	case 3:
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
	return vm.syncService.GetCurrentConfig()
}

// UpdateSyncFolder 更新同步文件夹
func (vm *ConfigViewModel) UpdateSyncFolder(index int, path string, mode interfaces.SyncMode, redirectPath string) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.SyncFolders) {
		return fmt.Errorf("无效的索引")
	}

	oldPath := config.SyncFolders[index].Path

	// 更新同步文件夹
	config.SyncFolders[index].Path = path
	config.SyncFolders[index].SyncMode = mode

	// 更新或添加重定向配置
	redirectFound := false
	for i, redirect := range config.FolderRedirects {
		if redirect.ServerPath == oldPath {
			if redirectPath != "" {
				config.FolderRedirects[i].ServerPath = path
				config.FolderRedirects[i].ClientPath = redirectPath
			} else {
				// 如果新的重定向路径为空，删除旧的重定向配置
				config.FolderRedirects = append(config.FolderRedirects[:i], config.FolderRedirects[i+1:]...)
			}
			redirectFound = true
			break
		}
	}

	// 如果没有找到旧的重定向配置，但有新的重定向路径，则添加新的重定向配置
	if !redirectFound && redirectPath != "" {
		redirect := interfaces.FolderRedirect{
			ServerPath: path,
			ClientPath: redirectPath,
		}
		config.FolderRedirects = append(config.FolderRedirects, redirect)
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":        err.Error(),
			"path":         path,
			"mode":         mode,
			"redirectPath": redirectPath,
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
func (vm *ConfigViewModel) AddSyncFolder(path string, mode interfaces.SyncMode, redirectPath string) error {
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

	// 如果有重定向路径，添加重定向配置
	if redirectPath != "" {
		redirect := interfaces.FolderRedirect{
			ServerPath: path,
			ClientPath: redirectPath,
		}
		config.FolderRedirects = append(config.FolderRedirects, redirect)
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":        err.Error(),
			"path":         path,
			"mode":         mode,
			"redirectPath": redirectPath,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// DeleteSyncFolder 删除同步文件夹
func (vm *ConfigViewModel) DeleteSyncFolder(index int) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.SyncFolders) {
		return fmt.Errorf("无效的索引")
	}

	// 获取要删除的文件夹路径
	path := config.SyncFolders[index].Path

	// 删除同步文件夹
	config.SyncFolders = append(
		config.SyncFolders[:index],
		config.SyncFolders[index+1:]...,
	)

	// 删除对应的重定向配置
	for i, redirect := range config.FolderRedirects {
		if redirect.ServerPath == path {
			config.FolderRedirects = append(
				config.FolderRedirects[:i],
				config.FolderRedirects[i+1:]...,
			)
			break
		}
	}

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

// IsServerRunning 返回服务器是否正在运行
func (vm *ConfigViewModel) IsServerRunning() bool {
	// 安全检查
	if vm == nil || vm.syncService == nil {
		return false
	}
	return vm.syncService.IsRunning()
}
