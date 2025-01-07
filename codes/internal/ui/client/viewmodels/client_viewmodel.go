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
	"synctools/codes/internal/ui/shared"
)

//
// -------------------- 类型定义 --------------------
//

// MainViewModel 客户端主视图模型
type MainViewModel struct {
	// 服务
	syncService interfaces.ClientSyncService
	logger      interfaces.Logger
	window      *walk.MainWindow

	// 输入框
	addressEdit  interfaces.LineEditIface // 服务器地址
	portEdit     interfaces.LineEditIface // 服务器端口
	syncPathEdit interfaces.LineEditIface // 同步路径

	// 按钮
	connectButton    *walk.PushButton // 连接按钮
	disconnectButton *walk.PushButton // 断开按钮
	saveButton       *walk.PushButton // 保存按钮
	browseButton     *walk.PushButton // 浏览按钮
	syncButton       *walk.PushButton // 同步按钮

	// 状态显示
	progressBar *walk.ProgressBar   // 进度条
	serverInfo  *walk.TextLabel     // 服务器信息
	StatusBar   *walk.StatusBarItem // 状态栏

	// 表格组件
	syncTable interfaces.TableViewIface
	syncList  *shared.TableModel

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
	}

	// 创建表格模型
	vm.syncList = shared.NewTableModel([]shared.TableColumn{
		{
			Title: "路径",
			Width: 200,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					return folder.Path
				}
				return nil
			},
		},
		{
			Title: "同步模式",
			Width: 100,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					return string(folder.SyncMode)
				}
				return nil
			},
		},
		{
			Title: "状态",
			Width: 100,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					if folder.IsEnabled {
						return "启用"
					}
					return "禁用"
				}
				return nil
			},
		},
	}, syncService, logger)

	// 设置数据源
	vm.syncList.SetDataSource(func() []interface{} {
		config := vm.GetCurrentConfig()
		if config == nil {
			return nil
		}
		rows := make([]interface{}, len(config.SyncFolders))
		for i := range config.SyncFolders {
			rows[i] = &config.SyncFolders[i]
		}
		return rows
	})

	// 设置排序函数
	vm.syncList.SetCompareSource(func(i, j int) bool {
		rows := vm.syncList.GetRows()
		if rows == nil {
			return false
		}
		a, ok1 := rows[i].(*interfaces.SyncFolder)
		b, ok2 := rows[j].(*interfaces.SyncFolder)
		if !ok1 || !ok2 {
			return false
		}

		col, order := vm.syncList.GetSortInfo()
		var less bool
		switch col {
		case 0: // 路径
			less = a.Path < b.Path
		case 1: // 同步模式
			less = string(a.SyncMode) < string(b.SyncMode)
		case 2: // 状态
			less = a.IsEnabled && !b.IsEnabled
		default:
			return false
		}

		if order == walk.SortDescending {
			return !less
		}
		return less
	})

	// 设置连接丢失回调
	vm.syncService.SetConnectionLostCallback(vm.handleConnectionLost)

	return vm
}

// Initialize 初始化视图模型
func (vm *MainViewModel) Initialize(window *walk.MainWindow) error {
	vm.window = window
	vm.logger.Debug("视图模型初始化完成", interfaces.Fields{})
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
	addrEdit, portEdit interfaces.LineEditIface,
	progress *walk.ProgressBar,
	saveBtn *walk.PushButton,
	syncPathEdit interfaces.LineEditIface,
	browseBtn *walk.PushButton,
	syncBtn *walk.PushButton,
	syncTable interfaces.TableViewIface,
	statusBar *walk.StatusBarItem,
) {
	vm.connectButton = connectBtn
	vm.addressEdit = addrEdit
	vm.portEdit = portEdit
	vm.progressBar = progress
	vm.saveButton = saveBtn
	vm.syncPathEdit = syncPathEdit
	vm.browseButton = browseBtn
	vm.syncButton = syncBtn
	vm.syncTable = syncTable
	vm.StatusBar = statusBar

	// 设置表格模型
	if vm.syncTable != nil {
		vm.syncTable.SetModel(vm.syncList)
	}

	// 从配置中读取服务器地址和端口
	if vm.syncService != nil {
		if config := vm.syncService.GetCurrentConfig(); config != nil {
			vm.addressEdit.SetText(config.Host)
			vm.portEdit.SetText(fmt.Sprintf("%d", config.Port))
			vm.syncPathEdit.SetText(config.SyncDir)
			vm.logger.Debug("从配置加载服务器信息", interfaces.Fields{
				"host":     config.Host,
				"port":     config.Port,
				"syncPath": config.SyncDir,
			})
		}
	}

	vm.UpdateUIState()
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
			vm.StatusBar.SetText(fmt.Sprintf("已连接到 %s:%s", vm.addressEdit.Text(), vm.portEdit.Text()))
		} else {
			vm.connectButton.SetText("连接服务器")
			vm.StatusBar.SetText("未连接")
		}

		// 更新输入框值
		vm.addressEdit.SetText(vm.addressEdit.Text())
		vm.portEdit.SetText(vm.portEdit.Text())
		vm.syncPathEdit.SetText(vm.syncPathEdit.Text())

		// 更新输入框状态
		vm.addressEdit.SetEnabled(!isConnected)
		vm.portEdit.SetEnabled(!isConnected)
		vm.syncPathEdit.SetEnabled(!isConnected)

		// 更新保存按钮状态
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
			vm.syncList.RefreshCache()
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

	config.Host = vm.addressEdit.Text()
	config.Port = port
	config.SyncDir = vm.syncPathEdit.Text()

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
	if vm.addressEdit.Text() == "" {
		return fmt.Errorf("服务器地址不能为空")
	}

	// 检查端口号
	port := vm.parsePort()
	if port <= 0 || port > 65535 {
		return fmt.Errorf("无效的端口号: %s", vm.portEdit.Text())
	}

	// 检查同步路径
	if vm.syncPathEdit.Text() == "" {
		return fmt.Errorf("同步路径不能为空")
	}

	// 检查同步路径是否存在
	if _, err := os.Stat(vm.syncPathEdit.Text()); os.IsNotExist(err) {
		return fmt.Errorf("同步路径不存在: %s", vm.syncPathEdit.Text())
	}

	// 尝试连接服务器
	if err := vm.syncService.Connect(vm.addressEdit.Text(), vm.portEdit.Text()); err != nil {
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
	_, err := fmt.Sscanf(vm.portEdit.Text(), "%d", &port)
	if err != nil {
		vm.logger.Error("解析端口号失败", interfaces.Fields{
			"port":  vm.portEdit.Text(),
			"error": err,
		})
		return 0
	}
	return port
}

// BrowseSyncDir 浏览同步目录
func (vm *MainViewModel) BrowseSyncDir() error {
	dlg := walk.FileDialog{
		Title:          "选择同步目录",
		InitialDirPath: "::{20D04FE0-3AEA-1069-A2D8-08002B30309D}",
	}

	if ok, err := dlg.ShowBrowseFolder(vm.window); err != nil {
		return err
	} else if !ok {
		return nil
	}

	vm.syncPathEdit.SetText(dlg.FilePath)
	return nil
}
