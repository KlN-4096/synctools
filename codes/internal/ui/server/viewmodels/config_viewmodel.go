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
	"strconv"
	"strings"
	"time"

	"github.com/lxn/walk"

	"synctools/codes/internal/interfaces"
	"synctools/codes/internal/ui/shared"
)

// ConfigViewModel 配置视图模型
type ConfigViewModel struct {
	syncService interfaces.ServerSyncService
	logger      interfaces.Logger

	// UI 状态
	isEditing     bool
	serverRunning bool // 服务器运行状态标志

	// UI 组件
	window          *walk.MainWindow
	configTable     interfaces.TableViewIface
	configList      *shared.TableModel
	redirectTable   interfaces.TableViewIface
	syncFolderTable interfaces.TableViewIface
	syncFolderList  *shared.TableModel
	statusBar       *walk.StatusBarItem

	// 编辑字段
	nameEdit    interfaces.LineEditIface
	versionEdit interfaces.LineEditIface
	hostEdit    interfaces.LineEditIface
	portEdit    interfaces.LineEditIface
	syncDirEdit interfaces.LineEditIface
	ignoreEdit  *walk.TextEdit

	// 按钮
	browseSyncDirButton *walk.PushButton
	startServerButton   *walk.PushButton
	saveButton          *walk.PushButton
	newConfigButton     *walk.PushButton
	delConfigButton     *walk.PushButton
	addSyncFolderButton *walk.PushButton
	delSyncFolderButton *walk.PushButton
}

// NewConfigViewModel 创建新的配置视图模型
func NewConfigViewModel(syncService interfaces.ServerSyncService, logger interfaces.Logger) *ConfigViewModel {
	vm := &ConfigViewModel{
		syncService: syncService,
		logger:      logger,
	}

	// 创建配置列表模型
	vm.configList = shared.NewTableModel([]shared.TableColumn{
		{
			Title: "名称",
			Width: 150,
			Value: func(row interface{}) interface{} {
				if config, ok := row.(*interfaces.Config); ok {
					return config.Name
				}
				return nil
			},
		},
		{
			Title: "版本",
			Width: 100,
			Value: func(row interface{}) interface{} {
				if config, ok := row.(*interfaces.Config); ok {
					return config.Version
				}
				return nil
			},
		},
		{
			Title: "同步目录",
			Width: 200,
			Value: func(row interface{}) interface{} {
				if config, ok := row.(*interfaces.Config); ok {
					return config.SyncDir
				}
				return nil
			},
		},
	}, syncService, logger)

	// 设置配置列表数据源
	vm.configList.SetDataSource(func() []interface{} {
		configs, err := vm.syncService.ListConfigs()
		if err != nil {
			vm.logger.Error("获取配置列表失败", interfaces.Fields{
				"error": err.Error(),
			})
			return nil
		}

		// 只保留服务器配置
		serverConfigs := make([]interface{}, 0)
		for _, config := range configs {
			if config.Type == interfaces.ConfigTypeServer {
				serverConfigs = append(serverConfigs, config)
			}
		}
		return serverConfigs
	})

	// 设置配置列表排序函数
	vm.configList.SetCompareSource(func(i, j int) bool {
		rows := vm.configList.GetRows()
		if rows == nil {
			return false
		}
		a, ok1 := rows[i].(*interfaces.Config)
		b, ok2 := rows[j].(*interfaces.Config)
		if !ok1 || !ok2 {
			return false
		}

		col, order := vm.configList.GetSortInfo()
		var less bool
		switch col {
		case 0: // 名称
			less = a.Name < b.Name
		case 1: // 版本
			less = a.Version < b.Version
		case 2: // 同步目录
			less = a.SyncDir < b.SyncDir
		default:
			return false
		}

		if order == walk.SortDescending {
			return !less
		}
		return less
	})

	// 创建同步文件夹列表模型
	vm.syncFolderList = shared.NewTableModel([]shared.TableColumn{
		{
			Title: "文件夹名称",
			Width: 150,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					return folder.Path
				}
				return nil
			},
		},
		{
			Title: "同步模式",
			Width: 100,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					return string(folder.SyncMode)
				}
				return nil
			},
		},
		{
			Title: "重定向路径",
			Width: 200,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					config := vm.GetCurrentConfig()
					if config != nil {
						for _, redirect := range config.FolderRedirects {
							if redirect.ServerPath == folder.Path {
								return redirect.ClientPath
							}
						}
					}
				}
				return ""
			},
		},
		{
			Title: "是否有效",
			Width: 80,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					config := vm.GetCurrentConfig()
					if config != nil {
						if _, err := os.Stat(filepath.Join(config.SyncDir, folder.Path)); os.IsNotExist(err) {
							return "×"
						}
						return "√"
					}
				}
				return "?"
			},
		},
	}, syncService, logger)

	// 设置同步文件夹列表数据源
	vm.syncFolderList.SetDataSource(func() []interface{} {
		config := vm.GetCurrentConfig()
		if config == nil {
			return nil
		}
		rows := make([]interface{}, len(config.SyncFolders))
		for i := range config.SyncFolders {
			rows[i] = &config.SyncFolders[i]
		}
		return rows
	})

	return vm
}

// Initialize 初始化视图模型
func (vm *ConfigViewModel) Initialize() error {
	vm.logger.Info("视图操作", interfaces.Fields{
		"action": "initialize",
		"type":   "config",
	})

	// 获取当前配置
	cfg := vm.syncService.GetCurrentConfig()
	if cfg == nil {
		vm.logger.Info("配置状态", interfaces.Fields{
			"status": "empty",
			"reason": "no_default",
		})
	}

	// 更新UI
	vm.UpdateUI()
	return nil
}

// SetupUI 设置UI组件
func (vm *ConfigViewModel) SetupUI(
	configTable interfaces.TableViewIface,
	redirectTable interfaces.TableViewIface,
	statusBar *walk.StatusBarItem,
	nameEdit interfaces.LineEditIface,
	versionEdit interfaces.LineEditIface,
	hostEdit interfaces.LineEditIface,
	portEdit interfaces.LineEditIface,
	browseSyncDirButton *walk.PushButton,
	syncDirEdit interfaces.LineEditIface,
	ignoreEdit *walk.TextEdit,
	syncFolderTable interfaces.TableViewIface,
	startServerButton *walk.PushButton,
	saveButton *walk.PushButton,
	newConfigButton *walk.PushButton,
	delConfigButton *walk.PushButton,
	addSyncFolderButton *walk.PushButton,
	delSyncFolderButton *walk.PushButton,
) {
	vm.logger.Info("视图操作", interfaces.Fields{
		"action": "setup",
		"type":   "config",
	})

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
	vm.browseSyncDirButton = browseSyncDirButton
	vm.syncDirEdit = syncDirEdit
	vm.ignoreEdit = ignoreEdit
	vm.startServerButton = startServerButton
	vm.saveButton = saveButton
	vm.newConfigButton = newConfigButton
	vm.delConfigButton = delConfigButton
	vm.addSyncFolderButton = addSyncFolderButton
	vm.delSyncFolderButton = delSyncFolderButton
	vm.window = startServerButton.Form().(*walk.MainWindow)

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

// setControlsEnabled 设置一组控件的启用状态
func (vm *ConfigViewModel) setControlsEnabled(enabled bool, controls ...interface{}) {
	for _, control := range controls {
		if control == nil {
			continue
		}
		if setter, ok := control.(interfaces.EnabledSetter); ok {
			setter.SetEnabled(enabled)
			if btn, ok := control.(*walk.PushButton); ok {
				vm.logger.Debug("设置控件状态", interfaces.Fields{"enabled": enabled, "type": "Button", "text": btn.Text()})
			} else {
				vm.logger.Debug("设置控件状态", interfaces.Fields{"enabled": enabled})
			}
		}
	}
}

// UpdateUI 更新 UI 显示
func (vm *ConfigViewModel) UpdateUI() {
	vm.logger.Info("视图操作", interfaces.Fields{
		"action": "update",
		"type":   "config",
	})

	// 获取当前配置
	cfg := vm.syncService.GetCurrentConfig()
	if cfg == nil {
		vm.logger.Warn("UI更新", interfaces.Fields{
			"status": "empty_config",
		})
		return
	}

	// 根据服务器运行状态设置编辑控件的启用状态
	editEnabled := !vm.serverRunning
	vm.setControlsEnabled(editEnabled,
		vm.nameEdit,
		vm.versionEdit,
		vm.hostEdit,
		vm.portEdit,
		vm.browseSyncDirButton,
		vm.syncDirEdit,
		vm.ignoreEdit,
		vm.configTable,
		vm.redirectTable,
		vm.syncFolderTable,
		vm.saveButton,
		vm.newConfigButton,
		vm.delConfigButton,
		vm.addSyncFolderButton,
		vm.delSyncFolderButton,
	)

	// 更新配置表格
	if vm.configTable != nil {
		vm.configList.RefreshCache()
		vm.configTable.SetModel(nil)
		vm.configTable.SetModel(vm.configList)
	} else {
		vm.logger.Warn("UI状态", interfaces.Fields{
			"component": "config_table",
			"status":    "empty",
		})
	}

	// 更新同步文件夹表格
	if vm.syncFolderTable != nil {
		vm.syncFolderList.RefreshCache()
		vm.syncFolderTable.SetModel(nil)
		vm.syncFolderTable.SetModel(vm.syncFolderList)
	} else {
		vm.logger.Warn("UI状态", interfaces.Fields{
			"component": "sync_folder_table",
			"status":    "empty",
		})
	}

	// 更新基本信息
	vm.logger.Debug("更新基本信息", interfaces.Fields{
		"name":     cfg.Name,
		"version":  cfg.Version,
		"host":     cfg.Host,
		"port":     cfg.Port,
		"sync_dir": cfg.SyncDir,
	})

	vm.nameEdit.SetText(cfg.Name)
	vm.versionEdit.SetText(cfg.Version)
	vm.hostEdit.SetText(cfg.Host)
	vm.portEdit.SetText(strconv.Itoa(cfg.Port))
	vm.syncDirEdit.SetText(cfg.SyncDir)
	vm.ignoreEdit.SetText(strings.Join(cfg.IgnoreList, "\n"))

	// 更新按钮状态
	vm.updateButtonStates()

	// 更新状态栏
	if vm.statusBar != nil {
		if vm.serverRunning {
			vm.setStatus("服务器运行中")
		} else {
			vm.setStatus("服务器已停止")
		}
	}

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
	if vm.startServerButton == nil {
		return
	}

	vm.logger.Debug("更新服务器按钮状态", interfaces.Fields{
		"isRunning": vm.serverRunning,
	})

	// 在UI线程中更新按钮状态
	if vm.window != nil {
		vm.window.Synchronize(func() {
			// 再次检查服务器状态
			isRunning := vm.syncService.IsRunning()

			if isRunning {
				vm.startServerButton.SetText("停止服务器")
			} else {
				vm.startServerButton.SetText("启动服务器")
			}
			vm.startServerButton.SetEnabled(true)

			// 更新内部状态
			vm.serverRunning = isRunning
		})
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
	vm.logger.Info("服务器操作", interfaces.Fields{
		"action": "start",
	})

	if err := vm.syncService.StartServer(); err != nil {
		vm.setStatus("启动服务器失败")
		vm.serverRunning = false
		vm.updateButtonStates()
		return err
	}

	// 等待服务器完全启动
	time.Sleep(100 * time.Millisecond)

	// 检查服务器状态
	if vm.syncService.GetNetworkServer() == nil || !vm.syncService.GetNetworkServer().IsRunning() {
		vm.setStatus("服务器启动失败")
		vm.serverRunning = false
		vm.updateButtonStates()
		return fmt.Errorf("服务器启动失败")
	}

	vm.serverRunning = true
	vm.setStatus("服务器已启动")
	vm.updateButtonStates()
	return nil
}

// StopServer 处理停止服务器的 UI 操作
func (vm *ConfigViewModel) StopServer() error {
	vm.logger.Info("服务器操作", interfaces.Fields{
		"action": "stop",
	})

	if err := vm.syncService.StopServer(); err != nil {
		vm.setStatus("停止服务器失败")
		vm.updateButtonStates()
		return err
	}

	// 等待服务器完全停止
	time.Sleep(100 * time.Millisecond)

	// 检查服务器状态
	if vm.syncService.GetNetworkServer() != nil && vm.syncService.GetNetworkServer().IsRunning() {
		vm.setStatus("服务器停止失败")
		vm.serverRunning = true
		vm.updateButtonStates()
		return fmt.Errorf("服务器停止失败")
	}

	vm.serverRunning = false
	vm.setStatus("服务器已停止")
	vm.updateButtonStates()
	return nil
}

// BrowseSyncDir 浏览同步目录
func (vm *ConfigViewModel) BrowseSyncDir() error {
	dlg := walk.FileDialog{
		Title:          "选择同步目录",
		InitialDirPath: "::{20D04FE0-3AEA-1069-A2D8-08002B30309D}",
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
		UUID:            fmt.Sprintf("cfg-%d", time.Now().UnixNano()), // 使用时间戳生成唯一ID
		Type:            interfaces.ConfigTypeServer,
		Name:            name, // 使用用户输入的名称
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

	// 刷新配置列表
	vm.configList.RefreshCache()
	vm.UpdateUI()

	return nil
}

// DeleteConfig 删除配置
func (vm *ConfigViewModel) DeleteConfig(uuid string) error {
	if err := vm.syncService.DeleteConfig(uuid); err != nil {
		return err
	}
	// 刷新配置列表
	vm.configList.RefreshCache()
	vm.UpdateUI()
	return nil
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

// IsServerRunning 检查服务器是否正在运行
func (vm *ConfigViewModel) IsServerRunning() bool {
	if vm == nil || vm.syncService == nil {
		return false
	}
	// 同时检查 syncService 和 networkServer 的状态
	isRunning := vm.syncService.IsRunning() && vm.syncService.GetNetworkServer() != nil && vm.syncService.GetNetworkServer().IsRunning()
	// 更新内部状态
	vm.serverRunning = isRunning
	return isRunning
}
