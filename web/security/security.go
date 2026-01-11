package security

import (
	"net"

	"x-ui/logger"
)

// SecurityConfig 安全配置结构体
type SecurityConfig struct {
	RateLimit *RateLimitConfig
	// 可以扩展其他安全配置
}

// DefaultSecurityConfig 默认安全配置
var DefaultSecurityConfig = &SecurityConfig{
	RateLimit: &RateLimitConfig{
		MaxConnsPerSec: 5,
		Burst:          10,
	},
}

// NewSecureListener 创建安全的监听器链
func NewSecureListener(baseListener net.Listener, config *SecurityConfig) net.Listener {
	if config == nil {
		config = DefaultSecurityConfig
	}

	logger.Info("初始化安全监听器链...")

	// 第一层：速率限制
	rateLimitedListener := NewRateLimitListener(baseListener, config.RateLimit)
	logger.Info("已启用连接速率限制")

	// 第二层：协议检测
	protoListener := NewProtoDetectListener(rateLimitedListener)
	logger.Info("已启用协议检测")

	return protoListener
}

// NewRateLimitListener 工厂函数：创建速率限制监听器
func NewRateLimitedListener(listener net.Listener) *RateLimitListener {
	return NewRateLimitListener(listener, DefaultSecurityConfig.RateLimit)
}

// NewProtoDetectListenerFactory 工厂函数：创建协议检测监听器
func NewProtoDetectListenerFactory(listener net.Listener) *ProtoDetectListener {
	return NewProtoDetectListener(listener)
}

// GetCertHealthChecker 获取证书健康检查器
func GetCertHealthChecker() CertHealthChecker {
	return NewCertHealthChecker()
}

// GetTLSLogger 获取TLS错误记录器
func GetTLSLogger() *TLSLogger {
	return NewTLSLogger()
}

// InitSecurity 初始化安全模块
func InitSecurity() {
	logger.Info("安全模块初始化完成")
}

// GetDefaultRateLimitConfig 获取默认速率限制配置
func GetDefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		MaxConnsPerSec: 5,
		Burst:          10,
	}
}