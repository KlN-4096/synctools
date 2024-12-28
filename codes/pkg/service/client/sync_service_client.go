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
	serverAddr    string
	serverPort    string
}

// NewClientSyncService 创建客户端同步服务
func NewClientSyncService(config *interfaces.Config, logger interfaces.Logger, storage interfaces.Storage) *ClientSyncService {
	base := base.NewBaseSyncService(config, logger, storage)
	srv := &ClientSyncService{
		BaseSyncService: base,
		serverAddr:      "localhost",
		serverPort:      "25000",
	}
	srv.networkClient = client.NewNetworkClient(logger, srv)
	return srv
}

// Connect 连接到服务器
func (s *ClientSyncService) Connect(addr, port string) error {
	if s.IsConnected() {
		return fmt.Errorf("已连接到服务器")
	}

	s.serverAddr = addr
	s.serverPort = port

	// 连接服务器
	if err := s.networkClient.Connect(addr, port); err != nil {
		return fmt.Errorf("连接服务器失败: %v", err)
	}

	// 标记服务开始运行
	if err := s.Start(); err != nil {
		s.Disconnect()
		return err
	}

	s.SetStatus("已连接")
	return nil
}

// Disconnect 断开连接
func (s *ClientSyncService) Disconnect() error {
	if !s.IsConnected() {
		return nil
	}

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
	defer s.SetStatus("同步完成")

	// 获取本地文件列表
	localFiles, err := s.getLocalFiles(sourcePath)
	if err != nil {
		return fmt.Errorf("获取本地文件列表失败: %v", err)
	}

	// 获取服务器文件列表
	serverFiles, err := s.getServerFiles()
	if err != nil {
		return fmt.Errorf("获取服务器文件列表失败: %v", err)
	}

	// 分析差异并同步
	for _, file := range localFiles {
		if err := s.syncFile(sourcePath, file); err != nil {
			s.Logger.Error("同步文件失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}
	}

	// 处理需要删除的文件
	for _, file := range serverFiles {
		if !s.fileExists(localFiles, file) {
			if err := s.deleteRemoteFile(file); err != nil {
				s.Logger.Error("删除远程文件失败", interfaces.Fields{
					"file":  file,
					"error": err,
				})
			}
		}
	}

	return nil
}

// syncFile 同步单个文件
func (s *ClientSyncService) syncFile(sourcePath, file string) error {
	fullPath := filepath.Join(sourcePath, file)

	// 获取文件信息
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 检查是否需要忽略
	if s.IsIgnored(file) {
		s.Logger.Debug("忽略文件", interfaces.Fields{
			"file": file,
		})
		return nil
	}

	// 获取同步模式
	mode := s.GetSyncMode(file)

	// 创建同步请求
	req := &interfaces.SyncRequest{
		Mode:      mode,
		Direction: interfaces.DirectionPush,
		Path:      file,
	}

	// 根据同步模式处理
	switch mode {
	case interfaces.MirrorSync:
		return s.mirrorSync(fullPath, info, req)
	case interfaces.PushSync:
		return s.pushSync(fullPath, info, req)
	case interfaces.PackSync:
		return s.packSync(fullPath, info, req)
	default:
		return s.autoSync(fullPath, info, req)
	}
}

// mirrorSync 镜像同步
func (s *ClientSyncService) mirrorSync(path string, info os.FileInfo, req *interfaces.SyncRequest) error {
	// 检查文件是否存在和可读
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("检查文件失败: %v", err)
	}

	// 发送同步请求
	if err := s.networkClient.SendData(req); err != nil {
		return fmt.Errorf("发送同步请求失败: %v", err)
	}

	// 发送文件数据
	progress := make(chan interfaces.Progress, 1)
	defer close(progress)

	go func() {
		for p := range progress {
			s.ReportProgress(&p)
		}
	}()

	if err := s.networkClient.SendFile(path, progress); err != nil {
		return fmt.Errorf("发送文件失败: %v", err)
	}

	return nil
}

// pushSync 推送同步
func (s *ClientSyncService) pushSync(path string, info os.FileInfo, req *interfaces.SyncRequest) error {
	// 检查服务器上是否存在该文件
	serverFiles, err := s.getServerFiles()
	if err != nil {
		return err
	}

	relPath, err := filepath.Rel(s.GetCurrentConfig().SyncDir, path)
	if err != nil {
		return err
	}

	needUpdate := true
	for _, serverFile := range serverFiles {
		if serverFile == relPath {
			// TODO: 检查文件哈希值
			needUpdate = false
			break
		}
	}

	if !needUpdate {
		s.Logger.Debug("文件无需更新", interfaces.Fields{
			"file": path,
		})
		return nil
	}

	// 执行推送
	return s.mirrorSync(path, info, req)
}

// packSync 打包同步
func (s *ClientSyncService) packSync(path string, info os.FileInfo, req *interfaces.SyncRequest) error {
	// TODO: 实现打包同步逻辑
	return fmt.Errorf("暂不支持打包同步")
}

// autoSync 自动同步
func (s *ClientSyncService) autoSync(path string, info os.FileInfo, req *interfaces.SyncRequest) error {
	// 根据文件类型选择同步模式
	ext := filepath.Ext(path)
	switch ext {
	case ".zip", ".rar", ".7z":
		return s.packSync(path, info, req)
	case ".exe", ".dll", ".jar":
		return s.mirrorSync(path, info, req)
	default:
		return s.pushSync(path, info, req)
	}
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
