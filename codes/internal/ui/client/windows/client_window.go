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

	"synctools/codes/internal/interfaces"
	"synctools/codes/internal/ui/client/viewmodels"
	"synctools/codes/internal/ui/client/views"
)

// handleWindowClosing 处理窗口关闭事件
func handleWindowClosing(viewModel *viewmodels.MainViewModel, logger interfaces.Logger) {
	if viewModel == nil {
		return
	}

	logger.Debug("窗口正在关闭", interfaces.Fields{})

	// 断开连接
	if viewModel.IsConnected() {
		if err := viewModel.Disconnect(); err != nil {
			logger.Error("断开连接失败", interfaces.Fields{
				"error": err,
			})
		}
	}

	logger.Debug("应用程序正在退出", interfaces.Fields{})
}

// CreateMainWindow 创建客户端主窗口
func CreateMainWindow(viewModel *viewmodels.MainViewModel, logger interfaces.Logger) error {
	// 设置panic处理
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				logger.Error("程序崩溃", interfaces.Fields{
					"error": err,
				})
			} else {
				logger.Error("程序崩溃", interfaces.Fields{
					"error": fmt.Errorf("%v", r),
				})
			}
			debug.PrintStack()
		}
	}()

	logger.Debug("开始创建主窗口", interfaces.Fields{})

	// 创建客户端标签页
	logger.Debug("正在创建客户端标签页", interfaces.Fields{})
	clientTab, err := views.NewClientTab(viewModel)
	if err != nil {
		logger.Error("创建客户端标签页失败", interfaces.Fields{
			"error": err,
		})
		return err
	}
	if clientTab == nil {
		logger.Error("客户端标签页为空", interfaces.Fields{})
		return fmt.Errorf("客户端标签页为空")
	}
	logger.Debug("客户端标签页创建成功", interfaces.Fields{})

	var mainWindow *walk.MainWindow

	// 设置窗口属性
	if err := (declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "同步工具客户端",
		MinSize:  declarative.Size{Width: 40, Height: 30},
		Size:     declarative.Size{Width: 400, Height: 300},
		Layout:   declarative.VBox{},
		MenuItems: []declarative.MenuItem{
			declarative.Menu{
				Text: "文件(&F)",
				Items: []declarative.MenuItem{
					declarative.Action{
						Text: "退出(&X)",
						OnTriggered: func() {
							handleWindowClosing(viewModel, logger)
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
							dlg, err := walk.NewDialog(mainWindow)
							if err != nil {
								return
							}
							defer dlg.Dispose()

							dlg.SetTitle("关于")
							dlg.SetLayout(walk.NewVBoxLayout())

							var debugCheckBox *walk.CheckBox
							if err := (declarative.Composite{
								Layout: declarative.VBox{},
								Children: []declarative.Widget{
									declarative.Label{
										Text: "同步工具客户端 v1.0\n\n" +
											"用于文件同步的客户端软件\n" +
											"支持多目录同步和自动同步",
									},
									declarative.HSpacer{},
									declarative.CheckBox{
										AssignTo: &debugCheckBox,
										Text:     "调试模式",
										Checked:  logger.GetLevel() == interfaces.DEBUG,
										OnCheckedChanged: func() {
											if debugCheckBox.Checked() {
												logger.SetLevel(interfaces.DEBUG)
											} else {
												logger.SetLevel(interfaces.INFO)
											}
										},
									},
								},
							}.Create(declarative.NewBuilder(dlg))); err != nil {
								return
							}

							dlg.Run()
						},
					},
				},
			},
		},
		Children: []declarative.Widget{
			declarative.TabWidget{
				Pages: []declarative.TabPage{
					{
						AssignTo: &clientTab.TabPage,
						Title:    "客户端",
						Layout:   declarative.VBox{},
					},
				},
			},
		},
		StatusBarItems: []declarative.StatusBarItem{
			{
				AssignTo: &clientTab.StatusBar,
				Text:     "未连接",
				Width:    200,
			},
		},
	}.Create()); err != nil {
		logger.Error("创建窗口失败", interfaces.Fields{
			"error": err,
		})
		return err
	}
	if mainWindow == nil {
		logger.Error("主窗口为空", interfaces.Fields{})
		return fmt.Errorf("主窗口为空")
	}
	logger.Debug("主窗口创建成功", interfaces.Fields{})

	// 初始化视图模型
	logger.Debug("正在初始化视图模型", interfaces.Fields{})
	if err := viewModel.Initialize(mainWindow); err != nil {
		logger.Error("初始化视图模型失败", interfaces.Fields{
			"error": err,
		})
		return err
	}
	logger.Debug("视图模型初始化成功", interfaces.Fields{})

	// 设置客户端标签页的UI
	logger.Debug("正在设置客户端标签页UI", interfaces.Fields{})
	if err := clientTab.Setup(); err != nil {
		logger.Error("设置客户端标签页UI失败", interfaces.Fields{
			"error": err,
		})
		return err
	}
	logger.Debug("客户端标签页UI设置成功", interfaces.Fields{})

	// 设置关闭事件处理
	mainWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		handleWindowClosing(viewModel, logger)
	})

	// 显示窗口
	logger.Debug("正在显示主窗口", interfaces.Fields{})
	mainWindow.Run()
	return nil
}
