package model

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// DefaultLogger 默认日志记录器
type DefaultLogger struct {
	DebugEnabled bool
	logFile      *os.File
}

// Log 记录普通日志
func (l *DefaultLogger) Log(format string, v ...interface{}) {
	l.writeLog("INFO", format, v...)
}

// Info 记录信息日志
func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	l.writeLog("INFO", msg, args...)
}

// Error 记录错误日志
func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	l.writeLog("ERROR", msg, args...)
}

// DebugLog 记录调试日志
func (l *DefaultLogger) DebugLog(format string, v ...interface{}) {
	if l.DebugEnabled {
		l.writeLog("DEBUG", format, v...)
	}
}

// SetDebugMode 设置调试模式
func (l *DefaultLogger) SetDebugMode(enabled bool) {
	l.DebugEnabled = enabled
}

// GetDebugMode 获取调试模式状态
func (l *DefaultLogger) GetDebugMode() bool {
	return l.DebugEnabled
}

// writeLog 写入日志
func (l *DefaultLogger) writeLog(level string, format string, v ...interface{}) {
	// 确保日志目录存在
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("创建日志目录失败: %v", err)
		return
	}

	// 如果日志文件未打开，则打开或创建
	if l.logFile == nil {
		logPath := filepath.Join(logDir, fmt.Sprintf("server_%s.log",
			time.Now().Format("2006-01-02")))
		file, err := os.OpenFile(logPath,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0644)
		if err != nil {
			log.Printf("打开日志文件失败: %v", err)
			return
		}
		l.logFile = file
	}

	// 格式化日志消息
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, v...)
	logMsg := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	// 写入日志文件
	if _, err := l.logFile.WriteString(logMsg); err != nil {
		log.Printf("写入日志失败: %v", err)
	}

	// 同时输出到控制台
	fmt.Print(logMsg)
}
