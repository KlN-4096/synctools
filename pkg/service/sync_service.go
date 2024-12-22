/*
文件作用:
- 实现文件同步服务
- 管理文件同步状态和进度
- 处理同步请求和响应
- 提供同步操作的核心功能

主要方法:
- NewSyncService: 创建新的同步服务
- Start: 启动同步服务
- Stop: 停止同步服务
- SyncFiles: 同步指定路径的文件
- HandleSyncRequest: 处理同步请求
- GetSyncStatus: 获取同步状态
- setStatus: 设置同步状态
*/

package service

import (
	"fmt"
	"path/filepath"
	"sync"

	"synctools/internal/interfaces"
	"synctools/pkg/errors"
)

// SyncService 同步服务实现
type SyncService struct {
	config     *interfaces.Config
	logger     interfaces.Logger
	storage    interfaces.Storage
	running    bool
	status     string
	statusLock sync.RWMutex
}

// NewSyncService 创建新的同步服务
func NewSyncService(config *interfaces.Config, logger interfaces.Logger, storage interfaces.Storage) *SyncService {
	return &SyncService{
		config:  config,
		logger:  logger,
		storage: storage,
		status:  "初始化",
	}
}

// Start 实现 interfaces.SyncService 接口
func (s *SyncService) Start() error {
	if s.running {
		return errors.ErrServiceStart
	}

	s.running = true
	s.setStatus("运行中")

	s.logger.Info("同步服务已启动", interfaces.Fields{
		"config": s.config.Name,
	})

	return nil
}

// Stop 实现 interfaces.SyncService 接口
func (s *SyncService) Stop() error {
	if !s.running {
		return nil
	}

	s.running = false
	s.setStatus("已停止")

	s.logger.Info("同步服务已停止", nil)
	return nil
}

// SyncFiles 实现 interfaces.SyncService 接口
func (s *SyncService) SyncFiles(path string) error {
	if !s.running {
		return errors.ErrServiceNotRunning
	}

	s.setStatus(fmt.Sprintf("正在同步: %s", path))

	// 验证路径
	absPath := filepath.Join(s.config.SyncDir, path)
	if !filepath.HasPrefix(absPath, s.config.SyncDir) {
		return errors.NewError("SYNC_PATH", "无效的同步路径", nil)
	}

	// TODO: 实现具体的同步逻辑
	s.logger.Info("开始同步文件", interfaces.Fields{
		"path": path,
	})

	s.setStatus("同步完成")
	return nil
}

// HandleSyncRequest 实现 interfaces.SyncService 接口
func (s *SyncService) HandleSyncRequest(request interface{}) error {
	if !s.running {
		return errors.ErrServiceNotRunning
	}

	req, ok := request.(*interfaces.SyncRequest)
	if !ok {
		return errors.ErrInvalid
	}

	s.setStatus(fmt.Sprintf("处理同步请求: %s", req.Path))

	// TODO: 实现具体的请求处理逻辑
	s.logger.Info("处理同步请求", interfaces.Fields{
		"path": req.Path,
		"mode": req.Mode,
	})

	s.setStatus("请求处理完成")
	return nil
}

// GetSyncStatus 实现 interfaces.SyncService 接口
func (s *SyncService) GetSyncStatus() string {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.status
}

// setStatus 设置同步状态
func (s *SyncService) setStatus(status string) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.status = status
	s.logger.Debug("状态更新", interfaces.Fields{
		"status": status,
	})
}
