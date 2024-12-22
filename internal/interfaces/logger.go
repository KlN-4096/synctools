package interfaces

// LogLevel 定义日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// Fields 定义日志字段类型
type Fields map[string]interface{}

// Logger 定义日志接口
type Logger interface {
	Debug(msg string, fields Fields)
	Info(msg string, fields Fields)
	Warn(msg string, fields Fields)
	Error(msg string, fields Fields)
	Fatal(msg string, fields Fields)
	WithFields(fields Fields) Logger
	SetLevel(level LogLevel)
}
