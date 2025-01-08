package viewmodels

import (
	"strconv"
	"strings"

	"synctools/codes/internal/interfaces"

	"github.com/lxn/walk"
)

// setControlsEnabled 设置一组控件的启用状态
func (vm *ConfigViewModel) setControlsEnabled(enabled bool, controls ...interface{}) {
	if vm == nil || vm.logger == nil {
		return
	}

	for _, control := range controls {
		if control == nil {
			continue
		}
		if setter, ok := control.(interfaces.EnabledSetter); ok {
			setter.SetEnabled(enabled)
			if btn, ok := control.(*walk.PushButton); ok && btn != nil {
				vm.logger.Debug("设置控件状态", interfaces.Fields{"enabled": enabled, "type": "Button", "text": btn.Text()})
			} else {
				vm.logger.Debug("设置控件状态", interfaces.Fields{"enabled": enabled})
			}
		}
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

// UpdateUI 更新 UI 显示
func (vm *ConfigViewModel) UpdateUI() {
	if vm == nil || vm.logger == nil {
		return
	}

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

	// 安全地设置控件状态
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
	if vm.configTable != nil && vm.configList != nil {
		vm.configList.ForceRefresh()
		vm.configTable.SetModel(nil)
		vm.configTable.SetModel(vm.configList)
	} else {
		vm.logger.Warn("UI状态", interfaces.Fields{
			"component": "config_table",
			"status":    "empty",
		})
	}

	// 更新同步文件夹表格
	if vm.syncFolderTable != nil && vm.syncFolderList != nil {
		vm.syncFolderList.ForceRefresh()
		vm.syncFolderTable.SetModel(nil)
		vm.syncFolderTable.SetModel(vm.syncFolderList)
	} else {
		vm.logger.Warn("UI状态", interfaces.Fields{
			"component": "sync_folder_table",
			"status":    "empty",
		})
	}

	// 安全地更新基本信息
	if vm.nameEdit != nil && vm.versionEdit != nil && vm.hostEdit != nil &&
		vm.portEdit != nil && vm.syncDirEdit != nil && vm.ignoreEdit != nil {
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
	}

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
