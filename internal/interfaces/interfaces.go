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

// NetworkOperations 定义网络操作接口
type NetworkOperations interface {
	// WriteJSON writes JSON data to connection
	WriteJSON(conn net.Conn, data interface{}) error

	// ReadJSON reads JSON data from connection
	ReadJSON(conn net.Conn, data interface{}) error

	// SendFile sends file through connection
	SendFile(conn net.Conn, path string, progress chan<- Progress) error

	// ReceiveFile receives file through connection
	ReceiveFile(conn net.Conn, path string, progress chan<- Progress) error

	// SendFiles sends multiple files through connection
	SendFiles(conn net.Conn, files []string, progress chan<- Progress) error

	// ReceiveFiles receives multiple files through connection
	ReceiveFiles(conn net.Conn, destDir string, progress chan<- Progress) error
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
}

// SyncService 同步服务接口
type SyncService interface {
	// 基本操作
	Start() error
	Stop() error
	IsRunning() bool
	GetSyncStatus() string

	// 服务器操作
	StartServer() error
	StopServer() error
	SetServer(server NetworkServer)
	GetNetworkServer() NetworkServer

	// 同步操作
	SyncFiles(path string) error
	HandleSyncRequest(request interface{}) error

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
