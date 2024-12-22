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

// NewDefaultLogger 创建新的默认日志记录器
func NewDefaultLogger(logDir string) (*DefaultLogger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}

	logFile := filepath.Join(logDir, fmt.Sprintf("sync_%s.log", time.Now().Format("20060102")))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %v", err)
	}

	// 创建一个同时写入文件和标准输出的writer
	multiWriter := io.MultiWriter(file, os.Stdout)

	return &DefaultLogger{
		level:  interfaces.INFO,
		logger: log.New(multiWriter, "", log.LstdFlags),
		fields: make(interfaces.Fields),
	}, nil
}

func (l *DefaultLogger) log(level interfaces.LogLevel, msg string, fields interfaces.Fields) {
	if level < l.level {
		return
	}

	mergedFields := make(interfaces.Fields)
	for k, v := range l.fields {
		mergedFields[k] = v
	}
	for k, v := range fields {
		mergedFields[k] = v
	}

	fieldStr := ""
	for k, v := range mergedFields {
		fieldStr += fmt.Sprintf(" %s=%v", k, v)
	}

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

	l.logger.Printf("[%s] %s%s", levelStr, msg, fieldStr)
}

func (l *DefaultLogger) Debug(msg string, fields interfaces.Fields) {
	l.log(interfaces.DEBUG, msg, fields)
}

func (l *DefaultLogger) Info(msg string, fields interfaces.Fields) {
	l.log(interfaces.INFO, msg, fields)
}

func (l *DefaultLogger) Warn(msg string, fields interfaces.Fields) {
	l.log(interfaces.WARN, msg, fields)
}

func (l *DefaultLogger) Error(msg string, fields interfaces.Fields) {
	l.log(interfaces.ERROR, msg, fields)
}

func (l *DefaultLogger) Fatal(msg string, fields interfaces.Fields) {
	l.log(interfaces.FATAL, msg, fields)
	os.Exit(1)
}

func (l *DefaultLogger) WithFields(fields interfaces.Fields) interfaces.Logger {
	newFields := make(interfaces.Fields)
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &DefaultLogger{
		level:  l.level,
		logger: l.logger,
		fields: newFields,
	}
}

func (l *DefaultLogger) SetLevel(level interfaces.LogLevel) {
	l.level = level
}

// GetLevel 获取当前日志级别
func (l *DefaultLogger) GetLevel() interfaces.LogLevel {
	return l.level
}
