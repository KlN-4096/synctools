package client

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/network/client"
	"synctools/codes/pkg/service/base"
)

// ClientSyncService 客户端同步服务实现
type ClientSyncService struct {
	*base.BaseSyncService
	networkClient *client.NetworkClient
	onConnLost    func() // 添加回调字段
}

// NewClientSyncService 创建客户端同步服务
func NewClientSyncService(config *interfaces.Config, logger interfaces.Logger, storage interfaces.Storage) *ClientSyncService {
	base := base.NewBaseSyncService(config, logger, storage)
	srv := &ClientSyncService{
		BaseSyncService: base,
	}
	srv.networkClient = client.NewNetworkClient(logger, srv)
	return srv
}

// Connect 连接服务器(指向client_network.go)
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

// SyncFiles 同步文件
func (s *ClientSyncService) SyncFiles(sourcePath string) error {
	// 设置同步状态为开始
	s.networkClient.SetSyncing(true)
	defer s.networkClient.SetSyncing(false) // 确保同步结束时重置状态

	s.SetStatus("同步中")
	s.Logger.Info("开始同步", interfaces.Fields{
		"source_path": sourcePath,
	})

	// 获取同步文件夹列表
	syncFolders := s.GetSyncFolders()
	if len(syncFolders) == 0 {
		s.Logger.Info("没有配置同步文件夹", interfaces.Fields{
			"source_path": sourcePath,
		})
		return nil
	}

	// 先获取所有文件夹的 MD5 信息
	allServerFiles := make(map[string]map[string]string)
	allLocalFiles := make(map[string]map[string]string)

	for _, folder := range syncFolders {
		// 获取本地文件列表和MD5
		localFolderPath := filepath.Join(sourcePath, folder)
		localFiles, err := s.getLocalFilesWithMD5(localFolderPath)
		if err != nil {
			s.Logger.Error("获取本地文件列表失败", interfaces.Fields{
				"folder": folder,
				"error":  err,
			})
			continue
		}
		allLocalFiles[folder] = localFiles

		// 获取服务器文件列表和MD5
		serverFiles, err := s.getServerFilesWithMD5WithFolder(folder)
		if err != nil {
			s.Logger.Error("获取服务器文件列表失败", interfaces.Fields{
				"folder": folder,
				"error":  err,
			})
			continue
		}
		allServerFiles[folder] = serverFiles
	}

	// 计算需要同步的文件数量
	var totalFiles, needSyncFiles int
	var filesToSync []string
	for folder, serverFiles := range allServerFiles {
		localFiles := allLocalFiles[folder]
		for serverPath, serverMD5 := range serverFiles {
			totalFiles++
			localMD5, exists := localFiles[serverPath]
			if !exists || localMD5 != serverMD5 {
				needSyncFiles++
				filesToSync = append(filesToSync, filepath.Join(folder, serverPath))
			}
		}
	}

	s.Logger.Info("文件对比完成", interfaces.Fields{
		"total_files": totalFiles,
		"need_sync":   needSyncFiles,
		"files":       filesToSync,
	})

	// 如果没有需要同步的文件,直接返回
	if needSyncFiles == 0 {
		s.SetStatus("无需同步")
		return nil
	}

	var totalDownloadCount, totalDeleteCount, totalSkipCount, totalFailedCount int

	// 遍历每个同步文件夹
	for _, folder := range syncFolders {
		// 获取当前文件夹的同步模式
		var mode interfaces.SyncMode
		for _, folderConfig := range s.Config.SyncFolders {
			if folderConfig.Path == folder {
				mode = folderConfig.SyncMode
				break
			}
		}

		s.Logger.Info("开始同步文件夹", interfaces.Fields{
			"folder": folder,
			"mode":   mode,
		})

		// 构建本地和服务器的文件夹路径
		localFolderPath := filepath.Join(sourcePath, folder)
		localFiles := allLocalFiles[folder]
		serverFiles := allServerFiles[folder]

		// 处理本地多余的文件
		if mode == interfaces.MirrorSync {
			for localPath := range localFiles {
				if _, exists := serverFiles[localPath]; !exists {
					s.Logger.Info("发现本地多余文件", interfaces.Fields{
						"folder": folder,
						"file":   localPath,
						"md5":    localFiles[localPath],
					})
					fullPath := filepath.Join(localFolderPath, localPath)
					s.Logger.Debug("删除本地多余文件", interfaces.Fields{
						"file": localPath,
					})
					if err := os.Remove(fullPath); err != nil {
						s.Logger.Error("删除本地文件失败", interfaces.Fields{
							"file":  localPath,
							"error": err,
						})
						totalFailedCount++
						continue
					}
					totalDeleteCount++
				}
			}
		}

		// 在mirror模式下处理本地多余的目录
		if mode == interfaces.MirrorSync {
			// 获取服务器目录列表
			req := &interfaces.SyncRequest{
				Mode:      interfaces.MirrorSync,
				Direction: interfaces.DirectionPull,
				Path:      folder,
			}

			if err := s.networkClient.SendData("list_request", req); err != nil {
				s.Logger.Error("获取服务器目录列表失败", interfaces.Fields{
					"folder": folder,
					"error":  err,
				})
			} else {
				var resp struct {
					Success bool     `json:"success"`
					Files   []string `json:"files"`
					Dirs    []string `json:"dirs"`
				}

				if err := s.networkClient.ReceiveData(&resp); err != nil {
					s.Logger.Error("接收服务器目录列表失败", interfaces.Fields{
						"folder": folder,
						"error":  err,
					})
				} else if resp.Success {
					// 将服务器目录转换为map便于查找
					serverDirs := make(map[string]bool)
					for _, dir := range resp.Dirs {
						serverDirs[dir] = true
					}

					// 获取本地目录列表
					var localDirs []string
					err := filepath.Walk(localFolderPath, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						if info.IsDir() {
							relPath, err := filepath.Rel(localFolderPath, path)
							if err != nil {
								return err
							}
							if relPath != "." {
								localDirs = append(localDirs, relPath)
							}
						}
						return nil
					})

					if err != nil {
						s.Logger.Error("获取本地目录列表失败", interfaces.Fields{
							"folder": folder,
							"error":  err,
						})
					} else {
						// 从最深层的目录开始删除
						for i := len(localDirs) - 1; i >= 0; i-- {
							localDir := localDirs[i]
							if !serverDirs[localDir] {
								fullPath := filepath.Join(localFolderPath, localDir)
								s.Logger.Info("发现本地多余目录", interfaces.Fields{
									"folder": folder,
									"dir":    localDir,
								})

								// 尝试删除目录（如果不为空会失败）
								if err := os.Remove(fullPath); err != nil {
									if !os.IsNotExist(err) {
										s.Logger.Error("删除目录失败", interfaces.Fields{
											"folder": folder,
											"dir":    localDir,
											"error":  err,
										})
										totalFailedCount++
									}
								} else {
									s.Logger.Debug("删除目录成功", interfaces.Fields{
										"folder": folder,
										"dir":    localDir,
									})
									totalDeleteCount++
								}
							}
						}
					}
				}
			}
		}

		// 从服务器下载MD5信息
		for serverPath, serverMD5 := range serverFiles {
			localMD5, exists := localFiles[serverPath]
			needDownload := true

			if exists {
				s.Logger.Info("对比文件MD5", interfaces.Fields{
					"folder":     folder,
					"file":       serverPath,
					"local_md5":  localMD5,
					"server_md5": serverMD5,
					"match":      localMD5 == serverMD5,
				})

				if localMD5 == serverMD5 {
					needDownload = false
					totalSkipCount++
					s.Logger.Debug("文件无需更新", interfaces.Fields{
						"folder": folder,
						"file":   serverPath,
						"md5":    serverMD5,
					})
				} else {
					s.Logger.Info("文件需要更新", interfaces.Fields{
						"folder":     folder,
						"file":       serverPath,
						"local_md5":  localMD5,
						"server_md5": serverMD5,
					})
				}
			} else {
				s.Logger.Info("发现新文件", interfaces.Fields{
					"folder": folder,
					"file":   serverPath,
					"md5":    serverMD5,
				})
			}

			if needDownload {
				fullPath := filepath.Join(localFolderPath, serverPath)
				// 确保目标目录存在
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					s.Logger.Error("创建目录失败", interfaces.Fields{
						"path":  filepath.Dir(fullPath),
						"error": err,
					})
					totalFailedCount++
					continue
				}

				// 发送下载请求
				req := &interfaces.SyncRequest{
					Mode:      mode,
					Direction: interfaces.DirectionPull,
					Path:      filepath.Join(folder, serverPath),
				}

				s.Logger.Info("开始下载文件", interfaces.Fields{
					"folder": folder,
					"file":   serverPath,
					"mode":   mode,
				})

				if err := s.downloadFile(req, fullPath, mode); err != nil {
					s.Logger.Error("下载文件失败", interfaces.Fields{
						"folder": folder,
						"file":   serverPath,
						"error":  err,
					})
					totalFailedCount++
					continue
				}

				// 验证下载后的文件MD5
				downloadedMD5, err := s.calculateFileMD5(fullPath)
				if err != nil {
					s.Logger.Error("计算下载文件MD5失败", interfaces.Fields{
						"folder": folder,
						"file":   serverPath,
						"error":  err,
					})
				} else {
					s.Logger.Info("验证下载文件MD5", interfaces.Fields{
						"folder":         folder,
						"file":           serverPath,
						"server_md5":     serverMD5,
						"downloaded_md5": downloadedMD5,
						"match":          downloadedMD5 == serverMD5,
					})
				}

				totalDownloadCount++
				s.Logger.Debug("文件下载成功", interfaces.Fields{
					"folder": folder,
					"file":   serverPath,
					"md5":    serverMD5,
				})
			}
		}

		s.Logger.Info("同步完成", interfaces.Fields{
			"downloaded":  totalDownloadCount,
			"deleted":     totalDeleteCount,
			"skipped":     totalSkipCount,
			"failed":      totalFailedCount,
			"source_path": sourcePath,
			"sync_mode":   mode,
		})
	}

	s.SetStatus("同步完成")

	// 同步完成后断开连接
	if err := s.Disconnect(); err != nil {
		s.Logger.Error("断开连接失败", interfaces.Fields{
			"error": err,
		})
	}

	return nil
}

// downloadFile 从服务器下载文件
func (s *ClientSyncService) downloadFile(req *interfaces.SyncRequest, destPath string, mode interfaces.SyncMode) error {
	// 发送下载请求
	if err := s.networkClient.SendData("file_request", req); err != nil {
		return fmt.Errorf("发送下载请求失败: %v", err)
	}

	// 接收文件
	progress := make(chan interfaces.Progress, 1)
	defer close(progress)

	go func() {
		for p := range progress {
			s.ReportProgress(&p)
		}
	}()

	// 如果是打包同步模式，需要特殊处理
	if mode == interfaces.PackSync {
		s.Logger.Info("打包同步模式", interfaces.Fields{
			"path": destPath,
		})

		// 创建临时目录
		tempDir, err := os.MkdirTemp("", "synctools_pack_*")
		if err != nil {
			return fmt.Errorf("创建临时目录失败: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// 先将压缩包下载到临时目录
		if err := s.networkClient.ReceiveFile(tempDir, progress); err != nil {
			return fmt.Errorf("接收压缩包失败: %v", err)
		}

		// 获取下载的压缩包路径
		packFile := filepath.Join(tempDir, filepath.Base(req.Path))

		// 获取目标目录（移除.zip后缀）
		targetDir := filepath.Dir(destPath)

		// 解压文件
		if err := s.unpackFile(packFile, targetDir); err != nil {
			return fmt.Errorf("解压文件失败: %v", err)
		}

		s.Logger.Info("解压完成", interfaces.Fields{
			"pack": packFile,
			"dest": targetDir,
		})
		return nil
	}

	// 获取重定向后的路径
	redirectedPath := s.getRedirectedPath(req.Path, filepath.Dir(destPath))

	// 确保目标目录存在
	targetDir := filepath.Dir(redirectedPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 接收文件
	if err := s.networkClient.ReceiveFile(targetDir, progress); err != nil {
		return fmt.Errorf("接收文件失败: %v", err)
	}

	return nil
}

// unpackFile 解压文件
func (s *ClientSyncService) unpackFile(packFile, destPath string) error {
	s.Logger.Info("开始解压文件", interfaces.Fields{
		"pack": packFile,
		"dest": destPath,
	})

	// 确保目标目录存在
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 打开压缩包
	reader, err := zip.OpenReader(packFile)
	if err != nil {
		return fmt.Errorf("打开压缩包失败: %v", err)
	}
	defer reader.Close()

	// 遍历压缩包中的文件
	for _, file := range reader.File {
		// 获取文件的重定向路径
		targetPath := s.getRedirectedPath(file.Name, destPath)

		if file.FileInfo().IsDir() {
			// 创建目录
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("创建目录失败: %v", err)
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %v", err)
		}

		// 创建目标文件
		target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("创建目标文件失败: %v", err)
		}

		// 打开压缩文件
		source, err := file.Open()
		if err != nil {
			target.Close()
			return fmt.Errorf("打开压缩文件失败: %v", err)
		}

		// 复制文件内容
		if _, err := io.Copy(target, source); err != nil {
			target.Close()
			source.Close()
			return fmt.Errorf("复制文件内容失败: %v", err)
		}

		target.Close()
		source.Close()

		s.Logger.Debug("解压文件完成", interfaces.Fields{
			"file": targetPath,
		})
	}

	return nil
}

// getRedirectedPath 获取重定向后的路径
func (s *ClientSyncService) getRedirectedPath(originalPath, destPath string) string {
	// 获取当前配置
	config := s.GetCurrentConfig()
	if config == nil || len(config.FolderRedirects) == 0 {
		return filepath.Join(destPath, originalPath)
	}

	// 检查是否需要重定向
	for _, redirect := range config.FolderRedirects {
		if strings.HasPrefix(originalPath, redirect.ServerPath) {
			// 替换路径前缀
			relativePath := strings.TrimPrefix(originalPath, redirect.ServerPath)
			targetDir := filepath.Join(destPath, redirect.ClientPath)
			return filepath.Join(targetDir, relativePath)
		}
	}

	return filepath.Join(destPath, originalPath)
}

// 辅助方法

// getLocalFiles 获取本地文件列表
func (s *ClientSyncService) getLocalFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		return nil
	})
	return files, err
}

// getServerFiles 获取服务器文件列表
func (s *ClientSyncService) getServerFiles() ([]string, error) {
	req := &interfaces.SyncRequest{
		Mode:      interfaces.MirrorSync,
		Direction: interfaces.DirectionPull,
	}

	if err := s.networkClient.SendData("list_request", req); err != nil {
		return nil, err
	}

	var resp struct {
		Success bool     `json:"success"`
		Files   []string `json:"files"`
	}

	if err := s.networkClient.ReceiveData(&resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("获取服务器文件列表失败")
	}

	return resp.Files, nil
}

// fileExists 检查文件是否在列表中
func (s *ClientSyncService) fileExists(files []string, target string) bool {
	for _, file := range files {
		if file == target {
			return true
		}
	}
	return false
}

// deleteRemoteFile 删除远程文件
func (s *ClientSyncService) deleteRemoteFile(file string) error {
	req := &interfaces.SyncRequest{
		Mode:      interfaces.MirrorSync,
		Direction: interfaces.DirectionPush,
		Path:      file,
	}

	if err := s.networkClient.SendData("delete_request", req); err != nil {
		return err
	}

	return nil
}

// 添加设置回调的方法
func (s *ClientSyncService) SetConnectionLostCallback(callback func()) {
	s.onConnLost = callback
	// 将回调传递给 networkClient
	s.networkClient.SetConnectionLostCallback(func() {
		if s.onConnLost != nil {
			s.onConnLost()
		}
	})
}

// getLocalFilesWithMD5 获取本地文件列表和MD5
func (s *ClientSyncService) getLocalFilesWithMD5(dir string) (map[string]string, error) {
	// 检查是否是打包模式
	for _, folder := range s.Config.SyncFolders {
		if strings.HasSuffix(dir, folder.Path) && folder.SyncMode == interfaces.PackSync {
			// 打包模式返回空映射
			return make(map[string]string), nil
		}
	}

	// 确保目录存在
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %v", err)
	}

	files := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// 如果dir是一个文件路径，我们需要特殊处理
			var relPath string
			if info.IsDir() || filepath.Dir(dir) == dir {
				// 正常的目录遍历情况
				relPath, err = filepath.Rel(dir, path)
			} else {
				// dir是一个文件路径的情况
				relPath = filepath.Base(path)
			}
			if err != nil {
				return err
			}

			// 检查是否需要忽略
			if s.IsIgnored(relPath) {
				return nil
			}

			// 计算文件MD5
			md5hash, err := s.calculateFileMD5(path)
			if err != nil {
				return err
			}

			files[relPath] = md5hash
		}
		return nil
	})

	return files, err
}

// calculateFileMD5 计算文件的MD5值
func (s *ClientSyncService) calculateFileMD5(path string) (string, error) {
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

// getServerFilesWithMD5WithFolder 获取指定文件夹的服务器文件列表和MD5
func (s *ClientSyncService) getServerFilesWithMD5WithFolder(folder string) (map[string]string, error) {
	req := &interfaces.SyncRequest{
		Mode:      interfaces.MirrorSync,
		Direction: interfaces.DirectionPull,
		Path:      folder,
	}

	if err := s.networkClient.SendData("md5_request", req); err != nil {
		return nil, fmt.Errorf("发送获取MD5请求失败: %v", err)
	}

	var resp struct {
		Success bool              `json:"success"`
		MD5Map  map[string]string `json:"md5_map"` // path -> md5
		Message string            `json:"message"`
	}

	if err := s.networkClient.ReceiveData(&resp); err != nil {
		return nil, fmt.Errorf("接收MD5列表失败: %v", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("获取服务器MD5列表失败: %s", resp.Message)
	}

	s.Logger.Debug("获取服务器MD5列表成功", interfaces.Fields{
		"folder": folder,
		"count":  len(resp.MD5Map),
	})

	return resp.MD5Map, nil
}

// GetSyncFolders 获取同步文件夹列表
func (s *ClientSyncService) GetSyncFolders() []string {
	config := s.GetCurrentConfig()
	if config == nil || len(config.SyncFolders) == 0 {
		return []string{""} // 如果没有配置，返回空字符串表示根目录
	}

	folders := make([]string, len(config.SyncFolders))
	for i, folder := range config.SyncFolders {
		folders[i] = folder.Path
	}
	return folders
}
