package viewmodels

import (
	"github.com/lxn/walk"

	"synctools/internal/service"
)

/*
Package viewmodels 的主窗口视图模型。

文件作用：
- 实现主窗口的视图模型
- 管理全局状态
- 协调各个子视图模型
- 处理主窗口事件

主要类型：
- MainViewModel: 主窗口视图模型
- WindowState: 窗口状态

主要方法：
- NewMainViewModel: 创建主窗口视图模型
- InitializeViewModels: 初始化子视图模型
- HandleCommand: 处理命令
- UpdateStatus: 更新状态
- ShowDialog: 显示对话框
*/

// MainViewModel 主视图模型
type MainViewModel struct {
	ConfigViewModel *ConfigViewModel
	logger          ViewModelLogger
	mainWindow      *walk.MainWindow
}

// NewMainViewModel 创建新的主视图模型
func NewMainViewModel(syncService *service.SyncService, logger ViewModelLogger) *MainViewModel {
	return &MainViewModel{
		ConfigViewModel: NewConfigViewModel(syncService, logger),
		logger:          logger,
	}
}

// Initialize 初始化视图模型
func (vm *MainViewModel) Initialize(mainWindow *walk.MainWindow) error {
	vm.mainWindow = mainWindow
	return vm.ConfigViewModel.Initialize(mainWindow)
}

// LogDebug 记录调试日志
func (vm *MainViewModel) LogDebug(msg string) {
	vm.logger.DebugLog(msg)
}

// LogError 记录错误日志
func (vm *MainViewModel) LogError(msg string, err error) {
	if err != nil {
		vm.logger.Error(msg, "error", err)
	} else {
		vm.logger.Error(msg)
	}
}
