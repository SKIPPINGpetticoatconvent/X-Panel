package service

import (
	"crypto/tls"
	"testing"

	"x-ui/logger"
)



func TestTLSCertManager_GetTLSConfig(t *testing.T) {
	// 初始化日志
	logger.InitLogger(5, false) // DEBUG level, no local log

	manager := NewTLSCertManager(nil)

	config := manager.GetTLSConfig()
	if config == nil {
		t.Fatal("GetTLSConfig should return a valid config")
	}

	if config.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion TLS 1.2, got %v", config.MinVersion)
	}

	if config.GetCertificate == nil {
		t.Fatal("GetCertificate callback should be set")
	}
}

func TestTLSCertManager_SetCertPaths(t *testing.T) {
	// 初始化日志
	logger.InitLogger(5, false) // DEBUG level, no local log

	manager := NewTLSCertManager(nil)

	// 设置证书路径
	manager.SetCertPaths("/path/to/cert.crt", "/path/to/cert.key")

	// 验证路径是否设置正确（无法直接测试私有字段）
	config := manager.GetTLSConfig()
	if config == nil {
		t.Fatal("GetTLSConfig should return a valid config")
	}
}

func TestTLSCertManager_SetCertPaths_Empty(t *testing.T) {
	// 初始化日志
	logger.InitLogger(5, false) // DEBUG level, no local log

	manager := NewTLSCertManager(nil)

	// 设置空路径
	manager.SetCertPaths("", "")

	// 验证配置仍然有效
	config := manager.GetTLSConfig()
	if config == nil {
		t.Fatal("GetTLSConfig should return a valid config even with empty paths")
	}
}