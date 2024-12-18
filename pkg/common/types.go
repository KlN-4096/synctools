package common

import (
	"fmt"
)

// SyncConfig 同步配置
type SyncConfig struct {
	Name       string   `json:"name"`        // 整合包名称
	Version    string   `json:"version"`     // 整合包版本
	Host       string   `json:"host"`        // 服务器主机地址
	Port       int      `json:"port"`        // 服务器端口
	SyncDir    string   `json:"sync_dir"`    // 同步目录
	IgnoreList []string `json:"ignore_list"` // 忽略文件列表
}

// SyncStatus 同步状态
type SyncStatus struct {
	Connected bool   // 是否已连接
	Running   bool   // 是否正在运行
	Message   string // 状态消息
}

// FileInfo 文件信息
type FileInfo struct {
	Hash string `json:"hash"` // 文件哈希值
	Size int64  `json:"size"` // 文件大小
}

// SyncInfo 同步信息
type SyncInfo struct {
	Files            map[string]FileInfo `json:"files"`              // 文件信息映射
	DeleteExtraFiles bool                `json:"delete_extra_files"` // 是否删除多余文件
	SyncMode         string              `json:"sync_mode"`          // 同步模式
}

// GUILogger GUI日志记录器
type GUILogger struct {
	debugEnabled bool
	logFunc      func(string)
}

// NewGUILogger 创建新的GUI日志记录器
func NewGUILogger(logFunc func(string)) *GUILogger {
	return &GUILogger{
		logFunc: logFunc,
	}
}

// Log 记录日志
func (l *GUILogger) Log(format string, v ...interface{}) {
	if l.logFunc != nil {
		l.logFunc(fmt.Sprintf(format, v...))
	}
}

// SetDebugMode 设置调试模式
func (l *GUILogger) SetDebugMode(enabled bool) {
	l.debugEnabled = enabled
}

// GetDebugMode 获取调试模式状态
func (l *GUILogger) GetDebugMode() bool {
	return l.debugEnabled
}

// DebugLog 记录调试日志
func (l *GUILogger) DebugLog(format string, v ...interface{}) {
	if l.debugEnabled && l.logFunc != nil {
		l.logFunc(fmt.Sprintf("[DEBUG] "+format, v...))
	}
}

// Error 记录错误日志
func (l *GUILogger) Error(format string, v ...interface{}) {
	if l.logFunc != nil {
		l.logFunc(fmt.Sprintf("[ERROR] "+format, v...))
	}
}

// Close 关闭日志记录器
func (l *GUILogger) Close() error {
	l.logFunc = nil
	return nil
}

// 错误定义
var (
	ErrNotConnected = fmt.Errorf("未连接到服务器")
	ErrNoSyncDir    = fmt.Errorf("未设置同步目录")
)
