package security

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockListenerForSecurity 模拟监听器用于security测试
type mockListenerForSecurity struct{}

func (m *mockListenerForSecurity) Accept() (net.Conn, error) { return nil, nil }
func (m *mockListenerForSecurity) Close() error              { return nil }
func (m *mockListenerForSecurity) Addr() net.Addr            { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080} }

// TestNewSecureListener 测试安全监听器创建
func TestNewSecureListener(t *testing.T) {
	mockListener := &mockListenerForSecurity{}
	config := &SecurityConfig{
		RateLimit: &RateLimitConfig{
			MaxConnsPerSec: 10,
			Burst:          20,
		},
	}

	listener := NewSecureListener(mockListener, config)
	assert.NotNil(t, listener, "应该成功创建安全监听器")
}

// TestNewSecureListener_DefaultConfig 测试默认配置
func TestNewSecureListener_DefaultConfig(t *testing.T) {
	mockListener := &mockListenerForSecurity{}

	listener := NewSecureListener(mockListener, nil)
	assert.NotNil(t, listener, "应该使用默认配置创建安全监听器")
}

// TestNewRateLimitedListener 测试速率限制监听器工厂函数
func TestNewRateLimitedListener(t *testing.T) {
	mockListener := &mockListenerForSecurity{}

	listener := NewRateLimitedListener(mockListener)
	assert.NotNil(t, listener, "应该成功创建速率限制监听器")
}

// TestNewProtoDetectListenerFactory 测试协议检测监听器工厂函数
func TestNewProtoDetectListenerFactory(t *testing.T) {
	mockListener := &mockListenerForSecurity{}

	listener := NewProtoDetectListenerFactory(mockListener)
	assert.NotNil(t, listener, "应该成功创建协议检测监听器")
}

// TestGetCertHealthChecker 测试证书健康检查器工厂函数
func TestGetCertHealthChecker(t *testing.T) {
	checker := GetCertHealthChecker()
	assert.NotNil(t, checker, "应该成功获取证书健康检查器")
}

// TestGetTLSLogger 测试TLS日志记录器工厂函数
func TestGetTLSLogger(t *testing.T) {
	logger := GetTLSLogger()
	assert.NotNil(t, logger, "应该成功获取TLS日志记录器")
}

// TestInitSecurity 测试安全模块初始化
func TestInitSecurity(t *testing.T) {
	// 这个函数主要是日志输出，应该不会出错
	InitSecurity()
}

// TestGetDefaultRateLimitConfig 测试获取默认速率限制配置
func TestGetDefaultRateLimitConfig(t *testing.T) {
	config := GetDefaultRateLimitConfig()
	assert.NotNil(t, config, "应该成功获取默认配置")
	assert.Equal(t, 5, config.MaxConnsPerSec, "默认最大连接数应该为5")
	assert.Equal(t, 10, config.Burst, "默认突发允许数应该为10")
}