package viewmodels

import (
	"fmt"

	"synctools/internal/interfaces"

	"github.com/lxn/walk"
)

/*
文件作用:
- 实现主窗口的视图模型
- 管理全局状态
- 协调各个子视图模型
- 处理主窗口事件

主要方法:
- NewMainViewModel: 创建主窗口视图模型
- InitializeViewModels: 初始化子视图模型
- HandleCommand: 处理命令
- UpdateStatus: 更新状态
- ShowDialog: 显示对话框
*/

// MainViewModel 主窗口视图模型
type MainViewModel struct {
	syncService interfaces.SyncService
	logger      Logger
	window      MainWindow
	status      string
}

// NewMainViewModel 创建主视图模型
func NewMainViewModel(syncService interfaces.SyncService, logger interfaces.Logger) *MainViewModel {
	return &MainViewModel{
		syncService: syncService,
		logger:      NewLoggerAdapter(logger),
		status:      "就绪",
	}
}

// Initialize 初始化视图模型
func (vm *MainViewModel) Initialize() error {
	vm.logger.Info("初始化主视图模型", interfaces.Fields{
		"status": vm.status,
	})
	return nil
}

// Shutdown 关闭视图模型
func (vm *MainViewModel) Shutdown() error {
	vm.logger.Info("关闭主视图模型", nil)
	return nil
}

// SetMainWindow 设置主窗口
func (vm *MainViewModel) SetMainWindow(window MainWindow) {
	vm.window = window
}

// GetStatus 获取状态
func (vm *MainViewModel) GetStatus() string {
	if vm.syncService != nil {
		return vm.syncService.GetSyncStatus()
	}
	return vm.status
}

// SetStatus 设置状态
func (vm *MainViewModel) SetStatus(status string) {
	vm.status = status
	vm.logger.Info("状态更新", interfaces.Fields{
		"status": status,
	})
}

// StartSync 开始同步
func (vm *MainViewModel) StartSync(path string) error {
	vm.logger.Info("开始同步", interfaces.Fields{
		"path": path,
	})

	if vm.syncService == nil {
		return fmt.Errorf("同步服务未初始化")
	}

	if err := vm.syncService.SyncFiles(path); err != nil {
		vm.logger.Error("同步失败", interfaces.Fields{
			"error": err,
		})
		if vm.window != nil {
			_, _ = vm.window.MsgBox("同步失败", err.Error(), walk.MsgBoxIconError)
		}
		return err
	}

	vm.logger.Info("同步完成", nil)
	return nil
}

// HandleSyncRequest 处理同步请求
func (vm *MainViewModel) HandleSyncRequest(request interface{}) error {
	vm.logger.Info("处理同步请求", interfaces.Fields{
		"request": fmt.Sprintf("%+v", request),
	})

	if vm.syncService == nil {
		return fmt.Errorf("同步服务未初始化")
	}

	if err := vm.syncService.HandleSyncRequest(request); err != nil {
		vm.logger.Error("处理同步请求失败", interfaces.Fields{
			"error": err,
		})
		if vm.window != nil {
			_, _ = vm.window.MsgBox("处理请求失败", err.Error(), walk.MsgBoxIconError)
		}
		return err
	}

	vm.logger.Info("请求处理完成", nil)
	return nil
}

// showError 显示错误消息
func (vm *MainViewModel) showError(title, message string) {
	if vm.window != nil {
		_, _ = vm.window.MsgBox(title, message, walk.MsgBoxIconError)
	}
}
