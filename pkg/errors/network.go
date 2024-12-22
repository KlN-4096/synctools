/*
文件作用:
- 定义网络相关错误类型
- 实现网络错误处理
- 提供网络错误创建和判断功能
- 实现net.Error接口

主要内容:
- NetworkError: 网络错误类型,实现error和net.Error接口
- NewNetworkError: 创建新的网络错误
- Error: 获取错误信息
- IsTimeout: 判断是否为超时错误
- IsTemporary: 判断是否为临时错误

预定义错误:
- ErrNetworkConnect: 网络连接失败
- ErrNetworkTimeout: 网络超时
- ErrNetworkClosed: 网络连接已关闭
- ErrNetworkInvalidData: 无效的网络数据
- ErrNetworkServerStart: 服务器启动失败
*/

package errors

import (
	"fmt"
	"net"
)

// Network errors
var (
	ErrNetworkConnect = &Error{
		Code:    "NETWORK_001",
		Message: "网络连接失败",
	}

	ErrNetworkTimeout = &Error{
		Code:    "NETWORK_002",
		Message: "网络超时",
	}

	ErrNetworkClosed = &Error{
		Code:    "NETWORK_003",
		Message: "网络连接已关闭",
	}

	ErrNetworkInvalidData = &Error{
		Code:    "NETWORK_004",
		Message: "无效的网络数据",
	}

	ErrNetworkServerStart = &Error{
		Code:    "NETWORK_005",
		Message: "服务器启动失败",
	}
)

// NetworkError 网络错误类型
type NetworkError struct {
	Op      string // 操作名称
	Message string // 错误消息
	Err     error  // 原始错误
}

// Error 实现error接口
func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// IsTimeout 实现net.Error接口
func (e *NetworkError) IsTimeout() bool {
	if netErr, ok := e.Err.(net.Error); ok {
		return netErr.Timeout()
	}
	return false
}

// IsTemporary 实现net.Error接口
func (e *NetworkError) IsTemporary() bool {
	if netErr, ok := e.Err.(net.Error); ok {
		return netErr.Temporary()
	}
	return false
}

// NewNetworkError 创建新的网络错误
func NewNetworkError(op, message string, err error) *NetworkError {
	return &NetworkError{
		Op:      op,
		Message: message,
		Err:     err,
	}
}
