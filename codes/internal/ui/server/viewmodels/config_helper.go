package viewmodels

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"synctools/codes/internal/interfaces"
)

// UpdateSyncFolder 更新同步文件夹
func (vm *ConfigViewModel) UpdateSyncFolder(index int, path string, mode interfaces.SyncMode, redirectPath string) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.SyncFolders) {
		return fmt.Errorf("无效的索引")
	}

	oldPath := config.SyncFolders[index].Path

	// 更新同步文件夹
	config.SyncFolders[index].Path = path
	config.SyncFolders[index].SyncMode = mode

	// 更新或添加重定向配置
	redirectFound := false
	for i, redirect := range config.FolderRedirects {
		if redirect.ServerPath == oldPath {
			if redirectPath != "" {
				config.FolderRedirects[i].ServerPath = path
				config.FolderRedirects[i].ClientPath = redirectPath
			} else {
				// 如果新的重定向路径为空，删除旧的重定向配置
				config.FolderRedirects = append(config.FolderRedirects[:i], config.FolderRedirects[i+1:]...)
			}
			redirectFound = true
			break
		}
	}

	// 如果没有找到旧的重定向配置，但有新的重定向路径，则添加新的重定向配置
	if !redirectFound && redirectPath != "" {
		redirect := interfaces.FolderRedirect{
			ServerPath: path,
			ClientPath: redirectPath,
		}
		config.FolderRedirects = append(config.FolderRedirects, redirect)
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":        err.Error(),
			"path":         path,
			"mode":         mode,
			"redirectPath": redirectPath,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// UpdateRedirect 更新重定向配置
func (vm *ConfigViewModel) UpdateRedirect(index int, serverPath, clientPath string) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.FolderRedirects) {
		return fmt.Errorf("无效的索引")
	}

	// 更新重定向配置
	config.FolderRedirects[index].ServerPath = serverPath
	config.FolderRedirects[index].ClientPath = clientPath

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":      err.Error(),
			"serverPath": serverPath,
			"clientPath": clientPath,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// CreateConfig 创建新配置
func (vm *ConfigViewModel) CreateConfig(name, version string) error {
	// 创建新的配置
	config := &interfaces.Config{
		UUID:            fmt.Sprintf("cfg-%d", time.Now().UnixNano()), // 使用时间戳生成唯一ID
		Type:            interfaces.ConfigTypeServer,
		Name:            name, // 使用用户输入的名称
		Version:         version,
		Host:            "0.0.0.0",
		Port:            8080,
		SyncDir:         filepath.Join(filepath.Dir(os.Args[0]), "sync"),
		SyncFolders:     make([]interfaces.SyncFolder, 0),
		IgnoreList:      make([]string, 0),
		FolderRedirects: make([]interfaces.FolderRedirect, 0),
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":   err.Error(),
			"name":    name,
			"version": version,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	// 刷新配置列表
	vm.configList.RefreshCache()
	vm.UpdateUI()

	return nil
}

// AddRedirect 添加重定向配置
func (vm *ConfigViewModel) AddRedirect(serverPath, clientPath string) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 创建新的重定向配置
	redirect := interfaces.FolderRedirect{
		ServerPath: serverPath,
		ClientPath: clientPath,
	}

	// 添加到列表
	config.FolderRedirects = append(config.FolderRedirects, redirect)

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":      err.Error(),
			"serverPath": serverPath,
			"clientPath": clientPath,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// DeleteRedirect 删除重定向配置
func (vm *ConfigViewModel) DeleteRedirect(index int) error {
	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.FolderRedirects) {
		return fmt.Errorf("无效的索引")
	}

	// 删除重定向
	config.FolderRedirects = append(
		config.FolderRedirects[:index],
		config.FolderRedirects[index+1:]...,
	)

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err.Error(),
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	vm.UpdateUI()
	return nil
}

// AddSyncFolder 添加同步文件夹
func (vm *ConfigViewModel) AddSyncFolder(path string, mode interfaces.SyncMode, redirectPath string) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	// 创建新的同步文件夹
	folder := interfaces.SyncFolder{
		Path:     path,
		SyncMode: mode,
	}

	// 添加到列表
	config.SyncFolders = append(config.SyncFolders, folder)

	// 如果有重定向路径，添加重定向配置
	if redirectPath != "" {
		redirect := interfaces.FolderRedirect{
			ServerPath: path,
			ClientPath: redirectPath,
		}
		config.FolderRedirects = append(config.FolderRedirects, redirect)
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error":        err.Error(),
			"path":         path,
			"mode":         mode,
			"redirectPath": redirectPath,
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	return nil
}

// DeleteSyncFolder 删除同步文件夹
func (vm *ConfigViewModel) DeleteSyncFolder(index int) error {
	config := vm.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if index < 0 || index >= len(config.SyncFolders) {
		return fmt.Errorf("无效的索引")
	}

	// 获取要删除的文件夹路径
	path := config.SyncFolders[index].Path

	// 删除同步文件夹
	config.SyncFolders = append(
		config.SyncFolders[:index],
		config.SyncFolders[index+1:]...,
	)

	// 删除对应的重定向配置
	for i, redirect := range config.FolderRedirects {
		if redirect.ServerPath == path {
			config.FolderRedirects = append(
				config.FolderRedirects[:i],
				config.FolderRedirects[i+1:]...,
			)
			break
		}
	}

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err.Error(),
		})
		return fmt.Errorf("保存配置失败: %v", err)
	}

	vm.UpdateUI()
	return nil
}

// LoadConfig 加载配置
func (vm *ConfigViewModel) LoadConfig(uuid string) error {
	if err := vm.syncService.LoadConfig(uuid); err != nil {
		vm.logger.Error("加载配置失败", interfaces.Fields{
			"error": err.Error(),
			"uuid":  uuid,
		})
		return fmt.Errorf("加载配置失败: %v", err)
	}

	// 获取当前配置
	config := vm.syncService.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("加载配置失败: 配置为空")
	}

	vm.UpdateUI()
	return nil
}

// SaveConfig 处理保存配置的 UI 操作
func (vm *ConfigViewModel) SaveConfig() error {
	// 安全检查
	if vm == nil || vm.syncService == nil {
		return fmt.Errorf("视图模型或同步服务未初始化")
	}

	// 检查是否有选中的配置
	if vm.syncService.GetCurrentConfig() == nil {
		if vm.statusBar != nil {
			vm.setStatus("没有选中的配置")
		}
		return fmt.Errorf("没有选中的配置")
	}

	// 从 UI 收集配置数据
	config := vm.collectConfigFromUI()

	// 调用服务层保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		if vm.statusBar != nil {
			vm.setStatus("保存配置失败")
		}
		if vm.saveButton != nil {
			vm.saveButton.SetEnabled(true)
		}
		return err
	}

	// 更新 UI 状态
	vm.isEditing = false
	if vm.saveButton != nil {
		vm.saveButton.SetEnabled(true)
	}
	if vm.statusBar != nil {
		vm.setStatus("配置已保存")
	}
	return nil
}

// DeleteConfig 删除配置
func (vm *ConfigViewModel) DeleteConfig(uuid string) error {
	if err := vm.syncService.DeleteConfig(uuid); err != nil {
		return err
	}
	// 刷新配置列表
	vm.configList.RefreshCache()
	vm.UpdateUI()
	return nil
}

// GetCurrentConfig 获取当前配置
func (vm *ConfigViewModel) GetCurrentConfig() *interfaces.Config {
	return vm.syncService.GetCurrentConfig()
}

// ListConfigs 获取配置列表
func (vm *ConfigViewModel) ListConfigs() ([]*interfaces.Config, error) {
	return vm.syncService.ListConfigs()
}
