package ui

import (
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"

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
							declarative.Composite{
								AssignTo: &server.RedirectComposite,
								Layout: declarative.VBox{
									MarginsZero: true,
									SpacingZero: true,
								},
								MinSize: declarative.Size{Height: 100},
							},
							// 添加按钮
							declarative.Composite{
								Layout: declarative.HBox{},
								Children: []declarative.Widget{
									declarative.HSpacer{},
									declarative.PushButton{
										Text:    "添加重定向",
										MinSize: declarative.Size{Width: 100},
										OnClicked: func() {
											addRedirectConfig(server, "新服务器文件夹", "新客户端文件夹")
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

// 添加新函数用于创建重定向配置行
func addRedirectConfig(server *server.SyncServer, initialServerPath, initialClientPath string) {
	composite, err := walk.NewComposite(server.RedirectComposite)
	if err != nil {
		return
	}

	// 创建水平布局并设置边距
	layout := walk.NewHBoxLayout()
	// 使用与 declarative 相同的方式设置布局
	if err := composite.SetLayout(layout); err != nil {
		return
	}

	// 设置复合组件的样式以匹配 declarative 的设置
	composite.SetMinMaxSize(walk.Size{Height: 22}, walk.Size{Height: 22})

	// 服务器路径标签和输入框
	serverLabel, err := walk.NewLabel(composite)
	if err != nil {
		return
	}
	serverLabel.SetText("服务器文件夹:")

	serverEdit, err := walk.NewLineEdit(composite)
	if err != nil {
		return
	}
	serverEdit.SetText(initialServerPath)
	// 设置输入框的最大高度
	serverEdit.SetMinMaxSize(walk.Size{Height: 20}, walk.Size{Height: 20})

	// 客户端路径标签和输入框
	clientLabel, err := walk.NewLabel(composite)
	if err != nil {
		return
	}
	clientLabel.SetText("客户端文件夹:")

	clientEdit, err := walk.NewLineEdit(composite)
	if err != nil {
		return
	}
	clientEdit.SetText(initialClientPath)
	// 设置输入框的最大高度
	clientEdit.SetMinMaxSize(walk.Size{Height: 20}, walk.Size{Height: 20})

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
	// 设置按钮的最大高度
	deleteBtn.SetMinMaxSize(walk.Size{Height: 20}, walk.Size{Height: 20})

	// 添加文本更改事件
	serverEdit.TextChanged().Attach(func() {
		server.UpdateRedirectConfig()
	})
	clientEdit.TextChanged().Attach(func() {
		server.UpdateRedirectConfig()
	})

	// 强制重新布局
	server.RedirectComposite.SendMessage(win.WM_SIZE, 0, 0)
}
