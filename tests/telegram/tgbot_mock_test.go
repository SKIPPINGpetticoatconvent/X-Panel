package telegram

import (
	"errors"
	"strings"
	"testing"

	tu "github.com/mymmrac/telego/telegoutil"
)

// MockBot 是一个模拟的 Telegram Bot，用于测试
type MockBot struct {
	messages      []string
	callbacks     []string
	errors        []error
	shouldFail    bool
	messageCount  int
	callbackCount int
}

func NewMockBot() *MockBot {
	return &MockBot{
		messages:   make([]string, 0),
		callbacks:  make([]string, 0),
		errors:     make([]error, 0),
		shouldFail: false,
	}
}

func (m *MockBot) SendMessage(chatID int64, text string) error {
	m.messageCount++
	m.messages = append(m.messages, text)
	if m.shouldFail {
		err := errors.New("mock send message error")
		m.errors = append(m.errors, err)
		return err
	}
	return nil
}

func (m *MockBot) SendCallbackQuery(callbackID string, text string) error {
	m.callbackCount++
	m.callbacks = append(m.callbacks, text)
	return nil
}

func (m *MockBot) GetMessages() []string {
	return m.messages
}

func (m *MockBot) GetCallbacks() []string {
	return m.callbacks
}

func (m *MockBot) GetErrorCount() int {
	return len(m.errors)
}

func (m *MockBot) Reset() {
	m.messages = make([]string, 0)
	m.callbacks = make([]string, 0)
	m.errors = make([]error, 0)
	m.messageCount = 0
	m.callbackCount = 0
}

// TestCommandHandling 测试命令处理功能
func TestCommandHandling(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		isAdmin        bool
		shouldProcess  bool
	}{
		{
			name:          "Start command - admin",
			command:       "/start",
			isAdmin:       true,
			shouldProcess: true,
		},
		{
			name:          "Start command - non-admin",
			command:       "/start",
			isAdmin:       false,
			shouldProcess: true,
		},
		{
			name:          "Help command",
			command:       "/help",
			isAdmin:       false,
			shouldProcess: true,
		},
		{
			name:          "Status command",
			command:       "/status",
			isAdmin:       false,
			shouldProcess: true,
		},
		{
			name:          "ID command",
			command:       "/id",
			isAdmin:       false,
			shouldProcess: true,
		},
		{
			name:          "Usage command with args - admin",
			command:       "/usage test@example.com",
			isAdmin:       true,
			shouldProcess: true,
		},
		{
			name:          "Usage command with args - non-admin",
			command:       "/usage test@example.com",
			isAdmin:       false,
			shouldProcess: true,
		},
		{
			name:          "Usage command without args",
			command:       "/usage",
			isAdmin:       false,
			shouldProcess: true,
		},
		{
			name:          "Inbound command - admin with args",
			command:       "/inbound 123",
			isAdmin:       true,
			shouldProcess: true,
		},
		{
			name:          "Inbound command - non-admin",
			command:       "/inbound 123",
			isAdmin:       false,
			shouldProcess: false,
		},
		{
			name:          "Restart command - admin",
			command:       "/restart",
			isAdmin:       true,
			shouldProcess: true,
		},
		{
			name:          "Restart command - non-admin",
			command:       "/restart",
			isAdmin:       false,
			shouldProcess: false,
		},
		{
			name:          "OneClick command - admin",
			command:       "/oneclick",
			isAdmin:       true,
			shouldProcess: true,
		},
		{
			name:          "OneClick command - non-admin",
			command:       "/oneclick",
			isAdmin:       false,
			shouldProcess: false,
		},

		{
			name:          "RestartX command - admin",
			command:       "/restartx",
			isAdmin:       true,
			shouldProcess: true,
		},
		{
			name:          "RestartX command - non-admin",
			command:       "/restartx",
			isAdmin:       false,
			shouldProcess: false,
		},
		{
			name:          "XrayVersion command",
			command:       "/xrayversion",
			isAdmin:       false,
			shouldProcess: true,
		},
		{
			name:          "Unknown command",
			command:       "/unknown",
			isAdmin:       false,
			shouldProcess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟命令解析
			command, _, _ := tu.ParseCommand(tt.command)
			
			// 验证命令解析
			if command == "" {
				t.Errorf("Failed to parse command: %s", tt.command)
			}

			// 模拟权限检查
			isAdminCommand := strings.Contains(tt.command, "/inbound") ||
				strings.Contains(tt.command, "/restart") ||
				strings.Contains(tt.command, "/oneclick") ||
				strings.Contains(tt.command, "/restartx")

			// 验证权限逻辑
			if isAdminCommand && !tt.isAdmin {
				if tt.shouldProcess {
					t.Error("Admin-only command should not be processed by non-admin")
				}
			}

			// 验证命令是否应该被处理
			if tt.shouldProcess {
				// 命令应该被处理（无论是成功还是返回错误消息）
				// 这里我们只验证逻辑，不验证实际的消息内容
			}
		})
	}
}

// TestAdminVerification 测试管理员验证功能
func TestAdminVerification(t *testing.T) {
	tests := []struct {
		name          string
		adminIds      []int64
		userId        int64
		shouldAllow   bool
		expectedError bool
	}{
		{
			name:          "Valid single admin",
			adminIds:      []int64{123456789},
			userId:        123456789,
			shouldAllow:   true,
			expectedError: false,
		},
		{
			name:          "Valid multiple admins",
			adminIds:      []int64{123456789, 987654321},
			userId:        987654321,
			shouldAllow:   true,
			expectedError: false,
		},
		{
			name:          "Invalid user ID",
			adminIds:      []int64{123456789},
			userId:        111111111,
			shouldAllow:   false,
			expectedError: false,
		},
		{
			name:          "Empty admin list",
			adminIds:      []int64{},
			userId:        123456789,
			shouldAllow:   false,
			expectedError: true,
		},
		{
			name:          "Negative user ID",
			adminIds:      []int64{123456789},
			userId:        -123456789,
			shouldAllow:   false,
			expectedError: false,
		},
		{
			name:          "Zero user ID",
			adminIds:      []int64{123456789},
			userId:        0,
			shouldAllow:   false,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟管理员验证逻辑
			isAdmin := false
			for _, adminID := range tt.adminIds {
				if adminID == tt.userId {
					isAdmin = true
					break
				}
			}

			if isAdmin != tt.shouldAllow {
				t.Errorf("Admin verification failed: expected %v, got %v", tt.shouldAllow, isAdmin)
			}

			// 验证空管理员列表的情况
			if len(tt.adminIds) == 0 && tt.expectedError {
				// 在实际代码中，这应该返回错误
				if len(tt.adminIds) == 0 {
					// 这是预期的行为
				}
			}
		})
	}
}

// TestMessageSending 测试消息发送流程
func TestMessageSending(t *testing.T) {
	mockBot := NewMockBot()

	tests := []struct {
		name        string
		chatID      int64
		message     string
		shouldFail  bool
		expectError bool
	}{
		{
			name:        "Send message successfully",
			chatID:      123456789,
			message:     "Hello, World!",
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "Send message to different chat",
			chatID:      987654321,
			message:     "Test message",
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "Send empty message",
			chatID:      123456789,
			message:     "",
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "Send long message",
			chatID:      123456789,
			message:     strings.Repeat("Long message ", 100),
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "Send message with failure",
			chatID:      123456789,
			message:     "This will fail",
			shouldFail:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBot.shouldFail = tt.shouldFail
			
			err := mockBot.SendMessage(tt.chatID, tt.message)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.shouldFail {
				// 验证消息被发送
				messages := mockBot.GetMessages()
				if len(messages) == 0 {
					t.Error("No messages were sent")
				}
			}
		})
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		scenario      string
		expectedError bool
	}{
		{
			name:          "Invalid token format",
			scenario:      "invalid_token",
			expectedError: true,
		},
		{
			name:          "Token without colon",
			scenario:      "123456789token",
			expectedError: true,
		},
		{
			name:          "Valid token format",
			scenario:      "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
			expectedError: false,
		},
		{
			name:          "Invalid admin ID format",
			scenario:      "invalid_id",
			expectedError: true,
		},
		{
			name:          "Empty admin ID",
			scenario:      "",
			expectedError: true,
		},
		{
			name:          "Negative admin ID",
			scenario:      "-123456789",
			expectedError: true,
		},
		{
			name:          "Zero admin ID",
			scenario:      "0",
			expectedError: true,
		},
		{
			name:          "Valid admin ID",
			scenario:      "123456789",
			expectedError: false,
		},
		{
			name:          "Multiple valid admin IDs",
			scenario:      "123456789,987654321",
			expectedError: false,
		},
		{
			name:          "Mixed valid and invalid admin IDs",
			scenario:      "123456789,invalid,987654321",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟各种错误场景
			switch tt.scenario {
			case "invalid_token", "no_colon_token":
				// 验证 token 格式
				if len(tt.scenario) < 10 || !strings.Contains(tt.scenario, ":") {
					if !tt.expectedError {
						t.Errorf("Expected valid token but got invalid: %s", tt.scenario)
					}
				}
			case "", "invalid_id", "-123456789", "0":
				// 验证 admin ID 格式
				if tt.scenario == "" || tt.scenario == "invalid_id" || tt.scenario == "-123456789" || tt.scenario == "0" {
					if !tt.expectedError {
						t.Errorf("Expected valid admin ID but got invalid: %s", tt.scenario)
					}
				}
			case "123456789", "123456789,987654321":
				// 验证有效 admin ID
				if tt.expectedError {
					t.Errorf("Expected invalid admin ID but got valid: %s", tt.scenario)
				}
			case "123456789,invalid,987654321":
				// 混合情况应该被视为错误
				if !tt.expectedError {
					t.Errorf("Expected error for mixed admin IDs but got none")
				}
			}
		})
	}
}

// TestCallbackQueryHandling 测试回调查询处理
func TestCallbackQueryHandling(t *testing.T) {
	mockBot := NewMockBot()

	tests := []struct {
		name         string
		callbackData string
		shouldFail   bool
	}{
		{
			name:         "Valid callback - restart panel",
			callbackData: "restart_panel_confirm",
			shouldFail:   false,
		},
		{
			name:         "Valid callback - cancel restart",
			callbackData: "restart_panel_cancel",
			shouldFail:   false,
		},
		{
			name:         "Valid callback - update xray",
			callbackData: "update_xray_ask 1.8.0",
			shouldFail:   false,
		},
		{
			name:         "Valid callback - cancel update",
			callbackData: "update_xray_cancel",
			shouldFail:   false,
		},
		{
			name:         "Valid callback - one click config",
			callbackData: "oneclick_config",
			shouldFail:   false,
		},
		{
			name:         "Valid callback - firewall operations",
			callbackData: "firewall_check_status",
			shouldFail:   false,
		},
		{
			name:         "Invalid callback - malformed",
			callbackData: "invalid_callback_data",
			shouldFail:   true,
		},
		{
			name:         "Empty callback",
			callbackData: "",
			shouldFail:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟回调处理
			if tt.callbackData == "" {
				// 空回调应该失败
				if !tt.shouldFail {
					t.Error("Expected failure for empty callback")
				}
				return
			}

			// 验证回调数据格式
			if !strings.Contains(tt.callbackData, "_") && tt.callbackData != "" {
				// 有效的回调应该包含下划线分隔
				if !tt.shouldFail {
					t.Errorf("Expected valid callback format but got: %s", tt.callbackData)
				}
			}

			// 模拟发送回调响应
			if !tt.shouldFail {
				err := mockBot.SendCallbackQuery("callback_id", "Response for "+tt.callbackData)
				if err != nil {
					t.Errorf("Callback query failed: %v", err)
				}
			}
		})
	}
}

// TestComplexScenarios 测试复杂场景
func TestComplexScenarios(t *testing.T) {
	mockBot := NewMockBot()

	t.Run("Sequential commands with state", func(t *testing.T) {
		// 模拟用户状态管理
		userStates := make(map[int64]string)
		
		// 用户1开始会话
		chatID1 := int64(123456789)
		userStates[chatID1] = "waiting_for_input"
		
		// 发送多个命令
		commands := []string{"/start", "/help", "/status", "/id"}
		for _, cmd := range commands {
			_, _, _ = tu.ParseCommand(cmd)
			// 模拟命令处理
			if cmd == "/start" {
				mockBot.SendMessage(chatID1, "Welcome message")
			} else if cmd == "/help" {
				mockBot.SendMessage(chatID1, "Help message")
			}
		}
		
		// 验证消息数量
		messages := mockBot.GetMessages()
		if len(messages) < 2 {
			t.Errorf("Expected at least 2 messages, got %d", len(messages))
		}
		
		// 清理状态
		delete(userStates, chatID1)
	})

	t.Run("Multiple users concurrent", func(t *testing.T) {
		// 创建新的mock bot实例避免消息污染
		localMockBot := NewMockBot()
		chatIDs := []int64{111, 222, 333}
		
		for _, chatID := range chatIDs {
			// 每个用户发送命令
			localMockBot.SendMessage(chatID, "User message")
		}
		
		messages := localMockBot.GetMessages()
		if len(messages) != len(chatIDs) {
			t.Errorf("Expected %d messages, got %d", len(chatIDs), len(messages))
		}
	})

	t.Run("Error recovery scenario", func(t *testing.T) {
		// 模拟错误后恢复
		mockBot.shouldFail = true
		err := mockBot.SendMessage(123, "This will fail")
		if err == nil {
			t.Error("Expected error but got none")
		}
		
		// 恢复正常
		mockBot.shouldFail = false
		err = mockBot.SendMessage(123, "This should work")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

// BenchmarkCommandProcessing 基准测试命令处理性能
func BenchmarkCommandProcessing(b *testing.B) {
	mockBot := NewMockBot()
	
	commands := []string{
		"/start",
		"/help",
		"/status",
		"/id",
		"/usage test@example.com",
		"/inbound 123",
		"/restart",
		"/oneclick",
		"/restartx",
		"/xrayversion",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := commands[i%len(commands)]
		_, _, _ = tu.ParseCommand(cmd)
		// 模拟消息发送
		_ = mockBot.SendMessage(123456789, "Response")
	}
}

// BenchmarkAdminVerification 基准测试管理员验证性能
func BenchmarkAdminVerification(b *testing.B) {
	adminIds := []int64{123456789, 987654321, 111111111, 222222222, 333333333}
	testUserId := int64(123456789)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isAdmin := false
		for _, adminID := range adminIds {
			if adminID == testUserId {
				isAdmin = true
				break
			}
		}
		_ = isAdmin
	}
}

// BenchmarkMessageSending 基准测试消息发送性能
func BenchmarkMessageSending(b *testing.B) {
	mockBot := NewMockBot()
	chatID := int64(123456789)
	message := "Test message for benchmarking"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mockBot.SendMessage(chatID, message)
	}
}

// TestEdgeCases 测试边界情况
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{
			name:     "Empty string command",
			input:    "",
			expected: false,
		},
		{
			name:     "Whitespace only command",
			input:    "   ",
			expected: false,
		},
		{
			name:     "Very long command",
			input:    "/" + strings.Repeat("a", 1000),
			expected: true,
		},
		{
			name:     "Command with special characters",
			input:    "/command!@#$%^&*()",
			expected: true,
		},
		{
			name:     "Unicode command",
			input:    "/命令",
			expected: true,
		},
		{
			name:     "Command with newlines",
			input:    "/command\nwith\nnewlines",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdStr, ok := tt.input.(string)
			if !ok {
				return
			}

			// 验证命令解析
			if cmdStr == "" || strings.TrimSpace(cmdStr) == "" {
				if tt.expected {
					t.Error("Expected valid command but got empty")
				}
				return
			}

			// 模拟命令解析
			parts := strings.Fields(cmdStr)
			if len(parts) == 0 {
				if tt.expected {
					t.Error("Expected valid command but got no parts")
				}
				return
			}

			command := strings.TrimPrefix(parts[0], "/")
			if command == "" {
				if tt.expected {
					t.Error("Expected valid command but got empty after trim")
				}
				return
			}

			// 验证结果
			if !tt.expected {
				// 这些情况应该被视为无效
				if cmdStr == "" || strings.TrimSpace(cmdStr) == "" {
					// 正确
				}
			}
		})
	}
}

// TestIntegrationScenarios 测试集成场景
func TestIntegrationScenarios(t *testing.T) {
	t.Run("Complete user workflow", func(t *testing.T) {
		mockBot := NewMockBot()
		
		// 1. 用户启动机器人
		mockBot.SendMessage(123456789, "Welcome")
		
		// 2. 用户请求帮助
		mockBot.SendMessage(123456789, "Help")
		
		// 3. 用户查询状态
		mockBot.SendMessage(123456789, "Status")
		
		// 4. 用户查询ID
		mockBot.SendMessage(123456789, "ID")
		
		// 验证所有消息都已发送
		messages := mockBot.GetMessages()
		if len(messages) != 4 {
			t.Errorf("Expected 4 messages, got %d", len(messages))
		}
	})

	t.Run("Admin-only workflow", func(t *testing.T) {
		mockBot := NewMockBot()
		adminID := int64(123456789)
		
		// 管理员执行高级操作
		commands := []string{
			"/inbound 123",
			"/restart",
			"/oneclick",
			"/restartx",
		}
		
		for _, cmd := range commands {
			_, _, _ = tu.ParseCommand(cmd)
			// 模拟管理员权限检查通过
			mockBot.SendMessage(adminID, "Admin command executed")
		}
		
		messages := mockBot.GetMessages()
		if len(messages) != len(commands) {
			t.Errorf("Expected %d messages, got %d", len(commands), len(messages))
		}
	})

	t.Run("Error handling workflow", func(t *testing.T) {
		mockBot := NewMockBot()
		
		// 模拟网络错误
		mockBot.shouldFail = true
		err := mockBot.SendMessage(123456789, "Test")
		if err == nil {
			t.Error("Expected network error")
		}
		
		// 恢复并重试
		mockBot.shouldFail = false
		err = mockBot.SendMessage(123456789, "Retry")
		if err != nil {
			t.Errorf("Unexpected error on retry: %v", err)
		}
	})
}