package handlers

import (
	"net"
	"os"
	"path/filepath"

	"synctools/pkg/common"
)

// ServerInterface 定义服务器接口
type ServerInterface interface {
	GetFolderConfig(path string) (*common.SyncFolder, bool)
}

// getFolderConfig 获取文件夹配置
var serverInstance ServerInterface

func SetServerInstance(s ServerInterface) {
	serverInstance = s
}

func getFolderConfig(path string) (*common.SyncFolder, bool) {
	if serverInstance != nil {
		return serverInstance.GetFolderConfig(path)
	}
	return nil, false
}

// HandleClient 处理客户端连接
func HandleClient(conn net.Conn, syncDir string, ignoreList []string, logger common.Logger, getRedirectedPath func(string) string, config common.SyncConfig) {
	defer conn.Close()
	clientAddr := conn.RemoteAddr().String()
	logger.Log("客户端连接: %s", clientAddr)

	// 首先发送服务器配置
	if err := common.WriteJSON(conn, config); err != nil {
		logger.Log("发送配置信息错误 %s: %v", clientAddr, err)
		return
	}

	// 接收客户端版本
	var clientVersion string
	if err := common.ReadJSON(conn, &clientVersion); err != nil {
		logger.Log("接收客户端版本错误 %s: %v", clientAddr, err)
		return
	}

	logger.Log("客户端版本: %s, 服务器版本: %s", clientVersion, config.Version)
	isVersionDifferent := clientVersion != config.Version
	if isVersionDifferent {
		logger.Log("版本不同，将删除服务端没有的文件")
	} else {
		logger.Log("版本相同，保留客户端文件")
	}

	// 接收客户端的同步路径
	var clientPath string
	if err := common.ReadJSON(conn, &clientPath); err != nil {
		logger.Log("接收同步路径错误 %s: %v", clientAddr, err)
		return
	}

	// 获取重定向后的路径
	serverPath := getRedirectedPath(clientPath)
	fullPath := filepath.Join(syncDir, serverPath)

	filesInfo, err := getFilesInfo(fullPath, ignoreList, logger)
	if err != nil {
		logger.Log("获取文件信息错误: %v", err)
		return
	}

	// 发送文件信息和同步信息
	syncInfo := common.SyncInfo{
		Files:            filesInfo,
		DeleteExtraFiles: isVersionDifferent,
		SyncMode:         "mirror", // 默认使用mirror模式
	}

	// 如果找到对应的文件夹配置，使用配置的同步模式
	if folder, ok := getFolderConfig(serverPath); ok {
		syncInfo.SyncMode = folder.SyncMode
		logger.DebugLog("使用文件夹配置的同步模式: %s", folder.SyncMode)
	}

	if err := common.WriteJSON(conn, syncInfo); err != nil {
		logger.Log("发送同步信息错误 %s: %v", clientAddr, err)
		return
	}

	for {
		var filename string
		if err := common.ReadJSON(conn, &filename); err != nil {
			if err != common.ErrConnectionClosed {
				logger.Log("接收文件请求错误 %s: %v", clientAddr, err)
			}
			return
		}

		if filename == "DONE" {
			logger.Log("客户端 %s 完成同步", clientAddr)
			return
		}

		logger.Log("客户端 %s 请求文件: %s", clientAddr, filename)

		// 获取重定向后的文件路径
		redirectedPath := getRedirectedPath(filename)
		filepath := filepath.Join(syncDir, redirectedPath)

		logger.DebugLog("重定向路径: %s -> %s", filename, redirectedPath)

		file, err := os.Open(filepath)
		if err != nil {
			logger.Log("打开文件错误 %s: %v", filename, err)
			continue
		}

		bytesSent, err := common.SendFile(conn, file)
		file.Close()

		if err != nil {
			logger.Log("发送文件错误 %s to %s: %v", filename, clientAddr, err)
			return
		}
		logger.Log("发送文件 %s to %s (%d bytes)", filename, clientAddr, bytesSent)
	}
}

// getFilesInfo 获取目录下所有文件的信息
func getFilesInfo(baseDir string, ignoreList []string, logger common.Logger) (map[string]common.FileInfo, error) {
	filesInfo := make(map[string]common.FileInfo)

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				return err
			}

			// 检查忽略列表
			for _, ignore := range ignoreList {
				matched, err := filepath.Match(ignore, relPath)
				if err == nil && matched {
					logger.DebugLog("忽略文件: %s (匹配规则: %s)", relPath, ignore)
					return nil
				}
			}

			hash, err := common.CalculateMD5(path)
			if err != nil {
				return err
			}

			filesInfo[relPath] = common.FileInfo{
				Hash: hash,
				Size: info.Size(),
			}
			if logger != nil {
				logger.Log("添加文件: %s, 大小: %d bytes", relPath, info.Size())
			}
		}
		return nil
	})

	return filesInfo, err
}
