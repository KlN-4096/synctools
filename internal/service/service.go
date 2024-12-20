package service

import (
	"fmt"
	"sync"

	"synctools/internal/model"
)

// SyncProgress 同步进度信息
type SyncProgress struct {
	TotalFiles     int    `json:"total_files"`
	ProcessedFiles int    `json:"processed_files"`
	CurrentFile    string `json:"current_file"`
	Status         string `json:"status"`
}

// SyncService 同步服务
type SyncService struct {
	configManager    model.ConfigManager
	server           model.Server
	logger           model.Logger
	running          bool
	runningMux       sync.RWMutex
	progressCallback func(*SyncProgress)
	onConfigChanged  func()
}

// NewSyncService 创建新的同步服务
func NewSyncService(configManager model.ConfigManager, logger model.Logger) *SyncService {
	s := &SyncService{
		configManager: configManager,
		logger:        logger,
	}

	// 设置配置管理器的变更回调
	configManager.SetOnChanged(func() {
		if s.onConfigChanged != nil {
			s.onConfigChanged()
		}
	})

	return s
}

// SetProgressCallback 设置进度回调函数
func (s *SyncService) SetProgressCallback(callback func(*SyncProgress)) {
	s.progressCallback = callback
}

// updateProgress 更新同步进度
func (s *SyncService) updateProgress(progress *SyncProgress) {
	if s.progressCallback != nil {
		s.progressCallback(progress)
	}
}

// Start 启动服务
func (s *SyncService) Start() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if s.running {
		return fmt.Errorf("服务已经在运行")
	}

	config := s.configManager.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	if s.server == nil {
		return fmt.Errorf("服务器未初始化")
	}

	if err := s.server.Start(); err != nil {
		return fmt.Errorf("启动服务器失败: %v", err)
	}

	s.running = true

	// 更新初始进度
	s.updateProgress(&SyncProgress{
		Status: "服务已启动",
	})

	return nil
}

// Stop 停止服务
func (s *SyncService) Stop() error {
	s.runningMux.Lock()
	defer s.runningMux.Unlock()

	if !s.running {
		return nil
	}

	if err := s.server.Stop(); err != nil {
		return fmt.Errorf("停止服务器失败: %v", err)
	}

	s.running = false

	// 更新停止状态
	s.updateProgress(&SyncProgress{
		Status: "服务已停止",
	})

	return nil
}

// IsRunning 检查服务是否正在运行
func (s *SyncService) IsRunning() bool {
	s.runningMux.RLock()
	defer s.runningMux.RUnlock()
	return s.running
}

// ListConfigs 获取配置列表
func (s *SyncService) ListConfigs() ([]*model.Config, error) {
	s.logger.DebugLog("获取配置列表")
	configs, err := s.configManager.ListConfigs()
	if err != nil {
		s.logger.Error("获取配置列表失败: %v", err)
	}
	return configs, err
}

// SaveConfig 保存配置
func (s *SyncService) SaveConfig() error {
	s.logger.DebugLog("保存当前配置")
	config := s.configManager.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}
	return s.Save(config)
}

// LoadConfig 加载配置
func (s *SyncService) LoadConfig(uuid string) error {
	s.logger.DebugLog("加载配置: %s", uuid)
	if err := s.configManager.LoadConfig(uuid); err != nil {
		s.logger.Error("加载配置失败: %v", err)
		return err
	}
	return nil
}

// DeleteConfig 删除配置
func (s *SyncService) DeleteConfig(uuid string) error {
	s.logger.DebugLog("删除配置: %s", uuid)
	if err := s.configManager.DeleteConfig(uuid); err != nil {
		s.logger.Error("删除配置失败: %v", err)
		return err
	}
	return nil
}

// GetCurrentConfig 获取当前配置
func (s *SyncService) GetCurrentConfig() *model.Config {
	config := s.configManager.GetCurrentConfig()
	if config == nil {
		s.logger.DebugLog("当前没有选中的配置")
	} else {
		s.logger.DebugLog("获取当前配置: %s", config.UUID)
	}
	return config
}

// ValidateConfig 验证配置
func (s *SyncService) ValidateConfig(config *model.Config) error {
	s.logger.DebugLog("验证配置: %s", config.UUID)
	if err := s.configManager.ValidateConfig(config); err != nil {
		s.logger.Error("配置验证失败: %v", err)
		return err
	}
	return nil
}

// Save 保存指定配置
func (s *SyncService) Save(config *model.Config) error {
	s.logger.DebugLog("保存配置: %s", config.UUID)
	if err := s.configManager.Save(config); err != nil {
		s.logger.Error("保存配置失败: %v", err)
		return err
	}
	return nil
}

// SetServer 设置服务器实例
func (s *SyncService) SetServer(server model.Server) {
	s.server = server
}

// SetOnConfigChanged 设置配置变更回调
func (s *SyncService) SetOnConfigChanged(callback func()) {
	s.onConfigChanged = callback
}
