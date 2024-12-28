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
	"fmt"

	"github.com/lxn/walk"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/network/client"
)

// MainViewModel 客户端主视图模型
type MainViewModel struct {
	syncService interfaces.ClientSyncService
	logger      interfaces.Logger
	window      *walk.MainWindow
	serverAddr  string
	serverPort  string
	syncPath    string

	// UI 组件
	connectButton    *walk.PushButton
	disconnectButton *walk.PushButton
	addressEdit      *walk.LineEdit
	portEdit         *walk.LineEdit
	progressBar      *walk.ProgressBar
	saveButton       *walk.PushButton
	syncPathEdit     *walk.LineEdit

	// 网络客户端
	networkClient *client.NetworkClient

	// UI 更新回调
	onUIUpdate func()
}

// NewMainViewModel 创建新的主视图模型
func NewMainViewModel(syncService interfaces.ClientSyncService, logger interfaces.Logger) *MainViewModel {
	vm := &MainViewModel{
		syncService:   syncService,
		logger:        logger,
		serverAddr:    "localhost",
		serverPort:    "9527",
		syncPath:      "",
		networkClient: client.NewNetworkClient(logger, syncService),
	}

	// 从配置中读取服务器地址和端口
	if syncService != nil {
		if config := syncService.GetCurrentConfig(); config != nil {
			vm.serverAddr = config.Host
			vm.serverPort = fmt.Sprintf("%d", config.Port)
			vm.syncPath = config.SyncDir
		}
	}

	vm.logger.Debug("创建主视图模型", interfaces.Fields{
		"defaultAddr": vm.serverAddr,
		"defaultPort": vm.serverPort,
		"syncPath":    vm.syncPath,
	})

	// 设置连接丢失回调
	vm.networkClient.SetConnectionLostCallback(vm.handleConnectionLost)

	return vm
}

// Initialize 初始化视图模型
func (vm *MainViewModel) Initialize(window *walk.MainWindow) error {
	vm.window = window

	// 从配置中读取服务器地址和端口
	if vm.syncService != nil {
		if config := vm.syncService.GetCurrentConfig(); config != nil {
			vm.serverAddr = config.Host
			vm.serverPort = fmt.Sprintf("%d", config.Port)
			vm.syncPath = config.SyncDir
			vm.logger.Debug("从配置加载服务器信息", interfaces.Fields{
				"host":     config.Host,
				"port":     config.Port,
				"syncPath": config.SyncDir,
			})
		}
	}

	vm.logger.Debug("视图模型初始化完成", interfaces.Fields{})
	vm.updateUIState()
	return nil
}

// Shutdown 关闭视图模型
func (vm *MainViewModel) Shutdown() error {
	if vm.IsConnected() {
		if err := vm.Disconnect(); err != nil {
			vm.logger.Error("关闭时断开连接失败", interfaces.Fields{
				"error": err,
			})
			return err
		}
	}
	return nil
}

// SetServerAddr 设置服务器地址
func (vm *MainViewModel) SetServerAddr(addr string) {
	vm.logger.Debug("设置服务器地址", interfaces.Fields{
		"oldAddr": vm.serverAddr,
		"newAddr": addr,
	})
	vm.serverAddr = addr
}

// SetServerPort 设置服务器端口
func (vm *MainViewModel) SetServerPort(port string) {
	vm.logger.Debug("设置服务器端口", interfaces.Fields{
		"oldPort": vm.serverPort,
		"newPort": port,
	})
	vm.serverPort = port
}

// GetServerAddr 获取服务器地址
func (vm *MainViewModel) GetServerAddr() string {
	return vm.serverAddr
}

// GetServerPort 获取服务器端口
func (vm *MainViewModel) GetServerPort() string {
	return vm.serverPort
}

// SetUIControls 设置UI控件引用
func (vm *MainViewModel) SetUIControls(connectBtn *walk.PushButton, addrEdit, portEdit *walk.LineEdit, progress *walk.ProgressBar, saveBtn *walk.PushButton, syncPathEdit *walk.LineEdit) {
	vm.connectButton = connectBtn
	vm.addressEdit = addrEdit
	vm.portEdit = portEdit
	vm.progressBar = progress
	vm.saveButton = saveBtn
	vm.syncPathEdit = syncPathEdit
	vm.updateUIState()
}

// SetUIUpdateCallback 设置UI更新回调
func (vm *MainViewModel) SetUIUpdateCallback(callback func()) {
	vm.onUIUpdate = callback
}

// updateUIState 更新UI状态
func (vm *MainViewModel) updateUIState() {
	if vm.window == nil {
		vm.logger.Debug("窗口未初始化，跳过UI更新", interfaces.Fields{})
		return
	}

	vm.logger.Debug("开始更新UI状态", interfaces.Fields{
		"isConnected": vm.IsConnected(),
	})

	// 在UI线程中执行
	vm.window.Synchronize(func() {
		// 更新连接按钮状态
		if vm.connectButton != nil {
			if vm.IsConnected() {
				vm.connectButton.SetText("断开连接")
				vm.connectButton.SetEnabled(true)
			} else {
				vm.connectButton.SetText("连接服务器")
				vm.connectButton.SetEnabled(true)
			}
		}

		// 更新输入框状态
		if vm.addressEdit != nil {
			vm.addressEdit.SetEnabled(!vm.IsConnected())
		}
		if vm.portEdit != nil {
			vm.portEdit.SetEnabled(!vm.IsConnected())
		}
		if vm.syncPathEdit != nil {
			vm.syncPathEdit.SetEnabled(!vm.IsConnected())
		}

		// 更新保存按钮状态
		if vm.saveButton != nil {
			vm.saveButton.SetEnabled(!vm.IsConnected())
		}

		// 更新进度条状态
		if vm.progressBar != nil {
			if !vm.IsConnected() {
				vm.progressBar.SetValue(0)
			}
		}

		// 调用自定义UI更新回调
		if vm.onUIUpdate != nil {
			vm.onUIUpdate()
		}
	})
}

// Connect 连接到服务器
func (vm *MainViewModel) Connect() error {
	vm.networkClient.Connect(vm.serverAddr, vm.serverPort)
	vm.updateUIState()
	return nil
}

// Disconnect 断开连接
func (vm *MainViewModel) Disconnect() error {
	vm.networkClient.Disconnect()
	vm.updateUIState()
	return nil
}

// IsConnected 检查是否已连接
func (vm *MainViewModel) IsConnected() bool {
	return vm.networkClient.IsConnected()
}

// handleConnectionLost 处理连接丢失
func (vm *MainViewModel) handleConnectionLost() {
	vm.logger.Debug("处理连接丢失", interfaces.Fields{})
	vm.updateUIState()
}

// SaveConfig 保存配置
func (vm *MainViewModel) SaveConfig() error {
	if vm.syncService == nil {
		return fmt.Errorf("同步服务未初始化")
	}

	port := vm.parsePort()
	if port <= 0 {
		return fmt.Errorf("无效的端口号")
	}
	config := vm.GetCurrentConfig()

	config.Host = vm.serverAddr
	config.Port = port
	config.SyncDir = vm.syncPath

	if err := vm.syncService.ValidateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	if err := vm.syncService.SaveConfig(config); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// parsePort 解析端口号
func (vm *MainViewModel) parsePort() int {
	var port int
	_, err := fmt.Sscanf(vm.serverPort, "%d", &port)
	if err != nil {
		vm.logger.Error("解析端口号失败", interfaces.Fields{
			"port":  vm.serverPort,
			"error": err,
		})
		return 0
	}
	return port
}

// SetSyncPath 设置同步路径
func (vm *MainViewModel) SetSyncPath(path string) {
	vm.logger.Debug("设置同步路径", interfaces.Fields{
		"path": path,
	})
	vm.syncPath = path
}

// GetSyncPath 获取同步路径
func (vm *MainViewModel) GetSyncPath() string {
	return vm.syncPath
}

// SyncFiles 同步文件
func (vm *MainViewModel) SyncFiles(path string) error {
	if !vm.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}

	if vm.syncService == nil {
		return fmt.Errorf("同步服务未初始化")
	}

	return vm.syncService.SyncFiles(path)
}

// GetCurrentConfig 获取当前配置
func (vm *MainViewModel) GetCurrentConfig() *interfaces.Config {
	if vm.syncService == nil {
		return nil
	}
	return vm.syncService.GetCurrentConfig()
}
