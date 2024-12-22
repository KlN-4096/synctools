package interfaces

import "time"

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
