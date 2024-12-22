package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"synctools/internal/storage"
	"synctools/pkg/common"
)

/*
文件作用:
- 实现配置管理功能
- 管理配置文件读写
- 维护配置缓存
- 提供配置验证

主要方法:
- NewManager: 创建配置管理器
- LoadConfig: 加载配置文件
- SaveConfig: 保存配置文件
- ValidateConfig: 验证配置有效性
- GetCurrentConfig: 获取当前配置
*/

// Manager 配置管理器
type Manager struct {
	storage       storage.Storage
	logger        common.Logger
	currentConfig *common.Config
	onChanged     func()
}

// NewManager 创建新的配置管理器
func NewManager(configDir string, logger common.Logger) (*Manager, error) {
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
func (m *Manager) GetCurrentConfig() *common.Config {
	return m.currentConfig
}

// LoadConfig 加载配置
func (m *Manager) LoadConfig(uuid string) error {
	m.logger.DebugLog("开始加载配置: UUID=%s", uuid)

	config := &common.Config{}
	if err := m.storage.Load(uuid+".json", config); err != nil {
		m.logger.Error("加载配置失败", "error", err)
		return fmt.Errorf("加载配置失败: %v", err)
	}
	m.logger.DebugLog("从存储加载配置成功: Name=%s", config.Name)

	// 确保 UUID 正确设置
	if config.UUID == "" {
		config.UUID = uuid
		m.logger.DebugLog("设置空 UUID: %s", uuid)
	}

	// 创建新的配置对象以避免引用问题
	m.currentConfig = &common.Config{
		UUID:            config.UUID,
		Type:            config.Type,
		Name:            config.Name,
		Version:         config.Version,
		Host:            config.Host,
		Port:            config.Port,
		SyncDir:         config.SyncDir,
		SyncFolders:     make([]common.SyncFolder, len(config.SyncFolders)),
		IgnoreList:      make([]string, len(config.IgnoreList)),
		FolderRedirects: make([]common.FolderRedirect, len(config.FolderRedirects)),
	}

	// 复制切片内容
	copy(m.currentConfig.SyncFolders, config.SyncFolders)
	copy(m.currentConfig.IgnoreList, config.IgnoreList)
	copy(m.currentConfig.FolderRedirects, config.FolderRedirects)

	m.logger.DebugLog("当前配置已更新: UUID=%s, Name=%s", m.currentConfig.UUID, m.currentConfig.Name)

	if m.onChanged != nil {
		m.onChanged()
		m.logger.DebugLog("触发配置变更回调")
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
func (m *Manager) ListConfigs() ([]*common.Config, error) {
	files, err := m.storage.List()
	if err != nil {
		return nil, fmt.Errorf("列出配置文件失败: %v", err)
	}

	var configs []*common.Config
	for _, file := range files {
		if filepath.Ext(file) != ".json" {
			continue
		}

		config := &common.Config{}
		if err := m.storage.Load(file, config); err != nil {
			m.logger.Error("加载配置文件失败", "file", file, "error", err)
			continue
		}

		// 确保 UUID 正确设置
		if config.UUID == "" {
			config.UUID = strings.TrimSuffix(file, ".json")
		}

		// 创建新的配置对象以避免引用问题
		configCopy := &common.Config{
			UUID:            config.UUID,
			Type:            config.Type,
			Name:            config.Name,
			Version:         config.Version,
			Host:            config.Host,
			Port:            config.Port,
			SyncDir:         config.SyncDir,
			SyncFolders:     make([]common.SyncFolder, len(config.SyncFolders)),
			IgnoreList:      make([]string, len(config.IgnoreList)),
			FolderRedirects: make([]common.FolderRedirect, len(config.FolderRedirects)),
		}

		// 复制切片内容
		copy(configCopy.SyncFolders, config.SyncFolders)
		copy(configCopy.IgnoreList, config.IgnoreList)
		copy(configCopy.FolderRedirects, config.FolderRedirects)

		configs = append(configs, configCopy)
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
func (m *Manager) ValidateConfig(config *common.Config) error {
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
func (m *Manager) Save(config *common.Config) error {
	m.logger.DebugLog("开始保存配置: UUID=%s, Name=%s", config.UUID, config.Name)

	if err := m.ValidateConfig(config); err != nil {
		m.logger.Error("配置验证失败", "error", err)
		return err
	}

	// 确保 UUID 正确设置
	if config.UUID == "" {
		uuid, err := common.NewUUID()
		if err != nil {
			m.logger.Error("生成UUID失败", "error", err)
			return fmt.Errorf("生成UUID失败: %v", err)
		}
		config.UUID = uuid
		m.logger.DebugLog("生成新的UUID: %s", uuid)
	}

	// 创建新的配置对象以避免引用问题
	configToSave := &common.Config{
		UUID:            config.UUID,
		Type:            config.Type,
		Name:            config.Name,
		Version:         config.Version,
		Host:            config.Host,
		Port:            config.Port,
		SyncDir:         config.SyncDir,
		SyncFolders:     make([]common.SyncFolder, len(config.SyncFolders)),
		IgnoreList:      make([]string, len(config.IgnoreList)),
		FolderRedirects: make([]common.FolderRedirect, len(config.FolderRedirects)),
	}

	// 复制切片内容
	copy(configToSave.SyncFolders, config.SyncFolders)
	copy(configToSave.IgnoreList, config.IgnoreList)
	copy(configToSave.FolderRedirects, config.FolderRedirects)

	if err := m.storage.Save(configToSave.UUID+".json", configToSave); err != nil {
		m.logger.Error("保存配置失败", "error", err)
		return fmt.Errorf("保存配置失败: %v", err)
	}
	m.logger.DebugLog("配置已保存到存储")

	// 如果当前没有选中的配置，将这个配置设置为当前配置
	if m.currentConfig == nil {
		m.currentConfig = configToSave
		m.logger.DebugLog("设置为当前配置: UUID=%s, Name=%s", configToSave.UUID, configToSave.Name)
		if m.onChanged != nil {
			m.onChanged()
			m.logger.DebugLog("触发配置变更回调")
		}
	}

	return nil
}
