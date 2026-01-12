package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"

	"x-ui/logger"
)

// AlertService 定义告警服务接口
type AlertService interface {
	SendAlert(title, message string, level string) error
}

// FallbackManager 定义回退管理接口
type FallbackManager interface {
	// CheckAndFallback 检查状态并执行回退
	CheckAndFallback(certPath string) error
}

// CertAlertFallback 证书告警与回退管理器
type CertAlertFallback struct {
	alertService        AlertService
	certService         *CertService
	settingService      *SettingService
	consecutiveFailures int
	lastSuccessTime     time.Time
	inFallbackMode      bool
}

// NewCertAlertFallback 创建新的告警与回退管理器
func NewCertAlertFallback(alertService AlertService, certService *CertService, settingService *SettingService) *CertAlertFallback {
	return &CertAlertFallback{
		alertService:        alertService,
		certService:         certService,
		settingService:      settingService,
		consecutiveFailures: 0,
		lastSuccessTime:     time.Now(),
		inFallbackMode:      false,
	}
}

// OnRenewalFailed 续期失败回调
func (c *CertAlertFallback) OnRenewalFailed(err error, attempt int) error {
	c.consecutiveFailures++

	logger.Warningf("Certificate renewal failed (attempt %d): %v", attempt, err)

	// 获取证书信息用于告警
	certInfo, certErr := c.getCertInfo()
	if certErr != nil {
		logger.Warningf("Failed to get certificate info for alert: %v", certErr)
		return err
	}

	// 检查是否需要发送告警（连续失败且剩余时间少于1天）
	if certInfo != nil {
		remaining := time.Until(certInfo.Expiry)
		if remaining < 24*time.Hour && c.consecutiveFailures > 0 {
			alertErr := c.CheckAndAlert()
			if alertErr != nil {
				logger.Warningf("Failed to send alert: %v", alertErr)
			}
		}
	}

	return err
}

// CheckAndAlert 检查并发送告警
func (c *CertAlertFallback) CheckAndAlert() error {
	// 获取证书信息
	certInfo, err := c.getCertInfo()
	if err != nil {
		return fmt.Errorf("failed to get certificate info: %w", err)
	}

	if certInfo == nil {
		return errors.New("certificate info is nil")
	}

	remaining := time.Until(certInfo.Expiry)

	// 严重告警：剩余时间小于1天
	if remaining < 24*time.Hour {
		message := fmt.Sprintf(
			"⚠️ **IP 证书紧急告警**\n\n"+
				"IP: `%s`\n"+
				"剩余时间: %s\n"+
				"连续失败次数: %d\n"+
				"最后成功时间: %s\n"+
				"状态: **即将过期**\n\n"+
				"请立即检查面板日志或手动续期！",
			certInfo.IP,
			remaining.String(),
			c.consecutiveFailures,
			c.lastSuccessTime.Format("2006-01-02 15:04:05"),
		)

		if err := c.SendTelegramAlert(message); err != nil {
			logger.Errorf("Failed to send Telegram alert: %v", err)
			return err
		}
	}

	return nil
}

// SendTelegramAlert 发送 Telegram 告警
func (c *CertAlertFallback) SendTelegramAlert(message string) error {
	if c.alertService == nil {
		return errors.New("alert service is not configured")
	}

	return c.alertService.SendAlert("Certificate Alert", message, "CRITICAL")
}

// TriggerFallback 触发回退机制
func (c *CertAlertFallback) TriggerFallback() error {
	logger.Warning("Triggering certificate fallback mechanism")

	// 获取证书路径
	certPath, err := c.settingService.GetIpCertPath()
	if err != nil {
		return fmt.Errorf("failed to get IP cert path: %w", err)
	}
	if certPath == "" {
		return errors.New("IP cert path is empty")
	}

	// 获取 IP 地址
	ip, err := c.settingService.GetIpCertTarget()
	if err != nil {
		return fmt.Errorf("failed to get IP cert target: %w", err)
	}
	if ip == "" {
		return errors.New("IP cert target is empty")
	}

	// 执行回退到自签名证书
	if err := c.SwitchToSelfSigned(certPath, ip); err != nil {
		return fmt.Errorf("failed to switch to self-signed certificate: %w", err)
	}

	c.inFallbackMode = true

	// 返回回退已激活的错误
	fallbackMessage := fmt.Sprintf(
		"🔄 **证书回退执行成功**\n\n"+
			"已切换到自签名证书以维持服务运行。\n"+
			"IP: `%s`\n"+
			"请尽快修复受信任的证书配置。",
		ip,
	)

	if err := c.SendTelegramAlert(fallbackMessage); err != nil {
		logger.Warningf("Failed to send fallback notification: %v", err)
	}

	return WrapError(ErrCodeFallbackActivated, nil)
}

// SwitchToSelfSigned 切换到自签名证书
func (c *CertAlertFallback) SwitchToSelfSigned(certPath, ip string) error {
	logger.Info("Generating self-signed certificate for fallback")

	// 生成自签名证书
	certPEM, keyPEM, err := c.generateSelfSignedCert(ip)
	if err != nil {
		return fmt.Errorf("failed to generate self-signed certificate: %w", err)
	}

	// 备份原有证书
	if err := c.backupExistingCerts(certPath); err != nil {
		logger.Warningf("Failed to backup existing certificates: %v", err)
		// 继续执行，不因备份失败而中断
	}

	// 写入新证书
	certFile := certPath + ".crt"
	keyFile := certPath + ".key"

	if err := os.WriteFile(certFile, certPEM, 0o644); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}

	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	logger.Infof("Successfully switched to self-signed certificate for IP %s", ip)
	return nil
}

// IsInFallbackMode 检查是否处于回退模式
func (c *CertAlertFallback) IsInFallbackMode() bool {
	return c.inFallbackMode
}

// generateSelfSignedCert 生成自签名证书
func (c *CertAlertFallback) generateSelfSignedCert(ip string) ([]byte, []byte, error) {
	// 生成 RSA 私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"X-Panel Fallback"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(30 * 24 * time.Hour), // 30 天有效期
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		IPAddresses: []net.IP{net.ParseIP(ip)},
	}

	// 创建自签名证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// 编码为 PEM 格式
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	keyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyDER,
	})

	return certPEM, keyPEM, nil
}

// backupExistingCerts 备份现有证书
func (c *CertAlertFallback) backupExistingCerts(certPath string) error {
	certFile := certPath + ".crt"
	keyFile := certPath + ".key"

	timestamp := time.Now().Format("20060102_150405")

	backupCert := certPath + ".crt.backup." + timestamp
	backupKey := certPath + ".key.backup." + timestamp

	// 备份证书文件
	if err := c.copyFile(certFile, backupCert); err != nil {
		return fmt.Errorf("failed to backup certificate: %w", err)
	}

	// 备份密钥文件
	if err := c.copyFile(keyFile, backupKey); err != nil {
		return fmt.Errorf("failed to backup key: %w", err)
	}

	logger.Infof("Backed up existing certificates to %s and %s", backupCert, backupKey)
	return nil
}

// copyFile 复制文件
func (c *CertAlertFallback) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// getCertInfo 获取证书信息
func (c *CertAlertFallback) getCertInfo() (*CertFallbackInfo, error) {
	// 获取证书路径
	certPath, err := c.settingService.GetIpCertPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get IP cert path: %w", err)
	}
	if certPath == "" {
		return nil, errors.New("IP cert path is empty")
	}

	// 获取 IP 地址
	ip, err := c.settingService.GetIpCertTarget()
	if err != nil {
		return nil, fmt.Errorf("failed to get IP cert target: %w", err)
	}

	certFile := certPath + ".crt"
	keyFile := certPath + ".key"

	// 检查文件是否存在
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return &CertFallbackInfo{
			Path:   certFile,
			IP:     ip,
			Expiry: time.Now().Add(-24 * time.Hour), // 视为已过期
		}, nil
	}

	// 加载证书
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	// 解析证书
	if len(cert.Certificate) == 0 {
		return nil, errors.New("no certificate data found")
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &CertFallbackInfo{
		Path:   certFile,
		IP:     ip,
		Expiry: x509Cert.NotAfter,
	}, nil
}

// CertFallbackInfo 证书回退信息
type CertFallbackInfo struct {
	Path   string
	IP     string
	Expiry time.Time
}
