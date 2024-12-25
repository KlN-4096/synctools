/*
文件作用:
- 实现主窗口界面
- 管理界面布局
- 处理用户交互
- 集成各个功能模块

主要方法:
- NewMainWindow: 创建主窗口
- Run: 运行窗口程序
- InitMenus: 初始化菜单
- InitTabs: 初始化标签页
- ShowError: 显示错误信息
- ShowMessage: 显示消息提示
*/

package windows

import (
	"fmt"
	"runtime/debug"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/codes/internal/ui/server/viewmodels"
	"synctools/codes/internal/ui/server/views"
)

// handleWindowClosing 处理窗口关闭事件
func handleWindowClosing(viewModel *viewmodels.MainViewModel) {
	// 安全检查
	if viewModel == nil {
		return
	}

	// 记录关闭事件
	viewModel.LogDebug("窗口正在关闭")

	// 如果服务器正在运行，尝试停止它
	if viewModel.ConfigViewModel != nil && viewModel.ConfigViewModel.IsServerRunning() {
		if err := viewModel.ConfigViewModel.StopServer(); err != nil {
			// 记录错误但继续关闭过程
			viewModel.LogError("停止服务器失败", err)
		}
	}

	// 保存配置（如果有选中的配置）
	if viewModel.ConfigViewModel != nil {
		if config := viewModel.ConfigViewModel.GetCurrentConfig(); config != nil {
			if err := viewModel.ConfigViewModel.SaveConfig(); err != nil {
				// 记录错误但继续关闭过程
				viewModel.LogError("保存配置失败", err)
			}
		} else {
			viewModel.LogDebug("没有选中的配置，跳过保存")
		}
	}

	viewModel.LogDebug("应用程序正在退出")
}

// CreateMainWindow 创建主窗口
func CreateMainWindow(viewModel *viewmodels.MainViewModel) error {
	// 设置panic处理
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				viewModel.LogError("程序崩溃", err)
			} else {
				viewModel.LogError("程序崩溃", fmt.Errorf("%v", r))
			}
			// 打印堆栈信息
			debug.PrintStack()
		}
	}()

	viewModel.LogDebug("开始创建主窗口")

	// 创建配置标签页
	viewModel.LogDebug("正在创建配置标签页")
	configTab, err := views.NewConfigTab(viewModel.ConfigViewModel)
	if err != nil {
		viewModel.LogError("创建配置标签页失败", err)
		return err
	}
	if configTab == nil {
		viewModel.LogError("配置标签页为空", nil)
		return fmt.Errorf("配置标签页为空")
	}
	viewModel.LogDebug("配置标签页创建成功")

	var mainWindow *walk.MainWindow

	// 设置窗口属性
	viewModel.LogDebug("正在创建主窗口")
	if err := (declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "同步工具",
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
										Text: "同步工具 v1.0\n\n" +
											"用于文件同步的工具软件\n" +
											"支持多目录同步和自动同步",
									},
									declarative.HSpacer{},
									declarative.CheckBox{
										AssignTo: &debugCheckBox,
										Text:     "调试模式",
										Checked:  viewModel.GetLogger().GetDebugMode(),
										OnCheckedChanged: func() {
											viewModel.GetLogger().SetDebugMode(debugCheckBox.Checked())
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
						AssignTo: &configTab.TabPage,
						Title:    "配置",
						Layout:   declarative.VBox{},
					},
				},
			},
		},
		StatusBarItems: []declarative.StatusBarItem{
			{
				AssignTo: &configTab.StatusBar,
				Text:     "就绪",
				Width:    200,
			},
		},
	}.Create()); err != nil {
		viewModel.LogError("创建窗口失败", err)
		return err
	}
	if mainWindow == nil {
		viewModel.LogError("主窗口为空", nil)
		return fmt.Errorf("主窗口为空")
	}
	viewModel.LogDebug("主窗口创建成功")

	// 初始化视图模型
	viewModel.LogDebug("正在初始化视图模型")
	if err := viewModel.Initialize(mainWindow); err != nil {
		viewModel.LogError("初始化视图模型失败", err)
		return err
	}
	viewModel.LogDebug("视图模型初始化成功")

	// 设置配置标签页的UI
	viewModel.LogDebug("正在设置配置标签页UI")
	if err := configTab.Setup(); err != nil {
		viewModel.LogError("设置配置标签页UI失败", err)
		return err
	}
	viewModel.LogDebug("配置标签页UI设置成功")

	// 设置关闭事件处理
	mainWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		handleWindowClosing(viewModel)
	})

	// 显示窗口
	viewModel.LogDebug("正在显示主窗口")
	mainWindow.Run()
	return nil
}

// ShowError 显示错误对话框
func ShowError(owner walk.Form, title string, err error) {
	walk.MsgBox(owner, title,
		fmt.Sprintf("发生错误: %v", err),
		walk.MsgBoxIconError)
}

// ShowMessage 显示消息对话框
func ShowMessage(owner walk.Form, title string, message string) {
	walk.MsgBox(owner, title, message,
		walk.MsgBoxIconInformation)
}
