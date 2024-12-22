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

// LineEdit 文本输入框接口
type LineEdit interface {
	Text() string
	SetText(text string) error
}

// NumberEdit 数字输入框接口
type NumberEdit interface {
	Value() float64
	SetValue(value float64) error
}

// TableView 表格视图接口
type TableView interface {
	Model() interface{}
	SetModel(model interface{}) error
	CurrentIndex() int
}

// TableModel 表格模型接口
type TableModel interface {
	RowCount() int
	Value(row, col int) interface{}
	RowChanged(row int)
	RowsChanged()
	Sort(col int, order walk.SortOrder) error
}

// MainWindow 主窗口接口
type MainWindow interface {
	MsgBox(title, message string, style walk.MsgBoxStyle) (int, error)
}

// Logger 视图模型日志接口
type Logger interface {
	interfaces.Logger
	SetDebugMode(enabled bool)
	GetDebugMode() bool
}

// LoggerAdapter 日志适配器
type LoggerAdapter struct {
	interfaces.Logger
	debugMode bool
}

// NewLoggerAdapter 创建新的日志适配器
func NewLoggerAdapter(logger interfaces.Logger) Logger {
	return &LoggerAdapter{
		Logger:    logger,
		debugMode: false,
	}
}

// SetDebugMode 设置调试模式
func (l *LoggerAdapter) SetDebugMode(enabled bool) {
	l.debugMode = enabled
	if enabled {
		l.Logger.SetLevel(interfaces.DEBUG)
	} else {
		l.Logger.SetLevel(interfaces.INFO)
	}
}

// GetDebugMode 获取调试模式状态
func (l *LoggerAdapter) GetDebugMode() bool {
	return l.debugMode
}

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
	Item(index int) interface{}
	ItemsReset()
	ItemChanged(index int)
	ItemsInserted(from, count int)
	ItemsRemoved(from, count int)
}

// ItemViewModel 列表项视图模型接口
type ItemViewModel interface {
	ID() string
	Title() string
	Description() string
	IsSelected() bool
	SetSelected(selected bool)
}

// WindowAdapter walk.MainWindow 适配器
type WindowAdapter struct {
	*walk.MainWindow
}

// NewWindowAdapter 创建新的窗口适配器
func NewWindowAdapter(window *walk.MainWindow) MainWindow {
	return &WindowAdapter{MainWindow: window}
}

// MsgBox 显示消息框
func (w *WindowAdapter) MsgBox(title, message string, style walk.MsgBoxStyle) (int, error) {
	result := walk.MsgBox(w.MainWindow, title, message, style)
	return result, nil
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

	vm.SetMainWindow(NewWindowAdapter(mainWindow))

	result := mainWindow.Run()
	if result != 0 {
		return fmt.Errorf("窗口运行异常，返回值: %d", result)
	}

	return nil
}
