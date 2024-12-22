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
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"synctools/internal/interfaces"
	"synctools/pkg/errors"
)

// SyncService 同步服务实现
type SyncService struct {
	config           *interfaces.Config
	logger           interfaces.Logger
	storage          interfaces.Storage
	server           interfaces.NetworkServer
	running          bool
	status           string
	statusLock       sync.RWMutex
	onConfigChanged  func()
	progressCallback func(progress *interfaces.Progress)
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

// StartServer 启动网络服务器
func (s *SyncService) StartServer() error {
	if !s.running {
		return errors.ErrServiceNotRunning
	}

	if s.server == nil {
		return errors.NewError("NETWORK_SERVER", "网络服务器未初始化", nil)
	}

	// 启动网络服务器
	if err := s.server.Start(); err != nil {
		s.logger.Error("启动网络服务器失败", interfaces.Fields{
			"error": err,
		})
		return err
	}

	s.setStatus("服务器运行中")
	s.logger.Info("网络服务器已启动", interfaces.Fields{
		"host": s.config.Host,
		"port": s.config.Port,
	})

	return nil
}

// StopServer 停止网络服务器
func (s *SyncService) StopServer() error {
	if s.server == nil {
		return nil
	}

	// 停止网络服务器
	if err := s.server.Stop(); err != nil {
		s.logger.Error("停止网络服务器失败", interfaces.Fields{
			"error": err,
		})
		return err
	}

	s.setStatus("服务器已停止")
	s.logger.Info("网络服务器已停止", nil)

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

	// 获取文件列表
	files, err := s.storage.List()
	if err != nil {
		return fmt.Errorf("获取文件列表失败: %v", err)
	}

	// 创建进度对象
	progress := &interfaces.Progress{
		Total:     int64(len(files)),
		Current:   0,
		Status:    "正在同步文件",
		FileName:  "",
		Speed:     0,
		Remaining: 0,
	}

	// 同步每个文件
	for _, file := range files {
		// 只处理指定路径下的文件
		if !strings.HasPrefix(file, path) {
			continue
		}

		// 检查是否在忽略列表中
		if s.isIgnored(file) {
			s.logger.Debug("忽略文件", interfaces.Fields{
				"file": file,
			})
			continue
		}

		progress.Current++
		progress.FileName = file
		progress.Status = fmt.Sprintf("正在同步: %s", file)

		if s.progressCallback != nil {
			s.progressCallback(progress)
		}

		// 读取文件
		var fileData []byte
		if err := s.storage.Load(file, &fileData); err != nil {
			s.logger.Error("读取文件失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}

		// 获取文件信息
		fileInfo := &interfaces.FileInfo{
			Hash: calculateFileHash(fileData),
			Size: int64(len(fileData)),
		}

		// 记录同步信息
		s.logger.Info("同步文件", interfaces.Fields{
			"file":     file,
			"hash":     fileInfo.Hash,
			"size":     fileInfo.Size,
			"syncMode": s.getSyncMode(file),
		})

		// 根据同步模式处理文件
		if err := s.handleFileSync(file, fileInfo, s.getSyncMode(file)); err != nil {
			s.logger.Error("文件同步失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}
	}

	s.setStatus("同步完成")
	return nil
}

// isIgnored 检查文件是否在忽略列表中
func (s *SyncService) isIgnored(file string) bool {
	for _, pattern := range s.config.IgnoreList {
		matched, err := filepath.Match(pattern, filepath.Base(file))
		if err != nil {
			s.logger.Error("匹配忽略模式失败", interfaces.Fields{
				"pattern": pattern,
				"file":    file,
				"error":   err,
			})
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// getSyncMode 获取文件的同步模式
func (s *SyncService) getSyncMode(file string) interfaces.SyncMode {
	// 检查文件是否在同步文件夹中
	for _, folder := range s.config.SyncFolders {
		if strings.HasPrefix(file, folder.Path) {
			return folder.SyncMode
		}
	}
	return interfaces.AutoSync
}

// handleFileSync 根据同步模式处理文件
func (s *SyncService) handleFileSync(file string, info *interfaces.FileInfo, mode interfaces.SyncMode) error {
	switch mode {
	case interfaces.MirrorSync:
		// 镜像同步：直接覆盖目标文件
		return s.mirrorSync(file, info)
	case interfaces.PackSync:
		// 打包同步：将文件添加到压缩包
		return s.packSync(file, info)
	case interfaces.PushSync:
		// 推送同步：将文件推送到目标
		return s.pushSync(file, info)
	case interfaces.AutoSync:
		// 自动同步：根据文件类型选择同步方式
		return s.autoSync(file, info)
	case interfaces.ManualSync:
		// 手动同步：记录变更等待用户确认
		return s.manualSync(file, info)
	default:
		return fmt.Errorf("不支持的同步模式: %s", mode)
	}
}

// mirrorSync 执行镜像同步
func (s *SyncService) mirrorSync(file string, info *interfaces.FileInfo) error {
	// 直接保存文件
	var data []byte
	if err := s.storage.Load(file, &data); err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	targetPath := filepath.Join(s.config.SyncDir, file)
	if err := s.storage.Save(targetPath, data); err != nil {
		return fmt.Errorf("保存目标文件失败: %v", err)
	}

	return nil
}

// packSync 执行打包同步
func (s *SyncService) packSync(file string, info *interfaces.FileInfo) error {
	// 获取打包目录
	packDir := filepath.Join(s.config.SyncDir, "packages")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		return fmt.Errorf("创建打包目录失败: %v", err)
	}

	// 生成打包文件名
	packName := fmt.Sprintf("%s_%s.zip", filepath.Base(file), time.Now().Format("20060102150405"))
	packPath := filepath.Join(packDir, packName)

	// 读取源文件
	var data []byte
	if err := s.storage.Load(file, &data); err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "sync_pack_*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 复制文件到临时目录
	tempFile := filepath.Join(tempDir, filepath.Base(file))
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %v", err)
	}

	// 创建压缩文件
	zipFile, err := os.Create(packPath)
	if err != nil {
		return fmt.Errorf("创建压缩文件失败: %v", err)
	}
	defer zipFile.Close()

	// 创建 zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 添加文件到压缩包
	fileToZip, err := os.Open(tempFile)
	if err != nil {
		return fmt.Errorf("打开临时文件失败: %v", err)
	}
	defer fileToZip.Close()

	// 获取文件信息
	fileInfo, err := fileToZip.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 创建 zip 文件头
	header, err := zip.FileInfoHeader(fileInfo)
	if err != nil {
		return fmt.Errorf("创建文件头失败: %v", err)
	}

	// 设置压缩方法
	header.Method = zip.Deflate

	// 创建 zip 文件
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("创建压缩文件失败: %v", err)
	}

	// 写入文件内容
	if _, err := io.Copy(writer, fileToZip); err != nil {
		return fmt.Errorf("写入压缩文件失败: %v", err)
	}

	s.logger.Info("文件打包完成", interfaces.Fields{
		"file":     file,
		"packFile": packPath,
		"size":     info.Size,
	})

	return nil
}

// pushSync 执行推送同步
func (s *SyncService) pushSync(file string, info *interfaces.FileInfo) error {
	// 读取源文件
	var data []byte
	if err := s.storage.Load(file, &data); err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	// 获取目标路径
	targetPath := filepath.Join(s.config.SyncDir, file)

	// 检查目标文件是否存在
	targetExists := s.storage.Exists(targetPath)
	if targetExists {
		// 如果目标文件存在，检查是否需要更新
		var targetData []byte
		if err := s.storage.Load(targetPath, &targetData); err != nil {
			return fmt.Errorf("读取目标文件失败: %v", err)
		}

		// 计算目标文件哈希
		targetHash := calculateFileHash(targetData)
		if targetHash == info.Hash {
			// 文件相同，不需要更新
			s.logger.Debug("文件未变更，跳过同步", interfaces.Fields{
				"file": file,
				"hash": info.Hash,
			})
			return nil
		}
	}

	// 创建目标目录
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 保存文件
	if err := s.storage.Save(targetPath, data); err != nil {
		return fmt.Errorf("保存目标文件失败: %v", err)
	}

	s.logger.Info("文件推送完成", interfaces.Fields{
		"file":   file,
		"target": targetPath,
		"size":   info.Size,
	})

	return nil
}

// autoSync 执行自动同步
func (s *SyncService) autoSync(file string, info *interfaces.FileInfo) error {
	// 根据文件类型选择同步方式
	ext := filepath.Ext(file)
	switch ext {
	case ".zip", ".rar", ".7z":
		return s.packSync(file, info)
	case ".exe", ".dll", ".jar":
		return s.mirrorSync(file, info)
	default:
		return s.pushSync(file, info)
	}
}

// manualSync 执行手动同步
func (s *SyncService) manualSync(file string, info *interfaces.FileInfo) error {
	// 记录变更等待用户确认
	s.logger.Info("等待用户确认同步", interfaces.Fields{
		"file": file,
		"hash": info.Hash,
		"size": info.Size,
	})
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

	// 验证路径
	absPath := filepath.Join(s.config.SyncDir, req.Path)
	if !filepath.HasPrefix(absPath, s.config.SyncDir) {
		return errors.NewError("SYNC_PATH", "无效的同步路径", nil)
	}

	// 检查文件是否存在
	exists := s.storage.Exists(req.Path)
	if !exists {
		s.logger.Warn("文件不存在", interfaces.Fields{
			"path": req.Path,
		})
		return nil
	}

	// 读取文件
	var fileData []byte
	if err := s.storage.Load(req.Path, &fileData); err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}

	// 获取文件信息
	fileInfo := &interfaces.FileInfo{
		Hash: calculateFileHash(fileData),
		Size: int64(len(fileData)),
	}

	// 记录同步信息
	s.logger.Info("处理同步请求", interfaces.Fields{
		"path":      req.Path,
		"mode":      req.Mode,
		"direction": req.Direction,
		"hash":      fileInfo.Hash,
		"size":      fileInfo.Size,
	})

	return nil
}

// calculateFileHash 计算文件哈希值
func calculateFileHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
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

// IsRunning 实现 interfaces.SyncService 接口
func (s *SyncService) IsRunning() bool {
	return s.running
}

// GetCurrentConfig 实现 interfaces.SyncService 接口
func (s *SyncService) GetCurrentConfig() *interfaces.Config {
	return s.config
}

// ListConfigs 实现配置列表功能
func (s *SyncService) ListConfigs() ([]*interfaces.Config, error) {
	// 使用storage接口来获取所有配置文件
	files, err := s.storage.List()
	if err != nil {
		return nil, fmt.Errorf("列出配置文件失败: %v", err)
	}

	configs := make([]*interfaces.Config, 0)
	for _, file := range files {
		// 只处理.json文件
		if !strings.HasSuffix(file, ".json") {
			continue
		}

		var config interfaces.Config
		if err := s.storage.Load(file, &config); err != nil {
			s.logger.Error("读取配置文件失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}

		configs = append(configs, &config)
	}

	return configs, nil
}

// LoadConfig 实现配置加载功能
func (s *SyncService) LoadConfig(id string) error {
	// 构造配置文件名
	filename := fmt.Sprintf("%s.json", id)

	// 读取配置文件
	var config interfaces.Config
	if err := s.storage.Load(filename, &config); err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 更新当前配置
	s.config = &config

	// 触发配置变更回调
	if s.onConfigChanged != nil {
		s.onConfigChanged()
	}

	return nil
}

// SaveConfig 实现配置保存功能
func (s *SyncService) SaveConfig(config *interfaces.Config) error {
	// 验证配置
	if err := s.ValidateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	// 保存配置文件
	filename := fmt.Sprintf("%s.json", config.UUID)
	if err := s.storage.Save(filename, config); err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}

	// 更新当前配置
	s.config = config

	// 触发配置变更回调
	if s.onConfigChanged != nil {
		s.onConfigChanged()
	}

	return nil
}

// DeleteConfig 实现配置删除功能
func (s *SyncService) DeleteConfig(uuid string) error {
	// 构造配置文件名
	filename := fmt.Sprintf("%s.json", uuid)

	// 删除配置文件
	if err := s.storage.Delete(filename); err != nil {
		return fmt.Errorf("删除配置文件失败: %v", err)
	}

	// 如果删除的是当前配置，清空当前配置
	if s.config != nil && s.config.UUID == uuid {
		s.config = nil
	}

	return nil
}

// ValidateConfig 实现配置验证功能
func (s *SyncService) ValidateConfig(config *interfaces.Config) error {
	if config == nil {
		return errors.NewError("CONFIG_INVALID", "配置不能为空", nil)
	}

	// 验证基本字段
	if config.UUID == "" {
		return errors.NewError("CONFIG_INVALID", "UUID不能为空", nil)
	}
	if config.Name == "" {
		return errors.NewError("CONFIG_INVALID", "名称不能为空", nil)
	}
	if config.Version == "" {
		return errors.NewError("CONFIG_INVALID", "版本不能为空", nil)
	}
	if config.Host == "" {
		return errors.NewError("CONFIG_INVALID", "主机地址不能为空", nil)
	}
	if config.Port <= 0 || config.Port > 65535 {
		return errors.NewError("CONFIG_INVALID", "端口号无效", nil)
	}
	if config.SyncDir == "" {
		return errors.NewError("CONFIG_INVALID", "同步目录不能为空", nil)
	}

	// 验证同步文件夹
	for i, folder := range config.SyncFolders {
		if folder.Path == "" {
			return errors.NewError("CONFIG_INVALID", fmt.Sprintf("同步文件夹 #%d 路径不能为空", i+1), nil)
		}
		if !filepath.IsAbs(folder.Path) {
			absPath := filepath.Join(config.SyncDir, folder.Path)
			if !filepath.HasPrefix(absPath, config.SyncDir) {
				return errors.NewError("CONFIG_INVALID", fmt.Sprintf("同步文件夹 #%d 路径必须在同步目录内", i+1), nil)
			}
		}
	}

	// 验证重定向配置
	for i, redirect := range config.FolderRedirects {
		if redirect.ServerPath == "" {
			return errors.NewError("CONFIG_INVALID", fmt.Sprintf("重定向 #%d 服务器路径不能为空", i+1), nil)
		}
		if redirect.ClientPath == "" {
			return errors.NewError("CONFIG_INVALID", fmt.Sprintf("重定向 #%d 客户端路径不能为空", i+1), nil)
		}
	}

	return nil
}

// SetOnConfigChanged 实现配置变更回调
func (s *SyncService) SetOnConfigChanged(callback func()) {
	s.onConfigChanged = callback
}

// SetProgressCallback 实现进度回调
func (s *SyncService) SetProgressCallback(callback func(progress *interfaces.Progress)) {
	s.progressCallback = callback
}

// SetServer 实现服务器设置
func (s *SyncService) SetServer(server interfaces.NetworkServer) {
	s.server = server
}
