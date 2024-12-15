package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/pkg/common"
)

type SyncServer struct {
	host         string
	port         int
	syncDir      string
	syncFolders  []string        // 需要同步的文件夹列表
	validFolders map[string]bool // 标记文件夹是否有效
	ignoreList   []string
	logBox       *walk.TextEdit
	folderEdit   *walk.TextEdit // 用于编辑同步文件夹列表
	status       *walk.StatusBarItem
	running      bool
	listener     net.Listener
	invalidLabel *walk.TextEdit // 用于显示无效文件夹列表
	debugMode    bool
}

func NewSyncServer() *SyncServer {
	server := &SyncServer{
		host: "0.0.0.0",
		port: 6666,

		syncDir:      "",
		syncFolders:  []string{"aaa", "bbb", "ccc", "ddd"},
		validFolders: make(map[string]bool),
		ignoreList:   []string{".clientconfig", ".DS_Store", "thumbs.db"},
		running:      false,
	}

	// 在窗口创建后设置初始文本
	defer func() {
		if server.folderEdit != nil {
			server.folderEdit.SetText(strings.Join(server.syncFolders, "\r\n"))
		}
	}()

	return server
}

func (s *SyncServer) log(format string, v ...interface{}) {
	common.WriteLog(s.logBox, format, v...)
}

func (s *SyncServer) debugLog(format string, v ...interface{}) {
	if !s.debugMode {
		return
	}
	s.log("[DEBUG] "+format, v...)
}

func (s *SyncServer) validateFolders() {
	s.validFolders = make(map[string]bool)
	var invalidFolders []string

	s.debugLog("开始验证文件夹列表...")
	s.debugLog("当前根目录: %s", s.syncDir)
	s.debugLog("待验证文件夹数: %d", len(s.syncFolders))

	for _, folder := range s.syncFolders {
		path := filepath.Join(s.syncDir, folder)
		valid := common.IsPathExists(path) && common.IsDir(path)
		s.validFolders[folder] = valid
		if valid {
			s.debugLog("有效的同步文件夹: %s", folder)
		} else {
			s.debugLog(">>> 无效的同步文件夹: %s <<<", folder)
			invalidFolders = append(invalidFolders, folder)
		}
	}

	// 更新无效文件夹文本框
	if len(invalidFolders) > 0 {
		s.invalidLabel.SetText(strings.Join(invalidFolders, "\r\n"))
		if s.debugMode {
			s.log("----------------------------------------")
			s.log("发现 %d 个无效文件夹:", len(invalidFolders))
			for i, folder := range invalidFolders {
				s.log("%d. %s", i+1, folder)
			}
			s.log("----------------------------------------")
		}
	} else {
		s.invalidLabel.SetText("")
		s.debugLog("所有文件夹都有效")
	}
}

func (s *SyncServer) getFilesInfo() (map[string]common.FileInfo, error) {
	filesInfo := make(map[string]common.FileInfo)

	for folder, valid := range s.validFolders {
		if !valid {
			continue
		}

		folderPath := filepath.Join(s.syncDir, folder)
		info, err := common.GetFilesInfo(folderPath, s.ignoreList, s.logBox)
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
	s.log("客户端连接: %s", clientAddr)

	filesInfo, err := s.getFilesInfo()
	if err != nil {
		s.log("获取文件信息错误: %v", err)
		return
	}

	if err := common.WriteJSON(conn, filesInfo); err != nil {
		s.log("发送文件信息错误 %s: %v", clientAddr, err)
		return
	}

	for {
		var filename string
		if err := common.ReadJSON(conn, &filename); err != nil {
			if err != common.ErrConnectionClosed {
				s.log("接收文件请求错误 %s: %v", clientAddr, err)
			}
			return
		}

		if filename == "DONE" {
			s.log("客户端 %s 完成同步", clientAddr)
			return
		}

		s.log("客户端 %s 请求文件: %s", clientAddr, filename)

		filepath := filepath.Join(s.syncDir, filename)
		file, err := os.Open(filepath)
		if err != nil {
			s.log("打开文件错误 %s: %v", filename, err)
			continue
		}

		bytesSent, err := common.SendFile(conn, file)
		file.Close()

		if err != nil {
			s.log("发送文件错误 %s to %s: %v", filename, clientAddr, err)
			return
		}
		s.log("发送文件 %s to %s (%d bytes)", filename, clientAddr, bytesSent)
	}
}

func (s *SyncServer) startServer() error {
	if s.running {
		return fmt.Errorf("服务器已经在运行")
	}

	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		return fmt.Errorf("启动服务器失败: %v", err)
	}

	s.running = true
	s.status.SetText("状态: 运行中")
	s.log("服务器启动于 %s:%d", s.host, s.port)
	s.log("同步目录: %s", s.syncDir)

	go func() {
		for s.running {
			conn, err := s.listener.Accept()
			if err != nil {
				if s.running {
					s.log("接受连接错误: %v", err)
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
		s.log("服务器已停止")
	}
}

func main() {
	server := NewSyncServer()

	var mainWindow *walk.MainWindow
	var hostEdit, portEdit *walk.LineEdit
	var dirLabel *walk.Label

	declarative.MainWindow{
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
										Text:     server.host,
										OnTextChanged: func() {
											server.host = hostEdit.Text()
										},
									},
									declarative.Label{Text: "端口:"},
									declarative.LineEdit{
										AssignTo: &portEdit,
										Text:     fmt.Sprintf("%d", server.port),
										OnTextChanged: func() {
											fmt.Sscanf(portEdit.Text(), "%d", &server.port)
										},
									},
									declarative.Label{Text: "同步目录:"},
									declarative.Label{
										AssignTo: &dirLabel,
										Text:     server.syncDir,
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
												server.syncDir = dlg.FilePath
												dirLabel.SetText(dlg.FilePath)
												server.log("同步目录已更改为: %s", dlg.FilePath)
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
									declarative.HSpacer{}, // 添加弹性空间
									declarative.CheckBox{
										Text: "调试模式",
										OnCheckedChanged: func() {
											server.debugMode = !server.debugMode
											if server.debugMode {
												server.log("调试模式已启用")
											} else {
												server.log("调试模式已关闭")
											}
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
											if server.syncDir != "" {
												server.validateFolders()
											}
										},
									},
									declarative.Label{
										Text:      "无效的文件夹列表:",
										TextColor: walk.RGB(192, 0, 0), // 使用红色文字
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
								AssignTo: &server.logBox,
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
	}.Run()
}
