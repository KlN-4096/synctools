package viewmodels

import "github.com/lxn/walk"

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
	MsgBox(title, message string, style walk.MsgBoxStyle) int
}

// Logger 日志记录器接口
type Logger interface {
	Log(format string, v ...interface{})
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	DebugLog(format string, v ...interface{})
	SetDebugMode(enabled bool)
	GetDebugMode() bool
}
