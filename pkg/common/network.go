/*
Package common 提供通用网络操作功能。

文件作用：
- 提供网络通信基础功能
- 实现JSON数据的收发
- 实现文件传输功能
- 提供文件信息获取功能
- 提供错误处理和日志记录

主要类型：
- NetworkError: 网络错误类型
- TransferProgress: 传输进度结构

主要方法：
- WriteJSON: 写入JSON数据到网络连接
- ReadJSON: 从网络连接读取JSON数据
- ReceiveFile: 接收文件数据并保存
- ReceiveFileToWriter: 接收文件数据并写入指定writer
- GetFilesInfo: 获取目录下所有文件的信息
*/

package common

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

// NetworkError 网络错误类型
type NetworkError struct {
	Op      string // 操作名称
	Message string // 错误消息
	Err     error  // 原始错误
}

// Error 实现error接口
func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// TransferProgress 传输进度结构
type TransferProgress struct {
	TotalSize   int64   // 总大小
	CurrentSize int64   // 当前大小
	Percentage  float64 // 完成百分比
	Speed       float64 // 传输速度 (bytes/s)
	StartTime   time.Time
}

// WriteJSON 写入JSON数据
func WriteJSON(conn net.Conn, data interface{}) error {
	if conn == nil {
		return &NetworkError{
			Op:      "WriteJSON",
			Message: "连接为空",
		}
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(data); err != nil {
		return &NetworkError{
			Op:      "WriteJSON",
			Message: "编码JSON数据失败",
			Err:     err,
		}
	}
	return nil
}

// ReadJSON 读取JSON数据
func ReadJSON(conn net.Conn, data interface{}) error {
	if conn == nil {
		return &NetworkError{
			Op:      "ReadJSON",
			Message: "连接为空",
		}
	}

	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(data); err != nil {
		return &NetworkError{
			Op:      "ReadJSON",
			Message: "解码JSON数据失败",
			Err:     err,
		}
	}
	return nil
}

// ReceiveFile 接收文件
func ReceiveFile(conn net.Conn, path string) (int64, error) {
	if conn == nil {
		return 0, &NetworkError{
			Op:      "ReceiveFile",
			Message: "连接为空",
		}
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return 0, &NetworkError{
			Op:      "ReceiveFile",
			Message: "创建目录失败",
			Err:     err,
		}
	}

	// 创建文件
	file, err := os.Create(path)
	if err != nil {
		return 0, &NetworkError{
			Op:      "ReceiveFile",
			Message: "创建文件失败",
			Err:     err,
		}
	}
	defer file.Close()

	// 接收文件数据
	written, err := io.Copy(file, conn)
	if err != nil {
		return written, &NetworkError{
			Op:      "ReceiveFile",
			Message: "接收文件数据失败",
			Err:     err,
		}
	}

	return written, nil
}

// ReceiveFileToWriter 接收文件并写入到指定的 writer
func ReceiveFileToWriter(conn net.Conn, writer io.Writer, size int64) (int64, error) {
	if conn == nil {
		return 0, &NetworkError{
			Op:      "ReceiveFileToWriter",
			Message: "连接为空",
		}
	}

	if writer == nil {
		return 0, &NetworkError{
			Op:      "ReceiveFileToWriter",
			Message: "writer为空",
		}
	}

	// 使用缓冲区进行数据传输
	buffer := make([]byte, 32*1024) // 32KB 缓冲区
	var totalWritten int64

	for totalWritten < size {
		// 计算本次需要读取的大小
		remainingSize := size - totalWritten
		if remainingSize > int64(len(buffer)) {
			remainingSize = int64(len(buffer))
		}

		// 读取数据
		n, err := io.ReadFull(conn, buffer[:remainingSize])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return totalWritten, &NetworkError{
				Op:      "ReceiveFileToWriter",
				Message: "读取数据失败",
				Err:     err,
			}
		}

		// 写入数据
		written, err := writer.Write(buffer[:n])
		if err != nil {
			return totalWritten, &NetworkError{
				Op:      "ReceiveFileToWriter",
				Message: "写入数据失败",
				Err:     err,
			}
		}

		totalWritten += int64(written)

	}

	return totalWritten, nil
}

// GetFilesInfo 获取目录下所有文件的信息
func GetFilesInfo(baseDir string, ignoreList []string, logger interface{ Log(string, ...interface{}) }) (map[string]FileInfo, error) {
	if baseDir == "" {
		return nil, &NetworkError{
			Op:      "GetFilesInfo",
			Message: "基础目录为空",
		}
	}

	// 检查目录是否存在
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil, &NetworkError{
			Op:      "GetFilesInfo",
			Message: "目录不存在",
			Err:     err,
		}
	}

	filesInfo := make(map[string]FileInfo)

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return &NetworkError{
				Op:      "GetFilesInfo",
				Message: "遍历目录失败",
				Err:     err,
			}
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				return &NetworkError{
					Op:      "GetFilesInfo",
					Message: "获取相对路径失败",
					Err:     err,
				}
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
				return &NetworkError{
					Op:      "GetFilesInfo",
					Message: "计算文件哈希失败",
					Err:     err,
				}
			}

			fileInfo, err := os.Stat(path)
			if err != nil {
				return &NetworkError{
					Op:      "GetFilesInfo",
					Message: "获取文件信息失败",
					Err:     err,
				}
			}

			filesInfo[relPath] = FileInfo{
				Path:         path,
				Hash:         hash,
				Size:         fileInfo.Size(),
				ModTime:      fileInfo.ModTime().Unix(),
				IsDirectory:  false,
				RelativePath: relPath,
			}

			if logger != nil {
				logger.Log("添加文件: %s, 大小: %d bytes", relPath, fileInfo.Size())
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return filesInfo, nil
}

// 错误定义
var (
	ErrConnectionClosed = &NetworkError{
		Op:      "Network",
		Message: "连接已关闭",
	}
	ErrInvalidData = &NetworkError{
		Op:      "Network",
		Message: "无效的数据",
	}
	ErrTimeout = &NetworkError{
		Op:      "Network",
		Message: "操作超时",
	}
)
