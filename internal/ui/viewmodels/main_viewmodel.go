package viewmodels

import (
	"github.com/lxn/walk"

	"synctools/internal/config"
	"synctools/internal/model"
	"synctools/internal/service"
)

// MainViewModel 主视图模型
type MainViewModel struct {
	ConfigViewModel *ConfigViewModel
	logger          model.Logger
	mainWindow      *walk.MainWindow
}

// NewMainViewModel 创建新的主视图模型
func NewMainViewModel(configManager *config.Manager, syncService *service.SyncService, logger model.Logger) *MainViewModel {
	return &MainViewModel{
		ConfigViewModel: NewConfigViewModel(configManager, syncService, logger),
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
