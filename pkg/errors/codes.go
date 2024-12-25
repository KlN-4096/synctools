package errors

// Common errors
var (
	ErrInternal = &Error{Code: "INTERNAL", Message: "内部错误"}
	ErrInvalid  = &Error{Code: "INVALID", Message: "无效的参数"}
	ErrNotFound = &Error{Code: "NOT_FOUND", Message: "资源未找到"}
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

// Config errors
var (
	ErrConfigNotFound = &Error{
		Code:    "CONFIG_001",
		Message: "配置未找到",
	}

	ErrInvalidConfig = &Error{
		Code:    "CONFIG_002",
		Message: "无效的配置",
	}

	ErrConfigExists = &Error{
		Code:    "CONFIG_003",
		Message: "配置已存在",
	}

	ErrConfigSave = &Error{
		Code:    "CONFIG_004",
		Message: "保存配置失败",
	}

	ErrConfigLoad = &Error{
		Code:    "CONFIG_005",
		Message: "加载配置失败",
	}
)

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

// Storage errors
var (
	ErrStorageNotFound = &Error{
		Code:    "STORAGE_001",
		Message: "存储项未找到",
	}

	ErrStorageExists = &Error{
		Code:    "STORAGE_002",
		Message: "存储项已存在",
	}

	ErrStorageSave = &Error{
		Code:    "STORAGE_003",
		Message: "保存存储项失败",
	}

	ErrStorageLoad = &Error{
		Code:    "STORAGE_004",
		Message: "加载存储项失败",
	}

	ErrStorageDelete = &Error{
		Code:    "STORAGE_005",
		Message: "删除存储项失败",
	}
)
