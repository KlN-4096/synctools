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

type SyncClient struct {
	serverHost string
	serverPort int
	syncDir    string
	logBox     *walk.TextEdit
	status     *walk.StatusBarItem
	connected  bool
	conn       net.Conn
}

func NewSyncClient() *SyncClient {
	return &SyncClient{
		serverHost: "localhost",
		serverPort: 6666,
		syncDir:    ".",
		connected:  false,
	}
}

func (c *SyncClient) log(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	logMsg := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	c.logBox.AppendText(logMsg)
}

func (c *SyncClient) getLocalFilesInfo() (map[string]common.FileInfo, error) {
	filesInfo := make(map[string]common.FileInfo)

	err := filepath.Walk(c.syncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(c.syncDir, path)
			if err != nil {
				return err
			}

			hash, err := common.CalculateMD5(path)
			if err != nil {
				return err
			}

			filesInfo[relPath] = common.FileInfo{
				Hash: hash,
				Size: info.Size(),
			}
		}
		return nil
	})

	return filesInfo, err
}

func (c *SyncClient) logCompare(format string, v ...interface{}) {
	fmt.Printf("\r"+format, v...)
}

func (c *SyncClient) syncWithServer() error {
	if !c.connected {
		return fmt.Errorf("未连接到服务器")
	}

	c.log("开始接收服务器文件信息...")
	var serverFiles map[string]common.FileInfo
	if err := common.ReadJSON(c.conn, &serverFiles); err != nil {
		return fmt.Errorf("接收服务器文件信息错误: %v", err)
	}
	c.log("服务器文件信息接收完成\n")

	c.log("开始获取本地文件信息...")
	localFiles, err := c.getLocalFilesInfo()
	if err != nil {
		return fmt.Errorf("获取本地文件信息错误: %v", err)
	}
	c.log("本地文件信息获取完成\n")

	c.log("开始比较文件...")
	total := len(serverFiles)
	current := 0
	needUpdate := make(map[string]common.FileInfo)

	for filename, serverInfo := range serverFiles {
		current++
		c.logCompare("正在比较文件 (%d/%d): %s", current, total, filename)

		localInfo, exists := localFiles[filename]
		if !exists || localInfo.Hash != serverInfo.Hash {
			needUpdate[filename] = serverInfo
		}
	}
	fmt.Println()
	c.log("文件比较完成，需要更新 %d 个文件\n", len(needUpdate))

	if len(needUpdate) > 0 {
		current = 0
		for filename, serverInfo := range needUpdate {
			current++
			c.log("正在更新文件 (%d/%d): %s", current, len(needUpdate), filename)

			if err := common.WriteJSON(c.conn, filename); err != nil {
				return fmt.Errorf("发送文件请求错误: %v", err)
			}

			filePath := filepath.Join(c.syncDir, filename)
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return fmt.Errorf("创建目录错误: %v", err)
			}

			file, err := os.Create(filePath)
			if err != nil {
				return fmt.Errorf("创建文件错误: %v", err)
			}

			bytesReceived, err := common.ReceiveFile(c.conn, file, serverInfo.Size)
			file.Close()

			if err != nil {
				os.Remove(filePath)
				return fmt.Errorf("接收文件错误: %v", err)
			}

			c.log("文件更新完成: %s (%d bytes)\n", filename, bytesReceived)
		}
	}

	if err := common.WriteJSON(c.conn, "DONE"); err != nil {
		return fmt.Errorf("发送完成信号错误: %v", err)
	}

	c.log("同步完成！\n")
	c.disconnect()
	return nil
}

func (c *SyncClient) connect() error {
	if c.connected {
		return fmt.Errorf("已经连接到服务器")
	}

	var err error
	c.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", c.serverHost, c.serverPort))
	if err != nil {
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	c.connected = true
	c.status.SetText("状态: 已连接")
	c.log("已连接到服务器 %s:%d", c.serverHost, c.serverPort)

	return nil
}

func (c *SyncClient) disconnect() {
	if c.connected {
		c.connected = false
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.status.SetText("状态: 未连接")
		c.log("已断开连接")
	}
}

func main() {
	client := NewSyncClient()

	var mainWindow *walk.MainWindow
	var hostEdit, portEdit *walk.LineEdit
	var dirLabel *walk.Label

	declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "文件同步客户端",
		MinSize:  declarative.Size{Width: 40, Height: 30},
		Size:     declarative.Size{Width: 800, Height: 600},
		Layout:   declarative.VBox{},
		Children: []declarative.Widget{
			declarative.Composite{
				Layout: declarative.Grid{Columns: 2},
				Children: []declarative.Widget{
					declarative.Label{Text: "服务器地址:"},
					declarative.LineEdit{
						AssignTo: &hostEdit,
						Text:     client.serverHost,
						OnTextChanged: func() {
							client.serverHost = hostEdit.Text()
						},
					},
					declarative.Label{Text: "服务器端口:"},
					declarative.LineEdit{
						AssignTo: &portEdit,
						Text:     fmt.Sprintf("%d", client.serverPort),
						OnTextChanged: func() {
							fmt.Sscanf(portEdit.Text(), "%d", &client.serverPort)
						},
					},
					declarative.Label{Text: "同步目录:"},
					declarative.Label{
						AssignTo: &dirLabel,
						Text:     client.syncDir,
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
								client.syncDir = dlg.FilePath
								dirLabel.SetText(dlg.FilePath)
								client.log("同步目录已更改为: %s", dlg.FilePath)
							}
						},
					},
					declarative.PushButton{
						Text: "连接服务器",
						OnClicked: func() {
							if !client.connected {
								if err := client.connect(); err != nil {
									walk.MsgBox(mainWindow, "错误", err.Error(), walk.MsgBoxIconError)
								}
							}
						},
					},
					declarative.PushButton{
						Text: "断开连接",
						OnClicked: func() {
							if client.connected {
								client.disconnect()
							}
						},
					},
					declarative.PushButton{
						Text: "开始同步",
						OnClicked: func() {
							if !client.connected {
								walk.MsgBox(mainWindow, "错误", "请先连接到服务器", walk.MsgBoxIconError)
								return
							}

							go func() {
								if err := client.syncWithServer(); err != nil {
									client.disconnect()
									walk.MsgBox(mainWindow, "错误", err.Error(), walk.MsgBoxIconError)
								} else {
									client.log("同步完成")
								}
							}()
						},
					},
				},
			},
			declarative.TextEdit{
				AssignTo: &client.logBox,
				ReadOnly: true,
				VScroll:  true,
			},
		},
		StatusBarItems: []declarative.StatusBarItem{
			{
				AssignTo: &client.status,
				Text:     "状态: 未连接",
			},
		},
	}.Run()
}
