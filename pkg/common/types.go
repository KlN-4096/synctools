package common

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// FileInfo 存储文件的基本信息
type FileInfo struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// 自定义错误
var (
	ErrConnectionClosed = errors.New("连接已关闭")
	ErrInvalidSize      = errors.New("无效的文件大小")
)

// CalculateMD5 计算文件的MD5哈希值
func CalculateMD5(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// WriteJSON 将数据编码为JSON并写入连接
func WriteJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(data)
}

// ReadJSON 从连接读取JSON并解码
func ReadJSON(r io.Reader, data interface{}) error {
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(data); err != nil {
		if err == io.EOF {
			return ErrConnectionClosed
		}
		return err
	}
	return nil
}

// SendFile 发送文件内容到连接
func SendFile(w io.Writer, file *os.File) (int64, error) {
	return io.Copy(w, file)
}

// ReceiveFile 从连接接收文件内容并写入文件
func ReceiveFile(r io.Reader, file *os.File, size int64) (int64, error) {
	return io.Copy(file, io.LimitReader(r, size))
}

// IsPathExists 检查路径是否存在
func IsPathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(dir string) error {
	if !IsPathExists(dir) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}
