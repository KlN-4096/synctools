package viewmodels

import (
	"github.com/lxn/walk"

	"synctools/codes/internal/interfaces"
)

// MainViewModel 主窗口视图模型
type MainViewModel struct {
	syncService     interfaces.ServerSyncService
	logger          interfaces.Logger
	window          *walk.MainWindow
	status          string
	ConfigViewModel *ConfigViewModel
}

// NewMainViewModel 创建主视图模型
func NewMainViewModel(syncService interfaces.SyncService, log interfaces.Logger) *MainViewModel {
	// 类型转换检查
	serverService, ok := syncService.(interfaces.ServerSyncService)
	if !ok {
		panic("必须提供服务器同步服务实例")
	}

	vm := &MainViewModel{
		syncService: serverService,
		logger:      log,
		status:      "就绪",
	}

	// 创建配置视图模型
	vm.ConfigViewModel = NewConfigViewModel(serverService, vm.logger)

	return vm
}

// Initialize 初始化视图模型
func (vm *MainViewModel) Initialize(window *walk.MainWindow) error {
	vm.window = window
	vm.logger.Info("初始化主视图模型", interfaces.Fields{
		"status": vm.status,
	})
	return nil
}

// LogDebug 记录调试日志
func (vm *MainViewModel) LogDebug(message string) {
	vm.logger.Debug(message, nil)
}

// LogError 记录错误日志
func (vm *MainViewModel) LogError(message string, err error) {
	vm.logger.Error(message, interfaces.Fields{
		"error": err,
	})
}

// showError 显示错误消息
func (vm *MainViewModel) showError(title, message string) {
	if vm.window != nil {
		walk.MsgBox(vm.window, title, message, walk.MsgBoxIconError)
	}
}

// GetLogger 获取日志记录器
func (vm *MainViewModel) GetLogger() interfaces.Logger {
	return vm.logger
}
