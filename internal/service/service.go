/*
Package service 实现了同步服务的业务逻辑。

文件作用：
- 实现核心同步逻辑
- 管理同步状态
- 处理配置变更
- 提供服务管理功能

主要类型：
- SyncService: 同步服务
- ServiceState: 服务状态
- SyncResult: 同步结果

主要方法：
- NewSyncService: 创建新的同步服务
- Start: 启动同步服务
- Stop: 停止同步服务
- HandleSync: 处理同步请求
- GetStatus: 获取服务状态
- UpdateConfig: 更新服务配置
- CleanupOldPacks: 清理旧的压缩包
*/

package service

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"synctools/pkg/common"
)

// PackProgress 压缩包同步进度
type PackProgress struct {
	FolderPath  string  `json:"folder_path"`  // 文件夹路径
	TotalSize   int64   `json:"total_size"`   // 总大小
	CurrentSize int64   `json:"current_size"` // 当前大小
	Percentage  float64 `json:"percentage"`   // 完成百分比
	Status      string  `json:"status"`       // 状态描述
}

// SyncProgress 同步进度
type SyncProgress struct {
	TotalFiles     int64  `json:"total_files"`     // 总文件数
	ProcessedFiles int64  `json:"processed_files"` // 已处理文件数
	CurrentFile    string `json:"current_file"`    // 当前处理的文件
	Status         string `json:"status"`          // 状态描述
	BytesTotal     int64  `json:"bytes_total"`     // 总字节数
	BytesProcessed int64  `json:"bytes_processed"` // 已处理字节数
	PackMode       bool   `json:"pack_mode"`       // 是否为pack模式
	PackMD5        string `json:"pack_md5"`        // pack模式下的MD5值
}

// NewSyncProgress 创建新的同步进度
func NewSyncProgress() *SyncProgress {
	return &SyncProgress{
		Status: "准备就绪",
	}
}

// UpdateProgress 更新进度
func (p *SyncProgress) UpdateProgress(processed, total int64, current, status string) {
	p.ProcessedFiles = processed
	p.TotalFiles = total
	p.CurrentFile = current
	p.Status = status
}

// UpdatePackProgress 更新pack模式进度
func (p *SyncProgress) UpdatePackProgress(processed, total int64, md5, status string) {
	p.BytesProcessed = processed
	p.BytesTotal = total
	p.PackMode = true
	p.PackMD5 = md5
	p.Status = status
}

// GetPercentage 获取完成百分比
func (p *SyncProgress) GetPercentage() float64 {
	if p.PackMode {
		if p.BytesTotal > 0 {
			return float64(p.BytesProcessed) / float64(p.BytesTotal) * 100
		}
	} else {
		if p.TotalFiles > 0 {
			return float64(p.ProcessedFiles) / float64(p.TotalFiles) * 100
		}
	}
	return 0
}

// SyncService 同步服务
type SyncService struct {
	configManager    common.ConfigManager
	server           common.Server
	logger           common.Logger
	running          bool
	runningMux       sync.RWMutex
	progressCallback func(*SyncProgress)
	onConfigChanged  func()
	clientStates     map[string]*common.ClientState
	statesMux        sync.RWMutex
}

// NewSyncService 创建新的同步服务
func NewSyncService(configManager common.ConfigManager, logger common.Logger) *SyncService {
	s := &SyncService{
		configManager: configManager,
		logger:        logger,
		clientStates:  make(map[string]*common.ClientState),
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
func (s *SyncService) ListConfigs() ([]*common.Config, error) {
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
func (s *SyncService) GetCurrentConfig() *common.Config {
	config := s.configManager.GetCurrentConfig()
	if config == nil {
		s.logger.DebugLog("当前没有选中的配置")
	} else {
		s.logger.DebugLog("获取当前配置: %s", config.UUID)
	}
	return config
}

// ValidateConfig 验证配置
func (s *SyncService) ValidateConfig(config *common.Config) error {
	s.logger.DebugLog("验证配置: %s", config.UUID)
	if err := s.configManager.ValidateConfig(config); err != nil {
		s.logger.Error("配置验证失败: %v", err)
		return err
	}
	return nil
}

// Save 保存指定配置
func (s *SyncService) Save(config *common.Config) error {
	s.logger.DebugLog("保存配置: %s", config.UUID)
	if err := s.configManager.Save(config); err != nil {
		s.logger.Error("保存配置失败: %v", err)
		return err
	}
	return nil
}

// SetServer 设置服务器实例
func (s *SyncService) SetServer(server common.Server) {
	s.server = server
}

// SetOnConfigChanged 设置配置变更回调
func (s *SyncService) SetOnConfigChanged(callback func()) {
	s.onConfigChanged = callback
}

// GetClientState 获取客户端状态
func (s *SyncService) GetClientState(uuid string) *common.ClientState {
	s.statesMux.RLock()
	defer s.statesMux.RUnlock()

	if state, exists := s.clientStates[uuid]; exists {
		return state
	}

	state := &common.ClientState{
		UUID:         uuid,
		LastSyncTime: time.Now().Unix(),
		FolderStates: make(map[string]common.PackState),
		IsOnline:     false,
	}
	s.clientStates[uuid] = state
	return state
}

// UpdateClientState 更新客户端状态
func (s *SyncService) UpdateClientState(state *common.ClientState) error {
	s.statesMux.Lock()
	defer s.statesMux.Unlock()

	s.clientStates[state.UUID] = state
	return nil
}

// RemoveClientState 移除客户端状态
func (s *SyncService) RemoveClientState(uuid string) error {
	s.statesMux.Lock()
	defer s.statesMux.Unlock()

	delete(s.clientStates, uuid)
	return nil
}

// ListClientStates 列出所有客户端状态
func (s *SyncService) ListClientStates() ([]*common.ClientState, error) {
	s.statesMux.RLock()
	defer s.statesMux.RUnlock()

	states := make([]*common.ClientState, 0, len(s.clientStates))
	for _, state := range s.clientStates {
		states = append(states, state)
	}
	return states, nil
}

// CleanupOldPacks 清理旧的压缩包
func (s *SyncService) CleanupOldPacks(maxAge time.Duration) error {
	config := s.configManager.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("没有选中的配置")
	}

	packDir := filepath.Join(config.SyncDir, "packs")
	return common.CleanupTempFiles(packDir, maxAge)
}
