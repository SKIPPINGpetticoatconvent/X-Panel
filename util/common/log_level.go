package common

import (
	logging "github.com/op/go-logging"
)

// ParseLogLevel 将字符串日志级别解析为 logging.Level
// 支持的级别: error, warn, warning, info, debug
// 默认返回 WARNING 级别
func ParseLogLevel(levelStr string) logging.Level {
	switch levelStr {
	case "error":
		return logging.ERROR
	case "warn", "warning":
		return logging.WARNING
	case "info":
		return logging.INFO
	case "debug":
		return logging.DEBUG
	default:
		return logging.WARNING
	}
}

// LogLevelToString 将 logging.Level 转换为字符串
func LogLevelToString(level logging.Level) string {
	switch level {
	case logging.ERROR:
		return "error"
	case logging.WARNING:
		return "warn"
	case logging.INFO:
		return "info"
	case logging.DEBUG:
		return "debug"
	default:
		return "warn"
	}
}
