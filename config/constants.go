package config

import "time"

// =================================================================
// 设备限制相关常量
// =================================================================

const (
	// MaxIPsPerEmail 单个用户最多跟踪的 IP 数
	MaxIPsPerEmail = 100

	// MaxTotalEmails 最多跟踪的用户数
	MaxTotalEmails = 5000

	// DeviceLimitCheckInterval 设备限制检查间隔
	DeviceLimitCheckInterval = 30 * time.Second

	// DeviceLimitActiveTTL 活跃判断窗口(TTL): 近3分钟内出现过就算"活跃"
	DeviceLimitActiveTTL = 3 * time.Minute

	// DeviceLimitStopTimeout 设备限制任务停止超时时间
	DeviceLimitStopTimeout = 10 * time.Second

	// DeviceLimitOperationDelay 封禁/解封操作延时，解决竞态条件问题
	DeviceLimitOperationDelay = 5 * time.Second
)

// =================================================================
// Telegram Bot 相关常量
// =================================================================

const (
	// TelegramMessageDelay 发送 Telegram 消息的间隔延时
	TelegramMessageDelay = 500 * time.Millisecond

	// TelegramPanelRestartWait 面板重启后等待时间
	TelegramPanelRestartWait = 20 * time.Second

	// TelegramNotifyDelay 一键配置通知延时
	TelegramNotifyDelay = 5 * time.Second
)

// =================================================================
// 网络相关常量
// =================================================================

const (
	// HTTPSRedirectDelay HTTPS 重定向后关闭连接的延时
	HTTPSRedirectDelay = 500 * time.Millisecond
)

// =================================================================
// 日志相关常量
// =================================================================

const (
	// LogBufferSize 日志缓冲区大小
	LogBufferSize = 100

	// LogFlushInterval 日志刷新间隔
	LogFlushInterval = 5 * time.Second
)
