package errors

import (
	"fmt"
	"net"
)

// Error 基础错误类型
type Error struct {
	Code    string // 错误代码
	Message string // 错误消息
	Cause   error  // 原始错误
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NetworkError 网络错误类型
type NetworkError struct {
	Op      string // 操作名称
	Message string // 错误消息
	Err     error  // 原始错误
}

func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *NetworkError) IsTimeout() bool {
	if netErr, ok := e.Err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

func (e *NetworkError) IsTemporary() bool {
	if netErr, ok := e.Err.(net.Error); ok {
		return netErr.Temporary()
	}
	return false
}

// StorageError 存储错误类型
type StorageError struct {
	Op      string // 操作名称
	Message string // 错误消息
	Err     error  // 原始错误
}

func (e *StorageError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// 错误创建函数
func NewError(code string, message string, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func NewNetworkError(op, message string, err error) *NetworkError {
	return &NetworkError{
		Op:      op,
		Message: message,
		Err:     err,
	}
}

func NewStorageError(op, message string, err error) *StorageError {
	return &StorageError{
		Op:      op,
		Message: message,
		Err:     err,
	}
}

// 错误判断函数
func IsNetworkError(err error) bool {
	_, ok := err.(*NetworkError)
	return ok
}

func IsStorageError(err error) bool {
	_, ok := err.(*StorageError)
	return ok
}

func IsTimeout(err error) bool {
	if netErr, ok := err.(*NetworkError); ok {
		return netErr.IsTimeout()
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

func IsTemporary(err error) bool {
	if netErr, ok := err.(*NetworkError); ok {
		return netErr.IsTemporary()
	}
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary()
	}
	return false
}
