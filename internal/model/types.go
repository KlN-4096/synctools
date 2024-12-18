package model

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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

// Validate 验证配置是否有效
func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	if c.Version == "" {
		return fmt.Errorf("版本不能为空")
	}
	if c.Host == "" {
		return fmt.Errorf("主机地址不能为空")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("端口号无效: %d", c.Port)
	}
	if c.SyncDir == "" {
		return fmt.Errorf("同步目录不能为空")
	}

	// 验证同步文件夹配置
	for _, folder := range c.SyncFolders {
		if folder.Path == "" {
			return fmt.Errorf("同步文件夹路径不能为空")
		}
		if folder.SyncMode != "mirror" && folder.SyncMode != "push" {
			return fmt.Errorf("无效的同步模式: %s", folder.SyncMode)
		}
	}

	return nil
}

// Clone 创建配置的深度复制
func (c *Config) Clone() *Config {
	clone := &Config{
		UUID:    c.UUID,
		Name:    c.Name,
		Version: c.Version,
		Host:    c.Host,
		Port:    c.Port,
		SyncDir: c.SyncDir,
	}

	// 复制同步文件夹列表
	if len(c.SyncFolders) > 0 {
		clone.SyncFolders = make([]SyncFolder, len(c.SyncFolders))
		copy(clone.SyncFolders, c.SyncFolders)
	}

	// 复制忽略列表
	if len(c.IgnoreList) > 0 {
		clone.IgnoreList = make([]string, len(c.IgnoreList))
		copy(clone.IgnoreList, c.IgnoreList)
	}

	// 复制文件夹重定向配置
	if len(c.FolderRedirects) > 0 {
		clone.FolderRedirects = make([]FolderRedirect, len(c.FolderRedirects))
		copy(clone.FolderRedirects, c.FolderRedirects)
	}

	return clone
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

// CalculateFileHash 计算文件的 MD5 哈希值
func CalculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
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
