package base

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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

// IsIgnored 检查文件是否需要忽略
func (s *BaseSyncService) IsIgnored(path string) bool {
	config := s.GetCurrentConfig()
	if config == nil || len(config.IgnoreList) == 0 {
		return false
	}

	// 统一路径分隔符并去除回车符
	path = strings.TrimSpace(filepath.ToSlash(path))

	for _, pattern := range config.IgnoreList {
		// 去除回车符
		pattern = strings.TrimSpace(pattern)
		if matched, _ := filepath.Match(pattern, path); matched {
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

// GetLocalFilesWithMD5 获取本地文件的MD5信息
func (s *BaseSyncService) GetLocalFilesWithMD5(dir string) (map[string]string, error) {
	// 检查路径是文件还是目录
	fileInfo, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// 如果路径不存在，返回空映射而不是错误
			// 这样可以触发后续的同步操作
			s.Logger.Debug("本地路径不存在，返回空映射", interfaces.Fields{
				"path": dir,
			})
			return make(map[string]string), nil
		}
		// 其他错误则返回
		return nil, fmt.Errorf("获取路径信息失败: %v", err)
	}

	// 如果是单个文件
	if !fileInfo.IsDir() {
		md5hash, err := s.calculateFileMD5(dir)
		if err != nil {
			if os.IsNotExist(err) {
				// 如果文件不存在，返回空映射
				return make(map[string]string), nil
			}
			return nil, err
		}
		return map[string]string{
			filepath.Base(dir): md5hash,
		}, nil
	}

	// 如果是目录
	files := make(map[string]string)
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				// 如果文件不存在，跳过该文件
				return nil
			}
			return err
		}
		if !info.IsDir() {
			// 获取相对路径
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			// 检查是否需要重定向
			config := s.GetCurrentConfig()
			if config != nil && len(config.FolderRedirects) > 0 {
				for _, redirect := range config.FolderRedirects {
					// 如果本地路径包含重定向的客户端路径
					if strings.Contains(filepath.ToSlash(relPath), redirect.ClientPath) {
						// 将客户端路径替换为服务器路径
						relPath = strings.Replace(filepath.ToSlash(relPath), redirect.ClientPath, redirect.ServerPath, 1)
						break
					}
				}
			}

			md5hash, err := s.calculateFileMD5(path)
			if err != nil {
				if os.IsNotExist(err) {
					// 如果文件不存在，跳过该文件
					return nil
				}
				return err
			}

			files[relPath] = md5hash
		}
		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			// 如果目录不存在，返回空映射
			return make(map[string]string), nil
		}
		return nil, err
	}

	return files, nil
}

func (s *BaseSyncService) calculateFileMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CompareMD5 比较本地和服务器文件的MD5，返回需要同步的文件信息
func (s *BaseSyncService) CompareMD5(
	localFiles map[string]string,
	serverFiles map[string]string,
) ([]string, map[string]struct{}, int, error) {
	var filesToSync []string
	filesToDelete := make(map[string]struct{})
	var ignoredFiles int

	// 检查本地多余的文件
	for localPath := range localFiles {
		if _, exists := serverFiles[localPath]; !exists && !s.IsIgnored(localPath) {
			s.Logger.Debug("发现本地多余文件", interfaces.Fields{
				"file": localPath,
			})
			filesToDelete[localPath] = struct{}{}
		}
	}

	// 检查需要同步的服务器文件
	for serverPath, serverMD5 := range serverFiles {
		// 检查文件是否需要忽略
		if s.IsIgnored(serverPath) {
			s.Logger.Debug("忽略文件", interfaces.Fields{
				"file": serverPath,
				"md5":  serverMD5,
			})
			ignoredFiles++
			continue
		}

		localMD5, exists := localFiles[serverPath]
		if !exists || localMD5 != serverMD5 {
			filesToSync = append(filesToSync, serverPath)
		}
	}

	s.Logger.Info("文件对比完成", interfaces.Fields{
		"need_sync":     len(filesToSync),
		"ignored_files": ignoredFiles,
		"to_delete":     len(filesToDelete),
	})

	return filesToSync, filesToDelete, ignoredFiles, nil
}
