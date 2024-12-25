package errors

import (
	"fmt"
	"net"
)

// 错误码常量定义
const (
	// Common error codes
	CodeInternal = "INTERNAL"
	CodeInvalid  = "INVALID"
	CodeNotFound = "NOT_FOUND"

	// Network error codes
	CodeNetworkConnect     = "NETWORK_001"
	CodeNetworkTimeout     = "NETWORK_002"
	CodeNetworkClosed      = "NETWORK_003"
	CodeNetworkInvalidData = "NETWORK_004"
	CodeNetworkServerStart = "NETWORK_005"

	// Config error codes
	CodeConfigNotFound = "CONFIG_001"
	CodeInvalidConfig  = "CONFIG_002"
	CodeConfigExists   = "CONFIG_003"
	CodeConfigSave     = "CONFIG_004"
	CodeConfigLoad     = "CONFIG_005"

	// Service error codes
	CodeServiceStart      = "SERVICE_001"
	CodeServiceStop       = "SERVICE_002"
	CodeServiceNotRunning = "SERVICE_003"
	CodeServiceBusy       = "SERVICE_004"
	CodeServiceSync       = "SERVICE_005"

	// Storage error codes
	CodeStorageNotFound = "STORAGE_001"
	CodeStorageExists   = "STORAGE_002"
	CodeStorageSave     = "STORAGE_003"
	CodeStorageLoad     = "STORAGE_004"
	CodeStorageDelete   = "STORAGE_005"
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

// 预定义错误实例
var (
	// Common errors
	ErrInternal = &Error{Code: CodeInternal, Message: "内部错误"}
	ErrInvalid  = &Error{Code: CodeInvalid, Message: "无效的参数"}
	ErrNotFound = &Error{Code: CodeNotFound, Message: "资源未找到"}

	// Network errors
	ErrNetworkConnect = &Error{
		Code:    CodeNetworkConnect,
		Message: "网络连接失败",
	}

	ErrNetworkTimeout = &Error{
		Code:    CodeNetworkTimeout,
		Message: "网络超时",
	}

	ErrNetworkClosed = &Error{
		Code:    CodeNetworkClosed,
		Message: "网络连接已关闭",
	}

	ErrNetworkInvalidData = &Error{
		Code:    CodeNetworkInvalidData,
		Message: "无效的网络数据",
	}

	ErrNetworkServerStart = &Error{
		Code:    CodeNetworkServerStart,
		Message: "服务器启动失败",
	}

	// Config errors
	ErrConfigNotFound = &Error{
		Code:    CodeConfigNotFound,
		Message: "配置未找到",
	}

	ErrInvalidConfig = &Error{
		Code:    CodeInvalidConfig,
		Message: "无效的配置",
	}

	ErrConfigExists = &Error{
		Code:    CodeConfigExists,
		Message: "配置已存在",
	}

	ErrConfigSave = &Error{
		Code:    CodeConfigSave,
		Message: "保存配置失败",
	}

	ErrConfigLoad = &Error{
		Code:    CodeConfigLoad,
		Message: "加载配置失败",
	}

	// Service errors
	ErrServiceStart = &Error{
		Code:    CodeServiceStart,
		Message: "服务启动失败",
	}

	ErrServiceStop = &Error{
		Code:    CodeServiceStop,
		Message: "服务停止失败",
	}

	ErrServiceNotRunning = &Error{
		Code:    CodeServiceNotRunning,
		Message: "服务未运行",
	}

	ErrServiceBusy = &Error{
		Code:    CodeServiceBusy,
		Message: "服务繁忙",
	}

	ErrServiceSync = &Error{
		Code:    CodeServiceSync,
		Message: "同步失败",
	}

	// Storage errors
	ErrStorageNotFound = &Error{
		Code:    CodeStorageNotFound,
		Message: "存储项未找到",
	}

	ErrStorageExists = &Error{
		Code:    CodeStorageExists,
		Message: "存储项已存在",
	}

	ErrStorageSave = &Error{
		Code:    CodeStorageSave,
		Message: "保存存储项失败",
	}

	ErrStorageLoad = &Error{
		Code:    CodeStorageLoad,
		Message: "加载存储项失败",
	}

	ErrStorageDelete = &Error{
		Code:    CodeStorageDelete,
		Message: "删除存储项失败",
	}
)

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
