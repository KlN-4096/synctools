package errors

// Service errors
var (
	ErrServiceStart = &Error{
		Code:    "SERVICE_001",
		Message: "服务启动失败",
	}

	ErrServiceStop = &Error{
		Code:    "SERVICE_002",
		Message: "服务停止失败",
	}

	ErrServiceNotRunning = &Error{
		Code:    "SERVICE_003",
		Message: "服务未运行",
	}

	ErrServiceBusy = &Error{
		Code:    "SERVICE_004",
		Message: "服务繁忙",
	}

	ErrServiceSync = &Error{
		Code:    "SERVICE_005",
		Message: "同步失败",
	}
)
