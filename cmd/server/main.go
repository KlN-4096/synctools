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
	host       string
	port       int
	syncDir    string
	ignoreList []string
	logBox     *walk.TextEdit
	status     *walk.StatusBarItem
	running    bool
	listener   net.Listener
}

func NewSyncServer() *SyncServer {
	return &SyncServer{
		host:       "0.0.0.0",
		port:       6666,
		syncDir:    ".",
		ignoreList: []string{".clientconfig", ".DS_Store", "thumbs.db"},
		running:    false,
	}
}

func (s *SyncServer) log(format string, v ...interface{}) {
	logMsg := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, v...))
	s.logBox.AppendText(logMsg)
}

func (s *SyncServer) getFilesInfo() (map[string]common.FileInfo, error) {
	filesInfo := make(map[string]common.FileInfo)

	err := filepath.Walk(s.syncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(s.syncDir, path)
			if err != nil {
				return err
			}

			for _, ignore := range s.ignoreList {
				if strings.Contains(relPath, ignore) {
					return nil
				}
			}

			hash, err := common.CalculateMD5(path)
			if err != nil {
				return err
			}

			filesInfo[relPath] = common.FileInfo{
				Hash: hash,
				Size: info.Size(),
			}
			s.log("添加文件: %s, 大小: %d bytes", relPath, info.Size())
		}
		return nil
	})

	return filesInfo, err
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
				},
			},
			declarative.TextEdit{
				AssignTo: &server.logBox,
				ReadOnly: true,
				VScroll:  true,
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
