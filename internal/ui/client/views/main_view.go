/*
文件作用:
- 实现客户端主视图
- 管理客户端UI布局
- 处理用户交互
- 显示同步状态和进度

主要方法:
- NewMainView: 创建主视图
- Setup: 设置UI布局
*/

package views

import (
	"github.com/lxn/walk"

	"synctools/internal/ui/client/viewmodels"
)

// MainView 客户端主视图
type MainView struct {
	*walk.Composite
	viewModel   *viewmodels.MainViewModel
	statusBar   *walk.StatusBarItem
	progressBar *walk.ProgressBar
}

// NewMainView 创建新的主视图
func NewMainView(parent walk.Container, viewModel *viewmodels.MainViewModel) (*MainView, error) {
	composite, err := walk.NewComposite(parent)
	if err != nil {
		return nil, err
	}

	view := &MainView{
		Composite: composite,
		viewModel: viewModel,
	}

	if err := view.initUI(); err != nil {
		return nil, err
	}

	return view, nil
}

// initUI 初始化UI组件
func (v *MainView) initUI() error {
	// 设置布局
	layout := walk.NewVBoxLayout()
	if err := v.SetLayout(layout); err != nil {
		return err
	}

	// 创建服务器连接组
	if err := v.createServerGroup(); err != nil {
		return err
	}

	// 创建同步状态组
	if err := v.createStatusGroup(); err != nil {
		return err
	}

	return nil
}

// createServerGroup 创建服务器连接组
func (v *MainView) createServerGroup() error {
	group, err := walk.NewGroupBox(v)
	if err != nil {
		return err
	}
	group.SetTitle("服务器连接")

	layout := walk.NewGridLayout()
	layout.SetSpacing(6)
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	if err := group.SetLayout(layout); err != nil {
		return err
	}

	// 添加服务器地址输入
	serverLabel, err := walk.NewLabel(group)
	if err != nil {
		return err
	}
	serverLabel.SetText("服务器地址:")

	if _, err := walk.NewLineEdit(group); err != nil {
		return err
	}

	// 添加端口输入
	portLabel, err := walk.NewLabel(group)
	if err != nil {
		return err
	}
	portLabel.SetText("端口:")

	if _, err := walk.NewLineEdit(group); err != nil {
		return err
	}

	// 添加连接按钮
	connectBtn, err := walk.NewPushButton(group)
	if err != nil {
		return err
	}
	connectBtn.SetText("连接")
	connectBtn.Clicked().Attach(func() {
		if !v.viewModel.IsConnected() {
			if err := v.viewModel.Connect(); err != nil {
				walk.MsgBox(nil, "错误",
					"连接服务器失败: "+err.Error(),
					walk.MsgBoxIconError)
			}
		}
	})

	// 添加断开按钮
	disconnectBtn, err := walk.NewPushButton(group)
	if err != nil {
		return err
	}
	disconnectBtn.SetText("断开")
	disconnectBtn.Clicked().Attach(func() {
		if v.viewModel.IsConnected() {
			if err := v.viewModel.Disconnect(); err != nil {
				walk.MsgBox(nil, "错误",
					"断开连接失败: "+err.Error(),
					walk.MsgBoxIconError)
			}
		}
	})

	return nil
}

// createStatusGroup 创建同步状态组
func (v *MainView) createStatusGroup() error {
	group, err := walk.NewGroupBox(v)
	if err != nil {
		return err
	}
	group.SetTitle("同步状态")

	layout := walk.NewVBoxLayout()
	layout.SetSpacing(6)
	layout.SetMargins(walk.Margins{10, 10, 10, 10})
	if err := group.SetLayout(layout); err != nil {
		return err
	}

	// 添加进度条
	v.progressBar, err = walk.NewProgressBar(group)
	if err != nil {
		return err
	}
	v.progressBar.SetRange(0, 100)

	return nil
}
