package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/errors"
	netserver "synctools/codes/pkg/network/server"
	"synctools/codes/pkg/service/base"
)

// ServerSyncService 服务端同步服务实现
type ServerSyncService struct {
	*base.BaseSyncService
	server interfaces.NetworkServer
}

// NewServerSyncService 创建服务端同步服务
func NewServerSyncService(config *interfaces.Config, Logger interfaces.Logger, storage interfaces.Storage) *ServerSyncService {
	return &ServerSyncService{
		BaseSyncService: base.NewBaseSyncService(config, Logger, storage),
	}
}

// StartServer 启动服务器
func (s *ServerSyncService) StartServer() error {
	if s.IsRunning() {
		return errors.ErrServiceStart
	}

	if s.server == nil {
		s.server = netserver.NewServer(s.GetCurrentConfig(), s, s.Logger)
	}

	if err := s.server.Start(); err != nil {
		s.SetStatus("启动失败")
		return err
	}

	if err := s.Start(); err != nil {
		s.StopServer()
		return err
	}

	s.SetStatus("服务器运行中")
	return nil
}

// StopServer 停止服务器
func (s *ServerSyncService) StopServer() error {
	if !s.IsRunning() {
		return nil
	}

	if s.server != nil {
		if err := s.server.Stop(); err != nil {
			return err
		}
		s.server = nil
	}

	s.Stop()
	s.SetStatus("服务器已停止")
	return nil
}

// SetServer 设置网络服务器
func (s *ServerSyncService) SetServer(server interfaces.NetworkServer) {
	if s.server != nil {
		s.StopServer()
	}
	s.server = server
}

// GetNetworkServer 获取网络服务器
func (s *ServerSyncService) GetNetworkServer() interfaces.NetworkServer {
	return s.server
}

// HandleSyncRequest 处理同步请求
func (s *ServerSyncService) HandleSyncRequest(request interface{}) error {
	req, ok := request.(*interfaces.SyncRequest)
	if !ok {
		return fmt.Errorf("无效的请求类型")
	}

	s.Logger.Info("处理同步请求", interfaces.Fields{
		"mode":      req.Mode,
		"direction": req.Direction,
		"path":      req.Path,
	})

	switch req.Mode {
	case interfaces.MirrorSync:
		return s.handleMirrorSync(req)
	case interfaces.PushSync:
		return s.handlePushSync(req)
	case interfaces.PackSync:
		return s.handlePackSync(req)
	default:
		return fmt.Errorf("不支持的同步模式: %s", req.Mode)
	}
}

// handleMirrorSync 处理镜像同步
func (s *ServerSyncService) handleMirrorSync(req *interfaces.SyncRequest) error {
	// 获取源文件列表
	files, err := s.getFileList(req.Path)
	if err != nil {
		return err
	}

	// 遍历处理每个文件
	for _, file := range files {
		sourcePath := filepath.Join(s.GetCurrentConfig().SyncDir, file)
		targetPath := filepath.Join(req.Path, file)

		// 检查是否需要忽略
		if s.IsIgnored(file) {
			continue
		}

		// 复制文件
		if err := s.copyFile(sourcePath, targetPath); err != nil {
			s.Logger.Error("复制文件失败", interfaces.Fields{
				"source": sourcePath,
				"target": targetPath,
				"error":  err,
			})
			continue
		}
	}

	return nil
}

// handlePushSync 处理推送同步
func (s *ServerSyncService) handlePushSync(req *interfaces.SyncRequest) error {
	// 检查目标路径是否存在
	targetDir := filepath.Join(s.GetCurrentConfig().SyncDir, req.Path)
	if _, err := os.Stat(targetDir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return fmt.Errorf("创建目标目录失败: %v", err)
			}
		} else {
			return fmt.Errorf("检查目标目录失败: %v", err)
		}
	}

	// 处理文件列表
	for _, file := range req.Files {
		sourcePath := filepath.Join(req.Path, file)
		targetPath := filepath.Join(targetDir, file)

		// 检查目标文件是否存在
		if _, err := os.Stat(targetPath); err == nil {
			// 如果文件存在，检查是否需要更新
			sourceInfo, err := os.Stat(sourcePath)
			if err != nil {
				s.Logger.Error("获取源文件信息失败", interfaces.Fields{
					"file":  sourcePath,
					"error": err,
				})
				continue
			}

			targetInfo, err := os.Stat(targetPath)
			if err != nil {
				s.Logger.Error("获取目标文件信息失败", interfaces.Fields{
					"file":  targetPath,
					"error": err,
				})
				continue
			}

			if sourceInfo.ModTime().Equal(targetInfo.ModTime()) {
				// 文件未修改，跳过
				continue
			}
		}

		// 复制文件
		if err := s.copyFile(sourcePath, targetPath); err != nil {
			s.Logger.Error("复制文件失败", interfaces.Fields{
				"source": sourcePath,
				"target": targetPath,
				"error":  err,
			})
			continue
		}
	}

	return nil
}

// handlePackSync 处理打包同步
func (s *ServerSyncService) handlePackSync(req *interfaces.SyncRequest) error {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "synctools_pack_*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建压缩包
	packFile := filepath.Join(tempDir, "pack.zip")
	if err := s.createPack(req.Path, packFile); err != nil {
		return fmt.Errorf("创建压缩包失败: %v", err)
	}

	// 发送压缩包
	if err := s.sendPack(packFile, req); err != nil {
		return fmt.Errorf("发送压缩包失败: %v", err)
	}

	return nil
}

// 辅助方法

// getFileList 获取目录下的所有文件
func (s *ServerSyncService) getFileList(dir string) ([]string, error) {
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

// copyFile 复制文件
func (s *ServerSyncService) copyFile(src, dst string) error {
	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 打开源文件
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %v", err)
	}
	defer source.Close()

	// 创建目标文件
	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer destination.Close()

	// 复制文件内容
	if _, err = io.Copy(destination, source); err != nil {
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	// 获取源文件信息
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("获取源文件信息失败: %v", err)
	}

	// 保持文件权限和时间戳
	if err := os.Chmod(dst, sourceInfo.Mode()); err != nil {
		return fmt.Errorf("设置文件权限失败: %v", err)
	}
	if err := os.Chtimes(dst, time.Now(), sourceInfo.ModTime()); err != nil {
		return fmt.Errorf("设置文件时间戳失败: %v", err)
	}

	return nil
}

// createPack 创建压缩包
func (s *ServerSyncService) createPack(srcPath string, packFile string) error {
	// 创建压缩包文件
	file, err := os.Create(packFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// TODO: 实现压缩逻辑
	// 可以使用 archive/zip 包来实现
	return nil
}

// sendPack 发送压缩包
func (s *ServerSyncService) sendPack(packFile string, req *interfaces.SyncRequest) error {
	// TODO: 实现发送逻辑
	// 可以分块发送大文件
	return nil
}
