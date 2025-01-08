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
			Title: "重定向路径",
			Width: 200,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					config := vm.GetCurrentConfig()
					if config != nil {
						for _, redirect := range config.FolderRedirects {
							if redirect.ServerPath == folder.Path {
								return redirect.ClientPath
							}
						}
					}
				}
				return ""
			},
		},
	}, syncService, logger)

	// 设置数据源
	vm.syncList.SetDataSource(func() []interface{} {
		config, _ := vm.syncService.LoadServerConfig()
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
		case 2: // 重定向路径
			less = a.Path < b.Path
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
