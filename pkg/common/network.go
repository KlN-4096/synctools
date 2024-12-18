package common

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
)

// WriteJSON 写入JSON数据
func WriteJSON(conn net.Conn, data interface{}) error {
	encoder := json.NewEncoder(conn)
	return encoder.Encode(data)
}

// ReadJSON 读取JSON数据
func ReadJSON(conn net.Conn, data interface{}) error {
	decoder := json.NewDecoder(conn)
	return decoder.Decode(data)
}

// ReceiveFile 接收文件
func ReceiveFile(conn net.Conn, path string) (int64, error) {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return 0, err
	}

	// 创建文件
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// 接收文件数据
	return io.Copy(file, conn)
}

// ReceiveFileToWriter 接收文件并写入到指定的 writer
func ReceiveFileToWriter(conn net.Conn, writer io.Writer, size int64) (int64, error) {
	return io.CopyN(writer, conn, size)
}

// GetFilesInfo 获取目录下所有文件的信息
func GetFilesInfo(baseDir string, ignoreList []string, logger interface{ Log(string, ...interface{}) }) (map[string]FileInfo, error) {
	filesInfo := make(map[string]FileInfo)

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
					if logger != nil {
						logger.Log("忽略文件: %s (匹配规则: %s)", relPath, ignore)
					}
					return nil
				}
			}

			hash, err := CalculateFileHash(path)
			if err != nil {
				return err
			}

			filesInfo[relPath] = FileInfo{
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
