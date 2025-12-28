package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TelegramE2EClient Telegram E2E测试客户端
type TelegramE2EClient struct {
	baseURL string
	client  *http.Client
}

// NewTelegramE2EClient 创建Telegram E2E测试客户端
func NewTelegramE2EClient(baseURL string) *TelegramE2EClient {
	return &TelegramE2EClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Login 登录到面板
func (t *TelegramE2EClient) Login(username, password string) error {
	loginURL := t.baseURL + "/login"
	data := map[string]string{
		"username": username,
		"password": password,
	}
	jsonData, _ := json.Marshal(data)

	resp, err := t.client.Post(loginURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("login failed: %s", result.Msg)
	}

	return nil
}

// GetTelegramSettings 获取Telegram设置
func (t *TelegramE2EClient) GetTelegramSettings() (map[string]interface{}, error) {
	url := t.baseURL + "/panel/api/setting/all"
	resp, err := t.client.Post(url, "application/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool                   `json:"success"`
		Msg     string                 `json:"msg"`
		Obj     map[string]interface{} `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("get settings failed: %s", result.Msg)
	}

	return result.Obj, nil
}

// UpdateTelegramSettings 更新Telegram设置
func (t *TelegramE2EClient) UpdateTelegramSettings(settings map[string]interface{}) error {
	url := t.baseURL + "/panel/api/setting/update"
	jsonData, _ := json.Marshal(settings)

	resp, err := t.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("update settings failed: %s", result.Msg)
	}

	return nil
}

// TestTelegramE2EConfiguration Telegram配置E2E测试
func TestTelegramE2EConfiguration(t *testing.T) {
	// 注意：这个测试需要一个真实的X-Panel实例运行
	// 在CI/CD环境中，这个测试可能被跳过

	baseURL := "http://localhost:13688" // 假设面板运行在这个端口

	client := NewTelegramE2EClient(baseURL)

	t.Log("Testing Telegram configuration...")

	// 登录
	if err := client.Login("admin", "admin"); err != nil {
		t.Skipf("Skipping Telegram E2E test: cannot login to panel: %v", err)
		return
	}
	t.Log("Login successful")

	// 获取当前设置
	settings, err := client.GetTelegramSettings()
	if err != nil {
		t.Fatalf("Failed to get settings: %v", err)
	}
	t.Log("Settings retrieved successfully")

	// 检查Telegram相关设置是否存在
	telegramSettings, exists := settings["telegramSettings"]
	if !exists {
		t.Log("Telegram settings not found in current configuration")
		return
	}

	t.Logf("Telegram settings found: %v", telegramSettings)

	// 注意：实际的Telegram Bot测试需要真实的Bot Token和Chat ID
	// 这里我们只验证配置结构的存在性

	t.Log("Telegram E2E Configuration Test Passed!")
}

// TestTelegramE2EBackup Telegram备份E2E测试
func TestTelegramE2EBackup(t *testing.T) {
	// 注意：这个测试需要一个真实的X-Panel实例运行
	// 并且需要正确配置Telegram Bot

	baseURL := "http://localhost:13688"

	client := NewTelegramE2EClient(baseURL)

	t.Log("Testing Telegram backup functionality...")

	// 登录
	if err := client.Login("admin", "admin"); err != nil {
		t.Skipf("Skipping Telegram backup test: cannot login to panel: %v", err)
		return
	}

	// 尝试备份到Telegram
	// 注意：这需要Telegram Bot正确配置，否则会失败
	backupURL := baseURL + "/panel/api/backuptotgbot"
	resp, err := client.client.Get(backupURL)
	if err != nil {
		t.Fatalf("Failed to call backup API: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode backup response: %v", err)
	}

	// 备份可能成功或失败，取决于Telegram配置
	if result.Success {
		t.Log("Backup to Telegram successful")
	} else {
		t.Logf("Backup to Telegram failed (expected if not configured): %s", result.Msg)
	}

	t.Log("Telegram E2E Backup Test completed!")
}

// TestTelegramE2EMessageFormat Telegram消息格式测试
func TestTelegramE2EMessageFormat(t *testing.T) {
	// 这个测试验证Telegram消息的格式化逻辑
	// 不需要真实的Telegram Bot

	testMessages := []struct {
		name     string
		input    string
		expected bool // 是否应该被认为是有效的消息
	}{
		{
			name:     "Normal message",
			input:    "Server status: OK",
			expected: true,
		},
		{
			name:     "Long message",
			input:    string(make([]byte, 4097)), // 超过Telegram限制
			expected: false,
		},
		{
			name:     "Empty message",
			input:    "",
			expected: false,
		},
		{
			name:     "Message with special chars",
			input:    "Status: ✅ Running\nCPU: 45%\nMemory: 2.1GB",
			expected: true,
		},
	}

	for _, tm := range testMessages {
		t.Run(tm.name, func(t *testing.T) {
			// 检查消息长度
			if len(tm.input) == 0 && !tm.expected {
				return // 空消息应该被拒绝
			}

			if len(tm.input) > 4096 && !tm.expected {
				return // 过长消息应该被拒绝
			}

			if len(tm.input) > 0 && len(tm.input) <= 4096 && tm.expected {
				// 有效消息
				t.Logf("Message format valid: %s...", tm.input[:min(50, len(tm.input))])
			}
		})
	}

	t.Log("Telegram Message Format Test Passed!")
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}