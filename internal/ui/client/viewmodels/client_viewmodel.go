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
	syncPath    string // 新增：同步目录路径

	// UI 组件
	connectButton    *walk.PushButton
	disconnectButton *walk.PushButton
	addressEdit      *walk.LineEdit
	portEdit         *walk.LineEdit
	progressBar      *walk.ProgressBar
	saveButton       *walk.PushButton
	syncPathEdit     *walk.LineEdit // 新增：同步目录输入框

	// 网络连接
	conn       net.Conn
	networkOps interfaces.NetworkOperations

	// UI 更新回调
	onUIUpdate func()
}

// NewMainViewModel 创建新的主视图模型
func NewMainViewModel(syncService interfaces.SyncService, logger interfaces.Logger) *MainViewModel {
	vm := &MainViewModel{
		syncService: syncService,
		logger:      logger,
		connected:   false,
		serverAddr:  "localhost",
		serverPort:  "9527",
		syncPath:    "", // 默认为空
		networkOps:  network.NewOperations(logger),
	}

	// 从配置中读取服务器地址和端口
	if syncService != nil {
		if config := syncService.GetCurrentConfig(); config != nil {
			vm.serverAddr = config.Host
			vm.serverPort = fmt.Sprintf("%d", config.Port)
			// 如果有重定向配置，使用第一个作为默认同步路径
			if len(config.FolderRedirects) > 0 {
				vm.syncPath = config.FolderRedirects[0].ClientPath
			}
		}
	}

	vm.logger.Debug("创建主视图模型", interfaces.Fields{
		"defaultAddr": vm.serverAddr,
		"defaultPort": vm.serverPort,
		"syncPath":    vm.syncPath,
	})
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
			vm.logger.Debug("从配置加载服务器信息", interfaces.Fields{
				"host": config.Host,
				"port": config.Port,
			})
		}
	}

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
		"isConnected": vm.connected,
	})

	// 在UI线程中执行
	vm.window.Synchronize(func() {
		if vm.onUIUpdate != nil {
			vm.onUIUpdate()
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

// SaveConfig 保存配置
func (vm *MainViewModel) SaveConfig() error {
	if vm.syncService == nil {
		return fmt.Errorf("同步服务未初始化")
	}

	// 获取当前配置
	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有当前配置")
	}

	// 保存原始值
	originalHost := config.Host
	originalPort := config.Port
	originalSyncPath := ""
	if len(config.FolderRedirects) > 0 {
		originalSyncPath = config.FolderRedirects[0].ClientPath
	}

	// 更新配置
	newPort := vm.parsePort()

	vm.logger.Debug("检查配置变更", interfaces.Fields{
		"originalHost":     originalHost,
		"newHost":          vm.serverAddr,
		"originalPort":     originalPort,
		"newPort":          newPort,
		"originalSyncPath": originalSyncPath,
		"newSyncPath":      vm.syncPath,
	})

	// 检查是否有变更
	if originalHost == vm.serverAddr && originalPort == newPort && originalSyncPath == vm.syncPath {
		vm.logger.Debug("配置未发生变化，无需保存", interfaces.Fields{
			"host":     originalHost,
			"port":     originalPort,
			"syncPath": originalSyncPath,
		})
		return nil
	}

	// 更新配置
	config.Host = vm.serverAddr
	config.Port = newPort

	// 更新重定向配置
	if vm.syncPath != "" {
		if len(config.FolderRedirects) == 0 {
			// 如果没有重定向配置，创建一个新的
			config.FolderRedirects = []interfaces.FolderRedirect{
				{
					ServerPath: "/", // 使用根路径作为服务器路径
					ClientPath: vm.syncPath,
				},
			}
		} else {
			// 更新第一个重定向配置
			config.FolderRedirects[0].ClientPath = vm.syncPath
		}
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	vm.logger.Info("配置已保存", interfaces.Fields{
		"host":     config.Host,
		"port":     config.Port,
		"syncPath": vm.syncPath,
	})

	return nil
}

// parsePort 解析端口号
func (vm *MainViewModel) parsePort() int {
	port := 0
	if _, err := fmt.Sscanf(vm.serverPort, "%d", &port); err != nil {
		vm.logger.Debug("端口号解析失败，使用默认端口", interfaces.Fields{
			"input": vm.serverPort,
			"error": err,
		})
		port = 9527 // 默认端口
	}
	if port <= 0 || port > 65535 {
		vm.logger.Debug("端口号超出范围，使用默认端口", interfaces.Fields{
			"input": vm.serverPort,
			"port":  port,
		})
		port = 9527 // 默认端口
	}
	return port
}

// SetSyncPath 设置同步路径
func (vm *MainViewModel) SetSyncPath(path string) {
	vm.logger.Debug("设置同步路径", interfaces.Fields{
		"oldPath": vm.syncPath,
		"newPath": path,
	})
	vm.syncPath = path
}

// GetSyncPath 获取同步路径
func (vm *MainViewModel) GetSyncPath() string {
	return vm.syncPath
}

// SyncFiles 同步指定目录的文件
func (vm *MainViewModel) SyncFiles(path string) error {
	if !vm.IsConnected() {
		return fmt.Errorf("未连接到服务器")
	}

	vm.logger.Info("开始同步文件", interfaces.Fields{
		"path": path,
	})

	// 调用同步服务进行同步
	if err := vm.syncService.SyncFiles(path); err != nil {
		vm.logger.Error("同步失败", interfaces.Fields{
			"path":  path,
			"error": err,
		})
		return fmt.Errorf("同步失败: %v", err)
	}

	vm.logger.Info("同步完成", interfaces.Fields{
		"path": path,
	})
	return nil
}
