/*
Package common 提供核心类型定义和基础功能。

文件作用：
- 定义所有系统使用的基础类型
- 提供配置相关的数据结构
- 提供同步状态和文件信息结构
- 定义核心接口
- 实现通用功能

主要类型：
- Config: 核心配置结构
- SyncConfig: 同步配置结构
- FileInfo: 文件信息结构
- Logger: 日志接口定义
- GUILogger: GUI日志记录器实现
- ConfigManager: 配置管理器接口
- SyncService: 同步服务接口
- Server: 服务器接口

主要方法：
- NewGUILogger: 创建GUI日志记录器
- NewUUID: 生成唯一标识符
- LoadConfig: 加载配置
- SaveConfig: 保存配置
- CalculateFileHash: 计算文件哈希值

常量定义：
- ConfigTypeServer: 服务器配置类型
- ConfigTypeClient: 客户端配置类型
- SyncModeMirror: 镜像同步模式
- SyncModePush: 推送同步模式
- SyncModePack: 打包同步模式
*/

package common

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// 配置类型常量
const (
	ConfigTypeServer = "server" // 服务器配置
	ConfigTypeClient = "client" // 客户端配置
)

// 同步模式常量
const (
	SyncModeMirror = "mirror" // 镜像同步模式
	SyncModePush   = "push"   // 推送同步模式
	SyncModePack   = "pack"   // 打包同步模式
)

// Config 核心配置结构
type Config struct {
	UUID            string           `json:"uuid"`             // 配置文件唯一标识
	Type            string           `json:"type"`             // 配置类型: "server" 或 "client"
	Name            string           `json:"name"`             // 整合包名称
	Version         string           `json:"version"`          // 整合包版本
	Host            string           `json:"host"`             // 服务器主机地址
	Port            int              `json:"port"`             // 服务器端口
	SyncDir         string           `json:"sync_dir"`         // 同步目录
	SyncFolders     []SyncFolder     `json:"sync_folders"`     // 同步文件夹列表
	IgnoreList      []string         `json:"ignore_list"`      // 忽略文件列表
	FolderRedirects []FolderRedirect `json:"folder_redirects"` // 文件夹重定向配置
	ServerConfig    *Config          `json:"server_config"`    // 服务器配置
}

// SyncFolder 同步文件夹配置
type SyncFolder struct {
	Path     string `json:"path"`      // 文件夹路径
	SyncMode string `json:"sync_mode"` // 同步模式
	PackMD5  string `json:"pack_md5"`  // pack模式下的压缩包MD5
}

// FolderRedirect 文件夹重定向配置
type FolderRedirect struct {
	ServerPath string `json:"server_path"` // 服务器端的文件夹名
	ClientPath string `json:"client_path"` // 客户端的文件夹名
}

// FileInfo 文件信息结构
type FileInfo struct {
	Path         string `json:"path"`          // 文件路径
	Hash         string `json:"hash"`          // 文件哈希值
	Size         int64  `json:"size"`          // 文件大小
	ModTime      int64  `json:"mod_time"`      // 修改时间
	IsDirectory  bool   `json:"is_directory"`  // 是否是目录
	RelativePath string `json:"relative_path"` // 相对路径
}

// SyncStatus 同步状态
type SyncStatus struct {
	Connected bool   // 是否已连接
	Running   bool   // 是否正在运行
	Message   string // 状态消息
}

// SyncInfo 同步信息
type SyncInfo struct {
	Files            map[string]FileInfo `json:"files"`              // 文件信息映射
	DeleteExtraFiles bool                `json:"delete_extra_files"` // 是否删除多余文件
	SyncMode         string              `json:"sync_mode"`          // 同步模式
}

// PackProgress 压缩包同步进度
type PackProgress struct {
	FolderPath  string  `json:"folder_path"`  // 文件夹路径
	TotalSize   int64   `json:"total_size"`   // 总大小
	CurrentSize int64   `json:"current_size"` // 当前大小
	Percentage  float64 `json:"percentage"`   // 完成百分比
	Status      string  `json:"status"`       // 状态描述
}

// GUILogger GUI日志记录器
type GUILogger struct {
	debugEnabled bool
	logFunc      func(string)
}

// ConfigManager 配置管理器接口
type ConfigManager interface {
	GetCurrentConfig() *Config
	LoadConfig(uuid string) error
	SaveCurrentConfig() error
	ListConfigs() ([]*Config, error)
	Save(config *Config) error
	DeleteConfig(uuid string) error
	ValidateConfig(config *Config) error
	SetOnChanged(callback func())
}

// SyncService 同步服务接口
type SyncService interface {
	Start() error
	Stop() error
	GetStatus() SyncStatus
	SyncFile(path string) error
}

// Server 服务器接口
type Server interface {
	Start() error
	Stop() error
	IsRunning() bool
}

// NewGUILogger 创建新的GUI日志记录器
func NewGUILogger(logFunc func(string)) *GUILogger {
	return &GUILogger{
		logFunc: logFunc,
	}
}

// Log 记录普通日志
func (l *GUILogger) Log(format string, v ...interface{}) {
	if l.logFunc != nil {
		l.logFunc(fmt.Sprintf(format, v...))
	}
}

// Error 记录错误日志
func (l *GUILogger) Error(msg string, args ...interface{}) {
	if l.logFunc != nil {
		l.logFunc(fmt.Sprintf("[ERROR] "+msg, args...))
	}
}

// DebugLog 记录调试日志
func (l *GUILogger) DebugLog(format string, v ...interface{}) {
	if l.debugEnabled && l.logFunc != nil {
		l.logFunc(fmt.Sprintf("[DEBUG] "+format, v...))
	}
}

// Info 记录信息日志
func (l *GUILogger) Info(msg string, keyvals ...interface{}) {
	if l.logFunc != nil {
		l.logFunc(fmt.Sprintf("[INFO] %s", msg))
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				l.logFunc(fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1]))
			}
		}
	}
}

// SetDebugMode 设置调试模式
func (l *GUILogger) SetDebugMode(enabled bool) {
	l.debugEnabled = enabled
}

// GetDebugMode 获取调试模式状态
func (l *GUILogger) GetDebugMode() bool {
	return l.debugEnabled
}

// Close 关闭日志记录器
func (l *GUILogger) Close() error {
	l.logFunc = nil
	return nil
}

// NewUUID 生成新的UUID
func NewUUID() (string, error) {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return "", fmt.Errorf("生成UUID失败: %v", err)
	}
	return hex.EncodeToString(uuid), nil
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			config := &Config{
				Type:    ConfigTypeClient,
				Host:    "localhost",
				Port:    6666,
				Version: "1.0.0",
				Name:    "默认配置",
			}
			if err := SaveConfig(path, config); err != nil {
				return nil, fmt.Errorf("保存默认配置失败: %v", err)
			}
			return config, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	if config.Type == "" {
		config.Type = ConfigTypeClient
	}

	return &config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(path string, config *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// Equal 比较两个文件信息是否相同
func (f *FileInfo) Equal(other *FileInfo) bool {
	if f == nil || other == nil {
		return false
	}
	return f.Hash == other.Hash &&
		f.Size == other.Size &&
		f.ModTime == other.ModTime &&
		f.IsDirectory == other.IsDirectory
}

// 错误定义
var (
	ErrNotConnected = fmt.Errorf("未连接到服务器")
	ErrNoSyncDir    = fmt.Errorf("未设置同步目录")
)

// PackState 压缩包状态
type PackState struct {
	MD5      string `json:"md5"`       // 压缩包MD5
	PackPath string `json:"pack_path"` // 压缩包路径
	Size     int64  `json:"size"`      // 压缩包大小
}

// ClientState 客户端状态
type ClientState struct {
	UUID         string               `json:"uuid"`           // 客户端UUID
	LastSyncTime int64                `json:"last_sync_time"` // 最后同步时间
	FolderStates map[string]PackState `json:"folder_states"`  // 文件夹状态
	IsOnline     bool                 `json:"is_online"`      // 是否在线
	Version      string               `json:"version"`        // 客户端版本
}
