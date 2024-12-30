package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"synctools/codes/internal/interfaces"
)

// DefaultLogger 默认日志实现
type DefaultLogger struct {
	level     interfaces.LogLevel
	logger    *log.Logger
	fields    interfaces.Fields
	debugMode bool
}

// NewDefaultLogger 创建默认日志记录器
func NewDefaultLogger(logDir string) (*DefaultLogger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 创建日志文件
	logFile := filepath.Join(logDir, fmt.Sprintf("sync_%s.log", time.Now().Format("20060102")))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %v", err)
	}

	// 创建多输出的日志记录器
	multiWriter := io.MultiWriter(file, os.Stdout)
	logger := log.New(multiWriter, "", 0)

	return &DefaultLogger{
		level:     interfaces.INFO,
		logger:    logger,
		fields:    make(interfaces.Fields),
		debugMode: false,
	}, nil
}

// log 内部日志记录方法
func (l *DefaultLogger) log(level interfaces.LogLevel, msg string, fields interfaces.Fields) {
	if level < l.level {
		return
	}

	// 合并默认字段和传入的字段
	mergedFields := make(interfaces.Fields)
	for k, v := range l.fields {
		mergedFields[k] = v
	}
	for k, v := range fields {
		mergedFields[k] = v
	}

	// 构建字段字符串
	fieldStr := ""
	for k, v := range mergedFields {
		fieldStr += fmt.Sprintf(" %s=%v", k, v)
	}

	// 获取日志级别字符串
	levelStr := "INFO"
	switch level {
	case interfaces.DEBUG:
		levelStr = "DEBUG"
	case interfaces.WARN:
		levelStr = "WARN"
	case interfaces.ERROR:
		levelStr = "ERROR"
	case interfaces.FATAL:
		levelStr = "FATAL"
	}

	// 记录日志
	l.logger.Printf("%s [%s] %s%s",
		time.Now().Format("2006-01-02 15:04:05"),
		levelStr,
		msg,
		fieldStr,
	)

	// 如果是致命错误，则退出程序
	if level == interfaces.FATAL {
		os.Exit(1)
	}
}

// Debug 记录调试日志
func (l *DefaultLogger) Debug(msg string, fields interfaces.Fields) {
	l.log(interfaces.DEBUG, msg, fields)
}

// Info 记录信息日志
func (l *DefaultLogger) Info(msg string, fields interfaces.Fields) {
	l.log(interfaces.INFO, msg, fields)
}

// Warn 记录警告日志
func (l *DefaultLogger) Warn(msg string, fields interfaces.Fields) {
	l.log(interfaces.WARN, msg, fields)
}

// Error 记录错误日志
func (l *DefaultLogger) Error(msg string, fields interfaces.Fields) {
	l.log(interfaces.ERROR, msg, fields)
}

// Fatal 记录致命错误日志
func (l *DefaultLogger) Fatal(msg string, fields interfaces.Fields) {
	l.log(interfaces.FATAL, msg, fields)
}

// WithFields 添加字段
func (l *DefaultLogger) WithFields(fields interfaces.Fields) interfaces.Logger {
	newLogger := &DefaultLogger{
		level:     l.level,
		logger:    l.logger,
		fields:    make(interfaces.Fields),
		debugMode: l.debugMode,
	}
	// 复制现有字段
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	// 添加新字段
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// SetLevel 设置日志级别
func (l *DefaultLogger) SetLevel(level interfaces.LogLevel) {
	l.level = level
}

// GetLevel 获取日志级别
func (l *DefaultLogger) GetLevel() interfaces.LogLevel {
	return l.level
}

// LoggerAdapter UI层日志适配器
type LoggerAdapter struct {
	logger interfaces.Logger
}

// NewLoggerAdapter 创建日志适配器
func NewLoggerAdapter(logger interfaces.Logger) *LoggerAdapter {
	return &LoggerAdapter{
		logger: logger,
	}
}

// Log 记录普通日志（UI特有）
func (l *LoggerAdapter) Log(format string, v ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, v...), nil)
}

// DebugLog 记录格式化的调试日志（UI特有）
func (l *LoggerAdapter) DebugLog(format string, v ...interface{}) {
	if l.logger.GetDebugMode() {
		l.logger.Debug(fmt.Sprintf(format, v...), nil)
	}
}

// SetDebugMode 设置调试模式
func (l *DefaultLogger) SetDebugMode(enabled bool) {
	l.debugMode = enabled
	if enabled {
		l.level = interfaces.DEBUG
	} else {
		l.level = interfaces.INFO
	}
}

// GetDebugMode 获取调试模式状态
func (l *DefaultLogger) GetDebugMode() bool {
	return l.debugMode
}
