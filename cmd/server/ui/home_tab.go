package ui

import (
	"fmt"
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/pkg/server"
)

// createHomeTab 创建主页标签页
func createHomeTab(server *server.SyncServer, hostEdit, portEdit **walk.LineEdit, dirLabel **walk.Label, logBox **walk.TextEdit) declarative.TabPage {
	return declarative.TabPage{
		Title:  "主页",
		Layout: declarative.VBox{},
		Children: []declarative.Widget{
			declarative.Composite{
				Layout: declarative.HBox{},
				Children: []declarative.Widget{
					declarative.Composite{
						Layout: declarative.VBox{},
						Children: []declarative.Widget{
							declarative.GroupBox{
								Title:  "基本设置",
								Layout: declarative.Grid{Columns: 2},
								Children: []declarative.Widget{
									declarative.Label{Text: "主机:"},
									declarative.LineEdit{
										AssignTo: hostEdit,
										Text:     server.Config.Host,
										OnTextChanged: func() {
											server.Config.Host = (*hostEdit).Text()
										},
									},
									declarative.Label{Text: "端口:"},
									declarative.LineEdit{
										AssignTo: portEdit,
										Text:     fmt.Sprintf("%d", server.Config.Port),
										OnTextChanged: func() {
											fmt.Sscanf((*portEdit).Text(), "%d", &server.Config.Port)
										},
									},
									declarative.Label{Text: "同步目录:"},
									declarative.Label{
										AssignTo: dirLabel,
										Text:     server.Config.SyncDir,
									},
								},
							},
							declarative.Composite{
								Layout: declarative.HBox{},
								Children: []declarative.Widget{
									declarative.PushButton{
										Text: "选择目录",
										OnClicked: func() {
											dlg := new(walk.FileDialog)
											dlg.Title = "选择同步目录"

											if ok, err := dlg.ShowBrowseFolder(nil); err != nil {
												walk.MsgBox(nil, "错误",
													"选择目录时发生错误: "+err.Error(),
													walk.MsgBoxIconError)
												return
											} else if !ok {
												return
											}

											if dlg.FilePath != "" {
												server.Config.SyncDir = dlg.FilePath
												(*dirLabel).SetText(dlg.FilePath)
												server.Logger.Log("同步目录已更改为: %s", dlg.FilePath)
												server.ValidateFolders()
											}
										},
									},
									declarative.PushButton{
										Text: "启动服务器",
										OnClicked: func() {
											if !server.Running {
												if err := server.StartServer(); err != nil {
													walk.MsgBox(nil, "错误", err.Error(), walk.MsgBoxIconError)
												}
											}
										},
									},
									declarative.PushButton{
										Text: "停止服务器",
										OnClicked: func() {
											if server.Running {
												server.StopServer()
											}
										},
									},
									declarative.HSpacer{},
									declarative.CheckBox{
										Text: "调试模式",
										OnCheckedChanged: func() {
											server.Logger.SetDebugMode(!server.Logger.DebugMode)
										},
									},
								},
							},
							declarative.Composite{
								Layout: declarative.VBox{},
								Children: []declarative.Widget{
									declarative.Label{Text: "同步文件夹列表 (每行一个):"},
									declarative.TextEdit{
										AssignTo: &server.FolderEdit,
										VScroll:  true,
										MinSize:  declarative.Size{Height: 100},
										OnTextChanged: func() {
											text := server.FolderEdit.Text()
											folders := strings.Split(text, "\r\n")
											var validFolders []string
											for _, folder := range folders {
												if strings.TrimSpace(folder) != "" {
													validFolders = append(validFolders, folder)
												}
											}
											server.SyncFolders = validFolders
											if server.Config.SyncDir != "" {
												server.ValidateFolders()
											}
										},
									},
									declarative.Label{
										Text:      "无效的文件夹列表:",
										TextColor: walk.RGB(192, 0, 0),
									},
									declarative.TextEdit{
										AssignTo:   &server.InvalidLabel,
										ReadOnly:   true,
										VScroll:    true,
										MinSize:    declarative.Size{Height: 60},
										Background: declarative.SolidColorBrush{Color: walk.RGB(255, 240, 240)},
									},
								},
							},
						},
					},
					declarative.GroupBox{
						Title:  "运行日志",
						Layout: declarative.VBox{},
						Children: []declarative.Widget{
							declarative.TextEdit{
								AssignTo: logBox,
								ReadOnly: true,
								VScroll:  true,
							},
						},
					},
				},
			},
		},
	}
}
