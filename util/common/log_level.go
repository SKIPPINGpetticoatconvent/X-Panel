package common

import (
	"x-ui/logger"
)

// ParseLogLevel 将字符串日志级别解析为 logger.Level
// 支持的级别: error, warn, warning, info, debug
// 默认返回 WARNING 级别
func ParseLogLevel(levelStr string) logger.Level {
	switch levelStr {
	case "error":
		return logger.ERROR
	case "warn", "warning":
		return logger.WARNING
	case "info":
		return logger.INFO
	case "debug":
		return logger.DEBUG
	default:
		return logger.WARNING
	}
}

// LogLevelToString 将 logger.Level 转换为字符串
func LogLevelToString(level logger.Level) string {
	switch level {
	case logger.ERROR:
		return "error"
	case logger.WARNING:
		return "warn"
	case logger.INFO:
		return "info"
	case logger.DEBUG:
		return "debug"
	default:
		return "warn"
	}
}
