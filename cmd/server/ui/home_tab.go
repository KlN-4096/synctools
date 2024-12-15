package ui

import (
	"fmt"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/pkg/common"
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
											if server.Logger != nil {
												server.Logger.SetDebugMode(!server.Logger.GetDebugMode())
											}
										},
									},
								},
							},
							declarative.Composite{
								Layout: declarative.VBox{},
								Children: []declarative.Widget{
									declarative.Label{Text: "同步文件夹列表:"},
									declarative.TableView{
										AssignTo:         &server.FolderTable,
										MinSize:          declarative.Size{Height: 150},
										AlternatingRowBG: true,
										Columns: []declarative.TableViewColumn{
											{Title: "文件夹路径", Width: 0, Alignment: declarative.AlignNear},
											{Title: "同步模式", Width: 120, Alignment: declarative.AlignCenter},
										},
										Model: server.FolderModel,
										OnBoundsChanged: func() {
											// 获取父容器
											if parent := server.FolderTable.Parent(); parent != nil {
												// 获取父容器的客户区宽度
												parentWidth := parent.ClientBounds().Width
												// 第二列固定宽度120，第一列自动填充剩余空间
												server.FolderTable.Columns().At(0).SetWidth(parentWidth - 120)
												server.FolderTable.Columns().At(1).SetWidth(120)
											}
										},
										OnItemActivated: func() {
											if index := server.FolderTable.CurrentIndex(); index >= 0 {
												folder := &server.Config.SyncFolders[index]
												if dlg, err := walk.NewDialog(server.FolderTable.Form()); err == nil {
													dlg.SetTitle("编辑同步文件夹")
													dlg.SetLayout(walk.NewVBoxLayout())

													var pathEdit *walk.LineEdit
													var modeCombo *walk.ComboBox

													declarative.Composite{
														Layout: declarative.Grid{Columns: 2},
														Children: []declarative.Widget{
															declarative.Label{Text: "文件夹路径:"},
															declarative.LineEdit{
																AssignTo: &pathEdit,
																Text:     folder.Path,
															},
															declarative.Label{Text: "同步模式:"},
															declarative.ComboBox{
																AssignTo: &modeCombo,
																Model:    []string{"mirror", "push"},
																Value:    folder.SyncMode,
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
																	folder.Path = pathEdit.Text()
																	folder.SyncMode = modeCombo.Text()
																	server.FolderModel.PublishRowsReset()
																	server.ValidateFolders()
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
												Text: "添加文件夹",
												OnClicked: func() {
													server.Config.SyncFolders = append(server.Config.SyncFolders, common.SyncFolder{
														Path:     "新文件夹",
														SyncMode: "mirror",
													})
													server.FolderModel.PublishRowsReset()
													server.ValidateFolders()
												},
											},
											declarative.PushButton{
												Text: "删除选中",
												OnClicked: func() {
													if index := server.FolderTable.CurrentIndex(); index >= 0 {
														server.Config.SyncFolders = append(
															server.Config.SyncFolders[:index],
															server.Config.SyncFolders[index+1:]...,
														)
														server.FolderModel.PublishRowsReset()
														server.ValidateFolders()
													}
												},
											},
										},
									},
									declarative.Label{
										Text:      "无效的文件夹列表:",
										TextColor: walk.RGB(192, 0, 0),
									},
									declarative.TextEdit{
										AssignTo: &server.InvalidLabel,
										ReadOnly: true,
										VScroll:  true,
										MinSize:  declarative.Size{Height: 60},
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
