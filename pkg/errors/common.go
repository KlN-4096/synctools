package errors

import "fmt"

// Error 自定义错误类型
type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewError 创建新的错误
func NewError(code string, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Common errors
var (
	ErrInternal = &Error{Code: "INTERNAL", Message: "内部错误"}
	ErrInvalid  = &Error{Code: "INVALID", Message: "无效的参数"}
	ErrNotFound = &Error{Code: "NOT_FOUND", Message: "资源未找到"}
)
