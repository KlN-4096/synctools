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
	invalidLabel *walk.TextEdit // 用于显示无效文件夹列表
	status       *walk.StatusBarItem
	running      bool
	listener     net.Listener
}

func NewSyncServer() *SyncServer {
	return &SyncServer{
		host:         "0.0.0.0",
		port:         6666,
		syncDir:      "",
		syncFolders:  make([]string, 0),
		validFolders: make(map[string]bool),
		ignoreList:   []string{".clientconfig", ".DS_Store", "thumbs.db"},
		running:      false,
	}
}

func (s *SyncServer) log(format string, v ...interface{}) {
	common.WriteLog(s.logBox, format, v...)
}

func (s *SyncServer) validateFolders() {
	s.validFolders = make(map[string]bool)
	var invalidFolders []string

	for _, folder := range s.syncFolders {
		path := filepath.Join(s.syncDir, folder)
		valid := common.IsPathExists(path) && common.IsDir(path)
		s.validFolders[folder] = valid
		if !valid {
			invalidFolders = append(invalidFolders, folder)
		}
	}

	// 更新无效文件夹列表显示
	if len(invalidFolders) > 0 {
		s.invalidLabel.SetText("无效的文件夹:\r\n" + strings.Join(invalidFolders, "\r\n"))
	} else {
		s.invalidLabel.SetText("")
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
		MinSize:  declarative.Size{Width: 800, Height: 600},
		Layout:   declarative.VBox{},
		Children: []declarative.Widget{
			// 顶部控制面板
			declarative.Composite{
				Layout: declarative.HBox{MarginsZero: true},
				Children: []declarative.Widget{
					// 左侧设置面板
					declarative.GroupBox{
						Title:  "服务器设置",
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
						},
					},
					// 右侧目录设置面板
					declarative.GroupBox{
						Title:  "目录设置",
						Layout: declarative.Grid{Columns: 2},
						Children: []declarative.Widget{
							declarative.Label{Text: "同步目录:"},
							declarative.Composite{
								Layout: declarative.HBox{MarginsZero: true},
								Children: []declarative.Widget{
									declarative.Label{
										AssignTo: &dirLabel,
										Text:     server.syncDir,
									},
									declarative.PushButton{
										Text:      "选择",
										MaxSize:   declarative.Size{Width: 60},
										OnClicked: server.selectDirectory(mainWindow, dirLabel),
									},
								},
							},
						},
					},
				},
			},
			// 控制按钮面板
			declarative.Composite{
				Layout: declarative.HBox{MarginsZero: true},
				Children: []declarative.Widget{
					declarative.HSpacer{},
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
				},
			},
			// 主要内容区域
			declarative.Composite{
				Layout: declarative.HBox{MarginsZero: true},
				Children: []declarative.Widget{
					// 左侧文件夹列表
					declarative.GroupBox{
						Title:  "同步文件夹列表",
						Layout: declarative.VBox{},
						Children: []declarative.Widget{
							declarative.TextEdit{
								AssignTo: &server.folderEdit,
								VScroll:  true,
								MinSize:  declarative.Size{Width: 200},
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
							declarative.TextEdit{
								AssignTo:   &server.invalidLabel,
								ReadOnly:   true,
								VScroll:    true,
								MaxSize:    declarative.Size{Height: 100},
								Background: declarative.SolidColorBrush{Color: walk.RGB(255, 240, 240)},
							},
						},
					},
					// 右侧日志区域
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

// 将目录选择功能提取为单独的方法
func (s *SyncServer) selectDirectory(owner walk.Form, dirLabel *walk.Label) func() {
	return func() {
		dlg := new(walk.FileDialog)
		dlg.Title = "选择同步目录"

		if ok, err := dlg.ShowBrowseFolder(owner); err != nil {
			walk.MsgBox(owner, "错误",
				"选择目录时发生错误: "+err.Error(),
				walk.MsgBoxIconError)
			return
		} else if !ok {
			return
		}

		if dlg.FilePath != "" {
			s.syncDir = dlg.FilePath
			dirLabel.SetText(dlg.FilePath)
			s.log("同步目录已更改为: %s", dlg.FilePath)
			if len(s.syncFolders) > 0 {
				s.validateFolders()
			}
		}
	}
}
