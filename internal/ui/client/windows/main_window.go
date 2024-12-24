/*
文件作用:
- 实现客户端主窗口界面
- 管理客户端界面布局
- 处理用户交互
- 集成客户端功能模块

主要方法:
- CreateMainWindow: 创建客户端主窗口
- handleWindowClosing: 处理窗口关闭事件
*/

package windows

import (
	"fmt"
	"runtime/debug"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/internal/ui/client/viewmodels"
)

// handleWindowClosing 处理窗口关闭事件
func handleWindowClosing(viewModel *viewmodels.MainViewModel) {
	if viewModel == nil {
		return
	}

	viewModel.LogDebug("窗口正在关闭")

	// 断开连接
	if viewModel.IsConnected() {
		if err := viewModel.Disconnect(); err != nil {
			viewModel.LogError("断开连接失败", err)
		}
	}

	viewModel.LogDebug("应用程序正在退出")
}

// CreateMainWindow 创建客户端主窗口
func CreateMainWindow(viewModel *viewmodels.MainViewModel) error {
	// 设置panic处理
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				viewModel.LogError("程序崩溃", err)
			} else {
				viewModel.LogError("程序崩溃", fmt.Errorf("%v", r))
			}
			debug.PrintStack()
		}
	}()

	viewModel.LogDebug("开始创建主窗口")

	var (
		mainWindow       *walk.MainWindow
		addressEdit      *walk.LineEdit
		portEdit         *walk.LineEdit
		connectButton    *walk.PushButton
		disconnectButton *walk.PushButton
		progressBar      *walk.ProgressBar
	)

	// 设置窗口属性
	if err := (declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "同步工具客户端",
		MinSize:  declarative.Size{Width: 800, Height: 600},
		Size:     declarative.Size{Width: 1024, Height: 768},
		Layout:   declarative.VBox{},
		MenuItems: []declarative.MenuItem{
			declarative.Menu{
				Text: "文件(&F)",
				Items: []declarative.MenuItem{
					declarative.Action{
						Text: "退出(&X)",
						OnTriggered: func() {
							handleWindowClosing(viewModel)
							mainWindow.Close()
						},
					},
				},
			},
			declarative.Menu{
				Text: "帮助(&H)",
				Items: []declarative.MenuItem{
					declarative.Action{
						Text: "关于(&A)",
						OnTriggered: func() {
							walk.MsgBox(mainWindow, "关于",
								"同步工具客户端 v1.0\n\n"+
									"用于文件同步的客户端软件\n"+
									"支持多目录同步和自动同步",
								walk.MsgBoxIconInformation)
						},
					},
				},
			},
		},
		Children: []declarative.Widget{
			declarative.Composite{
				Layout:  declarative.VBox{},
				MaxSize: declarative.Size{Width: 0, Height: 0},
				Children: []declarative.Widget{
					declarative.GroupBox{
						Title:  "服务器连接",
						Layout: declarative.Grid{Columns: 2},
						Children: []declarative.Widget{
							declarative.Label{Text: "服务器地址:"},
							declarative.LineEdit{
								AssignTo: &addressEdit,
								Text:     viewModel.GetServerAddr(),
								OnTextChanged: func() {
									viewModel.SetServerAddr(addressEdit.Text())
								},
							},
							declarative.Label{Text: "端口:"},
							declarative.LineEdit{
								AssignTo: &portEdit,
								Text:     viewModel.GetServerPort(),
								OnTextChanged: func() {
									viewModel.SetServerPort(portEdit.Text())
								},
							},
							declarative.PushButton{
								AssignTo: &connectButton,
								Text:     "连接",
								OnClicked: func() {
									if !viewModel.IsConnected() {
										if err := viewModel.Connect(); err != nil {
											walk.MsgBox(mainWindow, "错误",
												"连接服务器失败: "+err.Error(),
												walk.MsgBoxIconError)
										}
									}
								},
							},
							declarative.PushButton{
								AssignTo: &disconnectButton,
								Text:     "断开",
								OnClicked: func() {
									if viewModel.IsConnected() {
										if err := viewModel.Disconnect(); err != nil {
											walk.MsgBox(mainWindow, "错误",
												"断开连接失败: "+err.Error(),
												walk.MsgBoxIconError)
										}
									}
								},
							},
						},
					},
					declarative.GroupBox{
						Title:  "同步状态",
						Layout: declarative.VBox{},
						Children: []declarative.Widget{
							declarative.ProgressBar{
								AssignTo: &progressBar,
								MinValue: 0,
								MaxValue: 100,
							},
						},
					},
				},
			},
		},
		StatusBarItems: []declarative.StatusBarItem{
			{
				Text:  "未连接",
				Width: 200,
			},
		},
	}.Create()); err != nil {
		viewModel.LogError("创建窗口失败", err)
		return err
	}

	// 初始化视图模型
	if err := viewModel.Initialize(mainWindow); err != nil {
		viewModel.LogError("初始化视图模型失败", err)
		return err
	}

	// 设置UI控件引用
	viewModel.SetUIControls(connectButton, disconnectButton, addressEdit, portEdit, progressBar)

	// 设置关闭事件处理
	mainWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		handleWindowClosing(viewModel)
	})

	// 显示窗口
	mainWindow.Run()
	return nil
}
