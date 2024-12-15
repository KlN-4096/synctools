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

// FolderRedirect 文件夹重定向配置
type FolderRedirect struct {
	ServerPath string `json:"server_path"` // 服务器端的文件夹名
	ClientPath string `json:"client_path"` // 客户端的文件夹名
}

// SyncConfig 同步配置
type SyncConfig struct {
	Host            string           `json:"host"`
	Port            int              `json:"port"`
	SyncDir         string           `json:"sync_dir"`
	IgnoreList      []string         `json:"ignore_list"`
	FolderRedirects []FolderRedirect `json:"folder_redirects"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	Connected bool
	Running   bool
	Message   string
}

// Logger 定义日志接口
type Logger interface {
	AppendText(text string)
}

// GUILogger GUI日志记录器
type GUILogger struct {
	logBox     *walk.TextEdit
	fileLogger *FileLogger
	DebugMode  bool
}

// NewGUILogger 创建新的GUI日志记录器
func NewGUILogger(logBox *walk.TextEdit, logDir, prefix string) (*GUILogger, error) {
	fileLogger, err := NewFileLogger(logDir, prefix)
	if err != nil {
		return nil, err
	}

	return &GUILogger{
		logBox:     logBox,
		fileLogger: fileLogger,
		DebugMode:  false,
	}, nil
}

// AppendText 实现Logger接口
func (l *GUILogger) AppendText(text string) {
	l.logBox.AppendText(text)
}

// Log 记录普通日志
func (l *GUILogger) Log(format string, v ...interface{}) {
	msg := FormatLog(format, v...)
	l.AppendText(msg)
	if l.fileLogger != nil {
		if err := l.fileLogger.WriteLog(msg); err != nil {
			fmt.Printf("写入日志文件失败: %v\n", err)
		}
	}
}

// DebugLog 记录调试日志
func (l *GUILogger) DebugLog(format string, v ...interface{}) {
	if !l.DebugMode {
		return
	}
	l.Log("[DEBUG] "+format, v...)
}

// SetDebugMode 设置调试模式
func (l *GUILogger) SetDebugMode(enabled bool) {
	l.DebugMode = enabled
	l.Log("调试模式已%s", map[bool]string{true: "启用", false: "关闭"}[enabled])
}

// Close 关闭日志记录器
func (l *GUILogger) Close() error {
	if l.fileLogger != nil {
		return l.fileLogger.Close()
	}
	return nil
}

// 自定义错误
var (
	ErrConnectionClosed = errors.New("连接已关闭")
	ErrInvalidSize      = errors.New("无效的文件大小")
	ErrServerRunning    = errors.New("服务器已经在运行")
	ErrNotConnected     = errors.New("未连接到服务器")
	ErrNoSyncDir        = errors.New("请先选择同步目录")
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

			// 检查忽略列表
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
	prefix     string
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

// SaveConfig 保存配置到文件
func SaveConfig(config *SyncConfig, filename string) error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	return nil
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*SyncConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// 返回默认配置
			return &SyncConfig{
				Host:       "0.0.0.0",
				Port:       6666,
				IgnoreList: []string{".clientconfig", ".DS_Store", "thumbs.db"},
				FolderRedirects: []FolderRedirect{
					{ServerPath: "clientmods", ClientPath: "mods"},
				},
			}, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config SyncConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &config, nil
}
