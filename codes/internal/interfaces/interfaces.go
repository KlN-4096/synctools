package interfaces

import (
	"io"
	"net"
	"time"
)

// ConfigManager 定义配置管理的核心接口
type ConfigManager interface {
	// LoadConfig 加载配置文件
	LoadConfig(id string) error

	// SaveConfig 保存配置文件
	SaveConfig(config interface{}) error

	// ValidateConfig 验证配置有效性
	ValidateConfig(config interface{}) error

	// GetCurrentConfig 获取当前配置
	GetCurrentConfig() interface{}

	// GetLastModified 获取最后修改时间
	GetLastModified() time.Time

	// ListConfigs 获取所有配置列表
	ListConfigs() ([]*Config, error)

	// SetOnChanged 设置配置变更回调
	SetOnChanged(callback func())
}

// Logger 定义日志接口
type Logger interface {
	Debug(msg string, fields Fields)
	Info(msg string, fields Fields)
	Warn(msg string, fields Fields)
	Error(msg string, fields Fields)
	Fatal(msg string, fields Fields)
	WithFields(fields Fields) Logger
	SetLevel(level LogLevel)
	GetLevel() LogLevel
	SetDebugMode(enabled bool)
	GetDebugMode() bool
}

// NetworkServer 定义网络服务器的核心接口
type NetworkServer interface {
	// Start 启动服务器
	Start() error

	// Stop 停止服务器
	Stop() error

	// HandleClient 处理客户端连接
	HandleClient(conn net.Conn)

	// GetStatus 获取服务器状态
	GetStatus() string

	// IsRunning 检查服务器是否运行中
	IsRunning() bool
}

// NetworkClient 定义网络客户端的核心接口
type NetworkClient interface {
	// Connect 连接到服务器
	Connect(addr, port string) error

	// Disconnect 断开连接
	Disconnect() error

	// IsConnected 检查是否已连接
	IsConnected() bool

	// SendData 发送数据
	SendData(msgType string, data interface{}) error

	// ReceiveData 接收数据
	ReceiveData(v interface{}) error

	// SendFile 发送文件
	SendFile(path string, progress chan<- Progress) error

	// ReceiveFile 接收文件
	ReceiveFile(destDir string, progress chan<- Progress) error

	// SetConnectionLostCallback 设置连接丢失回调
	SetConnectionLostCallback(callback func())
}

// FileTransfer 定义文件传输操作接口
type FileTransfer interface {
	// CopyFile copies file with progress reporting
	CopyFile(dst io.Writer, src io.Reader, size int64, progress chan<- Progress) error

	// ValidateFile validates file integrity
	ValidateFile(path string) error

	// GetFileInfo gets file information
	GetFileInfo(path string) (*FileInfo, error)

	// ListFiles lists files in directory
	ListFiles(path string) ([]FileInfo, error)

	// CompressFiles 压缩文件到ZIP
	CompressFiles(srcPath, zipPath string, opts *CompressOptions) (*CompressProgress, error)

	// DecompressFiles 从ZIP解压文件
	DecompressFiles(zipPath, destPath string, progress chan<- Progress) error
}

// NetworkError 定义网络错误接口
type NetworkError interface {
	error
	IsTimeout() bool
	IsTemporary() bool
}

// Storage 定义存储接口
type Storage interface {
	// Save 保存数据
	Save(key string, data interface{}) error

	// Load 加载数据到指定对象
	Load(key string, data interface{}) error

	// Delete 删除数据
	Delete(key string) error

	// Exists 检查数据是否存在
	Exists(key string) bool

	// List 列出所有键
	List() ([]string, error)

	// BaseDir 获取基础目录
	BaseDir() string
}

// SyncService 基础同步服务接口
type SyncService interface {
	// 基本操作
	Start() error
	Stop() error
	IsRunning() bool
	GetSyncStatus() string

	// 配置操作
	GetCurrentConfig() *Config
	ListConfigs() ([]*Config, error)
	LoadConfig(id string) error
	SaveConfig(config *Config) error
	DeleteConfig(uuid string) error
	ValidateConfig(config *Config) error

	// 回调设置
	SetOnConfigChanged(callback func())
	SetProgressCallback(callback func(progress *Progress))
}

// ServerSyncService 服务端同步服务接口
type ServerSyncService interface {
	SyncService

	// 服务器操作
	StartServer() error
	StopServer() error
	SetServer(server NetworkServer)
	GetNetworkServer() NetworkServer

	// 同步操作
	HandleSyncRequest(request interface{}) error
}

// ClientSyncService 客户端同步服务接口
type ClientSyncService interface {
	SyncService

	// 客户端操作
	Connect(addr, port string) error
	Disconnect() error
	IsConnected() bool
	SetConnectionLostCallback(callback func())

	// 同步操作
	SyncFiles(path string) error
}

// CompressOptions 压缩选项接口
type CompressOptions interface {
	// GetIgnoreList 获取忽略文件列表
	GetIgnoreList() []string

	// SetIgnoreList 设置忽略文件列表
	SetIgnoreList(ignoreList []string)

	// AddIgnorePattern 添加忽略模式
	AddIgnorePattern(pattern string)

	// RemoveIgnorePattern 移除忽略模式
	RemoveIgnorePattern(pattern string)
}
