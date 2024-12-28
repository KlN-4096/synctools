package client

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"crypto/md5"
	"encoding/hex"
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
	s.SetStatus("同步中")
	s.Logger.Info("开始同步", interfaces.Fields{
		"source_path": sourcePath,
	})

	// 获取本地文件列表和MD5
	localFiles, err := s.getLocalFilesWithMD5(sourcePath)
	if err != nil {
		return fmt.Errorf("获取本地文件列表失败: %v", err)
	}
	s.Logger.Info("获取本地文件列表", interfaces.Fields{
		"count": len(localFiles),
		"files": localFiles,
	})

	// 获取服务器文件列表和MD5
	serverFiles, err := s.getServerFilesWithMD5()
	if err != nil {
		return fmt.Errorf("获取服务器文件列表失败: %v", err)
	}
	s.Logger.Info("获取服务器文件列表", interfaces.Fields{
		"count": len(serverFiles),
		"files": serverFiles,
	})

	// 分析差异并同步
	var downloadCount, deleteCount, skipCount, failedCount int
	mode := s.GetSyncMode("") // 获取默认同步模式

	s.Logger.Info("开始文件对比", interfaces.Fields{
		"mode": mode,
	})

	// 处理本地多余的文件
	if mode == interfaces.MirrorSync {
		for localPath := range localFiles {
			if _, exists := serverFiles[localPath]; !exists {
				s.Logger.Info("发现本地多余文件", interfaces.Fields{
					"file": localPath,
					"md5":  localFiles[localPath],
				})
				fullPath := filepath.Join(sourcePath, localPath)
				s.Logger.Debug("删除本地多余文件", interfaces.Fields{
					"file": localPath,
				})
				if err := os.Remove(fullPath); err != nil {
					s.Logger.Error("删除本地文件失败", interfaces.Fields{
						"file":  localPath,
						"error": err,
					})
					failedCount++
					continue
				}
				deleteCount++
			}
		}
	}

	// 从服务器下载文件
	for serverPath, serverMD5 := range serverFiles {
		localMD5, exists := localFiles[serverPath]
		needDownload := true

		if exists {
			s.Logger.Info("对比文件MD5", interfaces.Fields{
				"file":       serverPath,
				"local_md5":  localMD5,
				"server_md5": serverMD5,
				"match":      localMD5 == serverMD5,
			})

			if localMD5 == serverMD5 {
				needDownload = false
				skipCount++
				s.Logger.Debug("文件无需更新", interfaces.Fields{
					"file": serverPath,
					"md5":  serverMD5,
				})
			} else {
				s.Logger.Info("文件需要更新", interfaces.Fields{
					"file":       serverPath,
					"local_md5":  localMD5,
					"server_md5": serverMD5,
				})
			}
		} else {
			s.Logger.Info("发现新文件", interfaces.Fields{
				"file": serverPath,
				"md5":  serverMD5,
			})
		}

		if needDownload {
			fullPath := filepath.Join(sourcePath, serverPath)
			// 确保目标目录存在
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				s.Logger.Error("创建目录失败", interfaces.Fields{
					"path":  filepath.Dir(fullPath),
					"error": err,
				})
				failedCount++
				continue
			}

			// 发送下载请求
			req := &interfaces.SyncRequest{
				Mode:      mode,
				Direction: interfaces.DirectionPull,
				Path:      serverPath,
			}

			s.Logger.Info("开始下载文件", interfaces.Fields{
				"file": serverPath,
				"mode": mode,
			})

			if err := s.downloadFile(req, fullPath); err != nil {
				s.Logger.Error("下载文件失败", interfaces.Fields{
					"file":  serverPath,
					"error": err,
				})
				failedCount++
				continue
			}

			// 验证下载后的文件MD5
			downloadedMD5, err := s.calculateFileMD5(fullPath)
			if err != nil {
				s.Logger.Error("计算下载文件MD5失败", interfaces.Fields{
					"file":  serverPath,
					"error": err,
				})
			} else {
				s.Logger.Info("验证下载文件MD5", interfaces.Fields{
					"file":           serverPath,
					"server_md5":     serverMD5,
					"downloaded_md5": downloadedMD5,
					"match":          downloadedMD5 == serverMD5,
				})
			}

			downloadCount++
			s.Logger.Debug("文件下载成功", interfaces.Fields{
				"file": serverPath,
				"md5":  serverMD5,
			})
		}
	}

	s.Logger.Info("同步完成", interfaces.Fields{
		"total_local":  len(localFiles),
		"total_server": len(serverFiles),
		"downloaded":   downloadCount,
		"deleted":      deleteCount,
		"skipped":      skipCount,
		"failed":       failedCount,
		"source_path":  sourcePath,
		"sync_mode":    mode,
	})

	s.SetStatus("同步完成")
	return nil
}

// downloadFile 从服务器下载文件
func (s *ClientSyncService) downloadFile(req *interfaces.SyncRequest, destPath string) error {
	// 发送下载请求
	if err := s.networkClient.SendData(req); err != nil {
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

	if err := s.networkClient.ReceiveFile(filepath.Dir(destPath), progress); err != nil {
		return fmt.Errorf("接收文件失败: %v", err)
	}

	return nil
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

	if err := s.networkClient.SendData(req); err != nil {
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

	if err := s.networkClient.SendData(req); err != nil {
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
	files := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			// 检查是否需要忽略
			if s.IsIgnored(relPath) {
				s.Logger.Debug("忽略文件", interfaces.Fields{
					"file": relPath,
				})
				return nil
			}
			// 计算MD5
			md5, err := s.calculateFileMD5(path)
			if err != nil {
				return err
			}
			files[relPath] = md5
		}
		return nil
	})
	return files, err
}

// getServerFilesWithMD5 获取服务器文件列表和MD5
func (s *ClientSyncService) getServerFilesWithMD5() (map[string]string, error) {
	req := &interfaces.SyncRequest{
		Mode:      interfaces.MirrorSync,
		Direction: interfaces.DirectionPull,
	}

	if err := s.networkClient.SendData(req); err != nil {
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
		"count": len(resp.MD5Map),
	})

	return resp.MD5Map, nil
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
