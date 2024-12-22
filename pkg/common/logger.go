package common

import (
	"fmt"
	"time"
)

// LogCallback 日志回调函数类型
type LogCallback func(string)

// Logger 日志记录器
type Logger struct {
	callback  LogCallback
	debugMode bool
	isClosing bool
}

// NewLogger 创建新的日志记录器
func NewLogger(callback LogCallback) *Logger {
	return &Logger{
		callback:  callback,
		debugMode: false,
		isClosing: false,
	}
}

// Log 记录普通日志
func (l *Logger) Log(format string, args ...interface{}) {
	if l.isClosing {
		return
	}
	if l.callback != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		message := fmt.Sprintf(format, args...)
		l.callback(fmt.Sprintf("[%s] %s", timestamp, message))
	}
}

// Error 记录错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	if l.isClosing {
		return
	}
	if l.callback != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		message := fmt.Sprintf(format, args...)
		l.callback(fmt.Sprintf("[%s] ERROR: %s", timestamp, message))
	}
}

// Debug 记录调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.isClosing || !l.debugMode {
		return
	}
	if l.callback != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		message := fmt.Sprintf(format, args...)
		l.callback(fmt.Sprintf("[%s] DEBUG: %s", timestamp, message))
	}
}

// DebugLog 记录调试日志（别名）
func (l *Logger) DebugLog(format string, args ...interface{}) {
	l.Debug(format, args...)
}

// SetDebugMode 设置调试模式
func (l *Logger) SetDebugMode(enabled bool) {
	l.debugMode = enabled
}

// GetDebugMode 获取调试模式状态
func (l *Logger) GetDebugMode() bool {
	return l.debugMode
}

// Close 关闭日志记录器
func (l *Logger) Close() {
	l.isClosing = true
}
