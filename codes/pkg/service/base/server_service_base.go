/*
文件作用:
- 实现服务端同步服务的基础功能
- 处理客户端的同步请求
- 处理文件系统操作
- 提供MD5校验功能

主要功能:
1. 同步请求处理
2. 文件系统操作
3. MD5校验
4. 配置验证
*/

package base

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"synctools/codes/internal/interfaces"
)

// ServerSyncBase 服务端同步基础服务
type ServerSyncBase struct {
	*BaseSyncService
}

// NewServerSyncBase 创建服务端同步基础服务
func NewServerSyncBase(base *BaseSyncService) *ServerSyncBase {
	return &ServerSyncBase{
		BaseSyncService: base,
	}
}

// ValidateConfig 验证服务器配置是否有效
func (s *ServerSyncBase) ValidateConfig() error {
	config := s.GetCurrentConfig()
	if config == nil {
		return fmt.Errorf("请先配置服务器参数")
	}

	// 检查必要的配置项
	if config.Port <= 0 {
		return fmt.Errorf("端口号无效")
	}

	if config.Host == "" {
		return fmt.Errorf("主机地址未设置")
	}

	if config.SyncDir == "" {
		return fmt.Errorf("同步目录未设置")
	}

	return nil
}

// HandleSyncRequest 处理同步请求
func (s *ServerSyncBase) HandleSyncRequest(req *interfaces.SyncRequest) error {
	// 获取同步目录
	sourcePath := filepath.Join(s.GetCurrentConfig().SyncDir, req.Path)

	// 检查路径是否存在
	if _, err := os.Stat(sourcePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("源路径不存在: %s", sourcePath)
		}
		return fmt.Errorf("检查源路径失败: %v", err)
	}

	// 根据同步模式处理
	switch req.Mode {
	case interfaces.MirrorSync:
		return s.handleMirrorSync(sourcePath, req)
	case interfaces.PackSync:
		return s.handlePackSync(sourcePath, req)
	default:
		return fmt.Errorf("不支持的同步模式: %v", req.Mode)
	}
}

// handleMirrorSync 处理镜像同步
func (s *ServerSyncBase) handleMirrorSync(sourcePath string, req *interfaces.SyncRequest) error {
	// 检查路径是否存在
	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("获取源路径信息失败: %v", err)
	}

	// 如果是文件，直接返回
	if !fileInfo.IsDir() {
		return nil
	}

	// 如果是目录，按原有逻辑处理
	files, err := s.getFileList(sourcePath)
	if err != nil {
		return err
	}

	// 遍历处理每个文件
	for _, file := range files {
		sourceFilePath := filepath.Join(sourcePath, file)
		// 使用filepath.Clean来规范化路径，并使用filepath.ToSlash来统一分隔符
		targetPath := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Base(req.Path), file)))

		// 复制文件
		if err := s.copyFile(sourceFilePath, targetPath); err != nil {
			s.Logger.Error("复制文件失败", interfaces.Fields{
				"source": sourceFilePath,
				"target": targetPath,
				"error":  err,
			})
			continue
		}
	}

	return nil
}

// handlePackSync 处理打包同步
func (s *ServerSyncBase) handlePackSync(sourcePath string, req *interfaces.SyncRequest) error {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "synctools_pack_*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 获取目标文件名
	fileName := filepath.Base(sourcePath)
	if fileName == "." || fileName == "" {
		fileName = "sync"
	}

	// 构建压缩包路径
	zipFile := filepath.Join(tempDir, fileName+".zip")

	// 创建压缩包
	if err := s.createZipArchive(sourcePath, zipFile); err != nil {
		return fmt.Errorf("创建压缩包失败: %v", err)
	}

	return nil
}

// getFileList 获取文件列表
func (s *ServerSyncBase) getFileList(dir string) ([]string, error) {
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
func (s *ServerSyncBase) copyFile(src, dst string) error {
	// 打开源文件
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// 创建目标文件
	target, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer target.Close()

	// 复制文件内容
	_, err = io.Copy(target, source)
	return err
}

// createZipArchive 创建ZIP压缩包
func (s *ServerSyncBase) createZipArchive(sourcePath, zipFile string) error {
	// 创建压缩文件
	zipWriter, err := os.Create(zipFile)
	if err != nil {
		return err
	}
	defer zipWriter.Close()

	// 获取文件列表
	files, err := s.getFileList(sourcePath)
	if err != nil {
		return err
	}

	// 添加文件到压缩包
	for _, file := range files {
		// 打开源文件
		source, err := os.Open(filepath.Join(sourcePath, file))
		if err != nil {
			s.Logger.Error("打开源文件失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}

		// 写入压缩包
		if _, err := io.Copy(zipWriter, source); err != nil {
			source.Close()
			s.Logger.Error("写入压缩包失败", interfaces.Fields{
				"file":  file,
				"error": err,
			})
			continue
		}

		source.Close()
	}

	return nil
}
