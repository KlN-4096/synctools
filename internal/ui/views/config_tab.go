package views

import (
	"fmt"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"synctools/internal/ui/viewmodels"
)

// ConfigTab 配置界面
type ConfigTab struct {
	*walk.TabPage

	// UI 组件
	configTable     *walk.TableView
	redirectTable   *walk.TableView
	StatusBar       *walk.StatusBarItem
	nameEdit        *walk.LineEdit
	versionEdit     *walk.LineEdit
	hostEdit        *walk.LineEdit
	portEdit        *walk.LineEdit
	syncDirEdit     *walk.LineEdit
	ignoreEdit      *walk.TextEdit
	syncFolderTable *walk.TableView

	viewModel *viewmodels.ConfigViewModel
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
		Layout: VBox{},
		Children: []Widget{
			HSplitter{
				Children: []Widget{
					GroupBox{
						Title:  "配置列表",
						Layout: VBox{},
						Children: []Widget{
							TableView{
								AssignTo:         &t.configTable,
								CheckBoxes:       true,
								ColumnsOrderable: true,
								MultiSelection:   false,
								Columns: []TableViewColumn{
									{Title: "名称", Width: 150},
									{Title: "版本", Width: 100},
									{Title: "同步目录", Width: 200},
								},
								OnItemActivated: func() {
									// 设置列宽
									width := t.configTable.Width()
									columns := t.configTable.Columns()
									columns.At(0).SetWidth(int(float64(width) * 0.3))  // 名称列占30%
									columns.At(1).SetWidth(int(float64(width) * 0.2))  // 版本列占20%
									columns.At(2).SetWidth(int(float64(width) * 0.45)) // 同步目录列占50%

									// 处理配置选择
									t.onConfigActivated()
								},
								OnSizeChanged: func() {
									// 设置列宽
									width := t.configTable.Width()
									columns := t.configTable.Columns()
									columns.At(0).SetWidth(int(float64(width) * 0.3))  // 名称列占30%
									columns.At(1).SetWidth(int(float64(width) * 0.2))  // 版本列占20%
									columns.At(2).SetWidth(int(float64(width) * 0.45)) // 同步目录列占50%
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text:      "新建",
										OnClicked: t.onNewConfig,
									},
									PushButton{
										Text:      "删除",
										OnClicked: t.onDeleteConfig,
									},
								},
							},
						},
					},
					GroupBox{
						Title:  "配置详情",
						Layout: Grid{Columns: 2},
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
								Layout: HBox{},
								Children: []Widget{
									LineEdit{AssignTo: &t.syncDirEdit, ReadOnly: true},
									PushButton{
										Text:      "浏览",
										OnClicked: t.onBrowseDir,
									},
								},
							},
							Label{Text: "忽略列表:"},
							TextEdit{AssignTo: &t.ignoreEdit},
							Label{Text: "同步文件夹:"},
							Composite{
								Layout: VBox{},
								Children: []Widget{
									TableView{
										AssignTo:         &t.syncFolderTable,
										ColumnsOrderable: true,
										Columns: []TableViewColumn{
											{Title: "文件夹名称", Width: 150},
											{Title: "同步模式", Width: 100},
											{Title: "是否有效", Width: 80},
										},
										OnItemActivated: func() {
											width := t.syncFolderTable.Width()
											columns := t.syncFolderTable.Columns()
											columns.At(0).SetWidth(int(float64(width) * 0.45)) // 路径列占55%
											columns.At(1).SetWidth(int(float64(width) * 0.25)) // 模式列占25%
											columns.At(2).SetWidth(int(float64(width) * 0.25)) // 有效性列占15%
										},
										OnSizeChanged: func() {
											width := t.syncFolderTable.Width()
											columns := t.syncFolderTable.Columns()
											columns.At(0).SetWidth(int(float64(width) * 0.45)) // 路径列占55%
											columns.At(1).SetWidth(int(float64(width) * 0.25)) // 模式列占25%
											columns.At(2).SetWidth(int(float64(width) * 0.25)) // 有效性列占15%
										},
									},
									Composite{
										Layout: HBox{},
										Children: []Widget{
											PushButton{
												Text:      "添加",
												OnClicked: t.onAddSyncFolder,
											},
											PushButton{
												Text:      "删除",
												OnClicked: t.onDeleteSyncFolder,
											},
											PushButton{
												Text:      "刷新",
												OnClicked: t.onRefreshSyncFolders,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			GroupBox{
				Title:  "文件夹重定向",
				Layout: VBox{},
				Children: []Widget{
					TableView{
						AssignTo:         &t.redirectTable,
						ColumnsOrderable: true,
						Columns: []TableViewColumn{
							{Title: "服务器路径", Width: 200},
							{Title: "客户端路径", Width: 200},
						},
						OnItemActivated: func() {
							width := t.redirectTable.Width()
							columns := t.redirectTable.Columns()
							columns.At(0).SetWidth(int(float64(width) * 0.5))  // 服务器路径列占50%
							columns.At(1).SetWidth(int(float64(width) * 0.45)) // 客户端路径列占50%
						},
						OnSizeChanged: func() {
							width := t.redirectTable.Width()
							columns := t.redirectTable.Columns()
							columns.At(0).SetWidth(int(float64(width) * 0.5))  // 服务器路径列占50%
							columns.At(1).SetWidth(int(float64(width) * 0.45)) // 客户端路径列占50%
						},
					},
					Composite{
						Layout: HBox{},
						Children: []Widget{
							PushButton{
								Text:      "添加",
								OnClicked: t.onAddRedirect,
							},
							PushButton{
								Text:      "删除",
								OnClicked: t.onDeleteRedirect,
							},
						},
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					PushButton{
						Text:      "保存",
						OnClicked: t.onSave,
					},
					PushButton{
						Text:      "启动服务器",
						OnClicked: t.onStartServer,
					},
					PushButton{
						Text:      "停止服务器",
						OnClicked: t.onStopServer,
					},
				},
			},
		},
	}.Create(NewBuilder(t.TabPage))); err != nil {
		return err
	}

	// 设置UI组件
	t.viewModel.SetupUI(
		t.configTable,
		t.redirectTable,
		t.StatusBar,
		t.nameEdit,
		t.versionEdit,
		t.hostEdit,
		t.portEdit,
		t.syncDirEdit,
		t.ignoreEdit,
		t.syncFolderTable,
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
	dlg.SetLayout(walk.NewVBoxLayout())

	var nameEdit *walk.LineEdit
	var versionEdit *walk.LineEdit

	if err := (Composite{
		Layout: Grid{Columns: 2},
		Children: []Widget{
			Label{Text: "整合包名称:"},
			LineEdit{AssignTo: &nameEdit, Text: "新整合包"},
			Label{Text: "版本:"},
			LineEdit{AssignTo: &versionEdit, Text: "1.0.0"},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	if err := (Composite{
		Layout: HBox{},
		Children: []Widget{
			HSpacer{},
			PushButton{
				Text: "确定",
				OnClicked: func() {
					if err := t.viewModel.CreateConfig(nameEdit.Text(), versionEdit.Text()); err != nil {
						walk.MsgBox(dlg, "错误", err.Error(), walk.MsgBoxIconError)
						return
					}
					dlg.Accept()
				},
			},
			PushButton{
				Text:      "取消",
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
	}
}

// onBrowseDir 浏览目录
func (t *ConfigTab) onBrowseDir() {
	if err := t.viewModel.BrowseSyncDir(); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
	}
}

// onAddRedirect 添加重定向
func (t *ConfigTab) onAddRedirect() {
	dlg, err := walk.NewDialog(t.Form())
	if err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer dlg.Dispose()

	dlg.SetTitle("添加重定向")
	dlg.SetLayout(walk.NewVBoxLayout())

	var serverEdit *walk.LineEdit
	var clientEdit *walk.LineEdit

	if err := (Composite{
		Layout: Grid{Columns: 2},
		Children: []Widget{
			Label{Text: "服务器路径:"},
			LineEdit{AssignTo: &serverEdit},
			Label{Text: "客户端路径:"},
			LineEdit{AssignTo: &clientEdit},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	if err := (Composite{
		Layout: HBox{},
		Children: []Widget{
			HSpacer{},
			PushButton{
				Text: "确定",
				OnClicked: func() {
					if err := t.viewModel.AddRedirect(serverEdit.Text(), clientEdit.Text()); err != nil {
						walk.MsgBox(dlg, "错误", err.Error(), walk.MsgBoxIconError)
						return
					}
					dlg.Accept()
				},
			},
			PushButton{
				Text:      "取消",
				OnClicked: dlg.Cancel,
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	dlg.Run()
}

// onDeleteRedirect 删除重定向
func (t *ConfigTab) onDeleteRedirect() {
	index := t.redirectTable.CurrentIndex()
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
		fmt.Sprintf("确定要删除重定向 '%s -> %s' 吗？",
			config.FolderRedirects[index].ServerPath,
			config.FolderRedirects[index].ClientPath),
		walk.MsgBoxYesNo,
	) == walk.DlgCmdYes {
		if err := t.viewModel.DeleteRedirect(index); err != nil {
			walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
			return
		}
	}
}

// onSave 保存配置
func (t *ConfigTab) onSave() {
	if err := t.viewModel.SaveConfig(); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
	}
}

// onStartServer 启动服务器
func (t *ConfigTab) onStartServer() {
	if err := t.viewModel.StartServer(); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
	}
}

// onStopServer 停止服务器
func (t *ConfigTab) onStopServer() {
	if err := t.viewModel.StopServer(); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
	}
}

// onAddSyncFolder 添加同步文件夹
func (t *ConfigTab) onAddSyncFolder() {
	dlg, err := walk.NewDialog(t.Form())
	if err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer dlg.Dispose()

	dlg.SetTitle("添加同步文件夹")
	dlg.SetLayout(walk.NewVBoxLayout())

	var pathEdit *walk.LineEdit
	var modeComboBox *walk.ComboBox

	if err := (Composite{
		Layout: Grid{Columns: 2},
		Children: []Widget{
			Label{Text: "文件夹路径:"},
			LineEdit{AssignTo: &pathEdit},
			Label{Text: "同步模式:"},
			ComboBox{
				AssignTo: &modeComboBox,
				Model:    []string{"mirror", "push"},
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	if err := (Composite{
		Layout: HBox{},
		Children: []Widget{
			HSpacer{},
			PushButton{
				Text: "确定",
				OnClicked: func() {
					if err := t.viewModel.AddSyncFolder(pathEdit.Text(), modeComboBox.Text()); err != nil {
						walk.MsgBox(dlg, "错误", err.Error(), walk.MsgBoxIconError)
						return
					}
					dlg.Accept()
				},
			},
			PushButton{
				Text:      "取消",
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
	}
}

// onRefreshSyncFolders 刷新同步文件夹
func (t *ConfigTab) onRefreshSyncFolders() {
	if err := t.viewModel.RefreshSyncFolders(); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
	}
}
