package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"synctools/pkg/common"
	"synctools/pkg/server"
)

// CreateMainWindow 创建主窗口
func CreateMainWindow(server *server.SyncServer) (*walk.MainWindow, error) {
	var mainWindow *walk.MainWindow
	var hostEdit, portEdit *walk.LineEdit
	var dirLabel *walk.Label
	var logBox *walk.TextEdit
	var ignoreListEdit *walk.TextEdit

	if err := (declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "文件同步服务器",
		MinSize:  declarative.Size{Width: 40, Height: 30},
		Size:     declarative.Size{Width: 800, Height: 600},
		Layout:   declarative.VBox{},
		OnSizeChanged: func() {
			// 触发重定向配置的重新布局
			if server.RedirectComposite != nil {
				server.RedirectComposite.SendMessage(win.WM_SIZE, 0, 0)
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
							createHomeTab(server, &hostEdit, &portEdit, &dirLabel, &logBox),
							createConfigTab(server, &ignoreListEdit),
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
		server.Config.Host = hostEdit.Text()
		fmt.Sscanf(portEdit.Text(), "%d", &server.Config.Port)
		server.Config.SyncDir = dirLabel.Text()

		// 更新忽略列表
		text := ignoreListEdit.Text()
		items := strings.Split(text, "\r\n")
		var ignoreList []string
		for _, item := range items {
			if item = strings.TrimSpace(item); item != "" {
				ignoreList = append(ignoreList, item)
			}
		}
		server.Config.IgnoreList = ignoreList

		// 更新重定向配置
		server.UpdateRedirectConfig()

		// 保存配置
		if err := server.SaveConfig(); err != nil {
			server.Logger.Log("关闭前保存配置失败: %v", err)
		} else {
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

	// 启动自动保存定时器
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// 保存前更新配置
			server.Config.Host = hostEdit.Text()
			fmt.Sscanf(portEdit.Text(), "%d", &server.Config.Port)
			server.Config.SyncDir = dirLabel.Text()

			// 更新忽略列表
			text := ignoreListEdit.Text()
			items := strings.Split(text, "\r\n")
			var ignoreList []string
			for _, item := range items {
				if item = strings.TrimSpace(item); item != "" {
					ignoreList = append(ignoreList, item)
				}
			}
			server.Config.IgnoreList = ignoreList

			// 更新重定向配置
			server.UpdateRedirectConfig()

			// 保存配置
			if err := server.SaveConfig(); err != nil {
				server.Logger.Log("自动保存配置失败: %v", err)
			} else {
				server.Logger.DebugLog("配置已自动保存")
			}
		}
	}()

	return mainWindow, nil
}
