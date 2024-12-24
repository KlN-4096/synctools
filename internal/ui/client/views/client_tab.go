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

	"synctools/internal/ui/client/viewmodels"
)

// ClientTab 客户端界面
type ClientTab struct {
	*walk.TabPage

	// UI 组件
	addressEdit      *walk.LineEdit
	portEdit         *walk.LineEdit
	connectButton    *walk.PushButton
	disconnectButton *walk.PushButton
	progressBar      *walk.ProgressBar
	StatusBar        *walk.StatusBarItem
	saveButton       *walk.PushButton

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
								Text:     "连接",
								MinSize:  Size{Width: 80},
								OnClicked: func() {
									if err := t.viewModel.Connect(); err != nil {
										walk.MsgBox(t.Form(), "错误",
											"连接服务器失败: "+err.Error(),
											walk.MsgBoxIconError)
									}
								},
							},
							PushButton{
								AssignTo: &t.disconnectButton,
								Text:     "断开",
								MinSize:  Size{Width: 80},
								OnClicked: func() {
									if err := t.viewModel.Disconnect(); err != nil {
										walk.MsgBox(t.Form(), "错误",
											"断开连接失败: "+err.Error(),
											walk.MsgBoxIconError)
									}
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
	t.viewModel.SetUIControls(t.connectButton, t.disconnectButton, t.addressEdit, t.portEdit, t.progressBar, t.saveButton)

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

// Activating 实现 walk.Form 接口
func (t *ClientTab) Activating() bool {
	return true
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

	// 更新按钮状态
	isConnected := t.viewModel.IsConnected()
	if t.connectButton != nil {
		t.connectButton.SetEnabled(!isConnected)
	}
	if t.disconnectButton != nil {
		t.disconnectButton.SetEnabled(isConnected)
	}
	if t.saveButton != nil {
		t.saveButton.SetEnabled(!isConnected)
	}

	// 更新输入框状态
	if t.addressEdit != nil {
		t.addressEdit.SetEnabled(!isConnected)
	}
	if t.portEdit != nil {
		t.portEdit.SetEnabled(!isConnected)
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
