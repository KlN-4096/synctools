package viewmodels

import (
	"os"
	"path/filepath"

	"github.com/lxn/walk"

	"synctools/codes/internal/interfaces"
	"synctools/codes/internal/ui/shared"
)

// ConfigViewModel 配置视图模型
type ConfigViewModel struct {
	syncService interfaces.ServerSyncService
	logger      interfaces.Logger
	window      *walk.MainWindow
	status      string

	// UI 状态
	isEditing     bool
	serverRunning bool // 服务器运行状态标志

	// UI 组件
	configTable     interfaces.TableViewIface
	configList      *shared.TableModel
	redirectTable   interfaces.TableViewIface
	syncFolderTable interfaces.TableViewIface
	syncFolderList  *shared.TableModel
	statusBar       *walk.StatusBarItem

	// 编辑字段
	nameEdit    interfaces.LineEditIface
	versionEdit interfaces.LineEditIface
	hostEdit    interfaces.LineEditIface
	portEdit    interfaces.LineEditIface
	syncDirEdit interfaces.LineEditIface
	ignoreEdit  *walk.TextEdit

	// 按钮
	browseSyncDirButton *walk.PushButton
	startServerButton   *walk.PushButton
	saveButton          *walk.PushButton
	newConfigButton     *walk.PushButton
	delConfigButton     *walk.PushButton
	addSyncFolderButton *walk.PushButton
	delSyncFolderButton *walk.PushButton
}

// NewConfigViewModel 创建新的配置视图模型
func NewConfigViewModel(syncService interfaces.SyncService, logger interfaces.Logger) *ConfigViewModel {
	// 类型转换检查
	serverService, ok := syncService.(interfaces.ServerSyncService)
	if !ok {
		panic("必须提供服务器同步服务实例")
	}

	vm := &ConfigViewModel{
		syncService: serverService,
		logger:      logger,
		status:      "就绪",
	}

	// 创建配置列表模型
	vm.configList = shared.NewTableModel([]shared.TableColumn{
		{
			Title: "名称",
			Width: 150,
			Value: func(row interface{}) interface{} {
				if config, ok := row.(*interfaces.Config); ok {
					return config.Name
				}
				return nil
			},
		},
		{
			Title: "版本",
			Width: 100,
			Value: func(row interface{}) interface{} {
				if config, ok := row.(*interfaces.Config); ok {
					return config.Version
				}
				return nil
			},
		},
		{
			Title: "同步目录",
			Width: 200,
			Value: func(row interface{}) interface{} {
				if config, ok := row.(*interfaces.Config); ok {
					return config.SyncDir
				}
				return nil
			},
		},
	}, syncService, logger)

	// 设置配置列表数据源
	vm.configList.SetDataSource(func() []interface{} {
		configs, err := vm.syncService.ListConfigs()
		if err != nil {
			vm.logger.Error("获取配置列表失败", interfaces.Fields{
				"error": err.Error(),
			})
			return nil
		}

		// 只保留服务器配置
		serverConfigs := make([]interface{}, 0)
		for _, config := range configs {
			if config.Type == interfaces.ConfigTypeServer {
				serverConfigs = append(serverConfigs, config)
			}
		}
		return serverConfigs
	})

	// 设置配置列表排序函数
	vm.configList.SetCompareSource(func(i, j int) bool {
		rows := vm.configList.GetRows()
		if rows == nil {
			return false
		}
		a, ok1 := rows[i].(*interfaces.Config)
		b, ok2 := rows[j].(*interfaces.Config)
		if !ok1 || !ok2 {
			return false
		}

		col, order := vm.configList.GetSortInfo()
		var less bool
		switch col {
		case 0: // 名称
			less = a.Name < b.Name
		case 1: // 版本
			less = a.Version < b.Version
		case 2: // 同步目录
			less = a.SyncDir < b.SyncDir
		default:
			return false
		}

		if order == walk.SortDescending {
			return !less
		}
		return less
	})

	// 创建同步文件夹列表模型
	vm.syncFolderList = shared.NewTableModel([]shared.TableColumn{
		{
			Title: "文件夹名称",
			Width: 150,
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
		{
			Title: "是否有效",
			Width: 80,
			Value: func(row interface{}) interface{} {
				if folder, ok := row.(*interfaces.SyncFolder); ok {
					config := vm.GetCurrentConfig()
					if config != nil {
						if _, err := os.Stat(filepath.Join(config.SyncDir, folder.Path)); os.IsNotExist(err) {
							return "×"
						}
						return "√"
					}
				}
				return "?"
			},
		},
	}, syncService, logger)

	// 设置同步文件夹列表数据源
	vm.syncFolderList.SetDataSource(func() []interface{} {
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

	return vm
}

// Initialize 初始化视图模型
func (vm *ConfigViewModel) Initialize(window *walk.MainWindow) error {
	vm.window = window
	vm.logger.Info("初始化配置视图模型", interfaces.Fields{
		"status": vm.status,
	})

	// 初始化UI状态
	vm.logger.Info("视图操作", interfaces.Fields{
		"action": "initialize",
		"type":   "config",
	})

	// 获取当前配置
	cfg := vm.syncService.GetCurrentConfig()
	if cfg == nil {
		vm.logger.Info("配置状态", interfaces.Fields{
			"status": "empty",
			"reason": "no_default",
		})
	}

	// 更新UI状态
	vm.UpdateUI()
	return nil
}

// LogDebug 记录调试日志
func (vm *ConfigViewModel) LogDebug(message string) {
	vm.logger.Debug(message, nil)
}

// LogError 记录错误日志
func (vm *ConfigViewModel) LogError(message string, err error) {
	vm.logger.Error(message, interfaces.Fields{
		"error": err,
	})
}

// showError 显示错误消息
func (vm *ConfigViewModel) showError(title, message string) {
	if vm.window != nil {
		walk.MsgBox(vm.window, title, message, walk.MsgBoxIconError)
	}
}

// GetLogger 获取日志记录器
func (vm *ConfigViewModel) GetLogger() interfaces.Logger {
	return vm.logger
}

// HandleWindowClosing 处理窗口关闭事件
func (vm *ConfigViewModel) HandleWindowClosing() {
	vm.LogDebug("窗口正在关闭")

	if vm.IsServerRunning() {
		if err := vm.StopServer(); err != nil {
			vm.LogError("停止服务器失败", err)
		}
	}

	if config := vm.GetCurrentConfig(); config != nil {
		if err := vm.SaveConfig(); err != nil {
			vm.LogError("保存配置失败", err)
		}
	}
}
