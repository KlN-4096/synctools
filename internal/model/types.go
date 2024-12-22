package model

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

/*
Package model 定义了同步工具的核心领域模型和接口。

主要组件：
- Config: 配置管理
- SyncFolder: 同步文件夹定义
- FileInfo: 文件信息
- ClientState: 客户端状态管理
- Logger: 日志接口
- Server: 服务器接口

同步模式：
- mirror: 镜像模式，完全同步
- push: 推送模式，单向同步
- pack: 压缩包模式，整体同步
*/

// Config 同步配置
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

// 配置类型常量
const (
	ConfigTypeServer = "server" // 服务器配置
	ConfigTypeClient = "client" // 客户端配置
)

// Validate 验证配置是否有效
func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	if c.Version == "" {
		return fmt.Errorf("版本不能为空")
	}
	if c.Type == "" {
		return fmt.Errorf("配置类型不能为空")
	}
	if c.Type != ConfigTypeServer && c.Type != ConfigTypeClient {
		return fmt.Errorf("无效的配置类型: %s", c.Type)
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
		if folder.SyncMode != SyncModeMirror &&
			folder.SyncMode != SyncModePush &&
			folder.SyncMode != SyncModePack {
			return fmt.Errorf("无效的同步模式: %s", folder.SyncMode)
		}
	}

	return nil
}

// Clone 创建配置的深度复制
func (c *Config) Clone() *Config {
	clone := &Config{
		UUID:    c.UUID,
		Type:    c.Type,
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
	SyncMode string `json:"sync_mode"` // 同步模式: "mirror"、"push" 或 "pack"
	PackMD5  string `json:"pack_md5"`  // pack模式下的压缩包MD5
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

// ConfigManager 定义配置管理器接口
type ConfigManager interface {
	// GetCurrentConfig 获取当前配置
	GetCurrentConfig() *Config

	// LoadConfig 加载配置
	LoadConfig(uuid string) error

	// SaveCurrentConfig 保存当前配置
	SaveCurrentConfig() error

	// ListConfigs 获取配置列表
	ListConfigs() ([]*Config, error)

	// Save 保存指定配置
	Save(config *Config) error

	// DeleteConfig 删除配置
	DeleteConfig(uuid string) error

	// ValidateConfig 验证配置
	ValidateConfig(config *Config) error

	// SetOnChanged 设置配置变更回调
	SetOnChanged(callback func())
}

// Server 定义服务器接口
type Server interface {
	// Start 启动服务器
	Start() error

	// Stop 停止服务器
	Stop() error

	// IsRunning 检查服务器是否正在运行
	IsRunning() bool
}

// 同步模式常量
const (
	SyncModeMirror = "mirror" // 镜像同步模式
	SyncModePush   = "push"   // 推送同步模式
	SyncModePack   = "pack"   // 打包同步模式
)

// PackState 压缩包状态
type PackState struct {
	PackPath    string `json:"pack_path"`    // 压缩包路径
	MD5         string `json:"md5"`          // MD5校验值
	CreateTime  int64  `json:"create_time"`  // 创建时间
	LastSynced  int64  `json:"last_synced"`  // 最后同步时间
	Size        int64  `json:"size"`         // 压缩包大小
	IsTemporary bool   `json:"is_temporary"` // 是否为临时文件
}

// ClientState 客户端状态
type ClientState struct {
	UUID         string               `json:"uuid"`           // 客户端UUID
	LastSyncTime int64                `json:"last_sync_time"` // 最后同步时间
	FolderStates map[string]PackState `json:"folder_states"`  // 文件夹状态映射
	IsOnline     bool                 `json:"is_online"`      // 是否在线
	Version      string               `json:"version"`        // 客户端版本
}

// StateManager 状态管理接口
type StateManager interface {
	// GetClientState 获取客户端状态
	GetClientState(uuid string) (*ClientState, error)

	// UpdateClientState 更新客户端状态
	UpdateClientState(state *ClientState) error

	// RemoveClientState 移除客户端状态
	RemoveClientState(uuid string) error

	// ListClientStates 列出所有客户端状态
	ListClientStates() ([]*ClientState, error)
}

// PackProgress 压缩包同步进度
type PackProgress struct {
	FolderPath  string  `json:"folder_path"`  // 文件夹路径
	TotalSize   int64   `json:"total_size"`   // 总大小
	CurrentSize int64   `json:"current_size"` // 当前大小
	Percentage  float64 `json:"percentage"`   // 完成百分比
	Status      string  `json:"status"`       // 状态描述
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 如果配置文件不存在，创建默认配置
			config := &Config{
				Type:    ConfigTypeClient,
				Host:    "localhost",
				Port:    6666,
				SyncDir: "",
				Version: "1.0.0",
				Name:    "默认配置",
			}
			// 保存默认配置
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

	// 确保类型正确设置
	if config.Type == "" {
		config.Type = ConfigTypeClient
	}

	return &config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(path string, config *Config) error {
	// 确保配置目录存在
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
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
