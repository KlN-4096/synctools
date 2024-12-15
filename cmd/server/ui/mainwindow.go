package ui

import (
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

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
		Children: []declarative.Widget{
			declarative.TabWidget{
				Pages: []declarative.TabPage{
					createHomeTab(server, &hostEdit, &portEdit, &dirLabel, &logBox),
					createConfigTab(server, &ignoreListEdit),
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

	// 初始化日志记录器
	logger, err := common.NewGUILogger(logBox, "logs", "server")
	if err != nil {
		walk.MsgBox(nil, "错误", "创建日志记录器失败: "+err.Error(), walk.MsgBoxIconError)
		return nil, err
	}
	server.Logger = logger

	// 设置初始文本
	server.FolderEdit.SetText(strings.Join(server.SyncFolders, "\r\n"))

	// 初始化重定向配置UI
	for _, redirect := range server.Config.FolderRedirects {
		addRedirectConfig(server, redirect.ServerPath, redirect.ClientPath)
	}

	return mainWindow, nil
}
