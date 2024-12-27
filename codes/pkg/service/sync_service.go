/*
文件作用:
- 实现核心业务逻辑服务层
- 管理配置的存储和验证
- 管理文件同步状态和进度
- 处理同步请求和响应
- 管理网络服务器的生命周期

主要功能:
1. 配置管理:
   - 配置的 CRUD 操作
   - 配置的验证和存储
   - 配置变更通知

2. 同步服务:
   - 文件同步操作
   - 同步模式管理
   - 进度跟踪和回调

3. 网络服务:
   - 服务器的启动和停止
   - 请求的处理和响应
   - 状态管理

主要方法:
- 配置相关：LoadConfig, SaveConfig, ValidateConfig
- 同步相关：SyncFiles, HandleSyncRequest
- 服务器相关：StartServer, StopServer
- 状态管理：GetSyncStatus, IsRunning
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

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/errors"
	network "synctools/codes/pkg/network/server"
	"synctools/codes/pkg/storage"
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

// ==================== 1. 核心服务管理 ====================

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

	s.logger.Info("服务状态变更", interfaces.Fields{
		"status": "started",
		"type":   "sync",
		"config": s.config.Name,
	})

	return nil
}

// Stop 实现 interfaces.SyncService 接口
func (s *SyncService) Stop() error {
	if !s.running {
		return nil
	}

	// 确保先停止网络服务器
	if err := s.StopServer(); err != nil {
		s.logger.Error("服务操作失败", interfaces.Fields{
			"operation": "stop_server",
			"error":     err,
		})
		return err
	}

	s.running = false
	s.setStatus("已停止")
	s.logger.Info("服务状态变更", interfaces.Fields{
		"status": "stopped",
		"type":   "sync",
	})
	return nil
}

// IsRunning 实现 interfaces.SyncService 接口
func (s *SyncService) IsRunning() bool {
	return s.running
}

// ==================== 2. 配置管理 ====================

// GetCurrentConfig 实现 interfaces.SyncService 接口
func (s *SyncService) GetCurrentConfig() *interfaces.Config {
	return s.config
}

// ListConfigs 实现配置列表功能
func (s *SyncService) ListConfigs() ([]*interfaces.Config, error) {
	files, err := s.storage.List()
	if err != nil {
		return nil, fmt.Errorf("列出配置文件失败: %v", err)
	}

	configs := make([]*interfaces.Config, 0)
	for _, file := range files {
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
	filename := fmt.Sprintf("%s.json", id)

	var config interfaces.Config
	if err := s.storage.Load(filename, &config); err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	s.config = &config

	if s.onConfigChanged != nil {
		s.onConfigChanged()
	}

	return nil
}

// SaveConfig 实现配置保存功能
func (s *SyncService) SaveConfig(config *interfaces.Config) error {
	if err := s.ValidateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	filename := fmt.Sprintf("%s.json", config.UUID)
	if err := s.storage.Save(filename, config); err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}

	s.config = config

	if s.onConfigChanged != nil {
		s.onConfigChanged()
	}

	return nil
}

// DeleteConfig 实现配置删除功能
func (s *SyncService) DeleteConfig(uuid string) error {
	filename := fmt.Sprintf("%s.json", uuid)

	if err := s.storage.Delete(filename); err != nil {
		return fmt.Errorf("删除配置文件失败: %v", err)
	}

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

// ==================== 3. 同步操作 ====================

// SyncFiles 同步指定目录的文件
func (s *SyncService) SyncFiles(sourcePath string) error {
	if !s.running {
		return errors.ErrServiceNotRunning
	}

	s.setStatus("同步中")
	defer func() {
		s.setStatus("同步完成")
		s.logger.Info("同步完成", interfaces.Fields{
			"path": sourcePath,
		})
	}()

	// 获取源文件夹下的所有文件
	files, err := filepath.Glob(filepath.Join(sourcePath, "*"))
	if err != nil {
		s.logger.Error("获取源文件列表失败", interfaces.Fields{
			"path":  sourcePath,
			"error": err,
		})
		return fmt.Errorf("获取源文件列表失败: %v", err)
	}

	// 遍历处理每个文件
	for _, file := range files {
		relPath, err := filepath.Rel(sourcePath, file)
		if err != nil {
			s.logger.Error("获取相对路径失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}

		// 检查是否在忽略列表中
		if s.isIgnored(relPath) {
			s.logger.Debug("忽略文件", interfaces.Fields{
				"file": relPath,
			})
			continue
		}

		// 同步文件
		if err := s.syncFile(file, relPath, interfaces.MirrorSync, s.storage); err != nil {
			s.logger.Error("同步文件失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}
	}

	return nil
}

// syncFile 同步单个文件
func (s *SyncService) syncFile(sourcePath, relPath string, mode interfaces.SyncMode, storage interfaces.Storage) error {
	if sourcePath == "" {
		return fmt.Errorf("源文件路径不能为空")
	}
	if relPath == "" {
		return fmt.Errorf("相对路径不能为空")
	}
	if storage == nil {
		return fmt.Errorf("存储服务不能为空")
	}

	if _, err := os.Stat(sourcePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("源文件不存在: %s", sourcePath)
		}
		return fmt.Errorf("检查源文件失败: %v", err)
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	targetPath := relPath
	if s.config != nil && s.config.FolderRedirects != nil {
		for _, redirect := range s.config.FolderRedirects {
			if redirect.ServerPath != "" && strings.HasPrefix(relPath, redirect.ServerPath) {
				targetPath = strings.Replace(relPath, redirect.ServerPath, redirect.ClientPath, 1)
				s.logger.Debug("应用重定向规则", interfaces.Fields{
					"source": relPath,
					"target": targetPath,
					"rule":   fmt.Sprintf("%s -> %s", redirect.ServerPath, redirect.ClientPath),
				})
				break
			}
		}
	}

	switch mode {
	case interfaces.PushSync:
		if storage.Exists(targetPath) {
			var existingData []byte
			if err := storage.Load(targetPath, &existingData); err != nil {
				return fmt.Errorf("读取目标文件失败: %v", err)
			}
			if calculateFileHash(data) == calculateFileHash(existingData) {
				s.logger.Debug("文件未变更，跳过同步", interfaces.Fields{
					"file": targetPath,
				})
				return nil
			}
		}
	case interfaces.MirrorSync:
		s.logger.Debug("执行镜像同步", interfaces.Fields{
			"file": targetPath,
		})
	case interfaces.PackSync:
		s.logger.Debug("暂不支持pack模式", interfaces.Fields{
			"file": targetPath,
		})
		return fmt.Errorf("暂不支持pack模式")
	default:
		return fmt.Errorf("未知的同步模式: %s", mode)
	}

	if err := storage.Save(targetPath, data); err != nil {
		return fmt.Errorf("保存文件失败: %v", err)
	}

	s.logger.Debug("文件同步完成", interfaces.Fields{
		"source": sourcePath,
		"target": targetPath,
		"mode":   mode,
		"size":   len(data),
	})

	return nil
}

// HandleSyncRequest 实现 interfaces.SyncService 接口
func (s *SyncService) HandleSyncRequest(request interface{}) error {
	req, ok := request.(*interfaces.SyncRequest)
	if !ok {
		s.logger.Error("请求类型错误", interfaces.Fields{
			"type": fmt.Sprintf("%T", request),
		})
		return fmt.Errorf("无效的请求类型")
	}

	sourcePath := s.config.SyncDir
	if sourcePath == "" {
		s.logger.Error("服务端同步目录未配置", interfaces.Fields{})
		return fmt.Errorf("服务端同步目录未配置")
	}

	targetStorage := req.Storage
	if targetStorage == nil {
		s.logger.Debug("创建目标存储服务", interfaces.Fields{
			"path": req.Path,
		})
		var err error
		targetStorage, err = storage.NewFileStorage(req.Path, s.logger)
		if err != nil {
			s.logger.Error("创建存储服务失败", interfaces.Fields{
				"error": err,
				"path":  req.Path,
			})
			return fmt.Errorf("创建存储服务失败: %v", err)
		}
		req.Storage = targetStorage
	}

	for _, folder := range s.config.SyncFolders {
		sourceFolderPath := filepath.Join(sourcePath, folder.Path)
		targetFolderPath := filepath.Join(req.Path, folder.Path)

		for _, redirect := range s.config.FolderRedirects {
			if redirect.ServerPath == folder.Path {
				targetFolderPath = filepath.Join(req.Path, redirect.ClientPath)
				break
			}
		}

		s.logger.Info("处理同步文件夹", interfaces.Fields{
			"source": sourceFolderPath,
			"target": targetFolderPath,
			"mode":   folder.SyncMode,
		})

		if err := os.MkdirAll(targetFolderPath, 0755); err != nil {
			s.logger.Error("创建目标目录失败", interfaces.Fields{
				"path":  targetFolderPath,
				"error": err,
			})
			continue
		}

		files, err := filepath.Glob(filepath.Join(sourceFolderPath, "*"))
		if err != nil {
			s.logger.Error("获取源文件列表失败", interfaces.Fields{
				"path":  sourceFolderPath,
				"error": err,
			})
			continue
		}

		for _, file := range files {
			relPath, err := filepath.Rel(sourcePath, file)
			if err != nil {
				s.logger.Error("获取相对路径失败", interfaces.Fields{
					"file":  file,
					"error": err,
				})
				continue
			}

			ignored := false
			for _, pattern := range s.config.IgnoreList {
				if matched, _ := filepath.Match(pattern, filepath.Base(file)); matched {
					ignored = true
					break
				}
			}
			if ignored {
				s.logger.Debug("忽略文件", interfaces.Fields{
					"file": relPath,
				})
				continue
			}
			s.logger.Info("测试:", interfaces.Fields{
				"relPath": relPath,
			})

			if err := s.syncFile(file, relPath, folder.SyncMode, targetStorage); err != nil {
				s.logger.Error("同步文件失败", interfaces.Fields{
					"file":  file,
					"error": err,
				})
				continue
			}
		}
	}

	return nil
}

// handleFileSync 根据同步模式处理文件
// TODO 可能要删除,因为这个方法好像没有用到
func (s *SyncService) handleFileSync(file string, info *interfaces.FileInfo, mode interfaces.SyncMode, targetStorage interfaces.Storage) error {
	s.logger.Debug("开始处理文件同步", interfaces.Fields{
		"file": file,
		"mode": mode,
		"hash": info.Hash,
		"size": info.Size,
	})

	// 应用重定向规则
	targetPath := file
	for _, redirect := range s.config.FolderRedirects {
		if strings.HasPrefix(file, redirect.ServerPath) {
			targetPath = strings.Replace(file, redirect.ServerPath, redirect.ClientPath, 1)
			s.logger.Debug("应用重定向规则", interfaces.Fields{
				"source": file,
				"target": targetPath,
				"rule":   fmt.Sprintf("%s -> %s", redirect.ServerPath, redirect.ClientPath),
			})
			break
		}
	}

	s.logger.Debug("开始同步文件", interfaces.Fields{
		"source": file,
		"target": targetPath,
		"mode":   mode,
	})

	// 读取源文件
	sourceFilePath := filepath.Join(s.config.SyncDir, file)
	fileData, err := os.ReadFile(sourceFilePath)
	if err != nil {
		s.logger.Error("读取源文件失败", interfaces.Fields{
			"source": sourceFilePath,
			"error":  err,
		})
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	s.logger.Debug("读取源文件成功", interfaces.Fields{
		"source": sourceFilePath,
		"size":   len(fileData),
	})

	// 确保目标目录存在
	targetDir := filepath.Dir(targetPath)
	targetBaseDir := targetStorage.(*storage.FileStorage).BaseDir()
	targetFullDir := filepath.Join(targetBaseDir, targetDir)

	s.logger.Debug("创建目标目录", interfaces.Fields{
		"targetDir": targetDir,
		"baseDir":   targetBaseDir,
		"fullDir":   targetFullDir,
	})

	if err := os.MkdirAll(targetFullDir, 0755); err != nil {
		s.logger.Error("创建目标目录失败", interfaces.Fields{
			"dir":   targetFullDir,
			"error": err,
		})
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	s.logger.Debug("目标目录创建成功", interfaces.Fields{
		"dir": targetFullDir,
	})

	// 根据同步模式处理文件
	switch mode {
	case interfaces.MirrorSync:
		s.logger.Debug("执行镜像同步", interfaces.Fields{
			"source": file,
			"target": targetPath,
		})

		if err := targetStorage.Save(targetPath, fileData); err != nil {
			s.logger.Error("保存目标文件失败", interfaces.Fields{
				"source": file,
				"target": targetPath,
				"error":  err,
			})
			return fmt.Errorf("保存目标文件失败: %v", err)
		}

		s.logger.Debug("镜像同步完成", interfaces.Fields{
			"source": file,
			"target": targetPath,
			"size":   len(fileData),
		})

	case interfaces.PackSync:
		s.logger.Debug("执行打包同步", interfaces.Fields{
			"source": file,
			"target": targetPath,
		})
		return s.packSync(targetPath, info, targetStorage)

	case interfaces.PushSync:
		s.logger.Debug("执行推送同步", interfaces.Fields{
			"source": file,
			"target": targetPath,
		})

		// 检查目标文件是否存在
		if targetStorage.Exists(targetPath) {
			s.logger.Debug("目标文件已存在，检查是否需要更新", interfaces.Fields{
				"target": targetPath,
			})

			// 如果目标文件存在，检查是否需要更新
			var targetData []byte
			if err := targetStorage.Load(targetPath, &targetData); err != nil {
				s.logger.Error("读取目标文件失败", interfaces.Fields{
					"target": targetPath,
					"error":  err,
				})
				return fmt.Errorf("读取目标文件失败: %v", err)
			}

			// 计算目标文件哈希
			targetHash := calculateFileHash(targetData)
			if targetHash == info.Hash {
				// 文件相同，不需要更新
				s.logger.Debug("文件未变更，跳过同步", interfaces.Fields{
					"file": targetPath,
					"hash": info.Hash,
				})
				return nil
			}

			s.logger.Debug("文件已变更，需要更新", interfaces.Fields{
				"file":    targetPath,
				"oldHash": targetHash,
				"newHash": info.Hash,
			})
		}

		// 保存文件
		if err := targetStorage.Save(targetPath, fileData); err != nil {
			s.logger.Error("保存目标文件失败", interfaces.Fields{
				"source": file,
				"target": targetPath,
				"error":  err,
			})
			return fmt.Errorf("保存目标文件失败: %v", err)
		}

		s.logger.Debug("推送同步完成", interfaces.Fields{
			"source": file,
			"target": targetPath,
			"size":   len(fileData),
		})

	case interfaces.AutoSync:
		// 自动同步：根据文件类型选择同步模式
		ext := filepath.Ext(file)
		s.logger.Debug("执行自动同步", interfaces.Fields{
			"source": file,
			"target": targetPath,
			"ext":    ext,
		})

		switch ext {
		case ".zip", ".rar", ".7z":
			return s.packSync(targetPath, info, targetStorage)
		case ".exe", ".dll", ".jar":
			// 镜像同步：直接覆盖目标文件
			if err := targetStorage.Save(targetPath, fileData); err != nil {
				s.logger.Error("保存目标文件失败", interfaces.Fields{
					"source": file,
					"target": targetPath,
					"error":  err,
				})
				return fmt.Errorf("保存目标文件失败: %v", err)
			}
			s.logger.Debug("自动同步完成", interfaces.Fields{
				"source": file,
				"target": targetPath,
				"size":   len(fileData),
			})
		default:
			// 默认使用推送同步
			if targetStorage.Exists(targetPath) {
				var targetData []byte
				if err := targetStorage.Load(targetPath, &targetData); err != nil {
					return fmt.Errorf("读取目标文件失败: %v", err)
				}

				targetHash := calculateFileHash(targetData)
				if targetHash == info.Hash {
					s.logger.Debug("文件未变更，跳过同步", interfaces.Fields{
						"file": targetPath,
						"hash": info.Hash,
					})
					return nil
				}
			}

			if err := targetStorage.Save(targetPath, fileData); err != nil {
				s.logger.Error("保存目标文件失败", interfaces.Fields{
					"source": file,
					"target": targetPath,
					"error":  err,
				})
				return fmt.Errorf("保存目标文件失败: %v", err)
			}
			s.logger.Debug("自动同步完成", interfaces.Fields{
				"source": file,
				"target": targetPath,
				"size":   len(fileData),
			})
		}
	}

	return nil
}

// ==================== 4. 同步模式实现 ====================

// mirrorSync 执行镜像同步
func (s *SyncService) mirrorSync(file string, info *interfaces.FileInfo, targetStorage interfaces.Storage) error {
	var data []byte
	if err := s.storage.Load(file, &data); err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	targetDir := filepath.Dir(file)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		s.logger.Error("创建目标目录失败", interfaces.Fields{
			"dir":   targetDir,
			"error": err,
		})
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	if err := targetStorage.Save(file, data); err != nil {
		return fmt.Errorf("保存目标文件失败: %v", err)
	}

	s.logger.Debug("文件同步完成", interfaces.Fields{
		"file": file,
		"mode": "mirror",
	})

	return nil
}

// pushSync 执行推送同步
func (s *SyncService) pushSync(file string, info *interfaces.FileInfo, targetStorage interfaces.Storage) error {
	var data []byte
	if err := s.storage.Load(file, &data); err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	targetDir := filepath.Dir(file)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		s.logger.Error("创建目标目录失败", interfaces.Fields{
			"dir":   targetDir,
			"error": err,
		})
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	if targetStorage.Exists(file) {
		var targetData []byte
		if err := targetStorage.Load(file, &targetData); err != nil {
			return fmt.Errorf("读取目标文件失败: %v", err)
		}

		targetHash := calculateFileHash(targetData)
		if targetHash == info.Hash {
			s.logger.Debug("文件未变更，跳过同步", interfaces.Fields{
				"file": file,
				"hash": info.Hash,
			})
			return nil
		}
	}

	if err := targetStorage.Save(file, data); err != nil {
		return fmt.Errorf("保存目标文件失败: %v", err)
	}

	s.logger.Debug("文件同步完成", interfaces.Fields{
		"file": file,
		"mode": "push",
	})

	return nil
}

// packSync 执行打包同步
func (s *SyncService) packSync(file string, info *interfaces.FileInfo, targetStorage interfaces.Storage) error {
	var data []byte
	if err := s.storage.Load(file, &data); err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	tempDir, err := os.MkdirTemp("", "synctools_pack_*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, filepath.Base(file))
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %v", err)
	}

	zipFile, err := os.Create(filepath.Join(tempDir, "temp.zip"))
	if err != nil {
		return fmt.Errorf("创建zip文件失败: %v", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	fileToZip, err := os.Open(tempFile)
	if err != nil {
		return fmt.Errorf("打开临时文件失败: %v", err)
	}
	defer fileToZip.Close()

	fileInfo, err := fileToZip.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	header, err := zip.FileInfoHeader(fileInfo)
	if err != nil {
		return fmt.Errorf("创建文件头失败: %v", err)
	}

	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("创建压缩文件失败: %v", err)
	}

	if _, err := io.Copy(writer, fileToZip); err != nil {
		return fmt.Errorf("写入压缩文件失败: %v", err)
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("关闭zip写入器失败: %v", err)
	}

	zipData, err := os.ReadFile(filepath.Join(tempDir, "temp.zip"))
	if err != nil {
		return fmt.Errorf("读取压缩文件失败: %v", err)
	}

	if err := targetStorage.Save(file, zipData); err != nil {
		return fmt.Errorf("保存压缩文件失败: %v", err)
	}

	s.logger.Info("文件打包完成", interfaces.Fields{
		"file": file,
		"size": len(zipData),
	})

	return nil
}

// autoSync 执行自动同步
func (s *SyncService) autoSync(file string, info *interfaces.FileInfo, targetStorage interfaces.Storage) error {
	ext := filepath.Ext(file)
	switch ext {
	case ".zip", ".rar", ".7z":
		return s.packSync(file, info, targetStorage)
	case ".exe", ".dll", ".jar":
		return s.mirrorSync(file, info, targetStorage)
	default:
		return s.pushSync(file, info, targetStorage)
	}
}

// manualSync 执行手动同步
func (s *SyncService) manualSync(file string, info *interfaces.FileInfo) error {
	s.logger.Info("等待用户确认同步", interfaces.Fields{
		"file": file,
		"hash": info.Hash,
		"size": info.Size,
	})
	return nil
}

// ==================== 5. 网络服务管理 ====================

// StartServer 启动网络服务器
func (s *SyncService) StartServer() error {
	s.logger.Info("服务操作", interfaces.Fields{
		"action": "start_server",
	})

	if s.server == nil {
		s.logger.Debug("创建网络服务器", interfaces.Fields{})
		s.server = network.NewServer(s.config, s, s.logger)
	}

	if err := s.server.Start(); err != nil {
		s.running = false
		s.logger.Error("启动服务器失败", interfaces.Fields{
			"error": err,
		})
		return err
	}

	s.running = true
	s.logger.Info("服务器已启动", interfaces.Fields{})
	return nil
}

// StopServer 停止网络服务器
func (s *SyncService) StopServer() error {
	s.logger.Info("服务操作", interfaces.Fields{
		"action": "stop_server",
	})

	if s.server == nil {
		s.running = false
		s.logger.Debug("服务器未启动", interfaces.Fields{})
		return nil
	}

	if err := s.server.Stop(); err != nil {
		s.logger.Error("停止服务器失败", interfaces.Fields{
			"error": err,
		})
		return err
	}

	s.server = nil
	s.running = false
	s.logger.Info("服务器已停止", interfaces.Fields{})
	return nil
}

// SetServer 实现服务器设置
func (s *SyncService) SetServer(server interfaces.NetworkServer) {
	if s.server != nil {
		s.StopServer()
	}
	s.server = server
}

// GetNetworkServer 获取网络服务器实例
func (s *SyncService) GetNetworkServer() interfaces.NetworkServer {
	return s.server
}

// ==================== 6. 状态管理 ====================

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

// SetProgressCallback 实现进度回调
func (s *SyncService) SetProgressCallback(callback func(progress *interfaces.Progress)) {
	s.progressCallback = callback
}

// ==================== 7. 工具方法 ====================

// calculateFileHash 计算文件哈希值
func calculateFileHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
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
	for _, folder := range s.config.SyncFolders {
		if strings.HasPrefix(file, folder.Path) {
			return folder.SyncMode
		}
	}
	return interfaces.AutoSync
}
