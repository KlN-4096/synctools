/*
文件作用:
- 实现客户端主视图模型
- 管理客户端状态
- 处理客户端业务逻辑
- 提供UI数据绑定

主要方法:
- NewMainViewModel: 创建主视图模型
- Initialize: 初始化视图模型
- Shutdown: 关闭视图模型
- Connect/Disconnect: 连接/断开服务器
*/

package viewmodels

import (
	"github.com/lxn/walk"

	"synctools/internal/interfaces"
)

// MainViewModel 客户端主视图模型
type MainViewModel struct {
	syncService interfaces.SyncService
	logger      interfaces.Logger
	window      *walk.MainWindow
	connected   bool
}

// NewMainViewModel 创建新的主视图模型
func NewMainViewModel(syncService interfaces.SyncService, logger interfaces.Logger) *MainViewModel {
	return &MainViewModel{
		syncService: syncService,
		logger:      logger,
		connected:   false,
	}
}

// Initialize 初始化视图模型
func (vm *MainViewModel) Initialize(window *walk.MainWindow) error {
	vm.window = window
	vm.logger.Debug("视图模型初始化完成", interfaces.Fields{})
	return nil
}

// Shutdown 关闭视图模型
func (vm *MainViewModel) Shutdown() error {
	if vm.connected {
		if err := vm.Disconnect(); err != nil {
			return err
		}
	}
	return nil
}

// Connect 连接到服务器
func (vm *MainViewModel) Connect() error {
	// TODO: 实现连接逻辑
	vm.connected = true
	vm.logger.Info("已连接到服务器", interfaces.Fields{})
	return nil
}

// Disconnect 断开服务器连接
func (vm *MainViewModel) Disconnect() error {
	// TODO: 实现断开连接逻辑
	vm.connected = false
	vm.logger.Info("已断开服务器连接", interfaces.Fields{})
	return nil
}

// IsConnected 检查是否已连接
func (vm *MainViewModel) IsConnected() bool {
	return vm.connected
}

// LogDebug 记录调试日志
func (vm *MainViewModel) LogDebug(msg string) {
	vm.logger.Debug(msg, interfaces.Fields{})
}

// LogError 记录错误日志
func (vm *MainViewModel) LogError(msg string, err error) {
	vm.logger.Error(msg, interfaces.Fields{
		"error": err,
	})
}

// GetLogger 获取日志记录器
func (vm *MainViewModel) GetLogger() interfaces.Logger {
	return vm.logger
}
