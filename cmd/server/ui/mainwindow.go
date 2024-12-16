package ui

import (
	"fmt"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"synctools/pkg/common"
	"synctools/pkg/server"
)

// CreateMainWindow 创建主窗口
func CreateMainWindow(server *server.SyncServer) (*walk.MainWindow, error) {
	var mainWindow *walk.MainWindow
	var logBox *walk.TextEdit

	if err := (declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "文件同步服务器",
		MinSize:  declarative.Size{Width: 40, Height: 30},
		Size:     declarative.Size{Width: 800, Height: 600},
		Layout:   declarative.VBox{},
		OnSizeChanged: func() {
			// 触发重定向表格的重新布局
			if server.RedirectTable != nil {
				server.RedirectTable.SendMessage(win.WM_SIZE, 0, 0)
			}
		},
		Children: []declarative.Widget{
			declarative.Composite{
				Layout: declarative.HBox{
					Margins: declarative.Margins{
						Left:   5,
						Top:    5,
						Right:  5,
						Bottom: 0,
					},
				},
				Children: []declarative.Widget{
					declarative.TabWidget{
						Pages: []declarative.TabPage{
							createHomeTab(server, &logBox),
							createConfigTab(server),
						},
					},
				},
			},
		},
		StatusBarItems: []declarative.StatusBarItem{
			{
				AssignTo: &server.Status,
				Text:     "状态: 已停止",
			},
		},
	}.Create()); err != nil {
		return nil, err
	}

	// 设置窗口关闭事件
	mainWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		// 如果服务器还在运行，先停止服务器
		if server.Running {
			server.StopServer()
		}

		// 保存前更新配置
		if server.HostEdit != nil {
			server.Config.Host = server.HostEdit.Text()
		}
		if server.PortEdit != nil {
			fmt.Sscanf(server.PortEdit.Text(), "%d", &server.Config.Port)
		}
		if server.DirLabel != nil {
			server.Config.SyncDir = server.DirLabel.Text()
		}

		// 保存配置
		if err := server.SaveConfig(); err != nil {
			if server.Logger != nil {
				server.Logger.Log("关闭前保存配置失败: %v", err)
			}
		} else if server.Logger != nil {
			server.Logger.Log("程序关闭前配置已保存")
		}

		// 关闭日志记录器
		if logger, ok := server.Logger.(*common.GUILogger); ok {
			if err := logger.Close(); err != nil {
				fmt.Printf("关闭日志记录器失败: %v\n", err)
			}
		}
	})

	// 初始化日志记录器
	logger, err := common.NewGUILogger(logBox, "logs", "server")
	if err != nil {
		walk.MsgBox(nil, "错误", "创建日志记录器失败: "+err.Error(), walk.MsgBoxIconError)
		return nil, err
	}
	server.Logger = logger

	// UI初始化完成后进行文件夹验证
	server.ValidateFolders()

	return mainWindow, nil
}
