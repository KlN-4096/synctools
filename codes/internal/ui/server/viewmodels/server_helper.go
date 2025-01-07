package viewmodels

import (
	"fmt"
	"synctools/codes/internal/interfaces"
	"time"

	"github.com/lxn/walk"
)

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
