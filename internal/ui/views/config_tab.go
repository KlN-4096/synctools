package views

import (
	"fmt"
	"sync"

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
									// 调整列宽
									t.adjustTableColumns(t.configTable, []float64{0.3, 0.2, 0.45})
									// 处理配置选择
									t.onConfigActivated()
								},
								OnSizeChanged: func() {
									// 调整列宽
									t.adjustTableColumns(t.configTable, []float64{0.3, 0.2, 0.45})
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										Text:      "保存配置",
										OnClicked: t.onSave,
									},
									PushButton{
										Text:      "新建配置",
										OnClicked: t.onNewConfig,
									},
									PushButton{
										Text:      "删除配置",
										OnClicked: t.onDeleteConfig,
									},
								},
							},
							GroupBox{
								Title:  "基础配置信息",
								Layout: VBox{},
								Children: []Widget{
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
									Label{Text: "整合包名称:"},
									LineEdit{AssignTo: &t.nameEdit},
									Label{Text: "整合包版本:"},
									LineEdit{AssignTo: &t.versionEdit},
									Label{Text: "主机地址:"},
									LineEdit{AssignTo: &t.hostEdit},
									Label{Text: "端口:"},
									LineEdit{AssignTo: &t.portEdit},
								},
							},
							Composite{
								Layout: HBox{},
								Children: []Widget{
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
					},
					GroupBox{
						Title:  "文件配置",
						Layout: Grid{Columns: 1},
						Children: []Widget{
							Composite{
								Layout: VBox{},
								Children: []Widget{
									Label{Text: "忽略文件列表:"},
									TextEdit{AssignTo: &t.ignoreEdit},
								},
							},
							Composite{
								Layout: VBox{},
								Children: []Widget{
									Label{Text: "同步文件夹:"},
									TableView{
										AssignTo:         &t.syncFolderTable,
										ColumnsOrderable: true,
										Columns: []TableViewColumn{
											{Title: "文件夹名称", Width: 150},
											{Title: "同步模式", Width: 100},
											{Title: "是否有效", Width: 80},
										},
										OnItemActivated: func() {
											// 调整列宽
											t.adjustTableColumns(t.syncFolderTable, []float64{0.45, 0.25, 0.25})

											// 处理双击编辑
											if t.syncFolderTable != nil {
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

												dlg.SetTitle("编辑同步文件夹")
												dlg.SetLayout(walk.NewVBoxLayout())

												var pathEdit *walk.LineEdit
												var modeComboBox *walk.ComboBox

												if err := (Composite{
													Layout: Grid{Columns: 2},
													Children: []Widget{
														Label{Text: "文件夹路径:"},
														LineEdit{
															AssignTo: &pathEdit,
															Text:     config.SyncFolders[index].Path,
														},
														Label{Text: "同步模式:"},
														ComboBox{
															AssignTo: &modeComboBox,
															Model:    []string{"mirror", "push", "pack"},
															CurrentIndex: func() int {
																if config.SyncFolders[index].SyncMode == "push" {
																	return 1
																} else if config.SyncFolders[index].SyncMode == "pack" {
																	return 2
																}
																return 0
															}(),
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
																if err := t.viewModel.UpdateSyncFolder(index, pathEdit.Text(), modeComboBox.Text()); err != nil {
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
										},
										OnSizeChanged: func() {
											// 调整列宽
											t.adjustTableColumns(t.syncFolderTable, []float64{0.45, 0.25, 0.25})
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
										},
									},
								},
							},
							Composite{
								Layout: VBox{},
								Children: []Widget{
									Label{Text: "文件夹重定向:"},
									TableView{
										AssignTo:         &t.redirectTable,
										ColumnsOrderable: true,
										Columns: []TableViewColumn{
											{Title: "服务器路径", Width: 200},
											{Title: "客户端路径", Width: 200},
										},
										OnItemActivated: func() {
											// 调整列宽
											t.adjustTableColumns(t.redirectTable, []float64{0.45, 0.45})

											// 处理双击编辑
											if t.redirectTable != nil {
												index := t.redirectTable.CurrentIndex()
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

												dlg.SetTitle("编辑文件夹重定向")
												dlg.SetLayout(walk.NewVBoxLayout())

												var serverEdit *walk.LineEdit
												var clientEdit *walk.LineEdit

												if err := (Composite{
													Layout: Grid{Columns: 2},
													Children: []Widget{
														Label{Text: "服务器路径:"},
														LineEdit{
															AssignTo: &serverEdit,
															Text:     config.FolderRedirects[index].ServerPath,
														},
														Label{Text: "客户端路径:"},
														LineEdit{
															AssignTo: &clientEdit,
															Text:     config.FolderRedirects[index].ClientPath,
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
																if err := t.viewModel.UpdateRedirect(index, serverEdit.Text(), clientEdit.Text()); err != nil {
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
										},
										OnSizeChanged: func() {
											// 调整列宽
											t.adjustTableColumns(t.redirectTable, []float64{0.45, 0.45})
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
						},
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
				Model:    []string{"mirror", "push", "pack"},
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
