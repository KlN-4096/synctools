/*
文件作用:
- 实现日志记录功能
- 定义日志接口和实现
- 提供调试日志支持
- 管理日志输出格式

主要方法:
- NewDefaultLogger: 创建默认日志记录器
- NewCallbackLogger: 创建回调日志记录器
- Log: 记录普通日志
- Error: 记录错误日志
- DebugLog: 记录调试日志
- SetDebugMode: 设置调试模式
*/

package common

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Logger 日志记录器接口
type Logger interface {
	Log(format string, v ...interface{})
	Info(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
	DebugLog(format string, v ...interface{})
	SetDebugMode(enabled bool)
	GetDebugMode() bool
	Close() error
}

// DefaultLogger 基于文件的日志记录器
type DefaultLogger struct {
	DebugEnabled bool
	logFile      *os.File
	isClosing    bool
}

// CallbackLogger 基于回调的日志记录器
type CallbackLogger struct {
	callback     func(string)
	debugEnabled bool
	isClosing    bool
}

// NewDefaultLogger 创建基于文件的日志记录器
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		DebugEnabled: false,
		isClosing:    false,
	}
}

// NewCallbackLogger 创建基于回调的日志记录器
func NewCallbackLogger(callback func(string)) *CallbackLogger {
	return &CallbackLogger{
		callback:     callback,
		debugEnabled: false,
		isClosing:    false,
	}
}

// Log 记录普通日志
func (l *DefaultLogger) Log(format string, v ...interface{}) {
	l.writeLog("INFO", format, v...)
}

// Info 记录信息日志
func (l *DefaultLogger) Info(msg string, keyvals ...interface{}) {
	if l.isClosing {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] [INFO] %s", timestamp, msg)
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			logMsg += fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1])
		}
	}
	logMsg += "\n"

	if l.logFile != nil {
		l.logFile.WriteString(logMsg)
	}
	fmt.Print(logMsg)
}

// Error 记录错误日志
func (l *DefaultLogger) Error(msg string, keyvals ...interface{}) {
	if l.isClosing {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMsg := fmt.Sprintf("[%s] [ERROR] %s", timestamp, msg)
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			logMsg += fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1])
		}
	}
	logMsg += "\n"

	if l.logFile != nil {
		l.logFile.WriteString(logMsg)
	}
	fmt.Print(logMsg)
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

// Close 关闭日志记录器
func (l *DefaultLogger) Close() error {
	l.isClosing = true
	if l.logFile != nil {
		err := l.logFile.Close()
		l.logFile = nil
		return err
	}
	return nil
}

// writeLog 写入日志
func (l *DefaultLogger) writeLog(level string, format string, v ...interface{}) {
	if l.isClosing {
		return
	}

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

// Log 记录普通日志
func (l *CallbackLogger) Log(format string, v ...interface{}) {
	if l.isClosing {
		return
	}
	if l.callback != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		message := fmt.Sprintf(format, v...)
		l.callback(fmt.Sprintf("[%s] [INFO] %s", timestamp, message))
	}
}

// Info 记录信息日志
func (l *CallbackLogger) Info(msg string, keyvals ...interface{}) {
	if l.isClosing {
		return
	}
	if l.callback != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		logMsg := fmt.Sprintf("[%s] [INFO] %s", timestamp, msg)
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				logMsg += fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1])
			}
		}
		l.callback(logMsg)
	}
}

// Error 记录错误日志
func (l *CallbackLogger) Error(msg string, keyvals ...interface{}) {
	if l.isClosing {
		return
	}
	if l.callback != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		logMsg := fmt.Sprintf("[%s] [ERROR] %s", timestamp, msg)
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				logMsg += fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1])
			}
		}
		l.callback(logMsg)
	}
}

// DebugLog 记录调试日志
func (l *CallbackLogger) DebugLog(format string, v ...interface{}) {
	if l.isClosing || !l.debugEnabled {
		return
	}
	if l.callback != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		message := fmt.Sprintf(format, v...)
		l.callback(fmt.Sprintf("[%s] [DEBUG] %s", timestamp, message))
	}
}

// SetDebugMode 设置调试模式
func (l *CallbackLogger) SetDebugMode(enabled bool) {
	l.debugEnabled = enabled
}

// GetDebugMode 获取调试模式状态
func (l *CallbackLogger) GetDebugMode() bool {
	return l.debugEnabled
}

// Close 关闭日志记录器
func (l *CallbackLogger) Close() error {
	l.isClosing = true
	l.callback = nil
	return nil
}
