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
	config       common.SyncConfig
	syncFolders  []string
	validFolders map[string]bool
	logger       *common.GUILogger
	folderEdit   *walk.TextEdit
	status       *walk.StatusBarItem
	running      bool
	listener     net.Listener
	invalidLabel *walk.TextEdit
}

func NewSyncServer() *SyncServer {
	return &SyncServer{
		config: common.SyncConfig{
			Host:       "0.0.0.0",
			Port:       6666,
			SyncDir:    "",
			IgnoreList: []string{".clientconfig", ".DS_Store", "thumbs.db"},
		},
		syncFolders:  []string{"aaa", "bbb", "ccc", "ddd"},
		validFolders: make(map[string]bool),
		running:      false,
	}
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

		// 将文件路径加上文件夹前缀
		for file, fileInfo := range info {
			filesInfo[filepath.Join(folder, file)] = fileInfo
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

		filepath := filepath.Join(s.config.SyncDir, filename)
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
			panic(r) // 重新抛出 panic
		}
	}()

	server := NewSyncServer()

	var mainWindow *walk.MainWindow
	var hostEdit, portEdit *walk.LineEdit
	var dirLabel *walk.Label
	var logBox *walk.TextEdit

	mw := declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "文件同步服务器",
		MinSize:  declarative.Size{Width: 40, Height: 30},
		Size:     declarative.Size{Width: 800, Height: 600},
		Layout:   declarative.VBox{},
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

	// 设置初始文本
	server.folderEdit.SetText(strings.Join(server.syncFolders, "\r\n"))

	mainWindow.Run()
}
