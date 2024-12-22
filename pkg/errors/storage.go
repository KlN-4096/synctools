package errors

import "fmt"

// StorageError 存储错误类型
type StorageError struct {
	Op      string // 操作名称
	Message string // 错误消息
	Err     error  // 原始错误
}

// Error 实现error接口
func (e *StorageError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// NewStorageError 创建新的存储错误
func NewStorageError(op, message string, err error) *StorageError {
	return &StorageError{
		Op:      op,
		Message: message,
		Err:     err,
	}
}
