package service

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// "github.com/caddyserver/certmagic" // TODO: 需要下载依赖
	"x-ui/logger"
)

// CertMagicDomainService 实现 CertMagic 域名证书服务
type CertMagicDomainService struct {
	baseDir      string
	portResolver *PortConflictResolver
	certReloader *CertHotReloader
	config       *CertMagicConfig

	// TODO: CertMagic 相关配置 (需要安装依赖后启用)
	// certMagicConfig *certmagic.Config
	managedDomains map[string]*DomainCertInfo
	mu             sync.RWMutex

	// 自动续期控制
	autoRenewalStop chan struct{}
	autoRenewalDone chan struct{}
}

// CertMagicConfig CertMagic 服务配置
type CertMagicConfig struct {
	StoragePath      string
	ACMEServerURL    string
	UserAgent        string
	RenewalThreshold time.Duration
	CheckInterval    time.Duration
}

// DomainCertInfo 域名证书信息
type DomainCertInfo struct {
	Domain      string
	Email       string
	Expiry      time.Time
	Options     *CertOptions
	LastRenewal time.Time
}

// CertOptions 证书申请选项
type CertOptions struct {
	ChallengeType  string            // "http-01", "tls-alpn-01", "dns-01"
	DNSProvider    string            // 如 "cloudflare", "route53" 等
	DNSCredentials map[string]string // DNS API 凭证
}

// CertMagicCertInfo CertMagic 证书信息
type CertMagicCertInfo struct {
	Identifier string
	Type       string
	Provider   string
	Expiry     time.Time
	AutoRenew  bool
}

// NewCertMagicDomainService 创建新的 CertMagicDomainService 实例
func NewCertMagicDomainService(portResolver *PortConflictResolver, certReloader *CertHotReloader) *CertMagicDomainService {
	config := &CertMagicConfig{
		StoragePath:      getEnvOrDefault("CERTMAGIC_STORAGE_PATH", "bin/cert/domains"),
		ACMEServerURL:    getEnvOrDefault("CERTMAGIC_ACME_SERVER", "https://acme-v02.api.letsencrypt.org/directory"),
		UserAgent:        getEnvOrDefault("CERTMAGIC_USER_AGENT", "x-ui-certmagic/1.0.0"),
		RenewalThreshold: 30 * 24 * time.Hour, // 30 天
		CheckInterval:    24 * time.Hour,      // 每天检查一次
	}

	service := &CertMagicDomainService{
		baseDir:         config.StoragePath,
		portResolver:    portResolver,
		certReloader:    certReloader,
		config:          config,
		managedDomains:  make(map[string]*DomainCertInfo),
		autoRenewalStop: make(chan struct{}),
		autoRenewalDone: make(chan struct{}),
	}

	// 初始化 CertMagic 配置
	service.initCertMagic()

	return service
}

// initCertMagic 初始化 CertMagic 配置
func (s *CertMagicDomainService) initCertMagic() {
	// TODO: 需要安装 CertMagic 依赖后实现
	// 设置 CertMagic 默认配置
	// certmagic.DefaultACME.Email = "" // 将在申请时设置
	// certmagic.DefaultACME.Agreed = true
	// certmagic.DefaultACME.ServerURL = s.config.ACMEServerURL
	// certmagic.DefaultACME.UserAgent = s.config.UserAgent

	// 设置存储
	// certmagic.Default.Storage = &certmagic.FileStorage{Path: s.config.StoragePath}

	// 创建配置
	// s.certMagicConfig = certmagic.NewDefault()

	// 设置续期检查间隔
	// s.certMagicConfig.RenewalWindowRatio = 0.0 // 手动控制续期
	// s.certMagicConfig.MustStaple = false       // OCSP Stapling

	logger.Infof("CertMagic initialization skipped (dependency not available): %s", s.config.StoragePath)
}

// ObtainDomainCert 申请域名证书
func (s *CertMagicDomainService) ObtainDomainCert(domain, email string, opts *CertOptions) (*CertResult, error) {
	logger.Info("Starting domain certificate obtain")

	// 验证参数
	if err := s.ValidateDomain(domain); err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}
	if email == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}
	if opts == nil {
		opts = &CertOptions{ChallengeType: "http-01"}
	}

	// 处理端口冲突（如果需要 HTTP-01 或 TLS-ALPN-01）
	if opts.ChallengeType == "http-01" || opts.ChallengeType == "tls-alpn-01" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := s.acquirePortsForChallenge(ctx, opts.ChallengeType); err != nil {
			return nil, fmt.Errorf("failed to acquire ports for challenge: %w", err)
		}
		defer s.releasePortsForChallenge(opts.ChallengeType)
	}

	// 设置 DNS 提供商（如果使用 DNS-01）
	if opts.ChallengeType == "dns-01" {
		if err := s.setupDNSProvider(opts); err != nil {
			return nil, fmt.Errorf("failed to setup DNS provider: %w", err)
		}
	}

	// TODO: 使用 CertMagic 申请证书 (需要安装依赖)
	// 临时设置邮箱
	// certmagic.DefaultACME.Email = email
	// defer func() { certmagic.DefaultACME.Email = "" }()

	// TODO: 实现 CertMagic 证书申请
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	// defer cancel()
	// err := s.certMagicConfig.ObtainCert(ctx, domain, false)
	// if err != nil {
	//     return nil, fmt.Errorf("failed to obtain certificate: %w", err)
	// }

	// 临时实现：创建证书目录结构
	certDir := filepath.Join(s.baseDir, "certificates", domain)
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create certificate directory: %w", err)
	}

	logger.Warning("CertMagic certificate obtain not implemented yet")
	return nil, fmt.Errorf("CertMagic dependency not available - certificate obtain skipped")

	/*
		// 获取证书路径
		certPath, keyPath := s.getCertPaths(domain)

		// 解析证书过期时间
		expiry, err := s.parseCertificateExpiry(certPath)
		if err != nil {
			logger.Warningf("Failed to parse certificate expiry: %v", err)
			expiry = time.Now().Add(90 * 24 * time.Hour) // 默认 90 天
		}

		// 保存域名信息
		s.mu.Lock()
		s.managedDomains[domain] = &DomainCertInfo{
			Domain:      domain,
			Email:       email,
			Expiry:      expiry,
			Options:     opts,
			LastRenewal: time.Now(),
		}
		s.mu.Unlock()

		result := &CertResult{
			CertPath: certPath,
			KeyPath:  keyPath,
			Expiry:   expiry,
		}

		// 触发证书重载
		if s.certReloader != nil {
			if err := s.certReloader.OnCertRenewed(certPath, keyPath); err != nil {
				logger.Warningf("Failed to reload certificate: %v", err)
			}
		}

		logger.Info("Successfully obtained domain certificate")
		return result, nil
	*/
}

// RenewDomainCert 续期域名证书
func (s *CertMagicDomainService) RenewDomainCert(domain string) (*CertResult, error) {
	logger.Infof("Starting domain certificate renewal for %s", domain)

	s.mu.RLock()
	info, exists := s.managedDomains[domain]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("domain %s is not managed", domain)
	}

	// 检查证书是否存在
	if exists, err := s.certificateExists(domain); err != nil {
		return nil, fmt.Errorf("failed to check certificate existence: %w", err)
	} else if !exists {
		return nil, fmt.Errorf("certificate for domain %s does not exist", domain)
	}

	// 重新申请证书（续期本质上是重新申请）
	return s.ObtainDomainCert(domain, info.Email, info.Options)
}

// GetCertInfo 获取证书信息
func (s *CertMagicDomainService) GetCertInfo(domain string) (*CertMagicCertInfo, error) {
	certPath, _ := s.getCertPaths(domain)

	// 检查证书文件是否存在
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate file not found: %s", certPath)
	}

	// 读取证书
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	// 解析证书
	expiry, err := s.parseCertificateExpiryFromData(certData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate expiry: %w", err)
	}

	s.mu.RLock()
	info, exists := s.managedDomains[domain]
	autoRenew := exists && info.Options != nil
	s.mu.RUnlock()

	return &CertMagicCertInfo{
		Identifier: domain,
		Type:       "Domain",
		Provider:   "Let's Encrypt",
		Expiry:     expiry,
		AutoRenew:  autoRenew,
	}, nil
}

// NeedsRenewal 检查证书是否需要续期
func (s *CertMagicDomainService) NeedsRenewal(domain string) (bool, error) {
	info, err := s.GetCertInfo(domain)
	if err != nil {
		return false, err
	}

	// 证书过期前 30 天续期
	renewalThreshold := time.Now().Add(s.config.RenewalThreshold)
	return info.Expiry.Before(renewalThreshold), nil
}

// ListManagedDomains 获取所有托管的域名
func (s *CertMagicDomainService) ListManagedDomains() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	domains := make([]string, 0, len(s.managedDomains))
	for domain := range s.managedDomains {
		domains = append(domains, domain)
	}
	return domains, nil
}

// StartAutoRenewal 启动自动续期后台任务
func (s *CertMagicDomainService) StartAutoRenewal() error {
	logger.Info("Starting automatic certificate renewal")

	select {
	case <-s.autoRenewalDone:
		// 如果之前已经停止，重新创建通道
		s.autoRenewalStop = make(chan struct{})
		s.autoRenewalDone = make(chan struct{})
	default:
		// 已经在运行
		logger.Info("Automatic renewal already running")
		return nil
	}

	go s.autoRenewalLoop()
	return nil
}

// StopAutoRenewal 停止自动续期
func (s *CertMagicDomainService) StopAutoRenewal() {
	logger.Info("Stopping automatic certificate renewal")

	select {
	case s.autoRenewalStop <- struct{}{}:
		// 发送停止信号
	case <-time.After(5 * time.Second):
		logger.Warning("Timeout waiting to send stop signal")
		return
	}

	select {
	case <-s.autoRenewalDone:
		logger.Info("Automatic renewal stopped")
	case <-time.After(10 * time.Second):
		logger.Warning("Timeout waiting for renewal loop to stop")
	}
}

// autoRenewalLoop 自动续期循环
func (s *CertMagicDomainService) autoRenewalLoop() {
	defer close(s.autoRenewalDone)

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.autoRenewalStop:
			return
		case <-ticker.C:
			s.checkAndRenewCertificates()
		}
	}
}

// checkAndRenewCertificates 检查并续期到期的证书
func (s *CertMagicDomainService) checkAndRenewCertificates() {
	domains, err := s.ListManagedDomains()
	if err != nil {
		logger.Errorf("Failed to list managed domains: %v", err)
		return
	}

	for _, domain := range domains {
		needsRenewal, err := s.NeedsRenewal(domain)
		if err != nil {
			logger.Errorf("Failed to check renewal for %s: %v", domain, err)
			continue
		}

		if needsRenewal {
			logger.Infof("Certificate for %s needs renewal", domain)
			if _, err := s.RenewDomainCert(domain); err != nil {
				logger.Errorf("Failed to renew certificate for %s: %v", domain, err)
			}
		}
	}
}

// ValidateDomain 验证域名格式
func (s *CertMagicDomainService) ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	// 基本的域名格式检查
	if len(domain) > 253 {
		return fmt.Errorf("domain name too long")
	}

	// 检查是否包含无效字符
	if strings.ContainsAny(domain, " \t\n\r") {
		return fmt.Errorf("domain contains invalid characters")
	}

	// 检查标签格式
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) == 0 {
			return fmt.Errorf("domain contains empty label")
		}
		if len(label) > 63 {
			return fmt.Errorf("domain label too long")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("domain label cannot start or end with hyphen")
		}
	}

	return nil
}

// acquirePortsForChallenge 根据 Challenge 类型获取端口
func (s *CertMagicDomainService) acquirePortsForChallenge(ctx context.Context, challengeType string) error {
	switch challengeType {
	case "http-01":
		return s.portResolver.AcquirePort80(ctx)
	case "tls-alpn-01":
		// TLS-ALPN-01 需要 443 端口，这里简化处理，实际可能需要扩展 PortResolver
		return s.portResolver.AcquirePort80(ctx) // 先获取 80，后续扩展
	default:
		return nil // DNS-01 不需要端口
	}
}

// releasePortsForChallenge 释放端口
func (s *CertMagicDomainService) releasePortsForChallenge(challengeType string) {
	switch challengeType {
	case "http-01", "tls-alpn-01":
		if err := s.portResolver.ReleasePort80(); err != nil {
			logger.Warningf("Failed to release port 80: %v", err)
		}
	}
}

// setupDNSProvider 设置 DNS 提供商
func (s *CertMagicDomainService) setupDNSProvider(opts *CertOptions) error {
	if opts.DNSProvider == "" {
		return fmt.Errorf("DNS provider not specified")
	}

	// 这里需要根据不同 DNS 提供商设置相应的配置
	// CertMagic 支持多种 DNS 提供商，但需要相应的库支持
	// 这里是一个简化实现，实际需要根据提供商动态加载

	logger.Infof("Setting up DNS provider: %s", opts.DNSProvider)
	// TODO: 实现 DNS 提供商配置
	return fmt.Errorf("DNS provider %s not yet implemented", opts.DNSProvider)
}

// certificateExists 检查证书是否存在
func (s *CertMagicDomainService) certificateExists(domain string) (bool, error) {
	certPath, _ := s.getCertPaths(domain)
	_, err := os.Stat(certPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// getCertPaths 获取证书路径
func (s *CertMagicDomainService) getCertPaths(domain string) (certPath, keyPath string) {
	// CertMagic 默认存储结构
	certDir := filepath.Join(s.baseDir, "certificates", domain)
	certPath = filepath.Join(certDir, fmt.Sprintf("%s.crt", domain))
	keyPath = filepath.Join(certDir, fmt.Sprintf("%s.key", domain))
	return
}

// parseCertificateExpiry 解析证书过期时间
func (s *CertMagicDomainService) parseCertificateExpiry(certPath string) (time.Time, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return time.Time{}, err
	}
	return s.parseCertificateExpiryFromData(data)
}

// parseCertificateExpiryFromData 从证书数据解析过期时间
func (s *CertMagicDomainService) parseCertificateExpiryFromData(certData []byte) (time.Time, error) {
	block, _ := pem.Decode(certData)
	if block == nil {
		return time.Time{}, fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert.NotAfter, nil
}
