package service

import (
	"strings"
	"testing"
)

// TestGenerateEnhancedServerNames 测试增强域名生成逻辑
func TestGenerateEnhancedServerNames(t *testing.T) {
	serverService := &ServerService{}
	
	// 测试 Apple 域名
	appleDomains := serverService.GenerateEnhancedServerNames("apple.com")
	if len(appleDomains) == 0 {
		t.Error("Apple 域名生成失败")
	}
	
	// 验证包含主域名
	hasApple := false
	for _, domain := range appleDomains {
		if strings.Contains(domain, "apple.com") {
			hasApple = true
			break
		}
	}
	if !hasApple {
		t.Error("生成的域名列表不包含 apple.com 相关域名")
	}
	
	// 验证去重
	domainMap := make(map[string]bool)
	for _, domain := range appleDomains {
		if domainMap[domain] {
			t.Errorf("域名列表包含重复项: %s", domain)
		}
		domainMap[domain] = true
	}
	
	// 验证数量限制
	if len(appleDomains) > 8 {
		t.Errorf("域名列表长度超过限制: %d", len(appleDomains))
	}
}

// TestGetNewSNI 测试 SNI 获取逻辑
func TestGetNewSNI(t *testing.T) {
	serverService := &ServerService{}
	
	// 测试获取 SNI
	sni := serverService.GetNewSNI()
	if sni == "" {
		t.Error("GetNewSNI 返回空字符串")
	}
	
	// 验证格式
	if !strings.Contains(sni, ":443") {
		t.Errorf("SNI 格式不正确，缺少 :443 后缀: %s", sni)
	}
}

// TestRemoveDuplicatesFromSlice 测试去重功能
func TestRemoveDuplicatesFromSlice(t *testing.T) {
	serverService := &ServerService{}
	
	input := []string{"apple.com", "www.apple.com", "apple.com", "google.com", "www.google.com"}
	result := serverService.removeDuplicatesFromSlice(input)
	
	// 验证去重
	if len(result) >= len(input) {
		t.Error("去重功能未生效")
	}
	
	// 验证保留原始格式
	hasApple := false
	for _, domain := range result {
		if domain == "apple.com" {
			hasApple = true
			break
		}
	}
	if !hasApple {
		t.Error("去重后丢失了原始格式的域名")
	}
}

// TestNormalizeDomain 测试域名标准化
func TestNormalizeDomain(t *testing.T) {
	serverService := &ServerService{}
	
	tests := []struct {
		input    string
		expected string
	}{
		{" APPLE.COM ", "apple.com"},
		{"WwW.Apple.Com", "www.apple.com"},
		{"Apple.Com", "apple.com"},
	}
	
	for _, test := range tests {
		result := serverService.normalizeDomain(test.input)
		if result != test.expected {
			t.Errorf("标准化失败: 输入 '%s', 期望 '%s', 实际 '%s'", 
				test.input, test.expected, result)
		}
	}
}