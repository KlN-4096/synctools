package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/pkg/common"
)

type SyncClient struct {
	config     common.SyncConfig
	logger     *common.GUILogger
	status     *walk.StatusBarItem
	conn       net.Conn
	syncStatus common.SyncStatus
	version    *walk.LineEdit
}

func NewSyncClient() *SyncClient {
	return &SyncClient{
		config: common.SyncConfig{
			Host:       "localhost",
			Port:       6666,
			SyncDir:    "",
			IgnoreList: nil,
		},
		syncStatus: common.SyncStatus{
			Connected: false,
			Running:   false,
			Message:   "未连接",
		},
	}
}

func (c *SyncClient) getLocalFilesInfo() (map[string]common.FileInfo, error) {
	return common.GetFilesInfo(c.config.SyncDir, c.config.IgnoreList, c.logger)
}

func (c *SyncClient) logCompare(format string, v ...interface{}) {
	fmt.Printf("\r"+format, v...)
}

func (c *SyncClient) syncWithServer() error {
	if !c.syncStatus.Connected {
		return common.ErrNotConnected
	}

	// 发送同步路径
	if err := common.WriteJSON(c.conn, ""); err != nil {
		return fmt.Errorf("发送同步路径错误: %v", err)
	}

	// 接收同步信息
	var syncInfo common.SyncInfo
	if err := common.ReadJSON(c.conn, &syncInfo); err != nil {
		return fmt.Errorf("接收同步信息错误: %v", err)
	}

	c.logger.Log("开始获取本地文件信息...")
	localFiles, err := c.getLocalFilesInfo()
	if err != nil {
		return fmt.Errorf("获取本地文件信息错误: %v", err)
	}
	c.logger.Log("本地文件信息获取完成")

	c.logger.Log("开始比较文件...")
	total := len(syncInfo.Files)
	current := 0
	needUpdate := make(map[string]common.FileInfo)

	// 如果版本不同，需要删除服务端没有的文件
	if syncInfo.DeleteExtraFiles {
		c.logger.Log("版本不同，将删除服务端没有的文件")
		for filename := range localFiles {
			if _, exists := syncInfo.Files[filename]; !exists {
				filePath := filepath.Join(c.config.SyncDir, filename)
				c.logger.Log("删除文件: %s", filename)
				os.Remove(filePath)
			}
		}
	}

	for filename, serverInfo := range syncInfo.Files {
		current++
		c.logCompare("正在比较文件 (%d/%d): %s", current, total, filename)

		localInfo, exists := localFiles[filename]
		if !exists || localInfo.Hash != serverInfo.Hash {
			needUpdate[filename] = serverInfo
		}
	}
	fmt.Println()
	c.logger.Log("文件比较完成，需要更新 %d 个文件", len(needUpdate))

	if len(needUpdate) > 0 {
		current = 0
		for filename, serverInfo := range needUpdate {
			current++
			c.logger.Log("正在更新文件 (%d/%d): %s", current, len(needUpdate), filename)

			if err := common.WriteJSON(c.conn, filename); err != nil {
				return fmt.Errorf("发送文件请求错误: %v", err)
			}

			filePath := filepath.Join(c.config.SyncDir, filename)
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

			c.logger.Log("文件更新完成: %s (%d bytes)", filename, bytesReceived)
		}
	}

	if err := common.WriteJSON(c.conn, "DONE"); err != nil {
		return fmt.Errorf("发送完成信号错误: %v", err)
	}

	c.logger.Log("同步完成！")
	c.disconnect()
	return nil
}

func (c *SyncClient) connect() error {
	if c.syncStatus.Connected {
		return fmt.Errorf("已经连接到服务器")
	}

	if c.config.SyncDir == "" {
		return common.ErrNoSyncDir
	}

	var err error
	c.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", c.config.Host, c.config.Port))
	if err != nil {
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	// 接收服务器版本
	var serverVersion string
	if err := common.ReadJSON(c.conn, &serverVersion); err != nil {
		c.disconnect()
		return fmt.Errorf("接收服务器版本错误: %v", err)
	}

	// 发送客户端版本
	if err := common.WriteJSON(c.conn, serverVersion); err != nil {
		c.disconnect()
		return fmt.Errorf("发送客户端版本错误: %v", err)
	}

	c.version.SetText(serverVersion)
	c.syncStatus.Connected = true
	c.syncStatus.Message = "已连接"
	c.status.SetText("状态: " + c.syncStatus.Message)
	c.logger.Log("已连接到服务器 %s:%d", c.config.Host, c.config.Port)
	c.logger.Log("服务器版本: %s", serverVersion)

	return nil
}

func (c *SyncClient) disconnect() {
	if c.syncStatus.Connected {
		c.syncStatus.Connected = false
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.syncStatus.Message = "未连接"
		c.status.SetText("状态: " + c.syncStatus.Message)
		c.version.SetText("未连接")
		c.logger.Log("已断开连接")
	}
}

func main() {
	// 设置 panic 处理
	defer func() {
		if r := recover(); r != nil {
			// 创建应急日志文件
			logFile, err := os.OpenFile(
				filepath.Join("logs", fmt.Sprintf("client_crash_%s.log",
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

	client := NewSyncClient()

	var mainWindow *walk.MainWindow
	var hostEdit, portEdit *walk.LineEdit
	var dirLabel *walk.Label
	var logBox *walk.TextEdit

	mw := declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "文件同步客户端",
		MinSize:  declarative.Size{Width: 40, Height: 30},
		Size:     declarative.Size{Width: 800, Height: 600},
		Layout:   declarative.VBox{},
		Children: []declarative.Widget{
			declarative.GroupBox{
				Title:  "基本设置",
				Layout: declarative.Grid{Columns: 2},
				Children: []declarative.Widget{
					declarative.Label{Text: "服务器地址:"},
					declarative.LineEdit{
						AssignTo: &hostEdit,
						Text:     client.config.Host,
						OnTextChanged: func() {
							client.config.Host = hostEdit.Text()
						},
					},
					declarative.Label{Text: "端口:"},
					declarative.LineEdit{
						AssignTo: &portEdit,
						Text:     fmt.Sprintf("%d", client.config.Port),
						OnTextChanged: func() {
							fmt.Sscanf(portEdit.Text(), "%d", &client.config.Port)
						},
					},
					declarative.Label{Text: "同步目录:"},
					declarative.Label{
						AssignTo: &dirLabel,
						Text:     client.config.SyncDir,
					},
					declarative.Label{Text: "整合包版本:"},
					declarative.LineEdit{
						AssignTo: &client.version,
						ReadOnly: true,
						Text:     "未连接",
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
								client.config.SyncDir = dlg.FilePath
								dirLabel.SetText(dlg.FilePath)
								client.logger.Log("同步目录已更改为: %s", dlg.FilePath)
							}
						},
					},
					declarative.PushButton{
						Text: "连接服务器",
						OnClicked: func() {
							if !client.syncStatus.Connected {
								if err := client.connect(); err != nil {
									walk.MsgBox(mainWindow, "错误", err.Error(), walk.MsgBoxIconError)
								}
							}
						},
					},
					declarative.PushButton{
						Text: "断开连接",
						OnClicked: func() {
							if client.syncStatus.Connected {
								client.disconnect()
							}
						},
					},
					declarative.PushButton{
						Text: "开始同步",
						OnClicked: func() {
							if !client.syncStatus.Connected {
								walk.MsgBox(mainWindow, "错误", "请先连接到服务器", walk.MsgBoxIconError)
								return
							}

							go func() {
								if err := client.syncWithServer(); err != nil {
									client.disconnect()
									walk.MsgBox(mainWindow, "错误", err.Error(), walk.MsgBoxIconError)
								} else {
									client.logger.Log("同步完成")
								}
							}()
						},
					},
					declarative.HSpacer{},
					declarative.CheckBox{
						Text: "调试模式",
						OnCheckedChanged: func() {
							client.logger.SetDebugMode(!client.logger.DebugMode)
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
		StatusBarItems: []declarative.StatusBarItem{
			{
				AssignTo: &client.status,
				Text:     "状态: 未连接",
			},
		},
	}

	if err := mw.Create(); err != nil {
		walk.MsgBox(nil, "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	// 初始化日志记录器
	logger, err := common.NewGUILogger(logBox, "logs", "client")
	if err != nil {
		walk.MsgBox(mainWindow, "错误", "创建日志记录器失败: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	client.logger = logger
	defer client.logger.Close()

	mainWindow.Run()
}
