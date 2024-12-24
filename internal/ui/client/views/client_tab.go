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
				Layout: Grid{Columns: 2, Spacing: 10},
				Children: []Widget{
					Label{Text: "服务器地址:"},
					LineEdit{
						AssignTo: &t.addressEdit,
						Text:     t.viewModel.GetServerAddr(),
						OnTextChanged: func() {
							t.viewModel.SetServerAddr(t.addressEdit.Text())
						},
					},
					Label{Text: "端口:"},
					LineEdit{
						AssignTo: &t.portEdit,
						Text:     t.viewModel.GetServerPort(),
						OnTextChanged: func() {
							t.viewModel.SetServerPort(t.portEdit.Text())
						},
					},
					PushButton{
						AssignTo: &t.connectButton,
						Text:     "连接",
						OnClicked: func() {
							if !t.viewModel.IsConnected() {
								if err := t.viewModel.Connect(); err != nil {
									walk.MsgBox(t.Form(), "错误",
										"连接服务器失败: "+err.Error(),
										walk.MsgBoxIconError)
								}
							}
						},
					},
					PushButton{
						AssignTo: &t.disconnectButton,
						Text:     "断开",
						OnClicked: func() {
							if t.viewModel.IsConnected() {
								if err := t.viewModel.Disconnect(); err != nil {
									walk.MsgBox(t.Form(), "错误",
										"断开连接失败: "+err.Error(),
										walk.MsgBoxIconError)
								}
							}
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
	t.viewModel.SetUIControls(t.connectButton, t.disconnectButton, t.addressEdit, t.portEdit, t.progressBar)

	return nil
}

// Activating 实现 walk.Form 接口
func (t *ClientTab) Activating() bool {
	return true
}
