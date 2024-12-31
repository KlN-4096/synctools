/*
文件作用:
- 实现配置界面的UI布局和交互
- 管理配置界面的各个控件
- 处理用户界面事件
- 与视图模型层交互

主要方法:
- NewConfigTab: 创建新的配置界面
- Setup: 设置UI组件和布局
- onConfigActivated: 处理配置选择事件
- onNewConfig: 处理新建配置事件
- onDeleteConfig: 处理删除配置事件
- onSave: 处理保存配置事件
*/

package views

import (
	"fmt"
	"strings"
	"sync"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"synctools/codes/internal/interfaces"
	"synctools/codes/internal/ui/server/viewmodels"
)

// ConfigTab 配置界面
type ConfigTab struct {
	*walk.TabPage

	// UI 组件
	configTable         *walk.TableView
	StatusBar           *walk.StatusBarItem
	nameEdit            *walk.LineEdit
	versionEdit         *walk.LineEdit
	hostEdit            *walk.LineEdit
	portEdit            *walk.LineEdit
	browseSyncDirButton *walk.PushButton
	syncDirEdit         *walk.LineEdit
	ignoreEdit          *walk.TextEdit
	syncFolderTable     *walk.TableView
	startServerButton   *walk.PushButton
	saveButton          *walk.PushButton
	newConfigButton     *walk.PushButton
	delConfigButton     *walk.PushButton
	addSyncFolderButton *walk.PushButton
	delSyncFolderButton *walk.PushButton

	viewModel *viewmodels.ConfigViewModel

	// 互斥锁，用于防止并发调整列宽
	columnMutex sync.Mutex
}

// NewConfigTab 创建新的配置界面
func NewConfigTab(viewModel *viewmodels.ConfigViewModel) (*ConfigTab, error) {
	tab := &ConfigTab{
		viewModel: viewModel,
	}
	return tab, nil
}

// Setup 设置UI组件
func (t *ConfigTab) Setup() error {
	if err := (Composite{
		Layout: VBox{MarginsZero: true},
		Children: []Widget{
			HSplitter{
				Children: []Widget{
					// 左侧面板：配置列表
					GroupBox{
						Title:  "配置列表",
						Layout: VBox{Margins: Margins{Top: 10, Left: 10, Right: 10, Bottom: 10}},
						Children: []Widget{
							TableView{
								AssignTo:         &t.configTable,
								CheckBoxes:       true,
								ColumnsOrderable: true,
								MultiSelection:   false,
								MinSize:          Size{Width: 300},
								Columns: []TableViewColumn{
									{Title: "名称", Width: 150},
									{Title: "版本", Width: 100},
									{Title: "同步目录", Width: 200},
								},
								OnItemActivated: func() {
									t.adjustTableColumns(t.configTable, []float64{0.3, 0.2, 0.45})
									t.onConfigActivated()
								},
								OnSizeChanged: func() {
									t.adjustTableColumns(t.configTable, []float64{0.3, 0.2, 0.45})
								},
							},
							Composite{
								Layout: HBox{MarginsZero: true, Spacing: 5},
								Children: []Widget{
									PushButton{
										Text:      "保存配置",
										AssignTo:  &t.saveButton,
										MinSize:   Size{Width: 80},
										OnClicked: t.onSave,
									},
									PushButton{
										Text:      "新建配置",
										AssignTo:  &t.newConfigButton,
										MinSize:   Size{Width: 80},
										OnClicked: t.onNewConfig,
									},
									PushButton{
										Text:      "删除配置",
										AssignTo:  &t.delConfigButton,
										MinSize:   Size{Width: 80},
										OnClicked: t.onDeleteConfig,
									},
								},
							},
						},
					},
					// 右侧面板：配置详情
					Composite{
						Layout: VBox{Margins: Margins{Top: 10, Left: 5, Right: 10, Bottom: 10}},
						Children: []Widget{
							// 基础配置组
							GroupBox{
								Title:  "基础配置",
								Layout: Grid{Columns: 2, Spacing: 10},
								Children: []Widget{
									Label{Text: "整合包名称:"},
									LineEdit{AssignTo: &t.nameEdit},
									Label{Text: "整合包版本:"},
									LineEdit{AssignTo: &t.versionEdit},
									Label{Text: "主机地址:"},
									LineEdit{AssignTo: &t.hostEdit},
									Label{Text: "端口:"},
									LineEdit{AssignTo: &t.portEdit},
									Label{Text: "同步目录:"},
									Composite{
										Layout: HBox{Spacing: 5},
										Children: []Widget{
											LineEdit{
												AssignTo: &t.syncDirEdit,
												ReadOnly: true,
											},
											PushButton{
												Text:      "...",
												AssignTo:  &t.browseSyncDirButton,
												MaxSize:   Size{Width: 30},
												OnClicked: t.onBrowseDir,
											},
										},
									},
								},
							},
							// 服务器控制组
							GroupBox{
								Title:  "服务器控制",
								Layout: HBox{Margins: Margins{Top: 5, Bottom: 5}},
								Children: []Widget{
									HSpacer{},
									PushButton{
										AssignTo:  &t.startServerButton,
										Text:      "启动服务器",
										MinSize:   Size{Width: 100},
										OnClicked: t.onServerControl,
									},
									HSpacer{},
								},
							},
							// 同步配置组
							GroupBox{
								Title:  "同步配置",
								Layout: VBox{Spacing: 5},
								Children: []Widget{
									// 忽略文件列表
									Label{Text: "忽略文件列表:"},
									TextEdit{
										AssignTo: &t.ignoreEdit,
										MinSize:  Size{Height: 60},
										VScroll:  true,
										ToolTipText: "支持两种模式:\n" +
											"1. 文件名模式 - 无论在哪个目录下都忽略匹配的文件\n" +
											"   示例: *.txt, test.dat, temp.*\n" +
											"2. 路径模式 - 忽略指定路径下的所有文件\n" +
											"   示例: mods/custom/, config/test/\n" +
											"通配符说明:\n" +
											"* - 匹配任意字符序列\n" +
											"? - 匹配任意单个字符\n" +
											"[abc] - 匹配括号内任意字符",
									},
									// 同步文件/文件夹表格
									Label{Text: "同步项目:"},
									TableView{
										AssignTo:         &t.syncFolderTable,
										ColumnsOrderable: true,
										MinSize:          Size{Height: 150},
										Columns: []TableViewColumn{
											{Title: "路径", Width: 150},
											{Title: "同步模式", Width: 100},
											{Title: "重定向路径", Width: 150},
											{Title: "是否有效", Width: 80},
										},
										OnItemActivated: func() {
											t.adjustTableColumns(t.syncFolderTable, []float64{0.35, 0.2, 0.3, 0.15})
											t.onSyncFolderEdit()
										},
										OnSizeChanged: func() {
											t.adjustTableColumns(t.syncFolderTable, []float64{0.35, 0.2, 0.3, 0.15})
										},
									},
									Composite{
										Layout: HBox{Spacing: 5},
										Children: []Widget{
											HSpacer{},
											PushButton{
												Text:      "添加文件夹",
												AssignTo:  &t.addSyncFolderButton,
												MinSize:   Size{Width: 80},
												OnClicked: t.onAddSyncFolder,
											},
											PushButton{
												Text:      "删除文件夹",
												AssignTo:  &t.delSyncFolderButton,
												MinSize:   Size{Width: 80},
												OnClicked: t.onDeleteSyncFolder,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}.Create(NewBuilder(t.TabPage))); err != nil {
		return fmt.Errorf("创建配置界面失败: %v", err)
	}

	// 设置UI组件
	t.viewModel.SetupUI(
		t.configTable,
		nil,
		t.StatusBar,
		t.nameEdit,
		t.versionEdit,
		t.hostEdit,
		t.portEdit,
		t.browseSyncDirButton,
		t.syncDirEdit,
		t.ignoreEdit,
		t.syncFolderTable,
		t.startServerButton,
		t.saveButton,
		t.newConfigButton,
		t.delConfigButton,
		t.addSyncFolderButton,
		t.delSyncFolderButton,
	)

	return nil
}

// Activating 实现 walk.Form 接口
func (t *ConfigTab) Activating() bool {
	return true
}

// onConfigActivated 配置项被激活
func (t *ConfigTab) onConfigActivated() {
	index := t.configTable.CurrentIndex()
	if index < 0 {
		return
	}

	configs, err := t.viewModel.ListConfigs()
	if err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	if err := t.viewModel.LoadConfig(configs[index].UUID); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
}

// onNewConfig 新建配置
func (t *ConfigTab) onNewConfig() {
	dlg, err := walk.NewDialog(t.Form())
	if err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer dlg.Dispose()

	dlg.SetTitle("新建配置")
	dlg.SetSize(walk.Size{Width: 400, Height: 200})
	dlg.SetLayout(walk.NewVBoxLayout())

	var nameEdit *walk.LineEdit
	var versionEdit *walk.LineEdit

	if err := (Composite{
		Layout: Grid{Columns: 2, Spacing: 10},
		Children: []Widget{
			Label{Text: "整合包名称:"},
			LineEdit{
				AssignTo: &nameEdit,
				Text:     "新整合包",
				MinSize:  Size{Width: 200},
			},
			Label{Text: "版本:"},
			LineEdit{
				AssignTo: &versionEdit,
				Text:     "1.0.0",
				MinSize:  Size{Width: 200},
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	if err := (Composite{
		Layout: HBox{Spacing: 10},
		Children: []Widget{
			HSpacer{},
			PushButton{
				Text:    "确定",
				MinSize: Size{Width: 70},
				OnClicked: func() {
					if err := t.viewModel.CreateConfig(nameEdit.Text(), versionEdit.Text()); err != nil {
						walk.MsgBox(dlg, "错误", err.Error(), walk.MsgBoxIconError)
						return
					}
					dlg.Accept()
					t.viewModel.UpdateUI()
				},
			},
			PushButton{
				Text:      "取消",
				MinSize:   Size{Width: 70},
				OnClicked: dlg.Cancel,
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	dlg.Run()
}

// onDeleteConfig 删除配置
func (t *ConfigTab) onDeleteConfig() {
	index := t.configTable.CurrentIndex()
	if index < 0 {
		return
	}

	configs, err := t.viewModel.ListConfigs()
	if err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	if walk.MsgBox(
		t.Form(),
		"确认删除",
		fmt.Sprintf("确定要删除配置 '%s' 吗？", configs[index].Name),
		walk.MsgBoxYesNo,
	) == walk.DlgCmdYes {
		if err := t.viewModel.DeleteConfig(configs[index].UUID); err != nil {
			walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
			return
		}
		t.viewModel.UpdateUI()
	}
}

// onBrowseDir 浏览目录
func (t *ConfigTab) onBrowseDir() {
	if err := t.viewModel.BrowseSyncDir(); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	// 先保存配置然后再更新UI
	t.onSave()
	t.viewModel.UpdateUI()
}

// onSave 保存配置
func (t *ConfigTab) onSave() {
	if err := t.viewModel.SaveConfig(); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	walk.MsgBox(t.Form(), "提示", "配置已保存", walk.MsgBoxIconInformation)
	t.viewModel.UpdateUI()
}

// onServerControl 服务器控制
func (t *ConfigTab) onServerControl() {
	if t.viewModel.IsServerRunning() {
		if err := t.viewModel.StopServer(); err != nil {
			walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
			return
		}
	} else {
		if err := t.viewModel.StartServer(); err != nil {
			walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
			return
		}
	}
	t.viewModel.UpdateUI()
}

// onAddSyncFolder 添加同步文件夹
func (t *ConfigTab) onAddSyncFolder() {
	dlg, err := walk.NewDialog(t.Form())
	if err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer dlg.Dispose()

	dlg.SetTitle("添加同步项目")
	dlg.SetSize(walk.Size{Width: 400, Height: 200})
	dlg.SetLayout(walk.NewVBoxLayout())

	var pathEdit *walk.LineEdit
	var modeComboBox *walk.ComboBox
	var redirectPathEdit *walk.LineEdit

	if err := (Composite{
		Layout: Grid{Columns: 2, Spacing: 10},
		Children: []Widget{
			Label{Text: "路径:"},
			LineEdit{
				AssignTo: &pathEdit,
				ToolTipText: "输入要同步的文件或文件夹路径\n" +
					"例如: clientmods",
			},
			Label{Text: "同步模式:"},
			ComboBox{
				AssignTo:     &modeComboBox,
				Model:        []string{"mirror", "push", "pack"},
				CurrentIndex: 0,
			},
			Label{Text: "重定向路径:"},
			LineEdit{
				AssignTo: &redirectPathEdit,
				ToolTipText: "输入重定向的目标路径\n" +
					"例如: mods",
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	if err := (Composite{
		Layout: HBox{Spacing: 10},
		Children: []Widget{
			HSpacer{},
			PushButton{
				Text:    "确定",
				MinSize: Size{Width: 70},
				OnClicked: func() {
					if interfaces.SyncMode(modeComboBox.Text()) == interfaces.PackSync &&
						!strings.HasSuffix(strings.ToLower(redirectPathEdit.Text()), ".zip") {
						walk.MsgBox(dlg, "格式错误", "打包同步模式下，目标文件必须是ZIP压缩包格式", walk.MsgBoxIconWarning)
						return
					}

					if err := t.viewModel.AddSyncFolder(pathEdit.Text(), interfaces.SyncMode(modeComboBox.Text()), redirectPathEdit.Text()); err != nil {
						walk.MsgBox(dlg, "错误", err.Error(), walk.MsgBoxIconError)
						return
					}
					dlg.Accept()
					t.viewModel.UpdateUI()
				},
			},
			PushButton{
				Text:      "取消",
				MinSize:   Size{Width: 70},
				OnClicked: dlg.Cancel,
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	dlg.Run()
}

// onDeleteSyncFolder 删除同步文件夹
func (t *ConfigTab) onDeleteSyncFolder() {
	index := t.syncFolderTable.CurrentIndex()
	if index < 0 {
		return
	}

	config := t.viewModel.GetCurrentConfig()
	if config == nil {
		return
	}

	if walk.MsgBox(
		t.Form(),
		"确认删除",
		fmt.Sprintf("确定要删除同步文件夹 '%s' 吗？", config.SyncFolders[index].Path),
		walk.MsgBoxYesNo,
	) == walk.DlgCmdYes {
		if err := t.viewModel.DeleteSyncFolder(index); err != nil {
			walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
			return
		}
		t.viewModel.UpdateUI()
	}
}

// adjustTableColumns 调整表格列宽
func (t *ConfigTab) adjustTableColumns(table *walk.TableView, widthPercentages []float64) {
	t.columnMutex.Lock()
	defer t.columnMutex.Unlock()

	if table == nil {
		return
	}

	columns := table.Columns()
	if columns == nil {
		return
	}

	width := table.Width()
	if width <= 0 {
		return
	}

	// 使用 defer 来处理可能的异常
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Error adjusting column widths: %v\n", r)
		}
	}()

	// 检查列数是否匹配
	if columns.Len() != len(widthPercentages) {
		return
	}

	// 设置每列的宽度
	for i, percentage := range widthPercentages {
		if i < columns.Len() {
			columns.At(i).SetWidth(int(float64(width) * percentage))
		}
	}
}

// onSyncFolderEdit 处理同步文件夹编辑
func (t *ConfigTab) onSyncFolderEdit() {
	if t.syncFolderTable == nil {
		return
	}

	index := t.syncFolderTable.CurrentIndex()
	if index < 0 {
		return
	}

	config := t.viewModel.GetCurrentConfig()
	if config == nil {
		return
	}

	dlg, err := walk.NewDialog(t.Form())
	if err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer dlg.Dispose()

	dlg.SetTitle("编辑同步项目")
	dlg.SetSize(walk.Size{Width: 400, Height: 200})
	dlg.SetLayout(walk.NewVBoxLayout())

	var pathEdit *walk.LineEdit
	var modeComboBox *walk.ComboBox
	var redirectPathEdit *walk.LineEdit

	if err := (Composite{
		Layout: Grid{Columns: 2, Spacing: 10},
		Children: []Widget{
			Label{Text: "路径:"},
			LineEdit{
				AssignTo: &pathEdit,
				Text:     config.SyncFolders[index].Path,
				ToolTipText: "输入要同步的文件或文件夹路径\n" +
					"例如: clientmods",
			},
			Label{Text: "同步模式:"},
			ComboBox{
				AssignTo: &modeComboBox,
				Model:    []string{"mirror", "push", "pack"},
				CurrentIndex: func() int {
					switch config.SyncFolders[index].SyncMode {
					case interfaces.PushSync:
						return 1
					case interfaces.PackSync:
						return 2
					default:
						return 0
					}
				}(),
			},
			Label{Text: "重定向路径:"},
			LineEdit{
				AssignTo: &redirectPathEdit,
				Text: func() string {
					for _, redirect := range config.FolderRedirects {
						if redirect.ServerPath == config.SyncFolders[index].Path {
							return redirect.ClientPath
						}
					}
					return ""
				}(),
				ToolTipText: "输入重定向的目标路径\n" +
					"例如: mods",
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	if err := (Composite{
		Layout: HBox{Spacing: 10},
		Children: []Widget{
			HSpacer{},
			PushButton{
				Text:    "确定",
				MinSize: Size{Width: 70},
				OnClicked: func() {
					if interfaces.SyncMode(modeComboBox.Text()) == interfaces.PackSync &&
						!strings.HasSuffix(strings.ToLower(redirectPathEdit.Text()), ".zip") {
						walk.MsgBox(dlg, "格式错误", "打包同步模式下，目标文件必须是ZIP压缩包格式", walk.MsgBoxIconWarning)
						return
					}

					if err := t.viewModel.UpdateSyncFolder(index, pathEdit.Text(), interfaces.SyncMode(modeComboBox.Text()), redirectPathEdit.Text()); err != nil {
						walk.MsgBox(dlg, "错误", err.Error(), walk.MsgBoxIconError)
						return
					}
					dlg.Accept()
					t.viewModel.UpdateUI()
				},
			},
			PushButton{
				Text:      "取消",
				MinSize:   Size{Width: 70},
				OnClicked: dlg.Cancel,
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	dlg.Run()
}
