package base

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/errors"
)

// BaseSyncService 提供同步服务的基础实现
type BaseSyncService struct {
	Config           *interfaces.Config
	Logger           interfaces.Logger
	Storage          interfaces.Storage
	Running          bool
	Status           string
	statusLock       sync.RWMutex
	onConfigChanged  func()
	progressCallback func(progress *interfaces.Progress)
}

// SetStatus 设置服务状态
func (s *BaseSyncService) SetStatus(Status string) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.Status = Status
	s.Logger.Debug("状态更新", interfaces.Fields{
		"Status": Status,
	})
}

// GetStatus 获取服务状态
func (s *BaseSyncService) GetStatus() string {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.Status
}

// NewBaseSyncService 创建基础同步服务
func NewBaseSyncService(Config *interfaces.Config, Logger interfaces.Logger, Storage interfaces.Storage) *BaseSyncService {
	return &BaseSyncService{
		Config:  Config,
		Logger:  Logger,
		Storage: Storage,
		Status:  "初始化",
	}
}

// Start 启动服务,只是一个提示
func (s *BaseSyncService) Start() error {
	if s.Running {
		return errors.ErrServiceStart
	}

	s.Running = true
	s.setStatus("已连接")
	return nil
}

// Stop 停止服务
func (s *BaseSyncService) Stop() error {
	if !s.Running {
		return nil
	}

	s.Running = false
	s.setStatus("已停止")
	return nil
}

// IsRunning 检查服务是否运行中
func (s *BaseSyncService) IsRunning() bool {
	return s.Running
}

// GetCurrentConfig 获取当前配置
func (s *BaseSyncService) GetCurrentConfig() *interfaces.Config {
	return s.Config
}

// ListConfigs 获取配置列表
func (s *BaseSyncService) ListConfigs() ([]*interfaces.Config, error) {
	files, err := s.Storage.List()
	if err != nil {
		return nil, fmt.Errorf("列出配置文件失败: %v", err)
	}

	var configs []*interfaces.Config
	for _, file := range files {
		if filepath.Ext(file) != ".json" {
			continue
		}

		var Config interfaces.Config
		if err := s.Storage.Load(file, &Config); err != nil {
			s.Logger.Error("读取配置文件失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}

		configs = append(configs, &Config)
	}

	return configs, nil
}

// LoadConfig 加载配置
func (s *BaseSyncService) LoadConfig(id string) error {
	filename := fmt.Sprintf("%s.json", id)

	var Config interfaces.Config
	if err := s.Storage.Load(filename, &Config); err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	s.Config = &Config

	if s.onConfigChanged != nil {
		s.onConfigChanged()
	}

	return nil
}

// SaveConfig 保存配置
func (s *BaseSyncService) SaveConfig(Config *interfaces.Config) error {
	if err := s.ValidateConfig(Config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	filename := fmt.Sprintf("%s.json", Config.UUID)
	if err := s.Storage.Save(filename, Config); err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}

	s.Config = Config

	if s.onConfigChanged != nil {
		s.onConfigChanged()
	}

	return nil
}

// DeleteConfig 删除配置
func (s *BaseSyncService) DeleteConfig(uuid string) error {
	filename := fmt.Sprintf("%s.json", uuid)

	if err := s.Storage.Delete(filename); err != nil {
		return fmt.Errorf("删除配置文件失败: %v", err)
	}

	if s.Config != nil && s.Config.UUID == uuid {
		s.Config = nil
	}

	return nil
}

// ValidateConfig 验证配置
func (s *BaseSyncService) ValidateConfig(Config *interfaces.Config) error {
	if Config == nil {
		return errors.NewError("CONFIG_INVALID", "配置不能为空", nil)
	}

	if Config.UUID == "" {
		return errors.NewError("CONFIG_INVALID", "UUID不能为空", nil)
	}

	if Config.Name == "" {
		return errors.NewError("CONFIG_INVALID", "名称不能为空", nil)
	}

	if Config.Version == "" {
		return errors.NewError("CONFIG_INVALID", "版本不能为空", nil)
	}

	if Config.Host == "" {
		return errors.NewError("CONFIG_INVALID", "主机地址不能为空", nil)
	}

	if Config.Port <= 0 || Config.Port > 65535 {
		return errors.NewError("CONFIG_INVALID", "端口号无效", nil)
	}

	if Config.SyncDir == "" {
		return errors.NewError("CONFIG_INVALID", "同步目录不能为空", nil)
	}

	return nil
}

// SetOnConfigChanged 设置配置变更回调
func (s *BaseSyncService) SetOnConfigChanged(callback func()) {
	s.onConfigChanged = callback
}

// GetSyncStatus 获取同步状态
func (s *BaseSyncService) GetSyncStatus() string {
	s.statusLock.RLock()
	defer s.statusLock.RUnlock()
	return s.Status
}

// setStatus 设置状态
func (s *BaseSyncService) setStatus(Status string) {
	s.statusLock.Lock()
	defer s.statusLock.Unlock()
	s.Status = Status
	s.Logger.Debug("状态更新", interfaces.Fields{
		"Status": Status,
	})
}

// SetProgressCallback 设置进度回调
func (s *BaseSyncService) SetProgressCallback(callback func(progress *interfaces.Progress)) {
	s.progressCallback = callback
}

// 工具方法

// CalculateFileHash 计算文件哈希值
func (s *BaseSyncService) CalculateFileHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// IsIgnored 检查文件是否被忽略
func (s *BaseSyncService) IsIgnored(file string) bool {
	if s.Config == nil || s.Config.IgnoreList == nil {
		return false
	}

	// 标准化文件路径(使用正斜杠)
	normalizedPath := filepath.ToSlash(file)
	fileName := filepath.Base(file)

	for _, pattern := range s.Config.IgnoreList {
		// 标准化忽略模式
		normalizedPattern := filepath.ToSlash(pattern)

		// 检查是否是路径模式(包含路径分隔符)
		if strings.Contains(normalizedPattern, "/") {
			// 如果模式不以/结尾,添加/以匹配整个目录
			if !strings.HasSuffix(normalizedPattern, "/") {
				normalizedPattern += "/"
			}
			// 检查文件是否在忽略的目录下
			if strings.HasPrefix(normalizedPath, normalizedPattern) {
				s.Logger.Debug("文件在忽略目录下", interfaces.Fields{
					"pattern": pattern,
					"file":    file,
				})
				return true
			}
			continue
		}

		// 尝试匹配文件名
		matched, err := filepath.Match(pattern, fileName)
		if err != nil {
			s.Logger.Error("匹配忽略模式失败", interfaces.Fields{
				"pattern": pattern,
				"file":    fileName,
				"error":   err,
			})
			continue
		}
		if matched {
			s.Logger.Debug("文件名匹配忽略模式", interfaces.Fields{
				"pattern": pattern,
				"file":    fileName,
			})
			return true
		}
	}
	return false
}

// GetSyncMode 获取文件的同步模式
func (s *BaseSyncService) GetSyncMode(file string) interfaces.SyncMode {

	for _, folder := range s.Config.SyncFolders {
		if filepath.HasPrefix(file, folder.Path) {
			return folder.SyncMode
		}
	}
	return interfaces.PushSync
}

// ReportProgress 报告进度
func (s *BaseSyncService) ReportProgress(progress *interfaces.Progress) {
	if s.progressCallback != nil {
		s.progressCallback(progress)
	}
}
