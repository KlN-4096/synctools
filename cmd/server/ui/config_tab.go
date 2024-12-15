package ui

import (
	"fmt"
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"

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
					declarative.Label{Text: "文件夹重定向列表:"},
					declarative.Composite{
						AssignTo: &server.RedirectComposite,
						Layout:   declarative.VBox{},
					},
					declarative.PushButton{
						Text: "添加重定向",
						OnClicked: func() {
							addRedirectConfig(server, "新服务器文件夹", "新客户端文件夹")
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

// 添加新函数用于创建重定向配置行
func addRedirectConfig(server *server.SyncServer, initialServerPath, initialClientPath string) {
	composite, err := walk.NewComposite(server.RedirectComposite)
	if err != nil {
		return
	}

	if err := composite.SetLayout(walk.NewHBoxLayout()); err != nil {
		return
	}

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

	// 强制重新布局
	server.RedirectComposite.SendMessage(win.WM_SIZE, 0, 0)
}
