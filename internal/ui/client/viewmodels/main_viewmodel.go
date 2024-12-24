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
	"net"

	"github.com/lxn/walk"

	"synctools/internal/interfaces"
	"synctools/pkg/network"
)

// MainViewModel 客户端主视图模型
type MainViewModel struct {
	syncService interfaces.SyncService
	logger      interfaces.Logger
	window      *walk.MainWindow
	connected   bool
	serverAddr  string
	serverPort  string

	// UI 控件
	connectButton    *walk.PushButton
	disconnectButton *walk.PushButton
	addressEdit      *walk.LineEdit
	portEdit         *walk.LineEdit
	progressBar      *walk.ProgressBar

	// 网络连接
	conn       net.Conn
	networkOps interfaces.NetworkOperations
}

// NewMainViewModel 创建新的主视图模型
func NewMainViewModel(syncService interfaces.SyncService, logger interfaces.Logger) *MainViewModel {
	vm := &MainViewModel{
		syncService: syncService,
		logger:      logger,
		connected:   false,
		serverAddr:  "localhost",
		serverPort:  "9527",
		networkOps:  network.NewOperations(logger),
	}
	vm.logger.Debug("创建主视图模型", interfaces.Fields{
		"defaultAddr": vm.serverAddr,
		"defaultPort": vm.serverPort,
	})
	return vm
}

// Initialize 初始化视图模型
func (vm *MainViewModel) Initialize(window *walk.MainWindow) error {
	vm.window = window
	vm.logger.Debug("视图模型初始化完成", interfaces.Fields{})
	vm.updateUIState()
	return nil
}

// Shutdown 关闭视图模型
func (vm *MainViewModel) Shutdown() error {
	if vm.connected {
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
func (vm *MainViewModel) SetUIControls(connectBtn, disconnectBtn *walk.PushButton, addrEdit, portEdit *walk.LineEdit, progress *walk.ProgressBar) {
	vm.connectButton = connectBtn
	vm.disconnectButton = disconnectBtn
	vm.addressEdit = addrEdit
	vm.portEdit = portEdit
	vm.progressBar = progress
	vm.updateUIState()
}

// updateUIState 更新UI状态
func (vm *MainViewModel) updateUIState() {
	if vm.window == nil {
		vm.logger.Debug("窗口未初始化，跳过UI更新", interfaces.Fields{})
		return
	}

	vm.logger.Debug("开始更新UI状态", interfaces.Fields{
		"isConnected": vm.connected,
		"hasControls": vm.connectButton != nil && vm.disconnectButton != nil,
	})

	// 在UI线程中执行
	vm.window.Synchronize(func() {
		// 更新按钮状态
		if vm.connectButton != nil {
			vm.connectButton.SetEnabled(!vm.connected)
		}
		if vm.disconnectButton != nil {
			vm.disconnectButton.SetEnabled(vm.connected)
		}

		// 更新输入框状态
		if vm.addressEdit != nil {
			vm.addressEdit.SetEnabled(!vm.connected)
		}
		if vm.portEdit != nil {
			vm.portEdit.SetEnabled(!vm.connected)
		}

		// 更新状态栏
		if statusBar := vm.window.StatusBar(); statusBar != nil {
			if vm.connected {
				status := fmt.Sprintf("已连接到 %s:%s", vm.serverAddr, vm.serverPort)
				statusBar.Items().At(0).SetText(status)
				vm.logger.Debug("更新状态栏", interfaces.Fields{"status": status})
			} else {
				statusBar.Items().At(0).SetText("未连接")
				vm.logger.Debug("更新状态栏", interfaces.Fields{"status": "未连接"})
			}
		}
	})
}

// Connect 连接到服务器
func (vm *MainViewModel) Connect() error {
	vm.logger.Debug("开始连接服务器", interfaces.Fields{
		"isConnected": vm.IsConnected(),
		"serverAddr":  vm.serverAddr,
		"serverPort":  vm.serverPort,
	})

	if vm.IsConnected() {
		vm.logger.Debug("已经连接到服务器，无需重复连接", interfaces.Fields{})
		return fmt.Errorf("已经连接到服务器")
	}

	// 验证地址和端口
	if vm.serverAddr == "" || vm.serverPort == "" {
		vm.logger.Debug("服务器地址或端口为空", interfaces.Fields{
			"serverAddr": vm.serverAddr,
			"serverPort": vm.serverPort,
		})
		return fmt.Errorf("服务器地址或端口不能为空")
	}

	// 尝试连接服务器
	vm.logger.Info("正在连接服务器", interfaces.Fields{
		"address": vm.serverAddr,
		"port":    vm.serverPort,
	})

	// 建立连接
	addr := fmt.Sprintf("%s:%s", vm.serverAddr, vm.serverPort)
	vm.logger.Debug("开始建立TCP连接", interfaces.Fields{
		"fullAddr": addr,
	})

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		vm.logger.Error("连接服务器失败", interfaces.Fields{
			"error":    err,
			"fullAddr": addr,
		})
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	vm.conn = conn
	vm.connected = true
	vm.logger.Info("已连接到服务器", interfaces.Fields{
		"address":    vm.serverAddr,
		"port":       vm.serverPort,
		"localAddr":  conn.LocalAddr().String(),
		"remoteAddr": conn.RemoteAddr().String(),
	})

	// 更新UI状态
	vm.updateUIState()
	return nil
}

// Disconnect 断开服务器连接
func (vm *MainViewModel) Disconnect() error {
	vm.logger.Debug("开始断开服务器连接", interfaces.Fields{
		"isConnected": vm.IsConnected(),
		"hasConn":     vm.conn != nil,
	})

	if !vm.IsConnected() {
		vm.logger.Debug("未连接到服务器，无需断开", interfaces.Fields{})
		return fmt.Errorf("未连接到服务器")
	}

	vm.logger.Info("正在断开服务器连接", interfaces.Fields{})

	if vm.conn != nil {
		vm.logger.Debug("关闭网络连接", interfaces.Fields{
			"localAddr":  vm.conn.LocalAddr().String(),
			"remoteAddr": vm.conn.RemoteAddr().String(),
		})
		if err := vm.conn.Close(); err != nil {
			vm.logger.Error("关闭连接失败", interfaces.Fields{
				"error": err,
			})
			return fmt.Errorf("关闭连接失败: %v", err)
		}
		vm.conn = nil
	}

	vm.connected = false
	vm.logger.Info("已断开服务器连接", interfaces.Fields{})

	// 更新UI状态
	vm.updateUIState()
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

// UpdateProgress 更新进度条
func (vm *MainViewModel) UpdateProgress(progress interfaces.Progress) {
	if vm.window == nil {
		return
	}

	vm.window.Synchronize(func() {
		if vm.progressBar != nil {
			if progress.Total > 0 {
				percentage := int(float64(progress.Current) / float64(progress.Total) * 100)
				vm.progressBar.SetValue(percentage)
			} else {
				vm.progressBar.SetValue(0)
			}
		}

		// 更新状态栏显示传输速度
		if statusBar := vm.window.StatusBar(); statusBar != nil {
			if progress.Speed > 0 {
				speedMB := progress.Speed / 1024 / 1024
				statusBar.Items().At(0).SetText(fmt.Sprintf("传输速度: %.2f MB/s", speedMB))
			}
		}
	})
}
