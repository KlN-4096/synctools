package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"synctools/pkg/common"
)

// ConfigManager handles all configuration-related operations
type ConfigManager struct {
	CurrentConfig   common.SyncConfig
	ConfigList      []common.SyncConfig
	ConfigListModel *ConfigListModel
	SelectedUUID    string
	ConfigDir       string
	Logger          common.Logger
	OnConfigChanged func()
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(logger common.Logger) *ConfigManager {
	// 设置配置文件路径
	configDir := filepath.Join(os.Getenv("APPDATA"), "SyncTools")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("创建配置目录失败: %v\n", err)
	}

	// 生成默认UUID
	uuid := make([]byte, 16)
	rand.Read(uuid)
	uuidStr := hex.EncodeToString(uuid)

	cm := &ConfigManager{
		CurrentConfig: common.SyncConfig{
			UUID:    uuidStr,
			Name:    "默认整合包",
			Version: "1.0.0",
			Host:    "0.0.0.0",
			Port:    6666,
			SyncDir: "",
			IgnoreList: []string{
				".clientconfig",
				".DS_Store",
				"thumbs.db",
			},
			FolderRedirects: []common.FolderRedirect{
				{ServerPath: "clientmods", ClientPath: "mods"},
			},
		},
		ConfigDir:    configDir,
		Logger:       logger,
		SelectedUUID: uuidStr,
		ConfigList:   make([]common.SyncConfig, 0),
	}

	return cm
}

// LoadAllConfigs loads all configuration files
func (cm *ConfigManager) LoadAllConfigs() error {
	files, err := os.ReadDir(cm.ConfigDir)
	if err != nil {
		return fmt.Errorf("读取配置目录失败: %v", err)
	}

	// 尝试加载选中的UUID
	selectedPath := filepath.Join(cm.ConfigDir, "selected_uuid.txt")
	if data, err := os.ReadFile(selectedPath); err == nil {
		cm.SelectedUUID = strings.TrimSpace(string(data))
		if cm.Logger != nil {
			cm.Logger.DebugLog("已从文件加载选中的UUID: %s", cm.SelectedUUID)
		}
	}

	cm.ConfigList = make([]common.SyncConfig, 0)
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "config_") && strings.HasSuffix(file.Name(), ".json") {
			configPath := filepath.Join(cm.ConfigDir, file.Name())
			if config, err := common.LoadConfig(configPath); err == nil {
				cm.ConfigList = append(cm.ConfigList, *config)
			}
		}
	}

	// 如果没有配置文件，使用当前配置作为默认配置
	if len(cm.ConfigList) == 0 {
		configPath := filepath.Join(cm.ConfigDir, fmt.Sprintf("config_%s.json", cm.CurrentConfig.UUID))
		if err := common.SaveConfig(&cm.CurrentConfig, configPath); err == nil {
			cm.ConfigList = append(cm.ConfigList, cm.CurrentConfig)
			cm.SelectedUUID = cm.CurrentConfig.UUID
		}
	} else {
		// 如果没有选中的UUID或者选中的UUID不存在于配置列表中，使用第一个配置
		validUUID := false
		if cm.SelectedUUID != "" {
			for _, config := range cm.ConfigList {
				if config.UUID == cm.SelectedUUID {
					validUUID = true
					cm.CurrentConfig = config
					break
				}
			}
		}

		if !validUUID {
			cm.SelectedUUID = cm.ConfigList[0].UUID
			cm.CurrentConfig = cm.ConfigList[0]
			if cm.Logger != nil {
				cm.Logger.DebugLog("使用第一个配置作为默认配置: %s", cm.CurrentConfig.Name)
			}
		}
	}

	if cm.OnConfigChanged != nil {
		cm.OnConfigChanged()
	}

	return nil
}

// LoadConfigByUUID loads a configuration by its UUID
func (cm *ConfigManager) LoadConfigByUUID(uuid string) error {
	var newConfig common.SyncConfig

	// 先从内存中查找
	var sourceConfig *common.SyncConfig
	for i := range cm.ConfigList {
		if cm.ConfigList[i].UUID == uuid {
			sourceConfig = &cm.ConfigList[i]
			break
		}
	}

	// 如果内存中没有，尝试从文件加载
	if sourceConfig == nil {
		configPath := filepath.Join(cm.ConfigDir, fmt.Sprintf("config_%s.json", uuid))
		config, err := common.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置失败: %v", err)
		}
		sourceConfig = config
	}

	if sourceConfig == nil {
		return fmt.Errorf("找不到UUID为 %s 的配置", uuid)
	}

	// 复制配置
	newConfig = *sourceConfig

	// 验证配置
	if err := cm.validateConfig(&newConfig); err != nil {
		return err
	}

	// 更新当前配置
	cm.CurrentConfig = newConfig
	cm.SelectedUUID = uuid

	if cm.Logger != nil {
		cm.Logger.DebugLog("配置切换完成")
	}

	if cm.OnConfigChanged != nil {
		cm.OnConfigChanged()
	}

	return nil
}

// SaveConfig saves the current configuration
func (cm *ConfigManager) SaveConfig() error {
	// 校验UUID
	if cm.CurrentConfig.UUID == "" {
		uuid := make([]byte, 16)
		if _, err := rand.Read(uuid); err != nil {
			return fmt.Errorf("生成UUID失败: %v", err)
		}
		cm.CurrentConfig.UUID = hex.EncodeToString(uuid)
	}

	// 验证配置
	if err := cm.validateConfig(&cm.CurrentConfig); err != nil {
		return err
	}

	configPath := filepath.Join(cm.ConfigDir, fmt.Sprintf("config_%s.json", cm.CurrentConfig.UUID))
	if err := common.SaveConfig(&cm.CurrentConfig, configPath); err != nil {
		return err
	}

	// 更新配置列表
	found := false
	for i, config := range cm.ConfigList {
		if config.UUID == cm.CurrentConfig.UUID {
			cm.ConfigList[i] = cm.CurrentConfig
			found = true
			break
		}
	}

	if !found {
		cm.ConfigList = append(cm.ConfigList, cm.CurrentConfig)
	}

	// 保存选中的UUID
	selectedPath := filepath.Join(cm.ConfigDir, "selected_uuid.txt")
	if err := os.WriteFile(selectedPath, []byte(cm.SelectedUUID), 0644); err != nil {
		if cm.Logger != nil {
			cm.Logger.Log("保存选中UUID失败: %v", err)
		}
	}

	if cm.OnConfigChanged != nil {
		cm.OnConfigChanged()
	}

	return nil
}

// DeleteConfig deletes a configuration
func (cm *ConfigManager) DeleteConfig(uuid string) error {
	configPath := filepath.Join(cm.ConfigDir, fmt.Sprintf("config_%s.json", uuid))
	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("删除配置文件失败: %v", err)
	}

	// 从列表中移除
	for i, config := range cm.ConfigList {
		if config.UUID == uuid {
			cm.ConfigList = append(cm.ConfigList[:i], cm.ConfigList[i+1:]...)
			break
		}
	}

	// 如果删除的是当前配置，加载第一个配置
	if uuid == cm.SelectedUUID && len(cm.ConfigList) > 0 {
		cm.LoadConfigByUUID(cm.ConfigList[0].UUID)
	}

	if cm.OnConfigChanged != nil {
		cm.OnConfigChanged()
	}

	return nil
}

// validateConfig validates the configuration
func (cm *ConfigManager) validateConfig(config *common.SyncConfig) error {
	if config.Name == "" {
		return fmt.Errorf("整合包名称不能为空")
	}
	if config.Version == "" {
		return fmt.Errorf("整合包版本不能为空")
	}
	if config.Host == "" {
		return fmt.Errorf("主机地址不能为空")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("端口号无效 (1-65535)")
	}

	// 确保切片字段不为nil
	if config.SyncFolders == nil {
		config.SyncFolders = make([]common.SyncFolder, 0)
	}
	if config.IgnoreList == nil {
		config.IgnoreList = make([]string, 0)
	}
	if config.FolderRedirects == nil {
		config.FolderRedirects = make([]common.FolderRedirect, 0)
	}

	// 验证同步文件夹配置
	for i, folder := range config.SyncFolders {
		if folder.Path == "" {
			return fmt.Errorf("同步文件夹 #%d 的路径不能为空", i+1)
		}
		if folder.SyncMode != "mirror" && folder.SyncMode != "push" {
			return fmt.Errorf("同步文件夹 #%d 的同步模式无效 (mirror/push)", i+1)
		}
	}

	// 验证重定向配置
	for i, redirect := range config.FolderRedirects {
		if redirect.ServerPath == "" {
			return fmt.Errorf("重定向配置 #%d 的服务器路径不能为空", i+1)
		}
		if redirect.ClientPath == "" {
			return fmt.Errorf("重定向配置 #%d 的客户端路径不能为空", i+1)
		}
	}

	return nil
}

// GetCurrentConfig returns the current configuration
func (cm *ConfigManager) GetCurrentConfig() *common.SyncConfig {
	return &cm.CurrentConfig
}

// SetConfigListModel sets the configuration list model
func (cm *ConfigManager) SetConfigListModel(model *ConfigListModel) {
	cm.ConfigListModel = model
}

// UpdateConfigListModel updates the configuration list model
func (cm *ConfigManager) UpdateConfigListModel() {
	if cm.ConfigListModel != nil {
		cm.ConfigListModel.PublishRowsReset()
	}
}
