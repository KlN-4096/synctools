package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"synctools/internal/interfaces"
)

// DefaultLogger 默认日志实现
type DefaultLogger struct {
	level  interfaces.LogLevel
	logger *log.Logger
	fields interfaces.Fields
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
		level:  interfaces.INFO,
		logger: logger,
		fields: make(interfaces.Fields),
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
		level:  l.level,
		logger: l.logger,
		fields: make(interfaces.Fields),
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
	logger    interfaces.Logger
	debugMode bool
}

// NewLoggerAdapter 创建日志适配器
func NewLoggerAdapter(logger interfaces.Logger) *LoggerAdapter {
	adapter := &LoggerAdapter{
		logger:    logger,
		debugMode: logger.GetLevel() == interfaces.DEBUG,
	}
	return adapter
}

// Debug 记录调试日志
func (l *LoggerAdapter) Debug(message string, fields interfaces.Fields) {
	l.logger.Debug(message, fields)
}

// Info 记录信息日志
func (l *LoggerAdapter) Info(message string, fields interfaces.Fields) {
	l.logger.Info(message, fields)
}

// Warn 记录警告日志
func (l *LoggerAdapter) Warn(message string, fields interfaces.Fields) {
	l.logger.Warn(message, fields)
}

// Error 记录错误日志
func (l *LoggerAdapter) Error(message string, fields interfaces.Fields) {
	l.logger.Error(message, fields)
}

// Fatal 记录致命错误日志
func (l *LoggerAdapter) Fatal(message string, fields interfaces.Fields) {
	l.logger.Fatal(message, fields)
}

// WithFields 添加字段
func (l *LoggerAdapter) WithFields(fields interfaces.Fields) interfaces.Logger {
	return l.logger.WithFields(fields)
}

// SetLevel 设置日志级别
func (l *LoggerAdapter) SetLevel(level interfaces.LogLevel) {
	l.logger.SetLevel(level)
	l.debugMode = level == interfaces.DEBUG
}

// GetLevel 获取日志级别
func (l *LoggerAdapter) GetLevel() interfaces.LogLevel {
	return l.logger.GetLevel()
}

// Log 记录普通日志
func (l *LoggerAdapter) Log(format string, v ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, v...), nil)
}

// DebugLog 记录格式化的调试日志
func (l *LoggerAdapter) DebugLog(format string, v ...interface{}) {
	if l.debugMode {
		l.logger.Debug(fmt.Sprintf(format, v...), nil)
	}
}

// SetDebugMode 设置调试模式
func (l *LoggerAdapter) SetDebugMode(enabled bool) {
	l.debugMode = enabled
	if enabled {
		l.logger.SetLevel(interfaces.DEBUG)
	} else {
		l.logger.SetLevel(interfaces.INFO)
	}
}

// GetDebugMode 获取调试模式状态
func (l *LoggerAdapter) GetDebugMode() bool {
	return l.debugMode
}
