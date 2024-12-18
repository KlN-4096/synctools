package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// Config 同步配置
type Config struct {
	UUID            string           `json:"uuid"`             // 配置文件唯一标识
	Name            string           `json:"name"`             // 整合包名称
	Version         string           `json:"version"`          // 整合包版本
	Host            string           `json:"host"`             // 服务器主机地址
	Port            int              `json:"port"`             // 服务器端口
	SyncDir         string           `json:"sync_dir"`         // 同步目录
	SyncFolders     []SyncFolder     `json:"sync_folders"`     // 同步文件夹列表
	IgnoreList      []string         `json:"ignore_list"`      // 忽略文件列表
	FolderRedirects []FolderRedirect `json:"folder_redirects"` // 文件夹重定向配置
}

// SyncFolder 同步文件夹配置
type SyncFolder struct {
	Path     string `json:"path"`      // 文件夹路径
	SyncMode string `json:"sync_mode"` // 同步模式: "mirror" 或 "push"
}

// FolderRedirect 文件夹重定向配置
type FolderRedirect struct {
	ServerPath string `json:"server_path"` // 服务器端的文件夹名
	ClientPath string `json:"client_path"` // 客户端的文件夹名
}

// FileInfo 存储文件的基本信息
type FileInfo struct {
	Path         string `json:"path"`          // 文件路径
	Hash         string `json:"hash"`          // 文件的 MD5 哈希值
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

// Logger 定义日志接口
type Logger interface {
	Log(format string, v ...interface{})
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	DebugLog(format string, v ...interface{})
	SetDebugMode(enabled bool)
	GetDebugMode() bool
}

// ConfigRepository 定义配置仓储接口
type ConfigRepository interface {
	Save(config *Config) error
	Load(uuid string) (*Config, error)
	List() ([]*Config, error)
	Delete(uuid string) error
}

// SyncService 定义同步服务接口
type SyncService interface {
	Start() error
	Stop() error
	GetStatus() SyncStatus
	SyncFile(path string) error
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
