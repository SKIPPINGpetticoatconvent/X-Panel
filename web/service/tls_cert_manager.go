package service

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"x-ui/logger"
)

// TLSCertManager 管理 TLS 证书的动态加载
type TLSCertManager struct {
	mu           sync.RWMutex
	currentCert  *tls.Certificate
	certPath     string
	keyPath      string
	alertService AlertService
}

// NewTLSCertManager 创建新的 TLS 证书管理器
func NewTLSCertManager(alertService AlertService) *TLSCertManager {
	return &TLSCertManager{
		alertService: alertService,
	}
}

// GetTLSConfig 返回 tls.Config，使用 GetCertificate 回调实现热重载
func (m *TLSCertManager) GetTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return m.currentCert, nil
		},
		MinVersion: tls.VersionTLS12,
	}
}

// ReloadCert 从磁盘重新加载证书到内存
func (m *TLSCertManager) ReloadCert() error {
	cert, err := tls.LoadX509KeyPair(m.certPath, m.keyPath)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.currentCert = &cert
	m.mu.Unlock()

	// 证书重载成功，发送 TG 通知
	if m.alertService != nil {
		message := fmt.Sprintf(
			"🔐 证书已更新\n\n"+
				"📍 证书路径: %s\n"+
				"⏰ 更新时间: %s\n"+
				"✅ 状态: 热重载成功",
			m.certPath,
			time.Now().Format("2006-01-02 15:04:05"),
		)

		// 异步发送通知，不影响证书重载
		go func() {
			if sendErr := m.alertService.SendAlert("证书更新通知", message, "INFO"); sendErr != nil {
				logger.Warningf("Failed to send certificate update alert: %v", sendErr)
			}
		}()
	}

	return nil
}

// SetCertPaths 设置证书路径
func (m *TLSCertManager) SetCertPaths(certPath, keyPath string) {
	m.mu.Lock()
	m.certPath = certPath
	m.keyPath = keyPath
	m.mu.Unlock()
}
