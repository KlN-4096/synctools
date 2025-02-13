package interfaces

import (
	"encoding/json"
	"time"
)

// LogLevel 定义日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// Fields 定义日志字段类型
type Fields map[string]interface{}

// ConfigType 配置类型
type ConfigType string

const (
	// ConfigTypeServer 服务器配置
	ConfigTypeServer ConfigType = "server"
	// ConfigTypeClient 客户端配置
	ConfigTypeClient ConfigType = "client"
)

// SyncDirection 同步方向
type SyncDirection string

const (
	DirectionPush SyncDirection = "push" // 推送到服务器
	DirectionPull SyncDirection = "pull" // 从服务器拉取
)

// SyncMode 同步模式
type SyncMode string

const (
	MirrorSync SyncMode = "mirror" // 镜像同步
	PushSync   SyncMode = "push"   // 推送同步
	PackSync   SyncMode = "pack"   // 打包同步
	ManualSync SyncMode = "manual" // 手动同步
)

// FileAction 文件操作类型
type FileAction string

const (
	FileActionAdd    FileAction = "add"    // 添加文件
	FileActionDelete FileAction = "delete" // 删除文件
	FileActionUpdate FileAction = "update" // 更新文件
)

// Config represents configuration information
type Config struct {
	UUID            string           `json:"uuid"`             // 配置文件唯一标识
	Type            ConfigType       `json:"type"`             // 配置类型
	Name            string           `json:"name"`             // 整合包名称
	Version         string           `json:"version"`          // 整合包版本
	Host            string           `json:"host"`             // 服务器主机地址
	Port            int              `json:"port"`             // 服务器端口
	ConnTimeout     int              `json:"conn_timeout"`     // 连接超时时间(秒)
	SyncDir         string           `json:"sync_dir"`         // 同步目录
	SyncFolders     []SyncFolder     `json:"sync_folders"`     // 同步文件夹列表
	IgnoreList      []string         `json:"ignore_list"`      // 忽略文件列表
	FolderRedirects []FolderRedirect `json:"folder_redirects"` // 文件夹重定向配置
	ServerConfig    *Config          `json:"server_config"`    // 服务器配置
	LastModified    time.Time        `json:"last_modified"`    // 最后修改时间
	CreateTime      time.Time        `json:"create_time"`      // 创建时间
}

// SyncFolder represents synchronization folder configuration
type SyncFolder struct {
	Path      string   `json:"path"`       // 文件夹路径
	SyncMode  SyncMode `json:"sync_mode"`  // 同步模式
	PackMD5   string   `json:"pack_md5"`   // pack模式下的压缩包MD5
	IsEnabled bool     `json:"is_enabled"` // 是否启用
}

// FolderRedirect represents folder redirection configuration
type FolderRedirect struct {
	ServerPath string `json:"server_path"` // 服务器端的文件夹名
	ClientPath string `json:"client_path"` // 客户端的文件夹名
}

// FileInfo represents file information
type FileInfo struct {
	Path         string    `json:"path"`          // 文件路径
	Hash         string    `json:"hash"`          // 文件哈希值
	Size         int64     `json:"size"`          // 文件大小
	ModTime      int64     `json:"mod_time"`      // 修改时间
	IsDirectory  bool      `json:"is_directory"`  // 是否是目录
	RelativePath string    `json:"relative_path"` // 相对路径
	CreateTime   time.Time `json:"create_time"`   // 创建时间
	LastAccess   time.Time `json:"last_access"`   // 最后访问时间
}

// SyncStatus represents synchronization status
type SyncStatus struct {
	Connected     bool      `json:"connected"`      // 是否已连接
	Running       bool      `json:"running"`        // 是否正在运行
	Message       string    `json:"message"`        // 状态消息
	LastSyncTime  time.Time `json:"last_sync_time"` // 最后同步时间
	NextSyncTime  time.Time `json:"next_sync_time"` // 下次同步时间
	SyncInterval  int       `json:"sync_interval"`  // 同步间隔(秒)
	ErrorCount    int       `json:"error_count"`    // 错误计数
	RetryCount    int       `json:"retry_count"`    // 重试计数
	IsAutoSync    bool      `json:"is_auto_sync"`   // 是否自动同步
	CurrentAction string    `json:"current_action"` // 当前操作
}

// SyncInfo represents synchronization information
type SyncInfo struct {
	Files            map[string]FileInfo `json:"files"`              // 文件信息映射
	DeleteExtraFiles bool                `json:"delete_extra_files"` // 是否删除多余文件
	SyncMode         SyncMode            `json:"sync_mode"`          // 同步模式
	StartTime        time.Time           `json:"start_time"`         // 开始时间
	EndTime          time.Time           `json:"end_time"`           // 结束时间
	TotalFiles       int                 `json:"total_files"`        // 总文件数
	ProcessedFiles   int                 `json:"processed_files"`    // 已处理文件数
	FailedFiles      int                 `json:"failed_files"`       // 失败文件数
	TotalSize        int64               `json:"total_size"`         // 总大小
	ProcessedSize    int64               `json:"processed_size"`     // 已处理大小
}

// Progress represents progress information
type Progress struct {
	Total     int64   `json:"total"`     // 总大小
	Current   int64   `json:"current"`   // 当前进度
	Speed     float64 `json:"speed"`     // 速度(bytes/s)
	Remaining int64   `json:"remaining"` // 剩余时间(秒)
	FileName  string  `json:"file_name"` // 当前文件名
	Status    string  `json:"status"`    // 状态描述
}

// CompressProgress represents compress progress information
type CompressProgress struct {
	CurrentFile   string    `json:"current_file"`   // 当前处理的文件
	TotalFiles    int       `json:"total_files"`    // 总文件数
	ProcessedNum  int       `json:"processed_num"`  // 已处理文件数
	TotalSize     int64     `json:"total_size"`     // 总大小
	ProcessedSize int64     `json:"processed_size"` // 已处理大小
	StartTime     time.Time `json:"start_time"`     // 开始时间
	Speed         float64   `json:"speed"`          // 处理速度 (bytes/s)
}

// Message represents base message structure
type Message struct {
	Type    string          `json:"type"`    // 消息类型
	UUID    string          `json:"uuid"`    // 客户端UUID
	Payload json.RawMessage `json:"payload"` // 消息内容
}

// SyncRequest represents synchronization request
type SyncRequest struct {
	Mode      SyncMode      `json:"mode"`            // 同步模式
	Direction SyncDirection `json:"direction"`       // 同步方向
	Path      string        `json:"path"`            // 文件路径
	Files     []string      `json:"files,omitempty"` // 文件列表
	Storage   Storage       `json:"-"`               // 目标存储接口
}

// SyncResponse represents synchronization response
type SyncResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
	Error   string `json:"error"`   // 错误信息
}

// FileTransferRequest represents file transfer request
type FileTransferRequest struct {
	FilePath  string `json:"file_path"`  // 文件路径
	ChunkSize int    `json:"chunk_size"` // 分块大小
	Offset    int64  `json:"offset"`     // 传输偏移量
}

// FileTransferResponse represents file transfer response
type FileTransferResponse struct {
	Success   bool   `json:"success"`   // 是否成功
	Message   string `json:"message"`   // 消息
	Data      []byte `json:"data"`      // 数据块
	Offset    int64  `json:"offset"`    // 当前偏移量
	Size      int64  `json:"size"`      // 总大小
	Completed bool   `json:"completed"` // 是否传输完成
}

// ErrorResponse represents error response
type ErrorResponse struct {
	Code    string `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误消息
}

// FileMessageInfo represents file message information
type FileMessageInfo struct {
	Name string `json:"name"` // 文件名
	Size int64  `json:"size"` // 文件大小
	MD5  string `json:"md5"`  // 文件MD5值
}

// FileDataChunk represents file data chunk
type FileDataChunk struct {
	Data []byte `json:"data"` // 文件数据
}

// CompressOptionsImpl represents compress options implementation
type CompressOptionsImpl struct {
	IgnoreList []string `json:"ignore_list"` // 忽略文件列表
}
