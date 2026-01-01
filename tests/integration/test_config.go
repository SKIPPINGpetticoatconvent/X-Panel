package integration

import "os"

// DefaultTestConfig 默认测试配置
var DefaultTestConfig = map[string]string{
	"RUN_SYSTEM_INTEGRATION_TESTS": "false",
	"TEST_WEB_PORT":                "8080",
	"TEST_SUB_PORT":                "8081",
	"TEST_STARTUP_TIMEOUT":         "120",
	"TEST_TOTAL_TIMEOUT":           "300",
}

// GetTestConfig 获取测试配置，优先使用环境变量，否则使用默认值
func GetTestConfig(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if defaultValue, exists := DefaultTestConfig[key]; exists {
		return defaultValue
	}
	return ""
}
