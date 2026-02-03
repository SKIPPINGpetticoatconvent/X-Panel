package service

import (
	"testing"

	"x-ui/logger"
)

// mockTelegramService 用于测试的 Telegram 服务模拟
type mockTelegramService struct {
	running  bool
	messages []string
}

func (m *mockTelegramService) IsRunning() bool { return m.running }
func (m *mockTelegramService) SendMessage(msg string) error {
	m.messages = append(m.messages, msg)
	return nil
}

// mockSettingServiceForForwarder 用于测试的设置服务模拟
type mockSettingServiceForForwarder struct {
	logForwardEnabled bool
	tgLogLevel        string
}

func (m *mockSettingServiceForForwarder) GetTgLogForwardEnabled() (bool, error) {
	return m.logForwardEnabled, nil
}

func (m *mockSettingServiceForForwarder) GetTgLogLevel() (string, error) {
	return m.tgLogLevel, nil
}

func TestLogForwarder_ShouldSkipLog(t *testing.T) {
	lf := &LogForwarder{
		forwardLevel: logger.WARNING,
	}

	tests := []struct {
		name    string
		message string
		level   logger.Level
		want    bool
	}{
		{"低于转发级别", "normal info", logger.INFO, true},
		{"满足转发级别", "something failed", logger.WARNING, false},
		{"高于转发级别", "critical error", logger.ERROR, false},
		{"包含 Telegram 关键词", "Telegram bot connected", logger.ERROR, true},
		{"包含 bot 关键词", "Bot started", logger.ERROR, true},
		{"包含日志转发关键词", "LogForwarder started", logger.ERROR, true},
		{"包含 checkpoint 关键词", "checkpoint completed", logger.ERROR, true},
		{"包含 database 关键词", "database connection", logger.ERROR, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lf.shouldSkipLog(tt.message, "", tt.level)
			if got != tt.want {
				t.Errorf("shouldSkipLog(%q, %v) = %v, want %v", tt.message, tt.level, got, tt.want)
			}
		})
	}
}

func TestLogForwarder_IsImportantInfo(t *testing.T) {
	lf := &LogForwarder{}

	tests := []struct {
		message string
		want    bool
	}{
		{"Server started successfully", true},
		{"Service stopped", true},
		{"Connection failed", true},
		{"User login detected", true},
		{"Normal processing message", false},
		{"Reading configuration", false},
		{"System restart initiated", true},
		{"Client disconnected", true},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			got := lf.isImportantInfo(tt.message)
			if got != tt.want {
				t.Errorf("isImportantInfo(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestLogForwarder_FormatLogMessage(t *testing.T) {
	lf := &LogForwarder{}

	tests := []struct {
		name  string
		msg   *LogMessage
		empty bool
	}{
		{
			"ERROR 级别",
			&LogMessage{Level: logger.ERROR, Message: "test error"},
			false,
		},
		{
			"WARNING 级别",
			&LogMessage{Level: logger.WARNING, Message: "test warning"},
			false,
		},
		{
			"DEBUG 级别",
			&LogMessage{Level: logger.DEBUG, Message: "test debug"},
			false,
		},
		{
			"INFO 级别-重要",
			&LogMessage{Level: logger.INFO, Message: "Server started"},
			false,
		},
		{
			"INFO 级别-不重要",
			&LogMessage{Level: logger.INFO, Message: "normal processing"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lf.formatLogMessage(tt.msg)
			if tt.empty && got != "" {
				t.Errorf("formatLogMessage() = %q, want empty", got)
			}
			if !tt.empty && got == "" {
				t.Error("formatLogMessage() returned empty, want non-empty")
			}
		})
	}
}

func TestLogForwarder_IsEnabled_Default(t *testing.T) {
	lf := &LogForwarder{}
	if lf.IsEnabled() {
		t.Error("IsEnabled() should be false by default")
	}
}

func TestLogForwarder_SetForwardLevel(t *testing.T) {
	lf := &LogForwarder{
		forwardLevel: logger.WARNING,
	}

	lf.SetForwardLevel(logger.ERROR)

	if lf.forwardLevel != logger.ERROR {
		t.Errorf("forwardLevel = %v, want %v", lf.forwardLevel, logger.ERROR)
	}
}
