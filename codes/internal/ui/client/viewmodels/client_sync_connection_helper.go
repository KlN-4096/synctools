package viewmodels

import (
	"fmt"
	"path/filepath"

	"synctools/codes/internal/interfaces"
)

// Connect 连接服务器
func (vm *MainViewModel) Connect() error {
	// 保存配置
	if err := vm.SaveConfig(); err != nil {
		vm.logger.Error("连接前保存配置失败", interfaces.Fields{
			"error": err,
		})
		return err
	}

	// 连接服务器
	host := vm.addressEdit.Text()
	port := vm.portEdit.Text()
	if err := vm.syncService.Connect(host, port); err != nil {
		vm.logger.Error("连接服务器失败", interfaces.Fields{
			"error": err,
		})
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	vm.logger.Info("连接服务器成功", interfaces.Fields{})

	// 确保在UI线程中更新
	vm.window.Synchronize(func() {
		// 刷新表格数据
		if vm.syncTable != nil {
			vm.syncList.ForceRefresh()
		}
		// 更新整体UI状态
		vm.UpdateUIState()
	})

	return nil
}

// Disconnect 断开连接
func (vm *MainViewModel) Disconnect() error {
	return vm.syncService.Disconnect()
}

// IsConnected 检查连接状态
func (vm *MainViewModel) IsConnected() bool {
	return vm.syncService.IsConnected()
}

// handleConnectionLost 处理连接丢失
func (vm *MainViewModel) handleConnectionLost() {
	vm.logger.Warn("连接丢失", interfaces.Fields{})
	vm.UpdateUIState()
}

// SyncFiles 同步文件
func (vm *MainViewModel) SyncFiles(path string) error {
	if !vm.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		vm.logger.Error("获取绝对路径失败", interfaces.Fields{
			"path":  path,
			"error": err,
		})
		return err
	}

	// 开始同步
	if err := vm.syncService.SyncFiles(absPath); err != nil {
		vm.logger.Error("同步文件失败", interfaces.Fields{
			"path":  absPath,
			"error": err,
		})
		return err
	}

	vm.logger.Info("同步文件成功", interfaces.Fields{
		"path": absPath,
	})
	return nil
}
