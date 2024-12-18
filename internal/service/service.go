package service

import (
	"fmt"
	"sync"

	"synctools/internal/config"
	"synctools/internal/model"
	"synctools/internal/network"
)

// SyncProgress 同步进度信息
type SyncProgress struct {
	TotalFiles     int
	ProcessedFiles int
	CurrentFile    string
	Status         string
}

// SyncService 同步服务
type SyncService struct {
	configManager    *config.Manager
	server           *network.Server
	logger           model.Logger
	running          bool
	runningMux       sync.RWMutex
	progressCallback func(*SyncProgress)
}

// NewSyncService 创建新的同步服务
func NewSyncService(configManager *config.Manager, logger model.Logger) *SyncService {
	return &SyncService{
		configManager: configManager,
		logger:        logger,
	}
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

	server := network.NewServer(config, s.logger)
	if err := server.Start(); err != nil {
		return fmt.Errorf("启动服务器失败: %v", err)
	}

	s.server = server
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
	s.server = nil

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
