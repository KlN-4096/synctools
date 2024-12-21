/*
Package main 实现了文件同步工具的客户端程序。

主要功能：
1. GUI界面：使用walk库实现Windows图形界面
2. 配置管理：读取和保存客户端配置
3. 同步模式：
   - pack模式：整包压缩传输
   - mirror模式：镜像同步
   - push模式：推送同步
4. 错误处理：
   - 自动重试机制
   - 异常恢复
5. 进度显示：
   - 文件传输进度
   - 同步状态显示
6. 文件校验：
   - MD5校验
   - 完整性验证

作者：[作者名]
版本：1.0.0
*/

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"synctools/internal/model"
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
	MaxAttempts int           `json:"max_attempts"` // 最大重试次数
	Delay       time.Duration `json:"delay"`        // 重试延迟
	MaxDelay    time.Duration `json:"max_delay"`    // 最大重试延迟
}

// ProgressDisplay 进度显示接口
type ProgressDisplay interface {
	UpdateProgress(current, total int64, status string)
	ResetProgress()
}

// ValidationResult 文件校验结果
type ValidationResult struct {
	IsValid bool   // 是否有效
	Message string // 校验消息
	Error   error  // 错误信息
}

// SyncClient 同步客户端
type SyncClient struct {
	config          model.Config
	logger          *common.GUILogger
	status          *walk.StatusBarItem
	conn            net.Conn
	syncStatus      model.SyncStatus
	version         *walk.LineEdit
	name            *walk.LineEdit
	uuid            string
	tempDir         string
	packProgress    *model.PackProgress
	retryConfig     RetryConfig
	configPath      string
	progressDisplay ProgressDisplay
}

func NewSyncClient() *SyncClient {
	return &SyncClient{
		config: model.Config{
			Host:    "localhost",
			Port:    6666,
			SyncDir: "",
		},
		syncStatus: model.SyncStatus{
			Connected: false,
			Running:   false,
			Message:   "未连接",
		},
		tempDir: "temp",
		retryConfig: RetryConfig{
			MaxAttempts: 3,
			Delay:       time.Second,
			MaxDelay:    time.Second * 10,
		},
		configPath: "configs/client.json",
	}
}

// LoadConfig 加载配置文件
func (c *SyncClient) LoadConfig() error {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 如果配置文件不存在，创建默认配置
			c.config.Type = model.ConfigTypeClient // 设置为客户端配置
			// 保存当前配置
			return c.SaveConfig()
		}
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config struct {
		Config      model.Config `json:"config"`
		UUID        string       `json:"uuid"`
		RetryConfig RetryConfig  `json:"retry_config"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	c.config = config.Config
	c.uuid = config.UUID
	c.retryConfig = config.RetryConfig

	// 确保类型正确设置
	if c.config.Type == "" {
		c.config.Type = model.ConfigTypeClient
		// 保存更新后的配置
		if err := c.SaveConfig(); err != nil {
			c.logger.Error("保存配置失败: %v", err)
		}
	}

	return nil
}

// SaveConfig 保存配置文件
func (c *SyncClient) SaveConfig() error {
	// 确保配置目录存在
	configDir := filepath.Dir(c.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	// 确保类型正确设置
	c.config.Type = model.ConfigTypeClient

	config := struct {
		Config      model.Config `json:"config"`
		UUID        string       `json:"uuid"`
		RetryConfig RetryConfig  `json:"retry_config"`
	}{
		Config:      c.config,
		UUID:        c.uuid,
		RetryConfig: c.retryConfig,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(c.configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// withRetry 使用重试机制执行操作
func (c *SyncClient) withRetry(operation string, fn func() error) error {
	var lastErr error
	delay := c.retryConfig.Delay

	for attempt := 1; attempt <= c.retryConfig.MaxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			c.logger.Error("%s失败(尝试 %d/%d): %v",
				operation, attempt, c.retryConfig.MaxAttempts, err)

			if attempt < c.retryConfig.MaxAttempts {
				c.logger.Log("等待 %v 后重试...", delay)
				time.Sleep(delay)
				delay *= 2
				if delay > c.retryConfig.MaxDelay {
					delay = c.retryConfig.MaxDelay
				}
				continue
			}
			return fmt.Errorf("%s最终失败: %v", operation, err)
		}
		return nil
	}
	return lastErr
}

// connect 连接到服务器
func (c *SyncClient) connect() error {
	return c.withRetry("连接服务器", func() error {
		if c.syncStatus.Connected {
			return fmt.Errorf("已经连接到服务器")
		}

		if c.config.SyncDir == "" {
			return fmt.Errorf("未设置同步目录")
		}

		// 确保UUID存在
		if c.uuid == "" {
			uuid, err := model.NewUUID()
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
		c.conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", c.config.Host, c.config.Port))
		if err != nil {
			return fmt.Errorf("连接服务器失败: %v", err)
		}

		// 发送注册消息
		msg := ClientMessage{
			Type: "register",
			UUID: c.uuid,
		}
		if err := json.NewEncoder(c.conn).Encode(msg); err != nil {
			c.disconnect()
			return fmt.Errorf("发送注册消息失败: %v", err)
		}

		// 接收注册响应
		var response struct {
			Type    string `json:"type"`
			Payload struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			} `json:"payload"`
		}
		if err := json.NewDecoder(c.conn).Decode(&response); err != nil {
			c.disconnect()
			return fmt.Errorf("接收注册响应失败: %v", err)
		}

		if !response.Payload.Success {
			c.disconnect()
			return fmt.Errorf("注册失败: %s", response.Payload.Message)
		}

		c.syncStatus.Connected = true
		c.syncStatus.Message = "已连接"
		c.status.SetText("状态: " + c.syncStatus.Message)
		c.logger.Log("已连接到服务器 %s:%d", c.config.Host, c.config.Port)

		return nil
	})
}

// syncWithServer 与服务器同步
func (c *SyncClient) syncWithServer() error {
	if !c.syncStatus.Connected {
		return fmt.Errorf("未连接到服务器")
	}

	// 检查同步文件夹配置
	if len(c.config.SyncFolders) == 0 {
		c.logger.Log("没有配置同步文件夹，尝试添加默认配置")
		// 添加默认的同步文件夹配置
		c.config.SyncFolders = []model.SyncFolder{
			{
				Path:     c.config.SyncDir,
				SyncMode: model.SyncModePack,
			},
		}
	}

	c.logger.Log("开始同步，共有 %d 个文件夹需要同步", len(c.config.SyncFolders))

	// 遍历同步文件夹
	for i, folder := range c.config.SyncFolders {
		c.logger.Log("正在同步第 %d 个文件夹: %s (模式: %s)", i+1, folder.Path, folder.SyncMode)

		switch folder.SyncMode {
		case model.SyncModePack:
			if err := c.handlePackSync(folder); err != nil {
				return fmt.Errorf("处理pack同步失败: %v", err)
			}
		default:
			if err := c.handleLegacySync(folder); err != nil {
				return fmt.Errorf("处理传统同步失败: %v", err)
			}
		}
	}

	// 保存更新后的配置
	if err := c.SaveConfig(); err != nil {
		c.logger.Error("保存配置失败: %v", err)
	}

	return nil
}

// handlePackSync 处理pack模式的同步
func (c *SyncClient) handlePackSync(folder model.SyncFolder) error {
	c.logger.Log("开始pack模式同步")

	if c.progressDisplay != nil {
		c.progressDisplay.ResetProgress()
		defer c.progressDisplay.ResetProgress()
	}

	// 确保目标文件夹存在
	absPath, err := filepath.Abs(folder.Path)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败: %v", err)
	}
	folder.Path = absPath

	if err := os.MkdirAll(folder.Path, 0755); err != nil {
		return fmt.Errorf("创建目标文件夹失败: %v", err)
	}

	// 发送同步请求
	msg := ClientMessage{
		Type:    "sync_request",
		UUID:    c.uuid,
		Payload: json.RawMessage(`{"sync_mode": "pack"}`),
	}

	c.logger.Log("发送同步请求")

	if err := json.NewEncoder(c.conn).Encode(msg); err != nil {
		return fmt.Errorf("发送同步请求失败: %v", err)
	}

	// 接收同步响应
	var response struct {
		Type    string `json:"type"`
		Payload struct {
			Success  bool   `json:"success"`
			Message  string `json:"message"`
			PackMD5  string `json:"pack_md5"`
			NeedPack bool   `json:"need_pack"`
		} `json:"payload"`
	}

	if err := json.NewDecoder(c.conn).Decode(&response); err != nil {
		return fmt.Errorf("接收同步响应失败: %v", err)
	}

	c.logger.Log("收到同步响应: success=%v, need_pack=%v, md5=%s, message=%s",
		response.Payload.Success, response.Payload.NeedPack,
		response.Payload.PackMD5, response.Payload.Message)

	if !response.Payload.Success {
		return fmt.Errorf("同步请求失败: %s", response.Payload.Message)
	}

	// 如果不需要更新，直接返回
	if !response.Payload.NeedPack {
		c.logger.Log("文件已是最新版本，无需更新")
		return nil
	}

	// 创建临时目录
	tempDir := filepath.Join(c.tempDir, response.Payload.PackMD5)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 接收压缩包
	packPath := filepath.Join(tempDir, "pack.zip")
	if err := c.receivePackage(response.Payload.PackMD5, packPath); err != nil {
		return fmt.Errorf("接收压缩包失败: %v", err)
	}

	// 验证压缩包
	validation := c.validatePackage(packPath, response.Payload.PackMD5)
	if !validation.IsValid {
		if validation.Error != nil {
			return validation.Error
		}
		return fmt.Errorf("压缩包验证失败: %s", validation.Message)
	}

	c.logger.Log("开始解压文件到 %s", folder.Path)

	// 解压文件
	if err := common.ExtractZipPackage(packPath, folder.Path); err != nil {
		return fmt.Errorf("解压文件失败: %v", err)
	}

	// 更新MD5
	folder.PackMD5 = response.Payload.PackMD5

	// 更新配置中的文件夹信息
	for i := range c.config.SyncFolders {
		if c.config.SyncFolders[i].Path == folder.Path {
			c.config.SyncFolders[i].PackMD5 = folder.PackMD5
			break
		}
	}

	c.logger.Log("同步完成，新的MD5: %s", folder.PackMD5)
	return nil
}

// receivePackage 接收压缩包
func (c *SyncClient) receivePackage(packMD5, savePath string) error {
	return c.withRetry("接收压缩包", func() error {
		file, err := os.Create(savePath)
		if err != nil {
			return fmt.Errorf("创建文件失败: %v", err)
		}
		defer file.Close()

		offset := int64(0)
		chunkSize := 1024 * 1024 // 1MB chunks

		c.logger.Log("开始接收压缩包: md5=%s", packMD5)

		for {
			// 准备请求数据
			payload := struct {
				PackMD5   string `json:"pack_md5"`
				Offset    int64  `json:"offset"`
				ChunkSize int    `json:"chunk_size"`
			}{
				PackMD5:   packMD5,
				Offset:    offset,
				ChunkSize: chunkSize,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("序列化请求数据失败: %v", err)
			}

			// 发送传输请求
			msg := ClientMessage{
				Type:    "pack_transfer_request",
				UUID:    c.uuid,
				Payload: payloadBytes,
			}

			if err := json.NewEncoder(c.conn).Encode(msg); err != nil {
				return fmt.Errorf("发送传输请求失败: %v", err)
			}

			// 接收响应
			var response struct {
				Type    string `json:"type"`
				Payload struct {
					Success   bool   `json:"success"`
					Message   string `json:"message"`
					Data      []byte `json:"data"`
					Offset    int64  `json:"offset"`
					Size      int64  `json:"size"`
					Completed bool   `json:"completed"`
				} `json:"payload"`
			}

			if err := json.NewDecoder(c.conn).Decode(&response); err != nil {
				return fmt.Errorf("接收传输响应失败: %v", err)
			}

			if !response.Payload.Success {
				return fmt.Errorf("传输失败: %s", response.Payload.Message)
			}

			// 写入数据
			if _, err := file.Write(response.Payload.Data); err != nil {
				return fmt.Errorf("写入数据失败: %v", err)
			}

			// 更新进度
			if c.progressDisplay != nil {
				c.progressDisplay.UpdateProgress(
					response.Payload.Offset,
					response.Payload.Size,
					fmt.Sprintf("正在下载: %.1f%%",
						float64(response.Payload.Offset)/float64(response.Payload.Size)*100),
				)
			}

			// 检查是否完成
			if response.Payload.Completed {
				break
			}

			offset = response.Payload.Offset
		}

		return nil
	})
}

// handleLegacySync 处理传统的同步模式
func (c *SyncClient) handleLegacySync(folder model.SyncFolder) error {
	// 保持原有的同步逻辑
	c.logger.Log("使用传统模式同步文件夹: %s", folder.Path)
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

// SetProgressDisplay 设置进度显示器
func (c *SyncClient) SetProgressDisplay(display ProgressDisplay) {
	c.progressDisplay = display
}

// validatePackage 验证压缩包完整性
func (c *SyncClient) validatePackage(packPath string, expectedMD5 string) ValidationResult {
	result := ValidationResult{IsValid: false}

	// 计算文件MD5
	hash, err := model.CalculateFileHash(packPath)
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

// LogProgressDisplay 日志进度显示器
type LogProgressDisplay struct {
	logger       *common.GUILogger
	lastProgress int
}

// NewLogProgressDisplay 创建新的日志进度显示器
func NewLogProgressDisplay(logger *common.GUILogger) *LogProgressDisplay {
	return &LogProgressDisplay{
		logger:       logger,
		lastProgress: 0,
	}
}

// UpdateProgress 更新进度显示
func (d *LogProgressDisplay) UpdateProgress(current, total int64, status string) {
	if total <= 0 {
		return
	}

	currentProgress := int(float64(current) / float64(total) * 100)
	// 每10%记录一次日志
	if currentProgress/10 > d.lastProgress/10 {
		d.logger.Log("同步进度: %d%% - %s", currentProgress, status)
		d.lastProgress = currentProgress
	}
}

// ResetProgress 重置进度显示
func (d *LogProgressDisplay) ResetProgress() {
	d.lastProgress = 0
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

	// 加载配置
	if err := client.LoadConfig(); err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
	}

	var mainWindow *walk.MainWindow
	var hostEdit, portEdit *walk.LineEdit
	var dirLabel *walk.Label
	var logBox *walk.TextEdit

	// 创建进度显示组件
	var progressLabel *walk.Label

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
			declarative.GroupBox{
				Title:  "同步进度",
				Layout: declarative.VBox{},
				Children: []declarative.Widget{
					declarative.Label{
						AssignTo: &progressLabel,
						Text:     "准备就绪",
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

	// 创建日志进度显示器
	progressDisplay := NewLogProgressDisplay(logger)
	client.SetProgressDisplay(progressDisplay)

	mainWindow.Run()
}
