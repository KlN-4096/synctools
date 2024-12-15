package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/pkg/common"
)

type SyncServer struct {
	config            common.SyncConfig
	syncFolders       []string
	validFolders      map[string]bool
	logger            *common.GUILogger
	folderEdit        *walk.TextEdit
	status            *walk.StatusBarItem
	running           bool
	listener          net.Listener
	invalidLabel      *walk.TextEdit
	serverPath        *walk.LineEdit
	clientPath        *walk.LineEdit
	configFile        string          // 配置文件路径
	redirectComposite *walk.Composite // 用于存放重定向配置的容器
}

func NewSyncServer() *SyncServer {
	// 设置配置文件路径
	configDir := filepath.Join(os.Getenv("APPDATA"), "SyncTools")
	configFile := filepath.Join(configDir, "server_config.json")

	// 加载配置
	config, err := common.LoadConfig(configFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		config = &common.SyncConfig{
			Host:       "0.0.0.0",
			Port:       6666,
			SyncDir:    "",
			IgnoreList: []string{".clientconfig", ".DS_Store", "thumbs.db"},
			FolderRedirects: []common.FolderRedirect{
				{ServerPath: "clientmods", ClientPath: "mods"},
			},
		}
	}

	// 确保配置中至少有一个重定向配置
	if len(config.FolderRedirects) == 0 {
		config.FolderRedirects = []common.FolderRedirect{
			{ServerPath: "clientmods", ClientPath: "mods"},
		}
	}

	return &SyncServer{
		config:       *config,
		configFile:   configFile,
		syncFolders:  []string{},
		validFolders: make(map[string]bool),
		running:      false,
	}
}

// saveConfig 保存配置到文件
func (s *SyncServer) saveConfig() error {
	if err := common.SaveConfig(&s.config, s.configFile); err != nil {
		s.logger.Log("保存配置失败: %v", err)
		return err
	}
	s.logger.Log("配置已保存到: %s", s.configFile)
	return nil
}

func (s *SyncServer) validateFolders() {
	s.validFolders = make(map[string]bool)
	var invalidFolders []string

	s.logger.DebugLog("开始验证文件夹列表...")
	s.logger.DebugLog("当前根目录: %s", s.config.SyncDir)
	s.logger.DebugLog("待验证文件夹数: %d", len(s.syncFolders))

	for _, folder := range s.syncFolders {
		path := filepath.Join(s.config.SyncDir, folder)
		valid := common.IsPathExists(path) && common.IsDir(path)
		s.validFolders[folder] = valid
		if valid {
			s.logger.DebugLog("有效的同步文件夹: %s", folder)
		} else {
			s.logger.DebugLog(">>> 无效的同步文件夹: %s <<<", folder)
			invalidFolders = append(invalidFolders, folder)
		}
	}

	// 更新无效文件夹文本框
	if len(invalidFolders) > 0 {
		s.invalidLabel.SetText(strings.Join(invalidFolders, "\r\n"))
		s.logger.DebugLog("----------------------------------------")
		s.logger.DebugLog("发现 %d 个无效文件夹:", len(invalidFolders))
		for i, folder := range invalidFolders {
			s.logger.DebugLog("%d. %s", i+1, folder)
		}
		s.logger.DebugLog("----------------------------------------")
	} else {
		s.invalidLabel.SetText("")
		s.logger.DebugLog("所有文件夹都有效")
	}
}

func (s *SyncServer) getRedirectedPath(path string) string {
	for _, redirect := range s.config.FolderRedirects {
		if strings.HasPrefix(path, redirect.ClientPath+string(os.PathSeparator)) {
			return strings.Replace(path, redirect.ClientPath, redirect.ServerPath, 1)
		}
	}
	return path
}

func (s *SyncServer) getOriginalPath(path string) string {
	for _, redirect := range s.config.FolderRedirects {
		if strings.HasPrefix(path, redirect.ServerPath+string(os.PathSeparator)) {
			return strings.Replace(path, redirect.ServerPath, redirect.ClientPath, 1)
		}
	}
	return path
}

func (s *SyncServer) getFilesInfo() (map[string]common.FileInfo, error) {
	filesInfo := make(map[string]common.FileInfo)

	for folder, valid := range s.validFolders {
		if !valid {
			continue
		}

		folderPath := filepath.Join(s.config.SyncDir, folder)
		info, err := common.GetFilesInfo(folderPath, s.config.IgnoreList, s.logger)
		if err != nil {
			return nil, fmt.Errorf("获取文件夹 %s 信息失败: %v", folder, err)
		}

		// 将文件路径加上文件夹前缀，并处理重定向
		for file, fileInfo := range info {
			redirectedPath := s.getOriginalPath(filepath.Join(folder, file))
			filesInfo[redirectedPath] = fileInfo
		}
	}

	return filesInfo, nil
}

func (s *SyncServer) handleClient(conn net.Conn) {
	defer conn.Close()
	clientAddr := conn.RemoteAddr().String()
	s.logger.Log("客户端连接: %s", clientAddr)

	filesInfo, err := s.getFilesInfo()
	if err != nil {
		s.logger.Log("获取文件信息错误: %v", err)
		return
	}

	if err := common.WriteJSON(conn, filesInfo); err != nil {
		s.logger.Log("发送文件信息错误 %s: %v", clientAddr, err)
		return
	}

	for {
		var filename string
		if err := common.ReadJSON(conn, &filename); err != nil {
			if err != common.ErrConnectionClosed {
				s.logger.Log("接收文件请求错误 %s: %v", clientAddr, err)
			}
			return
		}

		if filename == "DONE" {
			s.logger.Log("客户端 %s 完成同步", clientAddr)
			return
		}

		s.logger.Log("客户端 %s 请求文件: %s", clientAddr, filename)

		// 获取重定向后的文件路径
		redirectedPath := s.getRedirectedPath(filename)
		filepath := filepath.Join(s.config.SyncDir, redirectedPath)

		s.logger.DebugLog("重定向路径: %s -> %s", filename, redirectedPath)

		file, err := os.Open(filepath)
		if err != nil {
			s.logger.Log("打开文件错误 %s: %v", filename, err)
			continue
		}

		bytesSent, err := common.SendFile(conn, file)
		file.Close()

		if err != nil {
			s.logger.Log("发送文件错误 %s to %s: %v", filename, clientAddr, err)
			return
		}
		s.logger.Log("发送文件 %s to %s (%d bytes)", filename, clientAddr, bytesSent)
	}
}

func (s *SyncServer) startServer() error {
	if s.running {
		return common.ErrServerRunning
	}

	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", s.config.Host, s.config.Port))
	if err != nil {
		return fmt.Errorf("启动服务器失败: %v", err)
	}

	s.running = true
	s.status.SetText("状态: 运行中")
	s.logger.Log("服务器启动于 %s:%d", s.config.Host, s.config.Port)
	s.logger.Log("同步目录: %s", s.config.SyncDir)

	go func() {
		for s.running {
			conn, err := s.listener.Accept()
			if err != nil {
				if s.running {
					s.logger.Log("接受连接错误: %v", err)
				}
				continue
			}

			go s.handleClient(conn)
		}
	}()

	return nil
}

func (s *SyncServer) stopServer() {
	if s.running {
		s.running = false
		if s.listener != nil {
			s.listener.Close()
		}
		s.status.SetText("状态: 已停止")
		s.logger.Log("服务器已停止")
	}
}

func (s *SyncServer) updateRedirectConfig() {
	// 清空当前配置
	s.config.FolderRedirects = nil

	// 添加固定的重定向配置
	s.config.FolderRedirects = append(s.config.FolderRedirects, common.FolderRedirect{
		ServerPath: s.serverPath.Text(),
		ClientPath: s.clientPath.Text(),
	})

	// 遍历所有动态重定向配置组件
	for i := 0; i < s.redirectComposite.Children().Len(); i++ {
		composite := s.redirectComposite.Children().At(i).(*walk.Composite)
		if composite.Children().Len() < 2 {
			continue
		}

		serverEdit := composite.Children().At(0).(*walk.LineEdit)
		clientEdit := composite.Children().At(1).(*walk.LineEdit)

		s.config.FolderRedirects = append(s.config.FolderRedirects, common.FolderRedirect{
			ServerPath: serverEdit.Text(),
			ClientPath: clientEdit.Text(),
		})
	}

	if s.logger != nil {
		s.logger.DebugLog("重定向配置已更新: %v", s.config.FolderRedirects)
	}
}

func main() {
	// 设置 panic 处理
	defer func() {
		if r := recover(); r != nil {
			// 创建应急日志文件
			logFile, err := os.OpenFile(
				filepath.Join("logs", fmt.Sprintf("server_crash_%s.log",
					time.Now().Format("2006-01-02_15-04-05"))),
				os.O_CREATE|os.O_WRONLY|os.O_APPEND,
				0644,
			)
			if err == nil {
				fmt.Fprintf(logFile, "[%s] 程序崩溃: %v\n",
					time.Now().Format("2006-01-02 15:04:05"), r)
				if err := logFile.Close(); err != nil {
					fmt.Printf("关闭崩溃日志文件失败: %v\n", err)
				}
			}
			// 显示错误对话框
			if _, err := os.Stat("logs"); err == nil {
				walk.MsgBox(nil, "错误",
					fmt.Sprintf("程序发生错误: %v\n详细信息请查看日志文件", r),
					walk.MsgBoxIconError)
			}
			panic(r) // 重新抛出 panic
		}
	}()

	// 确保日志目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		walk.MsgBox(nil, "错误",
			fmt.Sprintf("创建日志目录失败: %v", err),
			walk.MsgBoxIconError)
		return
	}

	server := NewSyncServer()

	var mainWindow *walk.MainWindow
	var hostEdit, portEdit *walk.LineEdit
	var dirLabel *walk.Label
	var logBox *walk.TextEdit
	var ignoreListEdit *walk.TextEdit

	mw := declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "文件同步服务器",
		MinSize:  declarative.Size{Width: 40, Height: 30},
		Size:     declarative.Size{Width: 800, Height: 600},
		Layout:   declarative.VBox{},
		Children: []declarative.Widget{
			declarative.TabWidget{
				Pages: []declarative.TabPage{
					// 主页标签
					{
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
														AssignTo: &hostEdit,
														Text:     server.config.Host,
														OnTextChanged: func() {
															server.config.Host = hostEdit.Text()
														},
													},
													declarative.Label{Text: "端口:"},
													declarative.LineEdit{
														AssignTo: &portEdit,
														Text:     fmt.Sprintf("%d", server.config.Port),
														OnTextChanged: func() {
															fmt.Sscanf(portEdit.Text(), "%d", &server.config.Port)
														},
													},
													declarative.Label{Text: "同步目录:"},
													declarative.Label{
														AssignTo: &dirLabel,
														Text:     server.config.SyncDir,
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

															if ok, err := dlg.ShowBrowseFolder(mainWindow); err != nil {
																walk.MsgBox(mainWindow, "错误",
																	"选择目录时发生错误: "+err.Error(),
																	walk.MsgBoxIconError)
																return
															} else if !ok {
																return
															}

															if dlg.FilePath != "" {
																server.config.SyncDir = dlg.FilePath
																dirLabel.SetText(dlg.FilePath)
																server.logger.Log("同步目录已更改为: %s", dlg.FilePath)
																server.validateFolders()
															}
														},
													},
													declarative.PushButton{
														Text: "启动服务器",
														OnClicked: func() {
															if !server.running {
																if err := server.startServer(); err != nil {
																	walk.MsgBox(mainWindow, "错误", err.Error(), walk.MsgBoxIconError)
																}
															}
														},
													},
													declarative.PushButton{
														Text: "停止服务器",
														OnClicked: func() {
															if server.running {
																server.stopServer()
															}
														},
													},
													declarative.HSpacer{},
													declarative.CheckBox{
														Text: "调试模式",
														OnCheckedChanged: func() {
															server.logger.SetDebugMode(!server.logger.DebugMode)
														},
													},
												},
											},
											declarative.Composite{
												Layout: declarative.VBox{},
												Children: []declarative.Widget{
													declarative.Label{Text: "同步文件夹列表 (每行一个):"},
													declarative.TextEdit{
														AssignTo: &server.folderEdit,
														VScroll:  true,
														MinSize:  declarative.Size{Height: 100},
														OnTextChanged: func() {
															text := server.folderEdit.Text()
															folders := strings.Split(text, "\r\n")
															var validFolders []string
															for _, folder := range folders {
																if strings.TrimSpace(folder) != "" {
																	validFolders = append(validFolders, folder)
																}
															}
															server.syncFolders = validFolders
															if server.config.SyncDir != "" {
																server.validateFolders()
															}
														},
													},
													declarative.Label{
														Text:      "无效的文件夹列表:",
														TextColor: walk.RGB(192, 0, 0),
													},
													declarative.TextEdit{
														AssignTo:   &server.invalidLabel,
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
												AssignTo: &logBox,
												ReadOnly: true,
												VScroll:  true,
											},
										},
									},
								},
							},
						},
					},
					// 配置标签
					{
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
												AssignTo: &server.serverPath,
												Text:     server.config.FolderRedirects[0].ServerPath,
												OnTextChanged: func() {
													if len(server.config.FolderRedirects) > 0 {
														server.config.FolderRedirects[0].ServerPath = server.serverPath.Text()
														if server.logger != nil {
															server.logger.DebugLog("服务器文件夹已更改为: %s", server.serverPath.Text())
														}
													}
												},
											},
											declarative.LineEdit{
												AssignTo: &server.clientPath,
												Text:     server.config.FolderRedirects[0].ClientPath,
												OnTextChanged: func() {
													if len(server.config.FolderRedirects) > 0 {
														server.config.FolderRedirects[0].ClientPath = server.clientPath.Text()
														if server.logger != nil {
															server.logger.DebugLog("客户端文件夹已更改为: %s", server.clientPath.Text())
														}
													}
												},
											},
										},
									},
									declarative.Label{Text: "额外的重定向配置:"},
									declarative.Composite{
										AssignTo: &server.redirectComposite,
										Layout:   declarative.VBox{},
									},
									declarative.PushButton{
										Text: "+",
										OnClicked: func() {
											composite, err := walk.NewComposite(server.redirectComposite)
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
													server.updateRedirectConfig()
												})
											}

											// 添加新的重定向配置
											server.config.FolderRedirects = append(server.config.FolderRedirects, common.FolderRedirect{
												ServerPath: "新服务器文件夹",
												ClientPath: "新客户端文件夹",
											})

											if server.logger != nil {
												server.logger.Log("已添加新的重定向配置")
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
											if err := server.saveConfig(); err != nil {
												walk.MsgBox(mainWindow, "错误",
													fmt.Sprintf("保存配置失败: %v", err),
													walk.MsgBoxIconError)
											} else {
												walk.MsgBox(mainWindow, "成功",
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
										AssignTo: &ignoreListEdit,
										Text:     strings.Join(server.config.IgnoreList, "\r\n"),
										VScroll:  true,
										OnTextChanged: func() {
											// 更新忽略列表
											text := ignoreListEdit.Text()
											items := strings.Split(text, "\r\n")
											var ignoreList []string
											for _, item := range items {
												if item = strings.TrimSpace(item); item != "" {
													ignoreList = append(ignoreList, item)
												}
											}
											server.config.IgnoreList = ignoreList
											if server.logger != nil {
												server.logger.DebugLog("忽略列表已更新: %v", ignoreList)
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
					},
				},
			},
		},
		StatusBarItems: []declarative.StatusBarItem{
			{
				AssignTo: &server.status,
				Text:     "状态: 已停止",
			},
		},
	}

	if err := mw.Create(); err != nil {
		walk.MsgBox(nil, "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	// 初始化日志记录器
	logger, err := common.NewGUILogger(logBox, "logs", "server")
	if err != nil {
		walk.MsgBox(mainWindow, "错误", "创建日志记录器失败: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	server.logger = logger
	defer server.logger.Close()

	// 初始化重定向配置显示
	for _, redirect := range server.config.FolderRedirects {
		composite, err := walk.NewComposite(server.redirectComposite)
		if err != nil {
			continue
		}

		if err := composite.SetLayout(walk.NewGridLayout()); err != nil {
			continue
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
			serverEdit.SetText(redirect.ServerPath)
		}

		// 客户端路径输入框
		var clientEdit *walk.LineEdit
		if clientEdit, err = walk.NewLineEdit(composite); err == nil {
			clientEdit.SetText(redirect.ClientPath)
		}

		// 删除按钮
		if deleteBtn, err := walk.NewPushButton(composite); err == nil {
			deleteBtn.SetText("X")
			deleteBtn.Clicked().Attach(func() {
				composite.Dispose()
				server.updateRedirectConfig()
			})
		}
	}

	// 设置初始文本
	server.folderEdit.SetText(strings.Join(server.syncFolders, "\r\n"))

	mainWindow.Run()
}
