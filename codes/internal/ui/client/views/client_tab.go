/*
文件作用:
- 实现客户端主界面的UI布局和交互
- 管理客户端界面的各个控件
- 处理用户界面事件
- 与视图模型层交互

主要方法:
- NewClientTab: 创建新的客户端界面
- Setup: 设置UI组件和布局
*/

package views

import (
	"fmt"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"synctools/codes/internal/ui/client/viewmodels"
)

// ClientTab 客户端界面
type ClientTab struct {
	*walk.TabPage

	// UI 组件
	addressEdit   *walk.LineEdit
	portEdit      *walk.LineEdit
	syncPathEdit  *walk.LineEdit
	browseButton  *walk.PushButton
	connectButton *walk.PushButton // 合并后的连接/断开按钮
	syncButton    *walk.PushButton // 添加同步按钮
	progressBar   *walk.ProgressBar
	StatusBar     *walk.StatusBarItem
	saveButton    *walk.PushButton
	serverInfo    *walk.TextLabel // 添加服务器信息标签

	viewModel *viewmodels.MainViewModel
}

// NewClientTab 创建新的客户端界面
func NewClientTab(viewModel *viewmodels.MainViewModel) (*ClientTab, error) {
	tab := &ClientTab{
		viewModel: viewModel,
	}
	return tab, nil
}

// Setup 设置UI组件
func (t *ClientTab) Setup() error {
	if err := (Composite{
		Layout: VBox{MarginsZero: true},
		Children: []Widget{
			GroupBox{
				Title:  "服务器连接",
				Layout: VBox{},
				Children: []Widget{
					Composite{
						Layout: Grid{Columns: 2, Spacing: 10},
						Children: []Widget{
							Label{Text: "服务器地址:"},
							LineEdit{
								AssignTo: &t.addressEdit,
								OnTextChanged: func() {
									t.viewModel.SetServerAddr(t.addressEdit.Text())
								},
							},
							Label{Text: "端口:"},
							LineEdit{
								AssignTo: &t.portEdit,
								OnTextChanged: func() {
									t.viewModel.SetServerPort(t.portEdit.Text())
								},
							},
							Label{Text: "同步目录:"},
							Composite{
								Layout: HBox{Spacing: 5},
								Children: []Widget{
									LineEdit{
										AssignTo: &t.syncPathEdit,
										OnTextChanged: func() {
											t.viewModel.SetSyncPath(t.syncPathEdit.Text())
										},
									},
									PushButton{
										AssignTo: &t.browseButton,
										Text:     "...",
										MaxSize:  Size{Width: 60},
										OnClicked: func() {
											t.onBrowse()
										},
									},
								},
							},
							Label{Text: "服务器信息:"},
							TextLabel{
								AssignTo: &t.serverInfo,
								Text:     "未连接",
							},
						},
					},
					Composite{
						Layout: HBox{Spacing: 5},
						Children: []Widget{
							HSpacer{},
							PushButton{
								AssignTo: &t.saveButton,
								Text:     "保存配置",
								MinSize:  Size{Width: 80},
								OnClicked: func() {
									t.onSave()
								},
							},
							PushButton{
								AssignTo: &t.connectButton,
								Text:     "连接服务器",
								MinSize:  Size{Width: 80},
								OnClicked: func() {
									t.onConnectOrDisconnect()
								},
							},
							PushButton{
								AssignTo: &t.syncButton,
								Text:     "开始同步",
								MinSize:  Size{Width: 80},
								OnClicked: func() {
									t.onSync()
								},
							},
						},
					},
				},
			},
			GroupBox{
				Title:  "同步状态",
				Layout: VBox{},
				Children: []Widget{
					ProgressBar{
						AssignTo: &t.progressBar,
						MinValue: 0,
						MaxValue: 100,
					},
				},
			},
		},
	}.Create(NewBuilder(t.TabPage))); err != nil {
		return fmt.Errorf("创建客户端界面失败: %v", err)
	}

	// 设置UI控件引用
	t.viewModel.SetUIControls(t.connectButton, t.addressEdit, t.portEdit, t.progressBar, t.saveButton, t.syncPathEdit)

	// 设置UI更新回调
	t.viewModel.SetUIUpdateCallback(t.UpdateUI)

	// 初始更新UI状态
	t.UpdateUI()

	return nil
}

// onSave 保存配置
func (t *ClientTab) onSave() {
	if err := t.viewModel.SaveConfig(); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	walk.MsgBox(t.Form(), "提示", "配置已保存", walk.MsgBoxIconInformation)
}

// onBrowse 处理浏览按钮点击
func (t *ClientTab) onBrowse() {
	dlg := new(walk.FileDialog)
	dlg.Title = "选择同步目录"
	dlg.FilePath = t.syncPathEdit.Text()

	if ok, err := dlg.ShowBrowseFolder(t.Form()); err != nil {
		walk.MsgBox(t.Form(), "错误", "选择目录失败: "+err.Error(), walk.MsgBoxIconError)
	} else if ok {
		t.syncPathEdit.SetText(dlg.FilePath)
	}
}

// onConnectOrDisconnect 处理连接/断开按钮点击
func (t *ClientTab) onConnectOrDisconnect() {
	if t.viewModel.IsConnected() {
		// 当前已连接，执行断开操作
		if err := t.viewModel.Disconnect(); err != nil {
			walk.MsgBox(t.Form(), "错误", "断开连接失败: "+err.Error(), walk.MsgBoxIconError)
		}
	} else {
		// 当前未连接，执行连接操作
		if err := t.viewModel.Connect(); err != nil {
			walk.MsgBox(t.Form(), "错误", "连接服务器失败: "+err.Error(), walk.MsgBoxIconError)
		}
	}
}

// onSync 处理同步按钮点击事件
func (t *ClientTab) onSync() {
	// 检查连接状态
	if !t.viewModel.IsConnected() {
		walk.MsgBox(t.Form(), "错误", "请先连接到服务器", walk.MsgBoxIconError)
		return
	}

	// 检查同步路径
	if t.syncPathEdit.Text() == "" {
		walk.MsgBox(t.Form(), "错误", "请选择同步目录", walk.MsgBoxIconError)
		return
	}

	// 创建确认对话框
	dlg, err := walk.NewDialog(t.Form())
	if err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}
	defer dlg.Dispose()

	dlg.SetTitle("同步确认")
	dlg.SetSizePixels(walk.Size{Width: 600, Height: 500})
	dlg.SetLayout(walk.NewVBoxLayout())

	// 显示同步信息
	config := t.viewModel.GetCurrentConfig()
	if config == nil {
		walk.MsgBox(t.Form(), "错误", "获取配置失败", walk.MsgBoxIconError)
		return
	}

	if err := (ScrollView{
		Layout: VBox{Spacing: 10, Margins: Margins{20, 20, 20, 20}},
		Children: []Widget{
			// 标题
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{
						Text: "同步信息",
						Font: Font{PointSize: 14, Bold: true},
					},
					HSpacer{},
				},
			},
			VSpacer{Size: 20},

			// 基本信息
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{
						Text: "【基本信息】",
						Font: Font{PointSize: 11, Bold: true},
					},
					HSpacer{},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{Text: "----------------------------------------"},
					HSpacer{},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{Text: fmt.Sprintf("服务器地址：%s:%s", t.viewModel.GetServerAddr(), t.viewModel.GetServerPort())},
					HSpacer{},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{Text: fmt.Sprintf("本地目录：%s", t.syncPathEdit.Text())},
					HSpacer{},
				},
			},
			VSpacer{Size: 20},

			// 服务器同步配置
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{
						Text: "【服务器同步配置】",
						Font: Font{PointSize: 11, Bold: true},
					},
					HSpacer{},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{Text: "----------------------------------------"},
					HSpacer{},
				},
			},
			Composite{
				Layout: VBox{Spacing: 10},
				Children: func() []Widget {
					var widgets []Widget
					if len(config.SyncFolders) == 0 {
						widgets = append(widgets,
							Composite{
								Layout: HBox{},
								Children: []Widget{
									HSpacer{},
									Label{Text: "暂无同步文件夹配置"},
									HSpacer{},
								},
							},
						)
					} else {
						for i, folder := range config.SyncFolders {
							widgets = append(widgets,
								Composite{
									Layout: HBox{},
									Children: []Widget{
										HSpacer{},
										Label{Text: fmt.Sprintf("%d. %s", i+1, folder.Path)},
										HSpacer{},
									},
								},
								Composite{
									Layout: HBox{},
									Children: []Widget{
										HSpacer{},
										Label{Text: fmt.Sprintf("同步模式：%s", folder.SyncMode)},
										HSpacer{},
									},
								},
							)
							// 查找重定向配置
							for _, redirect := range config.FolderRedirects {
								if redirect.ServerPath == folder.Path {
									widgets = append(widgets,
										Composite{
											Layout: HBox{},
											Children: []Widget{
												HSpacer{},
												Label{Text: fmt.Sprintf("重定向到：%s", redirect.ClientPath)},
												HSpacer{},
											},
										},
									)
									break
								}
							}
							if i < len(config.SyncFolders)-1 {
								widgets = append(widgets, VSpacer{Size: 10})
							}
						}
					}
					return widgets
				}(),
			},
			VSpacer{Size: 20},

			// 注意事项
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{
						Text: "【注意事项】",
						Font: Font{PointSize: 11, Bold: true},
					},
					HSpacer{},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{Text: "----------------------------------------"},
					HSpacer{},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{Text: "1. 同步开始前会先获取服务器文件的MD5信息进行对比"},
					HSpacer{},
				},
			},
			VSpacer{Size: 10},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{Text: "2. 只有文件内容发生变化的文件才会被同步"},
					HSpacer{},
				},
			},
			VSpacer{Size: 10},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					Label{Text: "3. 同步过程中请勿关闭程序"},
					HSpacer{},
				},
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	// 按钮布局
	if err := (Composite{
		Layout: HBox{Spacing: 10, Margins: Margins{10, 10, 10, 10}},
		Children: []Widget{
			HSpacer{},
			PushButton{
				Text:    "开始同步",
				MinSize: Size{Width: 90, Height: 30},
				Font:    Font{PointSize: 10},
				OnClicked: func() {
					dlg.Accept()
					// 开始同步
					if err := t.viewModel.SyncFiles(t.syncPathEdit.Text()); err != nil {
						walk.MsgBox(t.Form(), "错误", "同步失败: "+err.Error(), walk.MsgBoxIconError)
						return
					}
					t.UpdateUI()
				},
			},
			PushButton{
				Text:      "取消",
				MinSize:   Size{Width: 90, Height: 30},
				Font:      Font{PointSize: 10},
				OnClicked: dlg.Cancel,
			},
		},
	}.Create(NewBuilder(dlg))); err != nil {
		walk.MsgBox(t.Form(), "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	dlg.Run()
}

// UpdateUI 更新界面状态
func (t *ClientTab) UpdateUI() {
	// 更新地址和端口
	if t.addressEdit != nil {
		t.addressEdit.SetText(t.viewModel.GetServerAddr())
	}
	if t.portEdit != nil {
		t.portEdit.SetText(t.viewModel.GetServerPort())
	}
	if t.syncPathEdit != nil {
		t.syncPathEdit.SetText(t.viewModel.GetSyncPath())
	}

	// 更新按钮状态
	isConnected := t.viewModel.IsConnected()
	if t.connectButton != nil {
		if isConnected {
			t.connectButton.SetText("断开连接")
		} else {
			t.connectButton.SetText("连接服务器")
		}
	}
	if t.saveButton != nil {
		t.saveButton.SetEnabled(!isConnected)
	}
	if t.browseButton != nil {
		t.browseButton.SetEnabled(!isConnected)
	}
	if t.syncButton != nil {
		t.syncButton.SetEnabled(isConnected)
	}

	// 更新输入框状态
	if t.addressEdit != nil {
		t.addressEdit.SetEnabled(!isConnected)
	}
	if t.portEdit != nil {
		t.portEdit.SetEnabled(!isConnected)
	}
	if t.syncPathEdit != nil {
		t.syncPathEdit.SetEnabled(!isConnected)
	}

	// 更新服务器信息
	if t.serverInfo != nil {
		if isConnected {
			config := t.viewModel.GetCurrentConfig()
			if config != nil {
				t.serverInfo.SetText(fmt.Sprintf("%s (v%s)", config.Name, config.Version))
			} else {
				t.serverInfo.SetText("已连接")
			}
		} else {
			t.serverInfo.SetText("未连接")
		}
	}

	// 更新状态栏
	if t.StatusBar != nil {
		if isConnected {
			t.StatusBar.SetText(fmt.Sprintf("已连接到 %s:%s", t.viewModel.GetServerAddr(), t.viewModel.GetServerPort()))
		} else {
			t.StatusBar.SetText("未连接")
		}
	}
}
