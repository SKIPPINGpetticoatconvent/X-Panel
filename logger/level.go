package logger

import "log/slog"

// Level 定义日志级别，用于替代 go-logging 的 Level 类型
type Level int

const (
	// DEBUG 调试级别日志
	DEBUG Level = iota
	// INFO 信息级别日志
	INFO
	// NOTICE 通知级别日志
	NOTICE
	// WARNING 警告级别日志
	WARNING
	// ERROR 错误级别日志
	ERROR
	// CRITICAL 严重错误级别日志
	CRITICAL
)

// String 返回级别的字符串表示
func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case NOTICE:
		return "NOTICE"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case CRITICAL:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// ToSlogLevel 将 Level 转换为 slog.Level
func (l Level) ToSlogLevel() slog.Level {
	switch l {
	case DEBUG:
		return slog.LevelDebug
	case INFO, NOTICE:
		return slog.LevelInfo
	case WARNING:
		return slog.LevelWarn
	case ERROR, CRITICAL:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ParseLevel 从字符串解析日志级别
func ParseLevel(s string) Level {
	switch s {
	case "DEBUG", "debug":
		return DEBUG
	case "INFO", "info":
		return INFO
	case "NOTICE", "notice":
		return NOTICE
	case "WARNING", "warning", "WARN", "warn":
		return WARNING
	case "ERROR", "error":
		return ERROR
	case "CRITICAL", "critical":
		return CRITICAL
	default:
		return WARNING
	}
}

// LevelFromSlog 从 slog.Level 转换为 Level
func LevelFromSlog(l slog.Level) Level {
	switch {
	case l <= slog.LevelDebug:
		return DEBUG
	case l <= slog.LevelInfo:
		return INFO
	case l <= slog.LevelWarn:
		return WARNING
	default:
		return ERROR
	}
}
