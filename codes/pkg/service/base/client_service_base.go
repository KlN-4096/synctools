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
	"crypto/md5"
	"encoding/hex"
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

// PrepareSyncFiles 同步前的初始化准备
func (s *ClientSyncBase) PrepareSyncFiles(sourcePath string) (map[string]map[string]string, map[string]map[string]string, []string, map[string]map[string]struct{}, int, error) {
	s.Logger.Info("开始同步准备", interfaces.Fields{
		"source_path": sourcePath,
	})

	// 获取同步文件夹列表
	syncFolders := s.GetSyncFolders()
	if len(syncFolders) == 0 {
		s.Logger.Info("没有配置同步文件夹", interfaces.Fields{
			"source_path": sourcePath,
		})
		return nil, nil, nil, nil, 0, nil
	}

	// 获取所有文件夹的 MD5 信息
	allServerFiles := make(map[string]map[string]string)
	allLocalFiles := make(map[string]map[string]string)

	for _, folder := range syncFolders {
		// 检查是否需要重定向
		localFolderPath := filepath.Join(sourcePath, folder)
		config := s.GetCurrentConfig()
		if config != nil && len(config.FolderRedirects) > 0 {
			for _, redirect := range config.FolderRedirects {
				if strings.HasPrefix(folder, redirect.ServerPath) {
					// 替换路径前缀
					relativePath := strings.TrimPrefix(folder, redirect.ServerPath)
					// 确保relativePath不以分隔符开头
					relativePath = strings.TrimPrefix(relativePath, "/")
					localFolderPath = filepath.Join(sourcePath, redirect.ClientPath, relativePath)
					break
				}
			}
		}

		// 获取本地文件列表和MD5
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

	// 计算需要同步的文件
	var totalFiles, needSyncFiles, ignoredFiles, extraFiles int
	var filesToSync []string
	filesToDelete := make(map[string]map[string]struct{}) // 改为嵌套Map: folder -> set of files
	var ignoredMD5s []string                              // 记录被忽略文件的MD5

	for folder, serverFiles := range allServerFiles {
		localFiles := allLocalFiles[folder]
		filesToDelete[folder] = make(map[string]struct{})

		// 检查本地多余的文件
		for localPath := range localFiles {
			// 获取重定向后的路径
			redirectedPath := s.getRedirectedPathByConfig(localPath, false)
			if _, exists := serverFiles[redirectedPath]; !exists && !s.isIgnoredFile(redirectedPath) {
				s.Logger.Debug("发现本地多余文件", interfaces.Fields{
					"folder":         folder,
					"file":           localPath,
					"redirectedPath": redirectedPath,
				})
				extraFiles++
				filesToDelete[folder][localPath] = struct{}{}
			}
		}

		// 检查需要同步的服务器文件
		for serverPath, serverMD5 := range serverFiles {
			// 获取重定向后的路径
			redirectedPath := s.getRedirectedPathByConfig(serverPath, true)

			// 检查文件是否需要忽略
			if s.isIgnoredFile(redirectedPath) {
				s.Logger.Debug("忽略文件", interfaces.Fields{
					"folder":         folder,
					"file":           serverPath,
					"redirectedPath": redirectedPath,
					"md5":            serverMD5,
				})
				ignoredFiles++
				ignoredMD5s = append(ignoredMD5s, serverMD5)
				continue
			}

			totalFiles++
			localMD5, exists := localFiles[redirectedPath]
			if !exists || localMD5 != serverMD5 {
				needSyncFiles++
				filesToSync = append(filesToSync, filepath.Join(folder, serverPath))
			}
		}
	}

	s.Logger.Info("文件对比完成", interfaces.Fields{
		"total_files":   totalFiles,
		"need_sync":     needSyncFiles,
		"ignored_files": ignoredFiles,
		"extra_files":   extraFiles,
		"ignored_md5s":  ignoredMD5s,
		"files_to_sync": filesToSync,
		"files_to_del":  filesToDelete,
	})

	return allServerFiles, allLocalFiles, filesToSync, filesToDelete, ignoredFiles, nil
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

// GetSyncFolders 获取同步文件夹列表
func (s *ClientSyncBase) GetSyncFolders() []string {
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

func (s *ClientSyncBase) getLocalFilesWithMD5(dir string) (map[string]string, error) {
	// 检查是否是打包模式
	for _, folder := range s.Config.SyncFolders {
		if strings.HasSuffix(dir, folder.Path) && folder.SyncMode == interfaces.PackSync {
			// 打包模式返回空映射
			return make(map[string]string), nil
		}
	}

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

func (s *ClientSyncBase) calculateFileMD5(path string) (string, error) {
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

func (s *ClientSyncBase) getServerFilesWithMD5WithFolder(folder string) (map[string]string, error) {
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

func (s *ClientSyncBase) getRedirectedPathByConfig(path string, isServer bool) string {
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

func (s *ClientSyncBase) isIgnoredFile(path string) bool {
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
