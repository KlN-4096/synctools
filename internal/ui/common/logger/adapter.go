/*
文件作用:
- 提供UI层通用的日志适配器
- 实现日志接口的包装
- 提供调试模式支持
- 统一日志格式化

主要类型:
- Logger: UI层通用日志接口
- LoggerAdapter: 日志适配器实现
*/

package logger

import (
	"fmt"
	"synctools/internal/interfaces"
)

// Logger UI层通用日志接口
type Logger interface {
	Debug(message string, fields interfaces.Fields)
	Info(message string, fields interfaces.Fields)
	Warn(message string, fields interfaces.Fields)
	Error(message string, fields interfaces.Fields)
	Fatal(message string, fields interfaces.Fields)
	DebugLog(format string, args ...interface{})
}

// LoggerAdapter 日志适配器
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
