package ui

import (
	"fmt"
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

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

							if err := composite.SetLayout(walk.NewGridLayout()); err != nil {
								return
							}

							// 服务器路径标签
							if label, err := walk.NewLabel(composite); err == nil {
								label.SetText("服务器文件夹:")
							}

							// 客户端路径标签
							if label, err := walk.NewLabel(composite); err == nil {
								label.SetText("客户端文件夹:")
							}

							// 服务器路径输入框
							var serverEdit *walk.LineEdit
							if serverEdit, err = walk.NewLineEdit(composite); err == nil {
								serverEdit.SetText("新服务器文件夹")
							}

							// 客户端路径输入框
							var clientEdit *walk.LineEdit
							if clientEdit, err = walk.NewLineEdit(composite); err == nil {
								clientEdit.SetText("新客户端文件夹")
							}

							// 删除按钮
							if deleteBtn, err := walk.NewPushButton(composite); err == nil {
								deleteBtn.SetText("X")
								deleteBtn.Clicked().Attach(func() {
									composite.Dispose()
									server.UpdateRedirectConfig()
								})
							}

							// 添加新的重定向配置
							server.Config.FolderRedirects = append(server.Config.FolderRedirects, common.FolderRedirect{
								ServerPath: "新服务器文件夹",
								ClientPath: "新客户端文件夹",
							})

							if server.Logger != nil {
								server.Logger.Log("已添加新的重定向配置")
							}
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
