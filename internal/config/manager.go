package config

import (
	"fmt"
	"path/filepath"

	"synctools/internal/model"
	"synctools/internal/storage"
)

// Manager 配置管理器
type Manager struct {
	storage       storage.Storage
	logger        model.Logger
	currentConfig *model.Config
	onChanged     func()
}

// NewManager 创建新的配置管理器
func NewManager(configDir string, logger model.Logger) (*Manager, error) {
	return &Manager{
		storage: storage.NewFileStorage(configDir),
		logger:  logger,
	}, nil
}

// SetOnChanged 设置配置变更回调
func (m *Manager) SetOnChanged(callback func()) {
	m.onChanged = callback
}

// GetCurrentConfig 获取当前配置
func (m *Manager) GetCurrentConfig() *model.Config {
	return m.currentConfig
}

// LoadConfig 加载配置
func (m *Manager) LoadConfig(uuid string) error {
	var config model.Config
	if err := m.storage.Load(uuid+".json", &config); err != nil {
		return fmt.Errorf("加载配置失败: %v", err)
	}

	m.currentConfig = &config
	if m.onChanged != nil {
		m.onChanged()
	}

	return nil
}

// SaveCurrentConfig 保存当前配置
func (m *Manager) SaveCurrentConfig() error {
	if m.currentConfig == nil {
		return fmt.Errorf("没有当前配置")
	}

	if err := m.ValidateConfig(m.currentConfig); err != nil {
		return err
	}

	if err := m.storage.Save(m.currentConfig.UUID+".json", m.currentConfig); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	if m.onChanged != nil {
		m.onChanged()
	}

	return nil
}

// ListConfigs 获取配置列表
func (m *Manager) ListConfigs() ([]*model.Config, error) {
	files, err := m.storage.List()
	if err != nil {
		return nil, fmt.Errorf("列出配置文件失败: %v", err)
	}

	var configs []*model.Config
	for _, file := range files {
		if filepath.Ext(file) != ".json" {
			continue
		}

		var config model.Config
		if err := m.storage.Load(file, &config); err != nil {
			m.logger.Error("加载配置文件失败", "file", file, "error", err)
			continue
		}

		configs = append(configs, &config)
	}

	return configs, nil
}

// DeleteConfig 删除配置
func (m *Manager) DeleteConfig(uuid string) error {
	if err := m.storage.Delete(uuid + ".json"); err != nil {
		return fmt.Errorf("删除配置失败: %v", err)
	}

	if m.currentConfig != nil && m.currentConfig.UUID == uuid {
		m.currentConfig = nil
		if m.onChanged != nil {
			m.onChanged()
		}
	}

	return nil
}

// ValidateConfig 验证配置
func (m *Manager) ValidateConfig(config *model.Config) error {
	if config.UUID == "" {
		return fmt.Errorf("UUID不能为空")
	}
	if config.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	if config.Version == "" {
		return fmt.Errorf("版本不能为空")
	}
	if config.Host == "" {
		return fmt.Errorf("主机地址不能为空")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("端口号无效")
	}
	return nil
}

// Save 保存指定的配置
func (m *Manager) Save(config *model.Config) error {
	if err := m.ValidateConfig(config); err != nil {
		return err
	}

	if err := m.storage.Save(config.UUID+".json", config); err != nil {
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}
