package service

import (
	"strings"
	"testing"

	"github.com/op/go-logging"
)

// 模拟结构体来测试核心逻辑
type mockLogForwarder struct {
	isEnabled bool
}

func (m *mockLogForwarder) shouldSkipLog(message, formattedLog string) bool {
	// 跳过 DEBUG 级别日志
	if strings.Contains(formattedLog, "DEBUG") {
		return true
	}

	// 跳过与 Telegram Bot 相关的日志，避免死循环
	if strings.Contains(message, "Telegram") ||
		strings.Contains(message, "telegram") ||
		strings.Contains(message, "bot") ||
		strings.Contains(message, "Bot") ||
		strings.Contains(message, "SendMsgToTgbot") ||
		strings.Contains(message, "SendMessage") {
		return true
	}

	// 跳过与日志转发器本身相关的日志
	if strings.Contains(message, "LogForwarder") ||
		strings.Contains(message, "日志转发") {
		return true
	}

	// 跳过一些频繁的、无意义的日志
	if strings.Contains(message, "checkpoint") ||
		strings.Contains(message, "database") ||
		strings.Contains(message, "DB") {
		return true
	}

	return false
}

func (m *mockLogForwarder) isImportantInfo(message string) bool {
	importantKeywords := []string{
		"started",
		"stopped",
		"running",
		"failed",
		"error",
		"restart",
		"shutdown",
		"connected",
		"disconnected",
		"login",
		"logout",
	}

	messageLower := strings.ToLower(message)
	for _, keyword := range importantKeywords {
		if strings.Contains(messageLower, keyword) {
			return true
		}
	}

	return false
}

func (m *mockLogForwarder) formatLogMessage(level logging.Level, message string) string {
	formatted := level.String() + " - " + message
	
	// 只转发 ERROR、WARNING 和 INFO 级别
	switch level {
	case logging.ERROR:
		return "🚨 <b>ERROR</b>\n" + formatted
	case logging.WARNING:
		return "⚠️ <b>WARNING</b>\n" + formatted
	case logging.INFO:
		// INFO 级别只转发重要的消息
		if m.isImportantInfo(message) {
			return "ℹ️ <b>INFO</b>\n" + formatted
		}
	}

	return ""
}

func TestLogForwarderCore_shouldSkipLog(t *testing.T) {
	m := &mockLogForwarder{isEnabled: true}

	tests := []struct {
		name     string
		message  string
		formatted string
		expected bool
	}{
		{
			name:     "Skip DEBUG level",
			message:  "This is a debug message",
			formatted: "DEBUG - This is a debug message",
			expected: true,
		},
		{
			name:     "Skip Telegram related message",
			message:  "Sending message to Telegram bot",
			formatted: "INFO - Sending message to Telegram bot",
			expected: true,
		},
		{
			name:     "Skip lowercase telegram",
			message:  "telegram connection established",
			formatted: "INFO - telegram connection established",
			expected: true,
		},
		{
			name:     "Skip bot related message",
			message:  "Bot is running",
			formatted: "INFO - Bot is running",
			expected: true,
		},
		{
			name:     "Skip SendMsgToTgbot",
			message:  "SendMsgToTgbot called",
			formatted: "INFO - SendMsgToTgbot called",
			expected: true,
		},
		{
			name:     "Skip SendMessage",
			message:  "SendMessage executed",
			formatted: "INFO - SendMessage executed",
			expected: true,
		},
		{
			name:     "Skip LogForwarder",
			message:  "LogForwarder started",
			formatted: "INFO - LogForwarder started",
			expected: true,
		},
		{
			name:     "Skip checkpoint",
			message:  "checkpoint reached",
			formatted: "INFO - checkpoint reached",
			expected: true,
		},
		{
			name:     "Skip database",
			message:  "database connected",
			formatted: "INFO - database connected",
			expected: true,
		},
		{
			name:     "Allow ERROR level",
			message:  "An error occurred",
			formatted: "ERROR - An error occurred",
			expected: false,
		},
		{
			name:     "Allow WARNING level",
			message:  "This is a warning",
			formatted: "WARNING - This is a warning",
			expected: false,
		},
		{
			name:     "Allow important INFO",
			message:  "Server started successfully",
			formatted: "INFO - Server started successfully",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.shouldSkipLog(tt.message, tt.formatted)
			if result != tt.expected {
				t.Errorf("shouldSkipLog() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLogForwarderCore_formatLogMessage(t *testing.T) {
	m := &mockLogForwarder{isEnabled: true}

	tests := []struct {
		name     string
		level    logging.Level
		message  string
		expected string
	}{
		{
			name:     "Format ERROR message",
			level:    logging.ERROR,
			message:  "Database connection failed",
			expected: "🚨 <b>ERROR</b>\nERROR - Database connection failed",
		},
		{
			name:     "Format WARNING message",
			level:    logging.WARNING,
			message:  "High memory usage detected",
			expected: "⚠️ <b>WARNING</b>\nWARNING - High memory usage detected",
		},
		{
			name:     "Format important INFO message (started)",
			level:    logging.INFO,
			message:  "Server started on port 8080",
			expected: "ℹ️ <b>INFO</b>\nINFO - Server started on port 8080",
		},
		{
			name:     "Format important INFO message (failed)",
			level:    logging.INFO,
			message:  "Login failed for user admin",
			expected: "ℹ️ <b>INFO</b>\nINFO - Login failed for user admin",
		},
		{
			name:     "Skip non-important INFO message",
			level:    logging.INFO,
			message:  "Processing request from 192.168.1.1",
			expected: "",
		},
		{
			name:     "Skip DEBUG message",
			level:    logging.DEBUG,
			message:  "Debug information",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.formatLogMessage(tt.level, tt.message)
			if result != tt.expected {
				t.Errorf("formatLogMessage() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLogForwarderCore_isImportantInfo(t *testing.T) {
	m := &mockLogForwarder{isEnabled: true}

	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{"Important: started", "Server started successfully", true},
		{"Important: stopped", "Service stopped", true},
		{"Important: running", "Xray is running", true},
		{"Important: failed", "Connection failed", true},
		{"Important: error", "An error occurred", true},
		{"Important: restart", "Restarting service", true},
		{"Important: shutdown", "System shutdown initiated", true},
		{"Important: connected", "Database connected", true},
		{"Important: disconnected", "Client disconnected", true},
		{"Important: login", "User login successful", true},
		{"Important: logout", "User logout", true},
		{"Not important: processing", "Processing request", false},
		{"Not important: random", "Some random message", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.isImportantInfo(tt.message)
			if result != tt.expected {
				t.Errorf("isImportantInfo(%q) = %v, want %v", tt.message, result, tt.expected)
			}
		})
	}
}