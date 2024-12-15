package ui

import (
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/pkg/common"
	"synctools/pkg/server"
)

// createConfigTab 创建配置标签页
func createConfigTab(server *server.SyncServer, ignoreListEdit **walk.TextEdit) declarative.TabPage {
	return declarative.TabPage{
		Title: "配置",
		Layout: declarative.VBox{
			MarginsZero: true,
		},
		Children: []declarative.Widget{
			// 内容区域
			declarative.Composite{
				Layout: declarative.VBox{
					Margins: declarative.Margins{
						Left:   10,
						Top:    5,
						Right:  10,
						Bottom: 10,
					},
				},
				Children: []declarative.Widget{
					// 文件夹重定向配置组
					declarative.GroupBox{
						Title: "文件夹重定向配置",
						Layout: declarative.VBox{
							Margins: declarative.Margins{
								Left:   5,
								Top:    5,
								Right:  5,
								Bottom: 5,
							},
						},
						MinSize: declarative.Size{Width: 500},
						Children: []declarative.Widget{
							// 版本配置
							declarative.Composite{
								Layout: declarative.HBox{},
								Children: []declarative.Widget{
									declarative.Label{
										Text:      "整合包版本:",
										TextColor: walk.RGB(64, 64, 64),
										MinSize:   declarative.Size{Width: 80},
									},
									declarative.LineEdit{
										Text:    server.Config.Version,
										MinSize: declarative.Size{Width: 120},
										OnTextChanged: func() {
											server.Config.Version = server.VersionEdit.Text()
											if server.Logger != nil {
												server.Logger.DebugLog("版本已更新: %s", server.Config.Version)
											}
										},
										AssignTo: &server.VersionEdit,
									},
									declarative.Label{
										Text:      "说明: 版本不同时会删除服务端没有的文件，版本相同时保留客户端文件",
										TextColor: walk.RGB(128, 128, 128),
									},
								},
							},
							// 说明文本
							declarative.Composite{
								Layout: declarative.VBox{},
								Children: []declarative.Widget{
									declarative.Label{
										Text:      "说明: 重定向配置用于将服务器的文件夹映射到客户端的不同文件夹  示例: 服务器文件夹 'clientmods' 对应客户端文件夹 'mods'",
										TextColor: walk.RGB(128, 128, 128),
									},
									declarative.Label{
										Text:      "注意: 重定向配置修改后需要重启服务器生效",
										TextColor: walk.RGB(255, 0, 0),
									},
								},
							},
							// 重定向列表
							declarative.Label{Text: "重定向列表:"},
							declarative.TableView{
								AssignTo:         &server.RedirectTable,
								MinSize:          declarative.Size{Height: 150},
								AlternatingRowBG: true,
								Columns: []declarative.TableViewColumn{
									{Title: "服务器路径", Width: 0, Alignment: declarative.AlignNear},
									{Title: "客户端路径", Width: 120, Alignment: declarative.AlignCenter},
								},
								Model: server.RedirectModel,
								OnBoundsChanged: func() {
									if parent := server.RedirectTable.Parent(); parent != nil {
										parentWidth := parent.ClientBounds().Width
										server.RedirectTable.Columns().At(0).SetWidth(parentWidth - 200)
										server.RedirectTable.Columns().At(1).SetWidth(200)
									}
								},
								OnItemActivated: func() {
									if index := server.RedirectTable.CurrentIndex(); index >= 0 {
										redirect := &server.Config.FolderRedirects[index]
										if dlg, err := walk.NewDialog(server.RedirectTable.Form()); err == nil {
											dlg.SetTitle("编辑重定向配置")
											dlg.SetLayout(walk.NewVBoxLayout())

											var serverEdit *walk.LineEdit
											var clientEdit *walk.LineEdit

											declarative.Composite{
												Layout: declarative.Grid{Columns: 2},
												Children: []declarative.Widget{
													declarative.Label{Text: "服务器路径:"},
													declarative.LineEdit{
														AssignTo: &serverEdit,
														Text:     redirect.ServerPath,
													},
													declarative.Label{Text: "客户端路径:"},
													declarative.LineEdit{
														AssignTo: &clientEdit,
														Text:     redirect.ClientPath,
													},
												},
											}.Create(declarative.NewBuilder(dlg))

											declarative.Composite{
												Layout: declarative.HBox{},
												Children: []declarative.Widget{
													declarative.HSpacer{},
													declarative.PushButton{
														Text: "确定",
														OnClicked: func() {
															redirect.ServerPath = serverEdit.Text()
															redirect.ClientPath = clientEdit.Text()
															server.RedirectModel.PublishRowsReset()
															server.SaveConfig()
															dlg.Accept()
														},
													},
													declarative.PushButton{
														Text: "取消",
														OnClicked: func() {
															dlg.Cancel()
														},
													},
												},
											}.Create(declarative.NewBuilder(dlg))

											dlg.Run()
										}
									}
								},
							},
							declarative.Composite{
								Layout: declarative.HBox{},
								Children: []declarative.Widget{
									declarative.PushButton{
										Text: "添加重定向",
										OnClicked: func() {
											server.Config.FolderRedirects = append(server.Config.FolderRedirects, common.FolderRedirect{
												ServerPath: "新服务器文件夹",
												ClientPath: "新客户端文件夹",
											})
											server.RedirectModel.PublishRowsReset()
											server.SaveConfig()
										},
									},
									declarative.PushButton{
										Text: "删除选中",
										OnClicked: func() {
											if index := server.RedirectTable.CurrentIndex(); index >= 0 {
												server.Config.FolderRedirects = append(
													server.Config.FolderRedirects[:index],
													server.Config.FolderRedirects[index+1:]...,
												)
												server.RedirectModel.PublishRowsReset()
												server.SaveConfig()
											}
										},
									},
								},
							},
						},
					},
					// 忽略文件配置组
					declarative.GroupBox{
						Title: "忽略文件配置",
						Layout: declarative.VBox{
							Margins: declarative.Margins{
								Left:   5,
								Top:    5,
								Right:  5,
								Bottom: 5,
							},
						},
						Children: []declarative.Widget{
							declarative.Label{
								Text:      "说明: 每行一个规则，支持通配符 * 和 ? (* 匹配任意字符，? 匹配单个字符)",
								TextColor: walk.RGB(128, 128, 128),
							},
							declarative.TextEdit{
								AssignTo: ignoreListEdit,
								Text:     strings.Join(server.Config.IgnoreList, "\r\n"),
								VScroll:  true,
								MinSize:  declarative.Size{Height: 100},
								OnTextChanged: func() {
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
								Text:      "示例: *.tmp (所有.tmp文件), config?.ini (如config1.ini), .git/* (git目录下所有文件)",
								TextColor: walk.RGB(128, 128, 128),
							},
						},
					},
				},
			},
		},
	}
}
