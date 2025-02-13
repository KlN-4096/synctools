/*
文件作用:
- 实现客户端同步服务的基础功能
- 管理文件同步和传输的核心逻辑
- 处理文件系统操作
- 提供MD5校验功能

主要功能:
1. 文件同步准备
2. 文件下载处理
3. 文件系统操作
4. MD5校验
*/

package base

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/network/client"
)

// ClientSyncBase 客户端同步基础服务
type ClientSyncBase struct {
	*BaseSyncService
	networkClient *client.NetworkClient
}

// NewClientSyncBase 创建客户端同步基础服务
func NewClientSyncBase(base *BaseSyncService, networkClient *client.NetworkClient) *ClientSyncBase {
	return &ClientSyncBase{
		BaseSyncService: base,
		networkClient:   networkClient,
	}
}

// DownloadFile 从服务器下载文件
func (s *ClientSyncBase) DownloadFile(req *interfaces.SyncRequest, destPath string, sourcePath string, mode interfaces.SyncMode) error {
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
			"path": sourcePath,
		})

		// 创建临时目录
		tempDir, err := os.MkdirTemp("", "synctools_pack_*")
		if err != nil {
			return fmt.Errorf("创建临时目录失败: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// 构建临时文件路径
		fileName := filepath.Base(req.Path)
		tempFile := filepath.Join(tempDir, fileName)

		s.Logger.Debug("准备下载文件", interfaces.Fields{
			"temp_dir":  tempDir,
			"temp_file": tempFile,
			"req_path":  req.Path,
		})

		// 先将压缩包下载到临时目录
		if err := s.networkClient.ReceiveFile(tempFile, progress); err != nil {
			return fmt.Errorf("接收压缩包失败: %v", err)
		}

		// 获取目标目录（移除.zip后缀）
		targetDir := filepath.Dir(destPath)

		s.Logger.Debug("解压文件信息", interfaces.Fields{
			"temp_dir":   tempDir,
			"temp_file":  tempFile,
			"target_dir": targetDir,
			"req_path":   req.Path,
			"dest_path":  destPath,
		})

		// 解压文件
		if err := s.unpackFile(tempFile, targetDir); err != nil {
			return fmt.Errorf("解压文件失败: %v", err)
		}

		s.Logger.Info("解压完成", interfaces.Fields{
			"pack": tempFile,
			"dest": targetDir,
		})
		return nil
	}

	// 获取重定向后的路径
	redirectedPath := s.getRedirectedPath(req.Path, destPath)

	// 如果路径是"."，则使用目标目录本身
	var targetDir string
	if filepath.Base(req.Path) == "." {
		targetDir = filepath.Dir(redirectedPath)
	} else {
		targetDir = filepath.Dir(redirectedPath + "/")
	}

	// 接收文件
	if err := s.networkClient.ReceiveFile(targetDir, progress); err != nil {
		return fmt.Errorf("接收文件失败: %v", err)
	}

	return nil
}

// IsSingleFile 检查是否为单个文件
func (s *ClientSyncBase) IsSingleFile(path string) bool {
	// 获取当前配置
	config := s.GetCurrentConfig()
	if config == nil || len(config.SyncFolders) == 0 {
		return false
	}

	// 如果路径包含后缀名,则认为是单文件
	return filepath.Ext(path) != ""
}

// 其他辅助方法...

func (s *ClientSyncBase) unpackFile(packFile, destPath string) error {
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
		// 构建目标文件路径
		targetPath := filepath.Join(destPath, file.Name)

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

func (s *ClientSyncBase) getRedirectedPath(originalPath, destPath string) string {
	// 统一路径分隔符
	originalPath = filepath.ToSlash(originalPath)
	destPath = filepath.ToSlash(destPath)

	// 判断是否为单个文件（不包含路径分隔符）
	if !strings.Contains(originalPath, "/") {
		return destPath
	}

	// 获取当前配置
	config := s.GetCurrentConfig()
	if config == nil || len(config.FolderRedirects) == 0 {
		return destPath
	}

	// 检查是否需要重定向
	for _, redirect := range config.FolderRedirects {
		if strings.HasPrefix(originalPath, redirect.ServerPath) {
			// 替换路径前缀
			relativePath := strings.TrimPrefix(originalPath, redirect.ServerPath)
			// 确保relativePath不以分隔符开头
			relativePath = strings.TrimPrefix(relativePath, "/")
			targetDir := filepath.Join(filepath.Dir(filepath.Dir(destPath)), redirect.ClientPath)
			return filepath.ToSlash(filepath.Join(targetDir, relativePath))
		}
	}

	return destPath
}

// GetRedirectedPathByConfig 根据配置获取重定向路径
func (s *ClientSyncBase) GetRedirectedPathByConfig(path string, isServer bool) string {
	// 统一路径分隔符
	path = filepath.ToSlash(path)
	config := s.GetCurrentConfig()
	if config == nil || len(config.FolderRedirects) == 0 {
		return path
	}

	for _, redirect := range config.FolderRedirects {
		if isServer {
			// 服务器路径转客户端路径
			if strings.HasPrefix(path, redirect.ServerPath) {
				return strings.Replace(path, redirect.ServerPath, redirect.ClientPath, 1)
			}
		} else {
			// 客户端路径转服务器路径
			if strings.HasPrefix(path, redirect.ClientPath) {
				return strings.Replace(path, redirect.ClientPath, redirect.ServerPath, 1)
			}
		}
	}
	return path
}

// IsIgnoredFile 检查文件是否需要忽略
func (s *ClientSyncBase) IsIgnoredFile(path string) bool {
	serverConfig := s.GetCurrentConfig()
	if serverConfig == nil || len(serverConfig.IgnoreList) == 0 {
		return false
	}

	serverConfig.IgnoreList = s.filterIgnoreList(serverConfig.IgnoreList)
	for _, pattern := range serverConfig.IgnoreList {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}

func (s *ClientSyncBase) filterIgnoreList(ignoreList []string) []string {
	var validIgnoreList []string
	for _, pattern := range ignoreList {
		// 去除空白字符和\r
		pattern = strings.TrimSpace(pattern)
		pattern = strings.TrimSuffix(pattern, "\r")
		if pattern != "" {
			validIgnoreList = append(validIgnoreList, pattern)
		}
	}
	return validIgnoreList
}
