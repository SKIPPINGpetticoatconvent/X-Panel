package service

import (
	"testing"
)

func TestValidateTgBotToken(t *testing.T) {
	s := &SettingService{}

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"有效 token", "123456789:ABCdefGhIJKlmNoPQRsTUVwxyz", false},
		{"空值", "", true},
		{"缺少冒号", "123456789ABCdef", true},
		{"多个冒号", "123:456:789", true},
		{"非数字 bot ID", "abc:tokenpart", true},
		{"空 bot ID 部分", ":tokenpart", true},
		{"空 token 部分", "123456:", true},
		{"仅冒号", ":", true},
		{"数字 bot ID 有效格式", "1:t", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidateTgBotToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTgBotToken(%q) error = %v, wantErr %v", tt.token, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTgBotChatId(t *testing.T) {
	s := &SettingService{}

	tests := []struct {
		name    string
		chatId  string
		wantErr bool
	}{
		{"单个有效 ID", "123456789", false},
		{"逗号分隔多个 ID", "123,456,789", false},
		{"带空格的逗号分隔", "123, 456, 789", false},
		{"空值", "", true},
		{"纯空格", "   ", true},
		{"非数字 ID", "abc", true},
		{"包含非数字的混合 ID", "123,abc,456", true},
		{"空 ID 段", "123,,456", true},
		{"负号（非数字字符）", "-123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidateTgBotChatId(tt.chatId)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTgBotChatId(%q) error = %v, wantErr %v", tt.chatId, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTgBotSettings(t *testing.T) {
	s := &SettingService{}

	tests := []struct {
		name         string
		token        string
		chatId       string
		proxyURL     string
		apiServerURL string
		wantErrCount int
	}{
		{"全部为空（跳过验证）", "", "", "", "", 0},
		{"有效 token 和 chatId", "123:token", "456", "", "", 0},
		{"无效 proxy URL", "123:token", "456", "http://proxy", "", 1},
		{"有效 socks5 proxy", "123:token", "456", "socks5://127.0.0.1:1080", "", 0},
		{"无效 API server URL", "123:token", "456", "", "ftp://api.server", 1},
		{"有效 API server URL", "123:token", "456", "", "https://api.telegram.org", 0},
		{"多个错误", "invalidtoken", "abc", "badproxy", "badapi", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := s.ValidateTgBotSettings(tt.token, tt.chatId, tt.proxyURL, tt.apiServerURL)
			if len(errs) != tt.wantErrCount {
				t.Errorf("ValidateTgBotSettings() returned %d errors, want %d. Errors: %v",
					len(errs), tt.wantErrCount, errs)
			}
		})
	}
}

func TestSettingService_BasePath(t *testing.T) {
	setupTestDB(t)

	s := &SettingService{}

	tests := []struct {
		name     string
		input    string
		wantPath string
	}{
		{"普通路径", "panel", "/panel/"},
		{"已带斜杠前缀", "/panel", "/panel/"},
		{"已带斜杠后缀", "panel/", "/panel/"},
		{"已带双斜杠", "/panel/", "/panel/"},
		{"根路径", "/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.SetBasePath(tt.input)
			if err != nil {
				t.Fatalf("SetBasePath(%q) error: %v", tt.input, err)
			}

			got, err := s.GetBasePath()
			if err != nil {
				t.Fatalf("GetBasePath() error: %v", err)
			}
			if got != tt.wantPath {
				t.Errorf("SetBasePath(%q) → GetBasePath() = %q, want %q", tt.input, got, tt.wantPath)
			}
		})
	}
}
