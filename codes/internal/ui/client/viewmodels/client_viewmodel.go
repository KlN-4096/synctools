/*
文件作用:
- 实现客户端视图模型层的核心业务逻辑
- 管理客户端的状态和数据
- 处理UI事件和业务操作
- 与服务层交互

主要功能:
1. 初始化和配置管理
2. 服务器连接管理
3. 文件同步操作
4. UI状态更新
5. 错误处理
*/

package viewmodels

import (
	"fmt"
	"os"

	"github.com/lxn/walk"

	"synctools/codes/internal/interfaces"
)

//
// -------------------- 类型定义 --------------------
//

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
	browseButton     *walk.PushButton
	syncButton       *walk.PushButton
	syncPathEdit     *walk.LineEdit

	// 表格组件
	syncTable interfaces.TableViewIface
	syncList  *SyncListModel

	serverInfo *walk.TextLabel
	StatusBar  *walk.StatusBarItem

	// UI 更新回调
	onUIUpdate func()
}

//
// -------------------- 生命周期管理方法 --------------------
//

// NewMainViewModel 创建新的主视图模型
func NewMainViewModel(syncService interfaces.ClientSyncService, logger interfaces.Logger) *MainViewModel {
	vm := &MainViewModel{
		syncService: syncService,
		logger:      logger,
		serverAddr:  "localhost",
		serverPort:  "9527",
		syncPath:    "",
	}

	// 创建表格模型
	vm.syncList = NewSyncListModel(syncService, logger)

	// 从配置中读取服务器地址和端口
	if syncService != nil {
		config := syncService.GetCurrentConfig()
		vm.logger.Debug("测试", interfaces.Fields{
			"config": config,
		})
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
	vm.syncService.SetConnectionLostCallback(vm.handleConnectionLost)

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
	vm.UpdateUIState()
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

//
// -------------------- UI组件管理方法 --------------------
//

// SetUIControls 设置UI控件引用
func (vm *MainViewModel) SetUIControls(
	connectBtn *walk.PushButton,
	addrEdit, portEdit *walk.LineEdit,
	progress *walk.ProgressBar,
	saveBtn *walk.PushButton,
	syncPathEdit *walk.LineEdit,
	syncTable interfaces.TableViewIface,
) {
	vm.connectButton = connectBtn
	vm.addressEdit = addrEdit
	vm.portEdit = portEdit
	vm.progressBar = progress
	vm.saveButton = saveBtn
	vm.syncPathEdit = syncPathEdit
	vm.syncTable = syncTable

	// 设置表格模型
	if vm.syncTable != nil {
		vm.syncTable.SetModel(vm.syncList)
	}

	vm.UpdateUIState()
}

// SetUIUpdateCallback 设置UI更新回调
func (vm *MainViewModel) SetUIUpdateCallback(callback func()) {
	vm.onUIUpdate = callback
}

// UpdateUIState 更新UI状态
func (vm *MainViewModel) UpdateUIState() {
	if vm.window == nil {
		vm.logger.Debug("窗口未初始化，跳过UI更新", interfaces.Fields{})
		return
	}

	vm.logger.Debug("开始更新UI状态", interfaces.Fields{
		"isConnected": vm.IsConnected(),
	})

	// 在UI线程中执行
	vm.window.Synchronize(func() {
		isConnected := vm.IsConnected()
		// 更新连接按钮状态
		if isConnected {
			vm.connectButton.SetText("断开连接")
			vm.StatusBar.SetText(fmt.Sprintf("已连接到 %s:%s", vm.serverAddr, vm.serverPort))
		} else {
			vm.connectButton.SetText("连接服务器")
			vm.StatusBar.SetText("未连接")
		}

		// 更新输入框值
		vm.addressEdit.SetText(vm.serverAddr)
		vm.portEdit.SetText(vm.serverPort)
		vm.syncPathEdit.SetText(vm.syncPath)

		// 更新输入框状态
		vm.addressEdit.SetEnabled(!isConnected)
		vm.portEdit.SetEnabled(!isConnected)
		vm.syncPathEdit.SetEnabled(!isConnected)

		// 更新保存按钮状态
		vm.connectButton.SetEnabled(!isConnected)
		vm.saveButton.SetEnabled(!isConnected)
		vm.browseButton.SetEnabled(!isConnected)
		vm.syncButton.SetEnabled(isConnected)

		// 更新服务器信息
		if vm.serverInfo != nil {
			if isConnected {
				config := vm.GetCurrentConfig()
				if config != nil {
					vm.serverInfo.SetText(fmt.Sprintf("服务器信息: %s (v%s)", config.Name, config.Version))
				} else {
					vm.serverInfo.SetText("已连接")
				}
			} else {
				vm.serverInfo.SetText("未连接到服务器")
			}
		}

		// 更新进度条状态
		if vm.progressBar != nil {
			if !isConnected {
				vm.progressBar.SetValue(0)
			}
		}

		// 更新表格
		if vm.syncTable != nil {
			vm.syncList.PublishRowsReset()
		}

		// 调用自定义UI更新回调
		if vm.onUIUpdate != nil {
			vm.onUIUpdate()
		}
	})
}

//
// -------------------- 配置管理方法 --------------------
//

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

// GetCurrentConfig 获取当前配置
func (vm *MainViewModel) GetCurrentConfig() *interfaces.Config {
	if vm.syncService == nil {
		return nil
	}
	return vm.syncService.GetCurrentConfig()
}

//
// -------------------- 连接管理方法 --------------------
//

// Connect 连接到服务器
func (vm *MainViewModel) Connect() error {
	// 检查服务器地址
	if vm.serverAddr == "" {
		return fmt.Errorf("服务器地址不能为空")
	}

	// 检查端口号
	port := vm.parsePort()
	if port <= 0 || port > 65535 {
		return fmt.Errorf("无效的端口号: %s", vm.serverPort)
	}

	// 检查同步路径
	if vm.syncPath == "" {
		return fmt.Errorf("同步路径不能为空")
	}

	// 检查同步路径是否存在
	if _, err := os.Stat(vm.syncPath); os.IsNotExist(err) {
		return fmt.Errorf("同步路径不存在: %s", vm.syncPath)
	}

	// 尝试连接服务器
	if err := vm.syncService.Connect(vm.serverAddr, vm.serverPort); err != nil {
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	vm.UpdateUIState()
	return nil
}

// Disconnect 断开连接
func (vm *MainViewModel) Disconnect() error {
	vm.syncService.Disconnect()
	vm.UpdateUIState()
	return nil
}

// IsConnected 检查是否已连接
func (vm *MainViewModel) IsConnected() bool {
	return vm.syncService.IsConnected()
}

// handleConnectionLost 处理连接丢失
func (vm *MainViewModel) handleConnectionLost() {
	vm.logger.Debug("处理连接丢失", interfaces.Fields{})
	vm.UpdateUIState()
}

//
// -------------------- 同步管理方法 --------------------
//

// SyncFiles 同步文件
func (vm *MainViewModel) SyncFiles(path string) error {

	if vm.syncService == nil {
		return fmt.Errorf("同步服务未初始化")
	}
	fmt.Println(vm.IsConnected())
	return vm.syncService.SyncFiles(path)
}

//
// -------------------- 内部辅助方法 --------------------
//

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

//
// -------------------- 表格模型实现 --------------------
//

// SyncListModel 同步列表模型
type SyncListModel struct {
	walk.TableModelBase
	syncService interfaces.ClientSyncService
	logger      interfaces.Logger
	sortColumn  int
	sortOrder   walk.SortOrder
	filter      string
}

// NewSyncListModel 创建新的同步列表模型
func NewSyncListModel(syncService interfaces.ClientSyncService, logger interfaces.Logger) *SyncListModel {
	return &SyncListModel{
		syncService: syncService,
		logger:      logger,
	}
}

// RowCount 返回行数
func (m *SyncListModel) RowCount() int {
	if m.syncService == nil {
		return 0
	}
	config := m.syncService.GetCurrentConfig()
	if config == nil {
		return 0
	}
	return len(config.SyncFolders)
}

// ColumnCount 返回列数
func (m *SyncListModel) ColumnCount() int {
	return 3
}

// ColumnTitle 返回列标题
func (m *SyncListModel) ColumnTitle(col int) string {
	switch col {
	case 0:
		return "路径"
	case 1:
		return "同步模式"
	case 2:
		return "重定向路径"
	default:
		return ""
	}
}

// Value 返回单元格值
func (m *SyncListModel) Value(row, col int) interface{} {
	if m.syncService == nil {
		return nil
	}
	config := m.syncService.GetCurrentConfig()
	if config == nil || row >= len(config.SyncFolders) {
		return nil
	}

	folder := config.SyncFolders[row]
	switch col {
	case 0:
		return folder.Path
	case 1:
		return string(folder.SyncMode)
	case 2:
		return ""
	default:
		return nil
	}
}

// Sort 排序
func (m *SyncListModel) Sort(col int, order walk.SortOrder) error {
	m.sortColumn = col
	m.sortOrder = order
	m.PublishRowsReset()
	return nil
}

// PublishRowsReset 通知行重置
func (m *SyncListModel) PublishRowsReset() {
	m.TableModelBase.PublishRowsReset()
}
