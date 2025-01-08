/*
文件作用:
- 实现客户端同步服务的入口
- 管理连接状态
- 调用基础同步服务

主要功能:
1. 服务初始化和连接管理
2. 同步入口
*/

package client

import (
	"fmt"
	"os"
	"path/filepath"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/network/client"
	"synctools/codes/pkg/service/base"
)

// ClientSyncService 客户端同步服务实现
type ClientSyncService struct {
	*base.BaseSyncService
	networkClient *client.NetworkClient
	syncBase      *base.ClientSyncBase
	onConnLost    func() // 添加回调字段
}

// NewClientSyncService 创建客户端同步服务
func NewClientSyncService(config *interfaces.Config, logger interfaces.Logger, storage interfaces.Storage) *ClientSyncService {
	baseService := base.NewBaseSyncService(config, logger, storage)
	srv := &ClientSyncService{
		BaseSyncService: baseService,
	}
	srv.networkClient = client.NewNetworkClient(logger, srv)
	srv.syncBase = base.NewClientSyncBase(baseService, srv.networkClient)
	return srv
}

// Connect 连接服务器
func (s *ClientSyncService) Connect(addr, port string) error {
	// 连接服务器
	if err := s.networkClient.Connect(addr, port); err != nil {
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	// 标记服务开始运行
	s.Start()
	s.SetStatus("已连接")
	return nil
}

// Disconnect 断开连接
func (s *ClientSyncService) Disconnect() error {
	// 断开网络连接
	if err := s.networkClient.Disconnect(); err != nil {
		return fmt.Errorf("断开连接失败: %v", err)
	}

	// 停止服务
	s.Stop()
	s.SetStatus("未连接")
	return nil
}

// IsConnected 检查是否已连接
func (s *ClientSyncService) IsConnected() bool {
	return s.networkClient != nil && s.networkClient.IsConnected()
}

// SetConnectionLostCallback 设置连接丢失回调
func (s *ClientSyncService) SetConnectionLostCallback(callback func()) {
	s.onConnLost = callback
	// 将回调传递给 networkClient
	s.networkClient.SetConnectionLostCallback(func() {
		if s.onConnLost != nil {
			s.onConnLost()
		}
	})
}

// SyncFiles 同步文件
func (s *ClientSyncService) SyncFiles(sourcePath string) error {
	// 设置同步状态为开始
	s.networkClient.SetSyncing(true)
	defer s.networkClient.SetSyncing(false) // 确保同步结束时重置状态

	s.SetStatus("同步中")

	// 如果没有需要同步的文件且没有需要删除的文件,直接返回
	if len(filesToSync) == 0 && len(filesToDelete) == 0 {
		s.SetStatus("无需同步")
		s.Disconnect()
		return nil
	}

	var totalDownloadCount, totalDeleteCount, totalFailedCount int

	// 处理需要同步的文件
	for _, syncPath := range filesToSync {
		folder := filepath.Dir(syncPath)
		serverPath := filepath.Base(syncPath)

		// 获取当前文件夹的同步模式
		var mode interfaces.SyncMode
		// 统一使用斜杠作为分隔符进行比较
		folderSlash := filepath.ToSlash(folder)
		for _, folderConfig := range s.Config.SyncFolders {
			if folderConfig.Path == folderSlash {
				mode = folderConfig.SyncMode
				break
			}
		}

		// 构建本地路径
		localFolderPath := filepath.Join(sourcePath, folder)

		// 发送下载请求
		var reqPath string
		var fullPath string
		if s.syncBase.IsSingleFile(folder) {
			// 如果是单文件
			reqPath = folder
			fullPath = localFolderPath
		} else {
			// 如果是文件夹中的文件
			reqPath = filepath.Join(folder, serverPath)
			fullPath = filepath.Join(localFolderPath, serverPath)
		}

		req := &interfaces.SyncRequest{
			Mode:      mode,
			Direction: interfaces.DirectionPull,
			Path:      reqPath,
		}

		s.Logger.Info("开始下载文件", interfaces.Fields{
			"folder": folder,
			"file":   serverPath,
			"mode":   mode,
		})

		if err := s.syncBase.DownloadFile(req, fullPath, sourcePath, mode); err != nil {
			s.Logger.Error("下载文件失败", interfaces.Fields{
				"folder": folder,
				"file":   serverPath,
				"error":  err,
			})
			totalFailedCount++
			continue
		}

		totalDownloadCount++
		s.Logger.Debug("文件下载成功", interfaces.Fields{
			"folder": folder,
			"file":   serverPath,
		})
	}

	// 同步完成后处理需要删除的文件
	for folder, files := range filesToDelete {
		// 获取文件夹的同步模式
		var mode interfaces.SyncMode
		// 统一使用斜杠作为分隔符进行比较
		folderSlash := filepath.ToSlash(folder)
		for _, folderConfig := range s.Config.SyncFolders {
			if folderConfig.Path == folderSlash {
				mode = folderConfig.SyncMode
				break
			}
		}

		// 只在mirror模式下执行删除
		if mode == interfaces.MirrorSync && len(files) > 0 {
			s.Logger.Info("开始删除多余文件", interfaces.Fields{
				"folder": folder,
				"count":  len(files),
			})

			for file := range files {
				fullPath := filepath.Join(sourcePath, folder, file)
				if err := os.Remove(fullPath); err != nil {
					if !os.IsNotExist(err) {
						// 只记录非文件不存在的错误
						s.Logger.Error("删除文件失败", interfaces.Fields{
							"folder": folder,
							"file":   file,
							"error":  err,
						})
						totalFailedCount++
					}
				} else {
					totalDeleteCount++
					s.Logger.Info("成功删除文件", interfaces.Fields{
						"folder": folder,
						"file":   file,
					})
				}
			}
		}
	}

	s.Logger.Info("同步完成", interfaces.Fields{
		"downloaded": totalDownloadCount,
		"deleted":    totalDeleteCount,
		"skipped":    ignoredFiles,
		"failed":     totalFailedCount,
	})

	// 同步完成后断开连接
	if err := s.Disconnect(); err != nil {
		s.Logger.Error("断开连接失败", interfaces.Fields{
			"error": err,
		})
	}

	return nil
}

// SaveServerConfig 保存服务器配置到临时目录
func (s *ClientSyncService) SaveServerConfig(config *interfaces.Config) error {
	// 确保configs/temp目录存在
	configDir := "temp"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	// 保存到临时目录
	configPath := filepath.Join(configDir, "server_config.json")
	if err := s.Storage.Save(configPath, config); err != nil {
		return fmt.Errorf("保存服务器配置失败: %v", err)
	}

	return nil
}

// LoadServerConfig 从临时目录加载服务器配置
func (s *ClientSyncService) LoadServerConfig() (*interfaces.Config, error) {
	configPath := filepath.Join("temp", "server_config.json")

	var config interfaces.Config
	if err := s.Storage.Load(configPath, &config); err != nil {
		return nil, fmt.Errorf("加载服务器配置失败: %v", err)
	}

	return &config, nil
}

// GetLocalFilesWithMD5 获取本地文件的MD5信息
func (s *ClientSyncService) GetLocalFilesWithMD5(dir string) (map[string]string, error) {
	return s.syncBase.GetLocalFilesWithMD5(dir)
}

// CompareMD5 比较本地和服务器文件的MD5，返回需要同步的文件信息
func (s *ClientSyncService) CompareMD5(
	localFiles map[string]string,
	serverFiles map[string]string,
) ([]string, map[string]struct{}, int, error) {
	var filesToSync []string
	filesToDelete := make(map[string]struct{})
	var ignoredFiles int

	// 检查本地多余的文件
	for localPath := range localFiles {
		// 获取重定向后的路径
		redirectedPath := s.syncBase.GetRedirectedPathByConfig(localPath, false)
		if _, exists := serverFiles[redirectedPath]; !exists && !s.syncBase.IsIgnoredFile(redirectedPath) {
			s.Logger.Debug("发现本地多余文件", interfaces.Fields{
				"file":           localPath,
				"redirectedPath": redirectedPath,
			})
			filesToDelete[localPath] = struct{}{}
		}
	}

	// 检查需要同步的服务器文件
	for serverPath, serverMD5 := range serverFiles {
		// 获取重定向后的路径
		redirectedPath := s.syncBase.GetRedirectedPathByConfig(serverPath, true)

		// 检查文件是否需要忽略
		if s.syncBase.IsIgnoredFile(redirectedPath) {
			s.Logger.Debug("忽略文件", interfaces.Fields{
				"file":           serverPath,
				"redirectedPath": redirectedPath,
				"md5":            serverMD5,
			})
			ignoredFiles++
			continue
		}

		localMD5, exists := localFiles[redirectedPath]
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
