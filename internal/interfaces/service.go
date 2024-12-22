package interfaces

// SyncService 定义同步服务的核心接口
type SyncService interface {
	// Start 启动同步服务
	Start() error

	// Stop 停止同步服务
	Stop() error

	// SyncFiles 执行文件同步
	SyncFiles(path string) error

	// HandleSyncRequest 处理同步请求
	HandleSyncRequest(request interface{}) error

	// GetSyncStatus 获取同步状态
	GetSyncStatus() string
}
