/*
文件作用:
- 实现配置管理器
- 负责配置文件的加载、保存和验证
- 管理配置的生命周期和变更通知
- 提供配置的CRUD操作

主要方法:
- NewManager: 创建新的配置管理器
- LoadConfig: 加载配置文件
- SaveConfig: 保存配置到文件
- ValidateConfig: 验证配置有效性
- GetCurrentConfig: 获取当前配置
- GetLastModified: 获取最后修改时间
- ListConfigs: 获取所有配置列表
- SetOnChanged: 设置配置变更回调
*/

package config

import (
	"fmt"
	"path/filepath"
	"time"

	"synctools/internal/interfaces"
)

// Manager 配置管理器
type Manager struct {
	storage       interfaces.Storage
	logger        interfaces.Logger
	currentConfig *interfaces.Config
	lastModified  time.Time
	onChanged     func()
}

// NewManager 创建新的配置管理器
func NewManager(storage interfaces.Storage, logger interfaces.Logger) *Manager {
	return &Manager{
		storage: storage,
		logger:  logger,
	}
}

// LoadConfig 实现 interfaces.ConfigManager 接口
func (m *Manager) LoadConfig(id string) error {
	m.logger.Debug("开始加载配置", interfaces.Fields{"uuid": id})

	config := &interfaces.Config{}
	if err := m.storage.Load(id+".json", config); err != nil {
		m.logger.Error("加载配置失败", interfaces.Fields{"error": err})
		return fmt.Errorf("加载配置失败: %v", err)
	}

	// 确保 UUID 正确设置
	if config.UUID == "" {
		config.UUID = id
	}

	m.currentConfig = config
	m.lastModified = time.Now()

	if m.onChanged != nil {
		m.onChanged()
	}

	return nil
}

// SaveConfig 实现 interfaces.ConfigManager 接口
func (m *Manager) SaveConfig(config interface{}) error {
	cfg, ok := config.(*interfaces.Config)
	if !ok {
		return fmt.Errorf("无效的配置类型")
	}

	if err := m.ValidateConfig(cfg); err != nil {
		return err
	}

	if err := m.storage.Save(cfg.UUID+".json", cfg); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	m.lastModified = time.Now()

	if m.onChanged != nil {
		m.onChanged()
	}

	return nil
}

// ValidateConfig 实现 interfaces.ConfigManager 接口
func (m *Manager) ValidateConfig(config interface{}) error {
	cfg, ok := config.(*interfaces.Config)
	if !ok {
		return fmt.Errorf("无效的配置类型")
	}

	if cfg.UUID == "" {
		return fmt.Errorf("UUID不能为空")
	}
	if cfg.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	if cfg.Version == "" {
		return fmt.Errorf("版本不能为空")
	}
	if cfg.Host == "" {
		return fmt.Errorf("主机地址不能为空")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("端口号无效")
	}
	return nil
}

// GetCurrentConfig 实现 interfaces.ConfigManager 接口
func (m *Manager) GetCurrentConfig() interface{} {
	return m.currentConfig
}

// GetLastModified 实现 interfaces.ConfigManager 接口
func (m *Manager) GetLastModified() time.Time {
	return m.lastModified
}

// SetOnChanged 设置配置变更回调
func (m *Manager) SetOnChanged(callback func()) {
	m.onChanged = callback
}

// ListConfigs 获取配置列表
func (m *Manager) ListConfigs() ([]*interfaces.Config, error) {
	files, err := m.storage.List()
	if err != nil {
		return nil, fmt.Errorf("列出配置文件失败: %v", err)
	}

	var configs []*interfaces.Config
	for _, file := range files {
		if filepath.Ext(file) != ".json" {
			continue
		}

		config := &interfaces.Config{}
		if err := m.storage.Load(file, config); err != nil {
			m.logger.Error("加载配置文件失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}

		configs = append(configs, config)
	}

	return configs, nil
}
