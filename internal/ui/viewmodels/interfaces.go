package viewmodels

import (
	"fmt"
	"synctools/internal/interfaces"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
)

/*
文件作用:
- 定义视图模型接口
- 定义数据绑定接口
- 定义事件处理接口
- 提供通用接口约束

主要接口:
- ViewModel: 视图模型基础接口
- DataBinder: 数据绑定接口
- EventHandler: 事件处理接口
- ListModel: 列表模型接口
- ItemViewModel: 列表项视图模型接口
*/

// ViewModel 视图模型基础接口
type ViewModel interface {
	Initialize() error
	Shutdown() error
}

// DataBinder 数据绑定接口
type DataBinder interface {
	Bind(target interface{}) error
	Reset() error
	Submit() error
}

// EventHandler 事件处理接口
type EventHandler interface {
	HandleEvent(eventName string, data interface{}) error
}

// ListModel 列表模型接口
type ListModel interface {
	ItemCount() int
	Value(index int) interface{}
}

// ItemViewModel 列表项视图模型接口
type ItemViewModel interface {
	GetText() string
	GetIcon() interface{}
}

// Logger 日志接口
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

// CreateMainWindow 创建主窗口
func CreateMainWindow(vm *MainViewModel) error {
	var mainWindow *walk.MainWindow

	if err := (declarative.MainWindow{
		AssignTo: &mainWindow,
		Title:    "SyncTools Client",
		MinSize:  declarative.Size{Width: 600, Height: 400},
		Layout:   declarative.VBox{},
		Children: []declarative.Widget{
			// TODO: 添加UI组件
		},
	}.Create()); err != nil {
		return fmt.Errorf("创建主窗口失败: %v", err)
	}

	if err := vm.Initialize(mainWindow); err != nil {
		return fmt.Errorf("初始化视图模型失败: %v", err)
	}

	result := mainWindow.Run()
	if result != 0 {
		return fmt.Errorf("窗口运行异常，返回值: %d", result)
	}

	return nil
}
