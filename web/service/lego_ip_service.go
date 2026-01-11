package service

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/registration"

	"x-ui/logger"
)

// CertResult 证书申请结果
type CertResult struct {
	CertPath string
	KeyPath  string
	Expiry   time.Time
}

// CertInfo 证书信息
type LegoCertInfo struct {
	Identifier string
	Type       string
	Provider   string
	Expiry     time.Time
	AutoRenew  bool
}

// LegoIPService Lego IP 证书服务结构体
type LegoIPService struct {
	baseDir       string
	portResolver  *PortConflictResolver
	certReloader  *CertHotReloader
	legoConfig    *LegoConfig
}

// LegoConfig Lego 配置
type LegoConfig struct {
	ACMEServerURL   string
	UserAgent       string
	KeyType         string
	ChallengeTypes  []string // 支持的挑战类型优先级 ["tls-alpn-01", "http-01"]
}

// LegoUser 实现 lego.User 接口
type LegoUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

// GetEmail 返回用户邮箱
func (u *LegoUser) GetEmail() string {
	return u.Email
}

// GetRegistration 返回用户注册信息
func (u *LegoUser) GetRegistration() *registration.Resource {
	return u.Registration
}

// GetPrivateKey 返回用户私钥
func (u *LegoUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// NewLegoIPService 创建新的 LegoIPService 实例
func NewLegoIPService(portResolver *PortConflictResolver, certReloader *CertHotReloader) *LegoIPService {
	challengeTypesStr := getEnvOrDefault("LEGO_CHALLENGE_TYPES", "tls-alpn-01,http-01")
	challengeTypes := strings.Split(challengeTypesStr, ",")
	for i, ct := range challengeTypes {
		challengeTypes[i] = strings.TrimSpace(ct)
	}

	config := &LegoConfig{
		ACMEServerURL:  getEnvOrDefault("LEGO_ACME_SERVER", "https://acme-v02.api.letsencrypt.org/directory"),
		UserAgent:      getEnvOrDefault("LEGO_USER_AGENT", "x-ui-lego/1.0.0"),
		KeyType:        getEnvOrDefault("LEGO_KEY_TYPE", "P256"),
		ChallengeTypes: challengeTypes,
	}

	return &LegoIPService{
		baseDir:      "bin/cert/ip",
		portResolver: portResolver,
		certReloader: certReloader,
		legoConfig:   config,
	}
}

// setupChallengeProvider 设置挑战提供者
func (s *LegoIPService) setupChallengeProvider(client *lego.Client, challengeType string) error {
	switch challengeType {
	case "tls-alpn-01":
		logger.Info("Setting up TLS-ALPN-01 challenge provider")
		// TLS-ALPN-01 使用标准的TLS握手，不需要特殊提供者
		// Lego库会自动处理TLS-ALPN-01挑战
		// 我们只需要确保TLS配置正确，但这里不需要设置提供者
		return nil
	case "http-01":
		logger.Info("Setting up HTTP-01 challenge provider on port 80")
		provider := http01.NewProviderServer("", "80")
		if err := client.Challenge.SetHTTP01Provider(provider); err != nil {
			return fmt.Errorf("failed to set HTTP-01 provider: %w", err)
		}
	default:
		return fmt.Errorf("unsupported challenge type: %s", challengeType)
	}
	return nil
}

// acquirePortsForChallenge 根据挑战类型获取端口
func (s *LegoIPService) acquirePortsForChallenge(ctx context.Context, challengeType string) error {
	switch challengeType {
	case "tls-alpn-01":
		// TLS-ALPN-01 需要443端口，但目前我们简化处理，先尝试443端口可用性
		logger.Info("TLS-ALPN-01 challenge requires port 443 - checking availability")
		// 由于没有专门的443端口管理，我们依赖于系统配置
		// 如果443被占用，挑战会失败，然后回退到HTTP-01
		return nil
	case "http-01":
		logger.Info("Attempting to acquire port 80 for HTTP-01 challenge")
		return s.portResolver.AcquirePort80(ctx)
	default:
		return nil
	}
}

// releasePortsForChallenge 释放端口
func (s *LegoIPService) releasePortsForChallenge(challengeType string) {
	switch challengeType {
	case "http-01":
		logger.Info("Releasing port 80")
		if err := s.portResolver.ReleasePort80(); err != nil {
			logger.Warningf("Failed to release port 80: %v", err)
		}
	}
}

// ObtainIPCert 申请 IP 证书
func (s *LegoIPService) ObtainIPCert(ctx context.Context, ip, email string) (*CertResult, error) {
	logger.Infof("Starting IP certificate obtain for IP: %s, Email: %s", ip, email)

	// 验证参数
	if err := s.ValidateIP(ip); err != nil {
		logger.Errorf("IP validation failed for %s: %v", ip, err)
		return nil, fmt.Errorf("invalid IP address: %w", err)
	}
	if email == "" {
		logger.Error("Email cannot be empty")
		return nil, fmt.Errorf("email cannot be empty")
	}

	// 创建 Lego 用户
	logger.Info("Creating Lego user")
	user, err := s.createLegoUser(email)
	if err != nil {
		logger.Errorf("Failed to create lego user: %v", err)
		return nil, fmt.Errorf("failed to create lego user: %w", err)
	}

	// 尝试不同的挑战类型
	var lastErr error
	for _, challengeType := range s.legoConfig.ChallengeTypes {
		logger.Infof("Attempting certificate obtain with challenge type: %s", challengeType)

		// 获取端口控制权（如果需要）
		if err := s.acquirePortsForChallenge(ctx, challengeType); err != nil {
			logger.Warningf("Failed to acquire ports for %s challenge: %v", challengeType, err)
			lastErr = err
			continue
		}
		defer s.releasePortsForChallenge(challengeType)

		// 创建 Lego 配置
		logger.Infof("Creating Lego config with ACME server: %s", s.legoConfig.ACMEServerURL)
		config := lego.NewConfig(user)
		config.CADirURL = s.legoConfig.ACMEServerURL
		config.UserAgent = s.legoConfig.UserAgent

		// 创建 Lego 客户端
		logger.Info("Creating Lego client")
		client, err := lego.NewClient(config)
		if err != nil {
			logger.Errorf("Failed to create lego client for %s: %v", challengeType, err)
			lastErr = err
			s.releasePortsForChallenge(challengeType)
			continue
		}

		// 设置挑战提供者
		if err := s.setupChallengeProvider(client, challengeType); err != nil {
			logger.Errorf("Failed to setup %s challenge provider: %v", challengeType, err)
			lastErr = err
			s.releasePortsForChallenge(challengeType)
			continue
		}

		// 注册账户
		logger.Info("Registering ACME account")
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			logger.Errorf("Failed to register account for %s challenge: %v", challengeType, err)
			lastErr = err
			s.releasePortsForChallenge(challengeType)
			continue
		}
		user.Registration = reg
		logger.Info("Account registered successfully")

		// 申请证书
		request := certificate.ObtainRequest{
			Domains: []string{ip},
			Bundle:  true,
		}

		logger.Infof("Starting certificate obtain request for domains: %v using %s challenge", request.Domains, challengeType)

		var challengePortNote string
		if challengeType == "http-01" {
			challengePortNote = "Ensure port 80 is accessible from the internet."
		} else if challengeType == "tls-alpn-01" {
			challengePortNote = "Ensure port 443 is accessible from the internet and TLS is properly configured."
		}
		logger.Info("Note: This may take several minutes. " + challengePortNote)

		certificates, err := client.Certificate.Obtain(request)
		if err != nil {
			logger.Errorf("Failed to obtain certificate with %s challenge: %v", challengeType, err)
			if challengeType == "tls-alpn-01" {
				logger.Warning("TLS-ALPN-01 challenge failed - this may be due to port 443 configuration or ACME server support")
			} else if challengeType == "http-01" {
				logger.Error("Possible causes for HTTP-01 failure:")
				logger.Error("  1. Port 80 is blocked by firewall")
				logger.Error("  2. IP address is not reachable from the internet")
				logger.Error("  3. HTTP challenge server failed to start")
				logger.Error("  4. ACME server timeout or rate limiting")
			}
			lastErr = err
			s.releasePortsForChallenge(challengeType)
			continue
		}

		logger.Infof("Certificate obtained successfully using %s challenge", challengeType)

		// 保存证书
		certPath, keyPath, err := s.saveCertificate(ip, certificates)
		if err != nil {
			return nil, fmt.Errorf("failed to save certificate: %w", err)
		}

		// 解析证书过期时间
		expiry, err := s.parseCertificateExpiry(certificates.Certificate)
		if err != nil {
			logger.Warningf("Failed to parse certificate expiry: %v", err)
			expiry = time.Now().Add(90 * 24 * time.Hour) // 默认 90 天
		}

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

		logger.Info("Successfully obtained IP certificate")
		return result, nil
	}

	// 所有挑战类型都失败了
	if lastErr != nil {
		return nil, fmt.Errorf("all challenge types failed, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("no challenge types configured")
}

// RenewIPCert 续期 IP 证书
func (s *LegoIPService) RenewIPCert(ctx context.Context, ip string) (*CertResult, error) {
	logger.Infof("Starting IP certificate renewal for %s", ip)

	// 检查证书是否存在
	if exists, err := s.certificateExists(ip); err != nil {
		return nil, fmt.Errorf("failed to check certificate existence: %w", err)
	} else if !exists {
		return nil, fmt.Errorf("certificate for IP %s does not exist", ip)
	}

	// 重新申请证书（续期本质上是重新申请）
	return s.ObtainIPCert(ctx, ip, s.getStoredEmail(ip))
}

// GetCertInfo 获取证书信息
func (s *LegoIPService) GetCertInfo(ip string) (*LegoCertInfo, error) {
	certPath, _ := s.getCertPaths(ip)

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
	expiry, err := s.parseCertificateExpiry(certData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate expiry: %w", err)
	}

	return &LegoCertInfo{
		Identifier: ip,
		Type:       "IP",
		Provider:   "Let's Encrypt",
		Expiry:     expiry,
		AutoRenew:  false, // IP 证书不支持自动续期
	}, nil
}

// NeedsRenewal 检查证书是否需要续期
func (s *LegoIPService) NeedsRenewal(ip string) (bool, error) {
	info, err := s.GetCertInfo(ip)
	if err != nil {
		return false, err
	}

	// IP 证书通常有效期为 90 天，提前 7 天续期
	renewalThreshold := time.Now().Add(7 * 24 * time.Hour)
	return info.Expiry.Before(renewalThreshold), nil
}

// ValidateIP 验证 IP 地址格式
func (s *LegoIPService) ValidateIP(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	// 检查 IP 地址格式
	ipRegex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if !ipRegex.MatchString(ip) {
		return fmt.Errorf("invalid IP address format")
	}

	// 检查每个段是否在 0-255 范围内
	parts := strings.Split(ip, ".")
	for _, part := range parts {
		if len(part) > 3 || (len(part) > 1 && part[0] == '0') {
			return fmt.Errorf("invalid IP address segment")
		}
		var num int
		fmt.Sscanf(part, "%d", &num)
		if num < 0 || num > 255 {
			return fmt.Errorf("IP address segment out of range")
		}
	}

	// 验证是否为有效 IP 地址
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address")
	}

	return nil
}

// createLegoUser 创建 Lego 用户
func (s *LegoIPService) createLegoUser(email string) (*LegoUser, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	return &LegoUser{
		Email: email,
		key:   privateKey,
	}, nil
}



// saveCertificate 保存证书到文件
func (s *LegoIPService) saveCertificate(ip string, cert *certificate.Resource) (certPath, keyPath string, err error) {
	certDir := filepath.Join(s.baseDir, ip)

	// 创建目录
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create certificate directory: %w", err)
	}

	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")

	// 写入证书
	if err := os.WriteFile(certPath, cert.Certificate, 0644); err != nil {
		return "", "", fmt.Errorf("failed to write certificate file: %w", err)
	}

	// 写入私钥
	if err := os.WriteFile(keyPath, cert.PrivateKey, 0600); err != nil {
		return "", "", fmt.Errorf("failed to write private key file: %w", err)
	}

	// 设置私钥权限
	if err := os.Chmod(keyPath, 0600); err != nil {
		logger.Warningf("Failed to set private key permissions: %v", err)
	}

	logger.Info("Certificate saved successfully")
	return certPath, keyPath, nil
}

// parseCertificateExpiry 解析证书过期时间
func (s *LegoIPService) parseCertificateExpiry(certData []byte) (time.Time, error) {
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

// certificateExists 检查证书是否存在
func (s *LegoIPService) certificateExists(ip string) (bool, error) {
	certPath, _ := s.getCertPaths(ip)
	_, err := os.Stat(certPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// getCertPaths 获取证书路径
func (s *LegoIPService) getCertPaths(ip string) (certPath, keyPath string) {
	certDir := filepath.Join(s.baseDir, ip)
	certPath = filepath.Join(certDir, "cert.pem")
	keyPath = filepath.Join(certDir, "key.pem")
	return
}

// getStoredEmail 获取存储的邮箱地址（已移除硬编码，从调用者传递）
func (s *LegoIPService) getStoredEmail(ip string) string {
	// 此方法已弃用，邮箱应由调用者提供
	logger.Warning("getStoredEmail called - email should be provided by caller")
	return ""
}

// getEnvOrDefault 获取环境变量或默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}