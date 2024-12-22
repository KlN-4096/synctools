/*
Package main 实现了文件同步工具的客户端程序。

文件作用：
- 实现客户端的主程序入口
- 初始化GUI界面和各种组件
- 管理客户端配置和同步状态
- 处理与服务器的通信
- 提供文件同步功能
- 实现用户界面交互

主要类型：
- ClientMessage: 客户端消息结构
- RetryConfig: 重试配置结构
- ValidationResult: 文件校验结果结构
- SyncClient: 同步客户端结构

主要方法：
- NewSyncClient: 创建新的同步客户端
- connect: 连接到同步服务器
- syncWithServer: 执行文件同步操作
- handlePackSync: 处理打包同步模式
- handleMirrorSync: 处理镜像同步模式
- handlePushSync: 处理推送同步模式
- receiveFile: 接收单个文件
- validatePackage: 验证压缩包完整性
- SaveConfig: 保存客户端配置
- disconnect: 断开服务器连接
*/

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/pkg/common"
)

// ClientMessage 客户端消息
type ClientMessage struct {
	Type    string          `json:"type"`    // 消息类型
	UUID    string          `json:"uuid"`    // 客户端UUID
	Payload json.RawMessage `json:"payload"` // 消息内容
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries    int           `json:"max_retries"`    // 最大重试次数
	RetryInterval time.Duration `json:"retry_interval"` // 重试间隔
}

// ValidationResult 文件校验结果
type ValidationResult struct {
	IsValid bool   // 是否有效
	Message string // 校验消息
	Error   error  // 错误信息
}

// SyncClient 同步客户端
type SyncClient struct {
	config      *common.Config
	logger      *common.GUILogger
	status      *walk.StatusBarItem
	conn        net.Conn
	syncStatus  common.SyncStatus
	version     *walk.LineEdit
	name        *walk.LineEdit
	uuid        string
	tempDir     string
	retryConfig RetryConfig
	configPath  string
}

// NewSyncClient 创建同步客户端
func NewSyncClient(configPath string) (*SyncClient, error) {
	// 加载配置
	config, err := common.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %v", err)
	}

	// 创建日志记录器
	logger := common.NewGUILogger(func(msg string) {
		fmt.Println(msg) // 临时使用控制台输出
	})

	// 创建客户端
	client := &SyncClient{
		config:     config,
		configPath: configPath,
		uuid:       uuid.New().String(),
		logger:     logger,
		retryConfig: RetryConfig{
			MaxRetries:    3,
			RetryInterval: 5 * time.Second,
		},
	}

	return client, nil
}

// SaveConfig 保存配置
func (c *SyncClient) SaveConfig() error {
	if err := common.SaveConfig(c.configPath, c.config); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}
	return nil
}

// retry 执行带重试的操作
func (c *SyncClient) retry(operation func() error) error {
	var err error
	for i := 0; i < c.retryConfig.MaxRetries; i++ {
		if err = operation(); err == nil {
			return nil
		}
		c.logger.Log("操作失败，将在 %v 后重试 (%d/%d): %v",
			c.retryConfig.RetryInterval, i+1, c.retryConfig.MaxRetries, err)
		time.Sleep(c.retryConfig.RetryInterval)
	}
	return fmt.Errorf("重试 %d 次后仍然失败: %v", c.retryConfig.MaxRetries, err)
}

// connect 连接到服务器
func (c *SyncClient) connect() error {
	if c.syncStatus.Connected {
		return fmt.Errorf("已经连接到服务器")
	}

	if c.config.SyncDir == "" {
		return fmt.Errorf("未设置同步目录")
	}

	// 确保UUID存在
	if c.uuid == "" {
		uuid, err := common.NewUUID()
		if err != nil {
			return fmt.Errorf("生成UUID失败: %v", err)
		}
		c.uuid = uuid
		c.logger.Log("生成新的客户端UUID: %s", c.uuid)
		if err := c.SaveConfig(); err != nil {
			c.logger.Error("保存配置失败: %v", err)
		}
	}

	// 连接服务器
	var err error
	c.logger.DebugLog("尝试连接服务器: %s:%d", c.config.Host, c.config.Port)
	c.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", c.config.Host, c.config.Port))
	if err != nil {
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	// 发送注册消息
	payload := struct {
		ClientVersion string `json:"client_version"`
		SyncDir       string `json:"sync_dir"`
	}{
		ClientVersion: "1.0.0",
		SyncDir:       c.config.SyncDir,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		c.disconnect()
		return fmt.Errorf("序列化注册消息失败: %v", err)
	}

	msg := ClientMessage{
		Type:    "register",
		UUID:    c.uuid,
		Payload: payloadBytes,
	}

	c.logger.DebugLog("发送注册消息: %+v", msg)
	if err := json.NewEncoder(c.conn).Encode(msg); err != nil {
		c.disconnect()
		return fmt.Errorf("发送注册消息失败: %v", err)
	}

	// 接收注册响应
	var response struct {
		Type    string `json:"type"`
		Payload struct {
			Success bool           `json:"success"`
			Message string         `json:"message"`
			Config  *common.Config `json:"config"`
		} `json:"payload"`
	}

	c.logger.DebugLog("等待接收注册响应...")
	if err := json.NewDecoder(c.conn).Decode(&response); err != nil {
		c.disconnect()
		return fmt.Errorf("接收注册响应失败: %v", err)
	}
	c.logger.DebugLog("收到注册响应: %+v", response)

	if !response.Payload.Success {
		c.disconnect()
		return fmt.Errorf("注册失败: %s", response.Payload.Message)
	}

	// 更新服务器配置信息
	if response.Payload.Config != nil {
		c.logger.DebugLog("收到服务器配置: %+v", response.Payload.Config)
		// 保存服务器配置到本地
		c.config.ServerConfig = response.Payload.Config
		if err := c.SaveConfig(); err != nil {
			c.logger.Error("保存服务器配置失败: %v", err)
		}

		// 更新UI显示
		c.version.SetText(response.Payload.Config.Version)
		c.name.SetText(response.Payload.Config.Name)
		c.logger.Log("已获取服务器配置: 名称=%s, 版本=%s",
			response.Payload.Config.Name,
			response.Payload.Config.Version)
	} else {
		c.logger.DebugLog("服务器未返回配置信息")
		c.disconnect()
		return fmt.Errorf("服务器未返回配置信息")
	}

	c.syncStatus.Connected = true
	c.syncStatus.Message = "已连接"
	c.status.SetText("状态: " + c.syncStatus.Message)
	c.logger.Log("已连接到服务器 %s:%d", c.config.Host, c.config.Port)

	return nil
}

// syncWithServer 与服务器同步
func (c *SyncClient) syncWithServer() error {
	if !c.syncStatus.Connected {
		return fmt.Errorf("未连接到服务器")
	}

	// 发送同步请求
	payload := struct {
		TargetDir string `json:"target_dir"`
	}{
		TargetDir: c.config.SyncDir,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化同步请求失败: %v", err)
	}

	c.logger.DebugLog("准备发送同步请求: %s", string(payloadBytes))

	msg := ClientMessage{
		Type:    "sync_request",
		UUID:    c.uuid,
		Payload: payloadBytes,
	}

	if err := json.NewEncoder(c.conn).Encode(msg); err != nil {
		c.logger.DebugLog("发送同步请求失败: %v", err)
		return fmt.Errorf("发送同步请求失败: %v", err)
	}

	c.logger.DebugLog("同步请求已发送,等待服务器响应...")

	// 接收服务器的同步配置
	var syncConfig struct {
		Type    string `json:"type"`
		Payload struct {
			SyncMode    string `json:"sync_mode"`
			ServerFiles []struct {
				Path string `json:"path"`
				Hash string `json:"hash"`
				Size int64  `json:"size"`
			} `json:"server_files"`
			PackMD5 string `json:"pack_md5,omitempty"`
		} `json:"payload"`
	}

	if err := json.NewDecoder(c.conn).Decode(&syncConfig); err != nil {
		c.logger.DebugLog("接收同步配置失败: %v", err)
		return fmt.Errorf("接收同步配置失败: %v", err)
	}

	c.logger.DebugLog("收到服务器响应: type=%s", syncConfig.Type)
	c.logger.DebugLog("同步配置详情: mode=%s, files=%d, packMD5=%s",
		syncConfig.Payload.SyncMode,
		len(syncConfig.Payload.ServerFiles),
		syncConfig.Payload.PackMD5)

	if syncConfig.Payload.SyncMode == "" {
		c.logger.DebugLog("服务器未指定同步模式")
		return fmt.Errorf("服务器未指定同步模式")
	}

	c.logger.Log("收到服务器同步配置: 模式=%s", syncConfig.Payload.SyncMode)

	// 根据服务器指定的模式处理同步
	switch syncConfig.Payload.SyncMode {
	case common.SyncModePack:
		c.logger.DebugLog("开始处理pack模式同步...")
		return c.handlePackSync(syncConfig.Payload.PackMD5)
	case common.SyncModeMirror:
		c.logger.DebugLog("开始处理mirror模式同步...")
		return c.handleMirrorSync(syncConfig.Payload.ServerFiles)
	case common.SyncModePush:
		c.logger.DebugLog("开始处理push模式同步...")
		return c.handlePushSync(syncConfig.Payload.ServerFiles)
	default:
		c.logger.DebugLog("不支持的同步模式: %s", syncConfig.Payload.SyncMode)
		return fmt.Errorf("不支持的同步模式: %s", syncConfig.Payload.SyncMode)
	}
}

// handlePackSync 处理pack模式的同步
func (c *SyncClient) handlePackSync(packMD5 string) error {
	c.logger.Log("开始pack模式同步")
	c.logger.DebugLog("pack模式 - MD5=%s", packMD5)

	// 创建临时目录
	tempDir := filepath.Join(c.tempDir, packMD5)
	c.logger.DebugLog("创建临时目录: %s", tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.logger.DebugLog("创建临时目录失败: %v", err)
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 接收压缩包
	packPath := filepath.Join(tempDir, "pack.zip")
	c.logger.DebugLog("准备接收压缩包: %s", packPath)
	if err := c.receiveFile(packPath); err != nil {
		c.logger.DebugLog("接收压缩包失败: %v", err)
		return fmt.Errorf("接收压缩包失败: %v", err)
	}

	// 验证压缩包
	c.logger.DebugLog("开始验证压缩包...")
	validation := c.validatePackage(packPath, packMD5)
	if !validation.IsValid {
		if validation.Error != nil {
			c.logger.DebugLog("压缩包验证出错: %v", validation.Error)
			return validation.Error
		}
		c.logger.DebugLog("压缩包验证失败: %s", validation.Message)
		return fmt.Errorf("压缩包验证失败: %s", validation.Message)
	}

	c.logger.Log("开始解压文件到 %s", c.config.SyncDir)
	c.logger.DebugLog("压缩包验证成功,开始解压...")

	// 解压文件
	progress, err := common.DecompressFiles(packPath, c.config.SyncDir)
	if err != nil {
		c.logger.DebugLog("解压文件失败: %v", err)
		return fmt.Errorf("解压文件失败: %v", err)
	}

	c.logger.DebugLog("文件解压完成，共处理 %d 个文件，总大小 %d 字节", progress.ProcessedNum, progress.ProcessedSize)
	c.logger.Log("同步完成")
	return nil
}

// handleMirrorSync 处理镜像同步
func (c *SyncClient) handleMirrorSync(serverFiles []struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}) error {
	c.logger.Log("开始镜像同步")

	// 创建服务器文件映射
	serverFileMap := make(map[string]string)
	for _, file := range serverFiles {
		serverFileMap[file.Path] = file.Hash
	}

	// 获取本地文件列表
	localFiles := make(map[string]string)
	err := filepath.Walk(c.config.SyncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(c.config.SyncDir, path)
			if err != nil {
				return err
			}
			hash, err := common.CalculateFileHash(path)
			if err != nil {
				return err
			}
			localFiles[relPath] = hash
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("获取本地文件列表失败: %v", err)
	}

	// 删除本地多余的文件
	for localPath := range localFiles {
		if _, exists := serverFileMap[localPath]; !exists {
			fullPath := filepath.Join(c.config.SyncDir, localPath)
			if err := os.Remove(fullPath); err != nil {
				return fmt.Errorf("删除文件失败 [%s]: %v", localPath, err)
			}
			c.logger.Log("已删除文件: %s", localPath)
		}
	}

	// 接收新文件或更新不一致的文件
	for _, serverFile := range serverFiles {
		localHash, exists := localFiles[serverFile.Path]
		if !exists || localHash != serverFile.Hash {
			localPath := filepath.Join(c.config.SyncDir, serverFile.Path)
			if err := c.receiveFile(localPath); err != nil {
				return fmt.Errorf("接收文件失败 [%s]: %v", serverFile.Path, err)
			}
			c.logger.Log("已更新文件: %s", serverFile.Path)
		}
	}

	c.logger.Log("镜像同步完成")
	return nil
}

// handlePushSync 处理推送同步
func (c *SyncClient) handlePushSync(serverFiles []struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}) error {
	c.logger.Log("开始推送同步")

	// 接收新文件或更新的文件
	for _, serverFile := range serverFiles {
		localPath := filepath.Join(c.config.SyncDir, serverFile.Path)
		if err := c.receiveFile(localPath); err != nil {
			return fmt.Errorf("接收文件失败 [%s]: %v", serverFile.Path, err)
		}
		c.logger.Log("已更新文件: %s", serverFile.Path)
	}

	c.logger.Log("推送同步完成")
	return nil
}

// receiveFile 接收单个文件
func (c *SyncClient) receiveFile(localPath string) error {
	c.logger.DebugLog("开始接收文件: %s", localPath)

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		c.logger.DebugLog("创建目标目录失败: %v", err)
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 创建目标文件
	file, err := os.Create(localPath)
	if err != nil {
		c.logger.DebugLog("创建目标文件失败: %v", err)
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer file.Close()

	// 接收文件数据
	var totalSize int64
	var receivedSize int64

	for {
		var fileData struct {
			Type    string `json:"type"`
			Payload struct {
				Path      string `json:"path"`
				Data      []byte `json:"data"`
				Offset    int64  `json:"offset"`
				Size      int64  `json:"size"`
				Completed bool   `json:"completed"`
			} `json:"payload"`
		}

		c.logger.DebugLog("等待接收文件数据...")
		if err := json.NewDecoder(c.conn).Decode(&fileData); err != nil {
			c.logger.DebugLog("接收文件数据失败: %v", err)
			return fmt.Errorf("接收文件数据失败: %v", err)
		}

		totalSize = fileData.Payload.Size
		receivedSize = fileData.Payload.Offset

		c.logger.DebugLog("收到数据块: path=%s, size=%d, offset=%d, completed=%v",
			fileData.Payload.Path,
			len(fileData.Payload.Data),
			fileData.Payload.Offset,
			fileData.Payload.Completed)

		// 写入数据
		if len(fileData.Payload.Data) > 0 {
			if _, err := file.Write(fileData.Payload.Data); err != nil {
				c.logger.DebugLog("写入文件失败: %v", err)
				return fmt.Errorf("写入文件失败: %v", err)
			}
		}

		// 发送确认消息
		confirmMsg := ClientMessage{
			Type: "file_received",
			UUID: c.uuid,
			Payload: json.RawMessage(fmt.Sprintf(`{
				"path": "%s",
				"offset": %d
			}`, fileData.Payload.Path, receivedSize)),
		}

		c.logger.DebugLog("发送确认消息: offset=%d", receivedSize)
		if err := json.NewEncoder(c.conn).Encode(confirmMsg); err != nil {
			c.logger.DebugLog("发送确认消息失败: %v", err)
			return fmt.Errorf("发送确认消息失败: %v", err)
		}

		c.logger.Log("文件传输进度: %.1f%%", float64(receivedSize)/float64(totalSize)*100)

		// 检查是否完成
		if fileData.Payload.Completed {
			c.logger.DebugLog("文件传输完成")
			break
		}
	}

	return nil
}

// disconnect 断开连接
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
		c.name.SetText("未连接")
		c.logger.Log("已断开连接")
	}
}

// validatePackage 验证压缩包完整性
func (c *SyncClient) validatePackage(packPath string, expectedMD5 string) ValidationResult {
	result := ValidationResult{IsValid: false}

	// 计算文件MD5
	hash, err := common.CalculateFileHash(packPath)
	if err != nil {
		result.Error = fmt.Errorf("计算文件hash失败: %v", err)
		return result
	}

	// 比较MD5
	if hash != expectedMD5 {
		result.Message = fmt.Sprintf("MD5校验失败: 期望=%s, 实际=%s", expectedMD5, hash)
		return result
	}

	result.IsValid = true
	return result
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

	client, err := NewSyncClient("configs/client.json")
	if err != nil {
		fmt.Printf("创建同步客户端失败: %v\n", err)
		return
	}

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
						OnEditingFinished: func() {
							client.config.Host = hostEdit.Text()
						},
					},
					declarative.Label{Text: "端口:"},
					declarative.LineEdit{
						AssignTo: &portEdit,
						Text:     fmt.Sprintf("%d", client.config.Port),
						OnEditingFinished: func() {
							var port int
							if _, err := fmt.Sscanf(portEdit.Text(), "%d", &port); err == nil {
								client.config.Port = port
							}
						},
					},
					declarative.Label{Text: "同步目录:"},
					declarative.Label{
						AssignTo: &dirLabel,
						Text:     client.config.SyncDir,
					},
					declarative.Label{Text: "整合包名称:"},
					declarative.LineEdit{
						AssignTo: &client.name,
						ReadOnly: true,
						Text:     "未连接",
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
							client.logger.SetDebugMode(!client.logger.GetDebugMode())
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

	// 创建日志记录器
	logger := common.NewGUILogger(func(msg string) {
		logBox.AppendText(msg + "\n")
	})
	client.logger = logger
	defer client.logger.Close()

	// 检查调试模式
	if client.logger.GetDebugMode() {
		client.logger.DebugLog("调试模式已启用")
	}

	// 在窗口关闭前保存配置
	mainWindow.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if err := client.SaveConfig(); err != nil {
			walk.MsgBox(mainWindow, "错误",
				fmt.Sprintf("保存配置失败: %v", err),
				walk.MsgBoxIconError)
		}
	})

	mainWindow.Run()
}
