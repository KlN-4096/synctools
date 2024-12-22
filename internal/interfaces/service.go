package interfaces

// SyncService 同步服务接口
type SyncService interface {
	// 基本操作
	Start() error
	Stop() error
	IsRunning() bool

	// 配置管理
	GetCurrentConfig() *Config
	ListConfigs() ([]*Config, error)
	LoadConfig(id string) error
	SaveConfig(config *Config) error
	DeleteConfig(uuid string) error
	ValidateConfig(config *Config) error

	// 回调设置
	SetOnConfigChanged(callback func())
	SetProgressCallback(callback func(progress *Progress))

	// 服务器管理
	SetServer(server NetworkServer)

	// 同步操作
	SyncFiles(path string) error
	HandleSyncRequest(request interface{}) error
	GetSyncStatus() string
}
