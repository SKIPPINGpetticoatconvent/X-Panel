package common

import (
	"testing"

	"x-ui/logger"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected logger.Level
	}{
		{"error", logger.ERROR},
		{"warn", logger.WARNING},
		{"warning", logger.WARNING},
		{"info", logger.INFO},
		{"debug", logger.DEBUG},
		{"unknown", logger.WARNING}, // 默认值
	}

	for _, tc := range tests {
		got := ParseLogLevel(tc.input)
		if got != tc.expected {
			t.Errorf("ParseLogLevel(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestLogLevelToString(t *testing.T) {
	tests := []struct {
		level    logger.Level
		expected string
	}{
		{logger.ERROR, "error"},
		{logger.WARNING, "warn"},
		{logger.INFO, "info"},
		{logger.DEBUG, "debug"},
	}

	for _, tc := range tests {
		got := LogLevelToString(tc.level)
		if got != tc.expected {
			t.Errorf("LogLevelToString(%v) = %q, want %q", tc.level, got, tc.expected)
		}
	}
}
