package ui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
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
			// 配置文件列表
			declarative.GroupBox{
				Title: "配置文件列表",
				Layout: declarative.VBox{
					Margins: declarative.Margins{
						Left:   5,
						Top:    5,
						Right:  5,
						Bottom: 5,
					},
				},
				Children: []declarative.Widget{
					declarative.TableView{
						AssignTo:         &server.ConfigTable,
						MinSize:          declarative.Size{Height: 150},
						AlternatingRowBG: true,
						CheckBoxes:       true,
						MultiSelection:   false,
						Columns: []declarative.TableViewColumn{
							{Title: "选中", Width: 50},
							{Title: "整合包名称", Width: 200},
							{Title: "版本", Width: 100},
							{Title: "UUID", Width: 300},
						},

						Model: server.ConfigListModel,
						OnCurrentIndexChanged: func() {
							if index := server.ConfigTable.CurrentIndex(); index >= 0 {
								// 先取消所有选中项
								for i := 0; i < len(server.ConfigList); i++ {
									server.ConfigListModel.SetValue(i, 0, false)
								}
								// 设置新的选中项
								server.ConfigListModel.SetValue(index, 0, true)
								// 加载新配置
								server.Config = server.ConfigList[index]
								// 更新UI
								server.NameEdit.SetText(server.Config.Name)
								server.VersionEdit.SetText(server.Config.Version)
								server.RedirectModel.PublishRowsReset()
							}
						},
					},
					declarative.Composite{
						Layout: declarative.HBox{},
						Children: []declarative.Widget{
							declarative.PushButton{
								Text: "新建配置",
								OnClicked: func() {
									if dlg, err := walk.NewDialog(server.ConfigTable.Form()); err == nil {
										dlg.SetTitle("新建配置")
										dlg.SetLayout(walk.NewVBoxLayout())

										var nameEdit *walk.LineEdit
										var versionEdit *walk.LineEdit

										declarative.Composite{
											Layout: declarative.Grid{Columns: 2},
											Children: []declarative.Widget{
												declarative.Label{Text: "整合包名称:"},
												declarative.LineEdit{
													AssignTo: &nameEdit,
													Text:     "新整合包",
												},
												declarative.Label{Text: "版本:"},
												declarative.LineEdit{
													AssignTo: &versionEdit,
													Text:     "1.0.0",
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
														// 生成UUID
														uuid := make([]byte, 16)
														rand.Read(uuid)
														uuidStr := hex.EncodeToString(uuid)

														// 创建新配置
														newConfig := common.SyncConfig{
															UUID:    uuidStr,
															Name:    nameEdit.Text(),
															Version: versionEdit.Text(),
															Host:    "0.0.0.0",
															Port:    6666,
															IgnoreList: []string{
																".clientconfig",
																".DS_Store",
																"thumbs.db",
															},
															FolderRedirects: []common.FolderRedirect{
																{ServerPath: "clientmods", ClientPath: "mods"},
															},
														}

														// 保存新配置
														configPath := filepath.Join(
															filepath.Dir(server.ConfigFile),
															fmt.Sprintf("config_%s.json", uuidStr),
														)
														if err := common.SaveConfig(&newConfig, configPath); err != nil {
															walk.MsgBox(dlg, "错误",
																fmt.Sprintf("保存配置失败: %v", err),
																walk.MsgBoxIconError)
															return
														}

														// 添加到列表并选中
														server.ConfigList = append(server.ConfigList, newConfig)
														server.ConfigListModel.PublishRowsReset()
														server.ConfigTable.SetCurrentIndex(len(server.ConfigList) - 1)

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
								},
							},
							declarative.PushButton{
								Text: "删除配置",
								OnClicked: func() {
									if index := server.ConfigTable.CurrentIndex(); index >= 0 {
										config := server.ConfigList[index]
										if walk.MsgBox(server.ConfigTable.Form(),
											"确认删除",
											fmt.Sprintf("确定要删除配置 '%s' 吗？", config.Name),
											walk.MsgBoxYesNo) == walk.DlgCmdYes {

											// 删除配置文件
											configPath := filepath.Join(
												filepath.Dir(server.ConfigFile),
												fmt.Sprintf("config_%s.json", config.UUID),
											)
											if err := server.DeleteConfig(configPath, index); err != nil {
												walk.MsgBox(server.ConfigTable.Form(),
													"错误",
													fmt.Sprintf("删除配置失败: %v", err),
													walk.MsgBoxIconError)
											}
										}
									}
								},
							},
							declarative.PushButton{
								Text: "保存配置",
								OnClicked: func() {
									// 确保当前编辑框的内容已更新到配置对象
									server.Config.Name = server.NameEdit.Text()
									server.Config.Version = server.VersionEdit.Text()

									// 更新配置列表
									if index := server.ConfigTable.CurrentIndex(); index >= 0 {
										// 检查 UUID 是否匹配
										if server.Config.UUID != server.ConfigList[index].UUID {
											walk.MsgBox(server.ConfigTable.Form(),
												"错误",
												"配置 UUID 不匹配，无法保存",
												walk.MsgBoxIconError)
											return
										}

										server.ConfigList[index] = server.Config
										server.ConfigListModel.PublishRowsReset()
									}

									// 保存到文件
									if err := server.SaveConfig(); err != nil {
										walk.MsgBox(server.ConfigTable.Form(),
											"错误",
											fmt.Sprintf("保存配置失败: %v", err),
											walk.MsgBoxIconError)
									} else {
										walk.MsgBox(server.ConfigTable.Form(),
											"成功",
											"配置已保存",
											walk.MsgBoxIconInformation)
									}
								},
							},
						},
					},
				},
			},
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
					// 版本配置
					declarative.GroupBox{
						Title:  "整合包信息",
						Layout: declarative.Grid{Columns: 2},
						Children: []declarative.Widget{
							declarative.Label{Text: "整合包名称:"},
							declarative.LineEdit{
								Text: server.Config.Name,
								OnTextChanged: func() {
									server.Config.Name = server.NameEdit.Text()
									if server.Logger != nil {
										server.Logger.DebugLog("整合包名称已更新: %s", server.Config.Name)
									}
									// 更新当前配置到配置列表
									if index := server.ConfigTable.CurrentIndex(); index >= 0 {
										server.ConfigList[index] = server.Config
										server.ConfigListModel.PublishRowsReset()
										server.SaveConfig()
									}
								},
								AssignTo: &server.NameEdit,
							},
							declarative.Label{Text: "整合包版本:"},
							declarative.LineEdit{
								Text: server.Config.Version,
								OnTextChanged: func() {
									server.Config.Version = server.VersionEdit.Text()
									if server.Logger != nil {
										server.Logger.DebugLog("版本已更新: %s", server.Config.Version)
									}
									// 更新当前配置到配置列表
									if index := server.ConfigTable.CurrentIndex(); index >= 0 {
										server.ConfigList[index] = server.Config
										server.ConfigListModel.PublishRowsReset()
										server.SaveConfig()
									}
								},
								AssignTo: &server.VersionEdit,
							},
						},
					},
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
						Children: []declarative.Widget{
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
									server.SaveConfig()
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
