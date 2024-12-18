package ui

import (
	"fmt"
	"runtime/debug"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/internal/ui/viewmodels"
	"synctools/internal/ui/views"
)

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
		MinSize:  declarative.Size{Width: 40, Height: 30},
		Size:     declarative.Size{Width: 800, Height: 600},
		Layout:   declarative.VBox{},
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

	// 显示窗口
	viewModel.LogDebug("正在显示主窗口")
	mainWindow.Run()
	return nil
}
