package common

import (
	"testing"

	logging "github.com/op/go-logging"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected logging.Level
	}{
		{"error", logging.ERROR},
		{"warn", logging.WARNING},
		{"warning", logging.WARNING},
		{"info", logging.INFO},
		{"debug", logging.DEBUG},
		{"", logging.WARNING},
		{"unknown", logging.WARNING},
		{"ERROR", logging.WARNING}, // 大写不匹配，返回默认值
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLogLevelToString(t *testing.T) {
	tests := []struct {
		input    logging.Level
		expected string
	}{
		{logging.ERROR, "error"},
		{logging.WARNING, "warn"},
		{logging.INFO, "info"},
		{logging.DEBUG, "debug"},
		{logging.NOTICE, "warn"}, // 未知级别返回默认值
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := LogLevelToString(tt.input)
			if result != tt.expected {
				t.Errorf("LogLevelToString(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
