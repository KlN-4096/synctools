package errors

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
