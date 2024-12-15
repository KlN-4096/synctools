package common

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxn/walk"
)

// FileInfo 存储文件的基本信息
type FileInfo struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// Logger 定义日志接口
type Logger interface {
	AppendText(text string)
}

// 自定义错误
var (
	ErrConnectionClosed = errors.New("连接已关闭")
	ErrInvalidSize      = errors.New("无效的文件大小")
)

// FormatLog 格式化日志消息
func FormatLog(format string, v ...interface{}) string {
	msg := fmt.Sprintf(format, v...)
	if !strings.HasSuffix(msg, "\r\n") {
		msg = strings.TrimSuffix(msg, "\n")
		msg += "\r\n"
	}
	return fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
}

// WriteLog 写入日志到指定的Logger
func WriteLog(logger Logger, format string, v ...interface{}) {
	logger.AppendText(FormatLog(format, v...))
}

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

// GetFilesInfo 获取目录下所有文件的信息
func GetFilesInfo(baseDir string, ignoreList []string, logger Logger) (map[string]FileInfo, error) {
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

			// 简化忽略列表检查
			for _, ignore := range ignoreList {
				if strings.Contains(relPath, ignore) {
					return nil
				}
			}

			hash, err := CalculateMD5(path)
			if err != nil {
				return err
			}

			filesInfo[relPath] = FileInfo{
				Hash: hash,
				Size: info.Size(),
			}
			if logger != nil {
				WriteLog(logger, "添加文件: %s, 大小: %d bytes", relPath, info.Size())
			}
		}
		return nil
	})

	return filesInfo, err
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

// IsDir 检查路径是否为目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FileLogger 文件日志记录器
type FileLogger struct {
	logFile    *os.File
	currentDay string
	logDir     string
	prefix     string // "server" 或 "client"
}

// NewFileLogger 创建新的文件日志记录器
func NewFileLogger(logDir, prefix string) (*FileLogger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logger := &FileLogger{
		logDir: logDir,
		prefix: prefix,
	}

	if err := logger.rotateLog(); err != nil {
		return nil, err
	}

	return logger, nil
}

// rotateLog 根据日期切换日志文件
func (l *FileLogger) rotateLog() error {
	currentDay := time.Now().Format("2006-01-02")

	// 如果日期变化或文件未打开，则创建新文件
	if l.currentDay != currentDay || l.logFile == nil {
		// 关闭旧文件
		if l.logFile != nil {
			l.logFile.Close()
		}

		// 创建新文件
		filename := filepath.Join(l.logDir, fmt.Sprintf("%s_%s.log", l.prefix, currentDay))
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}

		l.logFile = file
		l.currentDay = currentDay
	}

	return nil
}

// WriteLog 写入日志
func (l *FileLogger) WriteLog(msg string) error {
	if err := l.rotateLog(); err != nil {
		return err
	}

	_, err := l.logFile.WriteString(msg)
	return err
}

// Close 关闭日志文件
func (l *FileLogger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// UIConfig 存储UI相关的配置
type UIConfig struct {
	Title     string
	MinWidth  int
	MinHeight int
	Width     int
	Height    int
}

// ServerConfig 存储服务器配置
type ServerConfig struct {
	Host       string
	Port       int
	SyncDir    string
	IgnoreList []string
}

// LogWriter 定义日志写入接口
type LogWriter interface {
	Logger
	SetText(text string)
	Text() string
}

// ValidateResult 存储验证结果
type ValidateResult struct {
	Valid   bool
	Message string
}

// UIHelper UI帮助方法
func ShowErrorDialog(owner walk.Form, title, message string) {
	walk.MsgBox(owner, title, message, walk.MsgBoxIconError)
}

// CreateLogBox 创建日志文本框
func CreateLogBox(parent walk.Container) (*walk.TextEdit, error) {
	logBox, err := walk.NewTextEdit(parent)
	if err != nil {
		return nil, err
	}
	logBox.SetReadOnly(true)
	return logBox, nil
}

// CreateColoredLabel 创建带颜色的标签
func CreateColoredLabel(text string, color walk.Color) *walk.Label {
	label, _ := walk.NewLabel(nil)
	label.SetText(text)
	label.SetTextColor(color)
	return label
}

// FormatBytes 格式化字节大小
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
