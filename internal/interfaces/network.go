package interfaces

import (
	"io"
	"net"
)

// NetworkServer 定义网络服务器的核心接口
type NetworkServer interface {
	// Start 启动服务器
	Start() error

	// Stop 停止服务器
	Stop() error

	// HandleClient 处理客户端连接
	HandleClient(conn net.Conn)

	// GetStatus 获取服务器状态
	GetStatus() string

	// IsRunning 检查服务器是否运行中
	IsRunning() bool
}

// NetworkOperations defines network operations interface
type NetworkOperations interface {
	// WriteJSON writes JSON data to connection
	WriteJSON(conn net.Conn, data interface{}) error

	// ReadJSON reads JSON data from connection
	ReadJSON(conn net.Conn, data interface{}) error

	// SendFile sends file through connection
	SendFile(conn net.Conn, path string, progress chan<- Progress) error

	// ReceiveFile receives file through connection
	ReceiveFile(conn net.Conn, path string, progress chan<- Progress) error

	// SendFiles sends multiple files through connection
	SendFiles(conn net.Conn, files []string, progress chan<- Progress) error

	// ReceiveFiles receives multiple files through connection
	ReceiveFiles(conn net.Conn, destDir string, progress chan<- Progress) error
}

// FileTransfer defines file transfer operations
type FileTransfer interface {
	// CopyFile copies file with progress reporting
	CopyFile(dst io.Writer, src io.Reader, size int64, progress chan<- Progress) error

	// ValidateFile validates file integrity
	ValidateFile(path string) error

	// GetFileInfo gets file information
	GetFileInfo(path string) (*FileInfo, error)

	// ListFiles lists files in directory
	ListFiles(path string) ([]FileInfo, error)
}

// NetworkError defines network error interface
type NetworkError interface {
	error
	IsTimeout() bool
	IsTemporary() bool
}
