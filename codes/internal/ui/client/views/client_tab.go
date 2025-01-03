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
	connectButton *walk.PushButton
	syncButton    *walk.PushButton
	progressBar   *walk.ProgressBar
	StatusBar     *walk.StatusBarItem
	saveButton    *walk.PushButton
	serverInfo    *walk.TextLabel
	syncTable     *walk.TableView
	syncModel     *walk.TableView

	viewModel *viewmodels.MainViewModel
}

//
// -------------------- 初始化方法 --------------------
//

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
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					// 左侧面板
					Composite{
						Layout: VBox{},
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
											},
											Label{Text: "端口:"},
											LineEdit{
												AssignTo: &t.portEdit,
											},
											Label{Text: "同步目录:"},
											Composite{
												Layout: HBox{Spacing: 5},
												Children: []Widget{
													LineEdit{
														AssignTo: &t.syncPathEdit,
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
					},
					// 右侧面板
					GroupBox{
						Title:   "同步配置",
						Layout:  VBox{},
						MinSize: Size{Width: 300},
						Children: []Widget{
							// 服务器信息
							TextLabel{
								AssignTo: &t.serverInfo,
								Text:     "未连接到服务器",
								Font:     Font{PointSize: 9},
							},
							HSpacer{Size: 10},
							// 同步文件夹表格
							TableView{
								AssignTo:         &t.syncTable,
								MinSize:          Size{Height: 200},
								AlternatingRowBG: true,
								Columns: []TableViewColumn{
									{Title: "路径", Width: 120},
									{Title: "同步模式", Width: 80},
									{Title: "重定向到", Width: 100},
								},
								Model: t.syncModel,
							},
						},
					},
				},
			},
		},
	}.Create(NewBuilder(t.TabPage))); err != nil {
		return fmt.Errorf("创建客户端界面失败: %v", err)
	}

	// 设置UI控件引用
	t.viewModel.SetUIControls(t.connectButton, t.addressEdit, t.portEdit, t.progressBar, t.saveButton, t.syncPathEdit, t.syncTable)

	// 设置UI更新回调
	t.viewModel.SetUIUpdateCallback(t.viewModel.UpdateUIState)

	// 初始更新UI状态
	t.viewModel.UpdateUIState()

	return nil
}

//
// -------------------- 配置管理方法 --------------------
//

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

//
// -------------------- 连接管理方法 --------------------
//

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

//
// -------------------- 同步管理方法 --------------------
//

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

	// 禁用同步按钮,防止重复点击
	t.syncButton.SetEnabled(false)

	// 在后台线程中执行同步操作
	go func() {
		// 同步完成后恢复按钮状态
		defer func() {
			t.Form().Synchronize(func() {
				t.syncButton.SetEnabled(true)
			})
		}()

		// 开始同步
		if err := t.viewModel.SyncFiles(t.syncPathEdit.Text()); err != nil {
			t.Form().Synchronize(func() {
				walk.MsgBox(t.Form(), "错误", "同步失败: "+err.Error(), walk.MsgBoxIconError)
			})
			return
		}

		// 更新UI
		t.Form().Synchronize(func() {
			t.viewModel.UpdateUIState()
		})
	}()
}

//
// -------------------- UI 更新方法 --------------------
//
