package ui

import (
	"fmt"
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"synctools/pkg/common"
	"synctools/pkg/server"
)

// createConfigTab 创建配置标签页
func createConfigTab(server *server.SyncServer, ignoreListEdit **walk.TextEdit) declarative.TabPage {
	return declarative.TabPage{
		Title:  "配置",
		Layout: declarative.VBox{},
		Children: []declarative.Widget{
			declarative.GroupBox{
				Title:  "文件夹重定向配置",
				Layout: declarative.VBox{},
				Children: []declarative.Widget{
					declarative.Composite{
						Layout: declarative.Grid{Columns: 2},
						Children: []declarative.Widget{
							declarative.Label{Text: "服务器文件夹:"},
							declarative.Label{Text: "客户端文件夹:"},
							declarative.LineEdit{
								AssignTo: &server.ServerPath,
								Text:     server.Config.FolderRedirects[0].ServerPath,
								OnTextChanged: func() {
									if len(server.Config.FolderRedirects) > 0 {
										server.Config.FolderRedirects[0].ServerPath = server.ServerPath.Text()
										if server.Logger != nil {
											server.Logger.DebugLog("服务器文件夹已更改为: %s", server.ServerPath.Text())
										}
									}
								},
							},
							declarative.LineEdit{
								AssignTo: &server.ClientPath,
								Text:     server.Config.FolderRedirects[0].ClientPath,
								OnTextChanged: func() {
									if len(server.Config.FolderRedirects) > 0 {
										server.Config.FolderRedirects[0].ClientPath = server.ClientPath.Text()
										if server.Logger != nil {
											server.Logger.DebugLog("客户端文件夹已更改为: %s", server.ClientPath.Text())
										}
									}
								},
							},
						},
					},
					declarative.Label{Text: "额外的重定向配置:"},
					declarative.Composite{
						AssignTo: &server.RedirectComposite,
						Layout:   declarative.VBox{},
					},
					declarative.PushButton{
						Text: "+",
						OnClicked: func() {
							composite, err := walk.NewComposite(server.RedirectComposite)
							if err != nil {
								return
							}

							// 使用水平布局来代替网格布局
							if err := composite.SetLayout(walk.NewHBoxLayout()); err != nil {
								return
							}

							// 创建一个容器用于标签和输入框
							inputContainer, err := walk.NewComposite(composite)
							if err != nil {
								return
							}
							if err := inputContainer.SetLayout(walk.NewHBoxLayout()); err != nil {
								return
							}

							// 服务器路径标签和输入框
							serverLabel, err := walk.NewLabel(inputContainer)
							if err != nil {
								return
							}
							serverLabel.SetText("服务器文件夹:")

							serverEdit, err := walk.NewLineEdit(inputContainer)
							if err != nil {
								return
							}
							serverEdit.SetText("新服务器文件夹")

							// 客户端路径标签和输入框
							clientLabel, err := walk.NewLabel(inputContainer)
							if err != nil {
								return
							}
							clientLabel.SetText("客户端文件夹:")

							clientEdit, err := walk.NewLineEdit(inputContainer)
							if err != nil {
								return
							}
							clientEdit.SetText("新客户端文件夹")

							// 删除按钮
							deleteBtn, err := walk.NewPushButton(composite)
							if err != nil {
								return
							}
							deleteBtn.SetText("X")
							deleteBtn.Clicked().Attach(func() {
								composite.Dispose()
								server.UpdateRedirectConfig()
							})

							// 添加文本更改事件
							serverEdit.TextChanged().Attach(func() {
								server.UpdateRedirectConfig()
							})
							clientEdit.TextChanged().Attach(func() {
								server.UpdateRedirectConfig()
							})

							// 添加新的重定向配置
							server.Config.FolderRedirects = append(server.Config.FolderRedirects, common.FolderRedirect{
								ServerPath: serverEdit.Text(),
								ClientPath: clientEdit.Text(),
							})

							if server.Logger != nil {
								server.Logger.Log("已添加新的重定向配置")
							}

							// 强制重新布局
							server.RedirectComposite.SendMessage(win.WM_SIZE, 0, 0)
						},
					},
					declarative.Label{
						Text:      "示例: 服务器文件夹 'clientmods' 对应客户端文件夹 'mods'",
						TextColor: walk.RGB(128, 128, 128),
					},
					declarative.Label{
						Text:      "注意: 重定向配置修改后需要重启服务器生效",
						TextColor: walk.RGB(255, 0, 0),
					},
					declarative.HSpacer{},
					declarative.PushButton{
						Text: "保存配置",
						OnClicked: func() {
							if err := server.SaveConfig(); err != nil {
								walk.MsgBox(nil, "错误",
									fmt.Sprintf("保存配置失败: %v", err),
									walk.MsgBoxIconError)
							} else {
								walk.MsgBox(nil, "成功",
									"配置已保存",
									walk.MsgBoxIconInformation)
							}
						},
					},
				},
			},
			declarative.GroupBox{
				Title:  "忽略文件配置",
				Layout: declarative.VBox{},
				Children: []declarative.Widget{
					declarative.TextEdit{
						AssignTo: ignoreListEdit,
						Text:     strings.Join(server.Config.IgnoreList, "\r\n"),
						VScroll:  true,
						OnTextChanged: func() {
							// 更新忽略列表
							text := (*ignoreListEdit).Text()
							items := strings.Split(text, "\r\n")
							var ignoreList []string
							for _, item := range items {
								if item = strings.TrimSpace(item); item != "" {
									ignoreList = append(ignoreList, item)
								}
							}
							server.Config.IgnoreList = ignoreList
							if server.Logger != nil {
								server.Logger.DebugLog("忽略列表已更新: %v", ignoreList)
							}
						},
					},
					declarative.Label{
						Text:      "每行一个文件名或通配符，例如: .DS_Store, *.tmp",
						TextColor: walk.RGB(128, 128, 128),
					},
				},
			},
		},
	}
}
