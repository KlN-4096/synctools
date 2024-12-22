package network

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"synctools/internal/interfaces"
	"synctools/pkg/errors"
)

// Operations 实现网络操作接口
type Operations struct {
	logger interfaces.Logger
}

// NewOperations 创建网络操作实例
func NewOperations(logger interfaces.Logger) interfaces.NetworkOperations {
	return &Operations{
		logger: logger,
	}
}

// TransferProgress 传输进度结构
type TransferProgress struct {
	TotalSize   int64     // 总大小
	CurrentSize int64     // 当前大小
	Percentage  float64   // 完成百分比
	Speed       float64   // 传输速度 (bytes/s)
	StartTime   time.Time // 开始时间
}

// WriteJSON 写入JSON数据
func (o *Operations) WriteJSON(conn net.Conn, data interface{}) error {
	if conn == nil {
		return errors.NewNetworkError("WriteJSON", "连接为空", nil)
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(data); err != nil {
		return errors.NewNetworkError("WriteJSON", "编码JSON数据失败", err)
	}
	return nil
}

// ReadJSON 读取JSON数据
func (o *Operations) ReadJSON(conn net.Conn, data interface{}) error {
	if conn == nil {
		return errors.NewNetworkError("ReadJSON", "连接为空", nil)
	}

	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(data); err != nil {
		return errors.NewNetworkError("ReadJSON", "解码JSON数据失败", err)
	}
	return nil
}

// SendFile 发送文件
func (o *Operations) SendFile(conn net.Conn, path string, progress chan<- interfaces.Progress) error {
	if conn == nil {
		return errors.NewNetworkError("SendFile", "连接为空", nil)
	}

	file, err := os.Open(path)
	if err != nil {
		return errors.NewNetworkError("SendFile", "打开文件失败", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return errors.NewNetworkError("SendFile", "获取文件信息失败", err)
	}

	buffer := make([]byte, 32*1024)
	totalWritten := int64(0)
	startTime := time.Now()

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.NewNetworkError("SendFile", "读取文件失败", err)
		}

		written, err := conn.Write(buffer[:n])
		if err != nil {
			return errors.NewNetworkError("SendFile", "写入数据失败", err)
		}

		totalWritten += int64(written)

		if progress != nil {
			elapsed := time.Since(startTime).Seconds()
			speed := float64(totalWritten) / elapsed
			remaining := int64((float64(info.Size()-totalWritten) / speed))

			progress <- interfaces.Progress{
				Total:     info.Size(),
				Current:   totalWritten,
				Speed:     speed,
				Remaining: remaining,
				FileName:  filepath.Base(path),
				Status:    "sending",
			}
		}
	}

	return nil
}

// ReceiveFile 接收文件
func (o *Operations) ReceiveFile(conn net.Conn, path string, progress chan<- interfaces.Progress) error {
	if conn == nil {
		return errors.NewNetworkError("ReceiveFile", "连接为空", nil)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return errors.NewNetworkError("ReceiveFile", "创建目录失败", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return errors.NewNetworkError("ReceiveFile", "创建文件失败", err)
	}
	defer file.Close()

	buffer := make([]byte, 32*1024)
	totalRead := int64(0)
	startTime := time.Now()

	for {
		n, err := conn.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.NewNetworkError("ReceiveFile", "读取数据失败", err)
		}

		written, err := file.Write(buffer[:n])
		if err != nil {
			return errors.NewNetworkError("ReceiveFile", "写入文件失败", err)
		}

		totalRead += int64(written)

		if progress != nil {
			elapsed := time.Since(startTime).Seconds()
			speed := float64(totalRead) / elapsed

			progress <- interfaces.Progress{
				Total:     -1, // 未知总大小
				Current:   totalRead,
				Speed:     speed,
				Remaining: -1, // 未知剩余时间
				FileName:  filepath.Base(path),
				Status:    "receiving",
			}
		}
	}

	return nil
}

// SendFiles 发送多个文件
func (o *Operations) SendFiles(conn net.Conn, files []string, progress chan<- interfaces.Progress) error {
	for _, file := range files {
		if err := o.SendFile(conn, file, progress); err != nil {
			return err
		}
	}
	return nil
}

// ReceiveFiles 接收多个文件
func (o *Operations) ReceiveFiles(conn net.Conn, destDir string, progress chan<- interfaces.Progress) error {
	for {
		var fileInfo interfaces.FileInfo
		if err := o.ReadJSON(conn, &fileInfo); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		destPath := filepath.Join(destDir, fileInfo.Path)
		if err := o.ReceiveFile(conn, destPath, progress); err != nil {
			return err
		}
	}
	return nil
}
