package service

import (
	"fmt"
	"testing"

	"github.com/op/go-logging"
)

// 简单的日志转发器测试，避免依赖项问题
func TestLogForwarder_shouldSkipLog(t *testing.T) {
	lf := &LogForwarder{}

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lf.shouldSkipLog(tt.message, tt.formatted)
			if result != tt.expected {
				t.Errorf("shouldSkipLog() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLogForwarder_formatLogMessage(t *testing.T) {
	lf := &LogForwarder{}

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
			name:     "Format important INFO message",
			level:    logging.INFO,
			message:  "Server started on port 8080",
			expected: "ℹ️ <b>INFO</b>\nINFO - Server started on port 8080",
		},
		{
			name:     "Skip non-important INFO message",
			level:    logging.INFO,
			message:  "Processing request from 192.168.1.1",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logMsg := &LogMessage{
				Level:     tt.level,
				Message:   tt.message,
				Formatted: fmt.Sprintf("%s - %s", tt.level, tt.message),
			}

			result := lf.formatLogMessage(logMsg)
			if result != tt.expected {
				t.Errorf("formatLogMessage() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLogForwarder_isImportantInfo(t *testing.T) {
	lf := &LogForwarder{}

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
		{"Not important: processing", "Processing request", false},
		{"Not important: random", "Some random message", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lf.isImportantInfo(tt.message)
			if result != tt.expected {
				t.Errorf("isImportantInfo(%q) = %v, want %v", tt.message, result, tt.expected)
			}
		})
	}
}