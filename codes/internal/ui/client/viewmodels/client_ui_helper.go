package viewmodels

import (
	"fmt"
	"os"

	"synctools/codes/internal/interfaces"

	"github.com/lxn/walk"
)

// SetUIControls 设置UI控件引用
func (vm *MainViewModel) SetUIControls(
	connectBtn *walk.PushButton,
	addrEdit, portEdit interfaces.LineEditIface,
	progress *walk.ProgressBar,
	saveBtn *walk.PushButton,
	syncPathEdit interfaces.LineEditIface,
	browseBtn *walk.PushButton,
	syncBtn *walk.PushButton,
	serverInfo *walk.TextLabel,
	syncTable interfaces.TableViewIface,
	statusBar *walk.StatusBarItem,
) {
	vm.connectButton = connectBtn
	vm.addressEdit = addrEdit
	vm.portEdit = portEdit
	vm.progressBar = progress
	vm.saveButton = saveBtn
	vm.syncPathEdit = syncPathEdit
	vm.browseButton = browseBtn
	vm.syncButton = syncBtn
	vm.syncTable = syncTable
	vm.serverInfo = serverInfo
	vm.StatusBar = statusBar

	// 设置表格模型
	if vm.syncTable != nil {
		vm.syncTable.SetModel(vm.syncList)
	}

	// 从配置中读取服务器地址和端口
	if vm.syncService != nil {
		if config := vm.GetCurrentConfig(); config != nil {
			vm.addressEdit.SetText(config.Host)
			vm.portEdit.SetText(fmt.Sprintf("%d", config.Port))
			vm.syncPathEdit.SetText(config.SyncDir)
			vm.logger.Debug("从配置加载服务器信息", interfaces.Fields{
				"host":     config.Host,
				"port":     config.Port,
				"syncPath": config.SyncDir,
			})
		}
	}

	vm.UpdateUIState()
}

// UpdateUIState 更新UI状态
func (vm *MainViewModel) UpdateUIState() {
	if vm.window == nil {
		vm.logger.Debug("窗口未初始化，跳过UI更新", interfaces.Fields{})
		return
	}

	vm.logger.Debug("开始更新UI状态", interfaces.Fields{
		"isConnected": vm.IsConnected(),
	})

	// 在UI线程中执行
	vm.window.Synchronize(func() {
		isConnected := vm.IsConnected()
		// 更新连接按钮状态
		if isConnected {
			vm.connectButton.SetText("断开连接")
			vm.StatusBar.SetText(fmt.Sprintf("已连接到 %s:%s", vm.addressEdit.Text(), vm.portEdit.Text()))
		} else {
			vm.connectButton.SetText("连接服务器")
			vm.StatusBar.SetText("未连接")
		}

		// 更新输入框和按钮状态
		vm.addressEdit.SetEnabled(!isConnected)
		vm.portEdit.SetEnabled(!isConnected)
		vm.syncPathEdit.SetEnabled(!isConnected)
		vm.browseButton.SetEnabled(!isConnected)
		vm.saveButton.SetEnabled(!isConnected)
		vm.syncButton.SetEnabled(isConnected)

		// 更新服务器信息
		if vm.serverInfo != nil {
			if isConnected {
				// 尝试加载服务器配置
				serverConfig, err := vm.syncService.LoadServerConfig()
				if err == nil && serverConfig != nil {
					vm.serverInfo.SetText(fmt.Sprintf("服务器信息: %s (v%s)", serverConfig.Name, serverConfig.Version))
				} else {
					vm.serverInfo.SetText("服务器配置未知")
				}
			} else {
				vm.serverInfo.SetText("未连接到服务器")
			}
		}

		// 更新进度条状态
		if vm.progressBar != nil && !isConnected {
			vm.progressBar.SetValue(0)
		}

		// 更新表格
		if vm.syncTable != nil {
			// 先刷新数据源缓存
			vm.syncList.RefreshCache()
			// 通知UI更新
			vm.syncList.PublishRowsReset()
		}

		// 调用自定义UI更新回调
		if vm.onUIUpdate != nil {
			vm.onUIUpdate()
		}
	})
}

// BrowseSyncDir 浏览同步目录
func (vm *MainViewModel) BrowseSyncDir() error {
	dlg := walk.FileDialog{
		Title:          "选择同步目录",
		FilePath:       vm.syncPathEdit.Text(),
		InitialDirPath: os.Getenv("USERPROFILE"),
	}

	if ok, err := dlg.ShowBrowseFolder(vm.window); err != nil {
		vm.logger.Error("打开目录选择对话框失败", interfaces.Fields{
			"error": err,
		})
		return err
	} else if !ok {
		return nil
	}

	vm.syncPathEdit.SetText(dlg.FilePath)
	return nil
}
