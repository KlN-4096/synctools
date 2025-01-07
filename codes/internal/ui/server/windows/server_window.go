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
func handleWindowClosing(viewModel *viewmodels.ConfigViewModel) {
	// 安全检查
	if viewModel == nil {
		return
	}
	viewModel.HandleWindowClosing()
}

// NewMainWindow 创建主窗口
func NewMainWindow(viewModel *viewmodels.ConfigViewModel) (*walk.MainWindow, error) {
	var mainWindow *walk.MainWindow
	var configTab *views.ConfigTab
	var statusBar *walk.StatusBarItem

	// 创建配置标签页
	configTab, err := views.NewConfigTab(viewModel)
	if err != nil {
		return nil, fmt.Errorf("创建配置标签页失败: %v", err)
	}

	// 设置窗口布局
	if err := (declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "同步工具 - 服务端",
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
			// 配置标签页
			declarative.TabWidget{
				Pages: []declarative.TabPage{
					// 配置页面
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
				AssignTo: &statusBar,
				Text:     "就绪",
				Width:    200,
			},
		},
	}.Create()); err != nil {
		return nil, fmt.Errorf("创建主窗口失败: %v", err)
	}

	// 设置配置标签页
	if err := configTab.Setup(); err != nil {
		return nil, fmt.Errorf("设置配置标签页失败: %v", err)
	}

	// 初始化视图模型
	if err := viewModel.Initialize(mainWindow); err != nil {
		return nil, fmt.Errorf("初始化视图模型失败: %v", err)
	}

	// 设置窗口关闭处理函数
	mainWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		handleWindowClosing(viewModel)
	})

	// 设置异常处理函数
	defer func() {
		if r := recover(); r != nil {
			viewModel.LogError("发生严重错误", fmt.Errorf("%v\n%s", r, debug.Stack()))
			walk.MsgBox(mainWindow, "错误", fmt.Sprintf("发生严重错误: %v", r), walk.MsgBoxIconError)
		}
	}()

	// 在所有UI组件都初始化完成后，再添加大小改变事件
	mainWindow.SizeChanged().Attach(func() {
		// 确保所有组件都已初始化
		if viewModel != nil && configTab != nil && configTab.TabPage != nil {
			viewModel.UpdateUI()
		}
	})

	return mainWindow, nil
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
