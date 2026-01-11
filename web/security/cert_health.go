package security

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"x-ui/logger"
)

// CertHealthChecker 证书健康检查器接口
type CertHealthChecker interface {
	Check(certPath, keyPath string) (time.Time, error)
	ValidateIP(certPath string, ip string) error
	ValidateChain(certPath, caPath string) error
}

// certHealthCheckerImpl 证书健康检查器的实现
type certHealthCheckerImpl struct{}

// NewCertHealthChecker 创建证书健康检查器
func NewCertHealthChecker() CertHealthChecker {
	return &certHealthCheckerImpl{}
}

// Check 检查证书文件并返回到期时间
func (chc *certHealthCheckerImpl) Check(certPath, keyPath string) (time.Time, error) {
	// 加载证书和密钥
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("加载证书失败: %w", err)
	}

	// 解析证书
	parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("解析证书失败: %w", err)
	}

	// 检查证书是否即将过期（30天内）
	if time.Until(parsedCert.NotAfter) < 30*24*time.Hour {
		logger.Warningf("证书即将过期: %s, 到期时间: %s", certPath, parsedCert.NotAfter.Format("2006-01-02"))
	}

	return parsedCert.NotAfter, nil
}

// ValidateIP 验证证书是否包含指定的IP地址
func (chc *certHealthCheckerImpl) ValidateIP(certPath string, ip string) error {
	certData, err := ioutil.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("读取证书文件失败: %w", err)
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return fmt.Errorf("无效的PEM格式")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("解析证书失败: %w", err)
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("无效的IP地址: %s", ip)
	}

	// 检查IP是否在证书的IP SAN中
	for _, certIP := range cert.IPAddresses {
		if certIP.Equal(parsedIP) {
			return nil
		}
	}

	// 检查是否在DNS名称中（虽然不标准，但有时会这样配置）
	for _, dnsName := range cert.DNSNames {
		if dnsName == ip {
			return nil
		}
	}

	return fmt.Errorf("IP地址 %s 不在证书的主题备用名称中", ip)
}

// ValidateChain 验证证书链完整性
func (chc *certHealthCheckerImpl) ValidateChain(certPath, caPath string) error {
	certData, err := ioutil.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("读取证书文件失败: %w", err)
	}

	caData, err := ioutil.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("读取CA证书文件失败: %w", err)
	}

	// 解码PEM
	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return fmt.Errorf("无效的证书PEM格式")
	}

	caBlock, _ := pem.Decode(caData)
	if caBlock == nil {
		return fmt.Errorf("无效的CA证书PEM格式")
	}

	// 解析证书
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析证书失败: %w", err)
	}

	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析CA证书失败: %w", err)
	}

	// 创建证书池
	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	// 验证证书链
	opts := x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: time.Now(),
	}

	chains, err := cert.Verify(opts)
	if err != nil {
		return fmt.Errorf("证书链验证失败: %w", err)
	}

	if len(chains) == 0 {
		return fmt.Errorf("未找到有效的证书链")
	}

	logger.Infof("证书链验证成功，共 %d 条链", len(chains))
	return nil
}

// CertMonitor 证书监控器
type CertMonitor struct {
	checker CertHealthChecker
}

// NewCertMonitor 创建证书监控器
func NewCertMonitor() *CertMonitor {
	return &CertMonitor{
		checker: NewCertHealthChecker(),
	}
}

// MonitorCert 监控证书健康状态
func (cm *CertMonitor) MonitorCert(certPath, keyPath string) error {
	expiry, err := cm.checker.Check(certPath, keyPath)
	if err != nil {
		return err
	}

	daysUntilExpiry := int(time.Until(expiry).Hours() / 24)
	if daysUntilExpiry <= 30 {
		logger.Warningf("证书将在 %d 天后过期: %s", daysUntilExpiry, certPath)
	} else {
		logger.Infof("证书健康检查通过，还有 %d 天到期", daysUntilExpiry)
	}

	return nil
}

// IsCertValid 检查证书是否有效
func IsCertValid(certPath, keyPath string) bool {
	checker := NewCertHealthChecker()
	_, err := checker.Check(certPath, keyPath)
	return err == nil
}