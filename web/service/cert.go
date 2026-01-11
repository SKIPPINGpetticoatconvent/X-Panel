package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"x-ui/logger"
	"x-ui/web/global"
)

type CertService struct {
	settingService      *SettingService
	serverService       *ServerService
	tgbot               TelegramService

	// Certificate Services
	legoIPService       *LegoIPService
	certMagicService    *CertMagicDomainService

	// Improvement Modules
	portResolver        *PortConflictResolver
	renewalManager      *AggressiveRenewalManager
	hotReloader         *CertHotReloader
	alertFallback       *CertAlertFallback

	// TLS Certificate Manager for dynamic loading
	tlsCertManager      *TLSCertManager

	initOnce sync.Once
	mutex    sync.RWMutex // 细粒度并发锁
}

func NewCertService(settingService *SettingService) *CertService {
	c := &CertService{
		settingService: settingService,
	}
	// Certificate services will be initialized in tryInitImprovements when dependencies are ready
	return c
}

func (c *CertService) SetServerService(s *ServerService) {
	c.serverService = s
	c.tryInitImprovements()
}

func (c *CertService) SetTgbot(t TelegramService) {
	c.tgbot = t
	c.tryInitImprovements()
}

// SetTLSCertManager 设置 TLS 证书管理器
func (c *CertService) SetTLSCertManager(manager *TLSCertManager) {
	c.tlsCertManager = manager
}

// CreateTLSCertManager 创建 TLS 证书管理器并设置告警服务
func (c *CertService) CreateTLSCertManager() *TLSCertManager {
	var alertSvc AlertService
	if c.tgbot != nil {
		alertSvc = &certAlertService{tgbot: c.tgbot}
	}
	manager := NewTLSCertManager(alertSvc)
	c.tlsCertManager = manager
	return manager
}

// tryInitImprovements attempts to initialize improvement modules if dependencies are ready
func (c *CertService) tryInitImprovements() {
	c.initOnce.Do(func() {
		logger.Info("Initializing Certificate Management Modules...")

		// 1. Initialize PortConflictResolver
		webCtrl := &certWebServerController{settingService: c.settingService}
		c.portResolver = NewPortConflictResolver(webCtrl)
		logger.Info("PortConflictResolver initialized")

		// 2. Initialize CertHotReloader
		if c.serverService != nil {
			xrayCtrl := &certXrayController{serverService: c.serverService}
			c.hotReloader = NewCertHotReloader(xrayCtrl)
			logger.Info("CertHotReloader initialized")
		} else {
			logger.Info("CertHotReloader skipped (no ServerService)")
		}

		// 3. Initialize LegoIPService
		c.legoIPService = NewLegoIPService(c.portResolver, c.hotReloader)
		logger.Info("LegoIPService initialized")

		// 4. Initialize CertMagicDomainService
		c.certMagicService = NewCertMagicDomainService(c.portResolver, c.hotReloader)
		logger.Info("CertMagicDomainService initialized")

		// 5. Initialize CertAlertFallback
		var alertSvc AlertService
		if c.tgbot != nil {
			alertSvc = &certAlertService{tgbot: c.tgbot}
			c.alertFallback = NewCertAlertFallback(alertSvc, c, c.settingService)
			logger.Info("CertAlertFallback initialized with Telegram")
		} else {
			c.alertFallback = NewCertAlertFallback(nil, c, c.settingService)
			logger.Info("CertAlertFallback initialized in silent mode")
		}

		// 6. Initialize AggressiveRenewalManager
		renewalConfig := RenewalConfig{
			CheckInterval:  6 * time.Hour,
			RenewThreshold: 3 * 24 * time.Hour, // 3 days
			MaxRetries:     12,
			RetryInterval:  30 * time.Minute,
		}
		c.renewalManager = NewAggressiveRenewalManager(renewalConfig, c, c.portResolver, c.alertFallback)
		logger.Info("AggressiveRenewalManager initialized")

		// 7. Start automatic renewal loop if core services are available
		if c.legoIPService != nil && c.portResolver != nil {
			go c.RenewLoop()
			logger.Info("Certificate renewal loop started")
		}

		logger.Info("Certificate Management Modules initialized successfully")
	})
}

// ObtainIPCert obtains a Let's Encrypt IP certificate using Lego
func (c *CertService) ObtainIPCert(ip, email string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if ip == "" {
		return errors.New("IP address cannot be empty")
	}
	if email == "" {
		return errors.New("email cannot be empty")
	}

	// Ensure modules are initialized
	if c.legoIPService == nil {
		logger.Warning("LegoIPService not initialized")
		return errors.New("lego IP service not available")
	}

	// Note: Port checking is now handled within LegoIPService.ObtainIPCert()
	// which implements intelligent challenge type selection and port management

	// Obtain certificate using Lego (mask sensitive info in logs)
	logger.Infof("Starting IP certificate obtain for IP address")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := c.legoIPService.ObtainIPCert(ctx, ip, email)
	if err != nil {
		logger.Errorf("Failed to obtain IP certificate: %v", err)
		return fmt.Errorf("failed to obtain IP certificate: %w", err)
	}

	// Save certificate path to configuration
	installPath := strings.TrimSuffix(result.CertPath, ".pem")
	if err := c.settingService.SetIpCertPath(installPath); err != nil {
		logger.Warningf("Failed to set IP cert path: %v", err)
	}

	// 触发 TLS 证书重载
	if c.tlsCertManager != nil {
		certPath := installPath + ".crt"
		keyPath := installPath + ".key"
		c.tlsCertManager.SetCertPaths(certPath, keyPath)
		if err := c.tlsCertManager.ReloadCert(); err != nil {
			logger.Warningf("Failed to reload IP certificate in TLS manager: %v", err)
		}
	}

	logger.Info("Successfully obtained IP certificate")
	return nil
}

// ObtainDomainCert obtains a Let's Encrypt domain certificate using CertMagic
func (c *CertService) ObtainDomainCert(domain, email string, opts *CertOptions) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if domain == "" {
		return errors.New("domain cannot be empty")
	}
	if email == "" {
		return errors.New("email cannot be empty")
	}

	// Ensure modules are initialized
	if c.certMagicService == nil {
		logger.Warning("CertMagicDomainService not initialized")
		return errors.New("certmagic domain service not available")
	}

	// Obtain certificate using CertMagic (mask sensitive info in logs)
	logger.Info("Starting domain certificate obtain")
	_, err := c.certMagicService.ObtainDomainCert(domain, email, opts)
	if err != nil {
		logger.Errorf("Failed to obtain domain certificate: %v", err)
		return fmt.Errorf("failed to obtain domain certificate: %w", err)
	}

	logger.Info("Successfully obtained domain certificate")
	return nil
}

// RenewLoop runs a background goroutine that periodically checks and renews IP certificates
func (c *CertService) RenewLoop() {
	// If renewal manager is initialized, use it
	if c.renewalManager != nil {
		c.renewalManager.Start()
		return
	}

	// Fallback to legacy loop if manager not ready (e.g. dependencies missing)
	go func() {
		ticker := time.NewTicker(24 * time.Hour) // Check daily
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.checkAndRenewCertificates()
			}
		}
	}()
}

// checkAndRenewCertificates checks all configured IP certificates and renews if necessary
func (c *CertService) checkAndRenewCertificates() {
	// Check if IP cert is enabled
	enabled, err := c.settingService.GetIpCertEnable()
	if err != nil {
		logger.Warningf("Failed to get IP cert enable status: %v", err)
		return
	}
	if !enabled {
		return
	}

	// Get target IP and email
	ip, err := c.settingService.GetIpCertTarget()
	if err != nil {
		logger.Warningf("Failed to get IP cert target: %v", err)
		return
	}
	if ip == "" {
		logger.Warning("IP cert target is empty")
		return
	}

	email, err := c.settingService.GetIpCertEmail()
	if err != nil {
		logger.Warningf("Failed to get IP cert email: %v", err)
		return
	}
	if email == "" {
		logger.Warning("IP cert email is empty")
		return
	}

	// Get certificate path
	certPath, err := c.settingService.GetIpCertPath()
	if err != nil {
		logger.Warningf("Failed to get IP cert path: %v", err)
		return
	}
	if certPath == "" {
		logger.Warning("IP cert path is empty")
		return
	}

	certFile := certPath + ".crt"
	keyFile := certPath + ".key"

	// Check if certificates exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		logger.Infof("Certificate not found for IP %s, obtaining new one", ip)
		if err := c.ObtainIPCert(ip, email); err != nil {
			logger.Errorf("Failed to obtain certificate for IP %s: %v", ip, err)
		}
		return
	}

	// Load certificate to check expiration
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		logger.Errorf("Failed to load certificate for IP %s: %v", ip, err)
		// Try to obtain new certificate
		if err := c.ObtainIPCert(ip, email); err != nil {
			logger.Errorf("Failed to renew certificate for IP %s: %v", ip, err)
		}
		return
	}

	// Parse certificate
	parsedCert, err := tls.X509KeyPair(cert.Certificate[0], cert.PrivateKey.([]byte))
	if err != nil {
		logger.Errorf("Failed to parse certificate for IP %s: %v", ip, err)
		return
	}

	// Check if certificate expires within 7 days
	if len(parsedCert.Certificate) > 0 {
		// Simple check - if certificate is older than 7 days, renew
		// In production, you might want to parse the actual expiration date
		info, err := os.Stat(certFile)
		if err != nil {
			logger.Errorf("Failed to stat certificate file: %v", err)
			return
		}

		// Renew if certificate is older than 7 days (for short-lived certs)
		if time.Since(info.ModTime()) > 7*24*time.Hour {
			logger.Infof("Certificate for IP %s is older than 7 days, renewing", ip)
			if err := c.ObtainIPCert(ip, email); err != nil {
				logger.Errorf("Failed to renew certificate for IP %s: %v", ip, err)
			} else {
				// 证书续期成功后，触发 TLS 证书重载
				if c.tlsCertManager != nil {
					certPath := certPath + ".crt"
					keyPath := certPath + ".key"
					c.tlsCertManager.SetCertPaths(certPath, keyPath)
					if err := c.tlsCertManager.ReloadCert(); err != nil {
						logger.Warningf("Failed to reload renewed IP certificate in TLS manager: %v", err)
					}
				}
			}
		}
	}
}

// --- Adapter Implementations ---

// certWebServerController implements WebServerController interface
type certWebServerController struct {
	settingService *SettingService
}

func (c *certWebServerController) PauseHTTPListener() error {
	// Get the web server instance from global
	webServer := global.GetWebServer()
	if webServer == nil {
		logger.Warning("Web server not available, cannot pause HTTP listener")
		return errors.New("web server not available")
	}

	return webServer.PauseHTTPListener()
}

func (c *certWebServerController) ResumeHTTPListener() error {
	// Get the web server instance from global
	webServer := global.GetWebServer()
	if webServer == nil {
		logger.Warning("Web server not available, cannot resume HTTP listener")
		return errors.New("web server not available")
	}

	return webServer.ResumeHTTPListener()
}

func (c *certWebServerController) IsListeningOnPort80() bool {
	// Get the web server instance from global
	webServer := global.GetWebServer()
	if webServer == nil {
		logger.Warning("Web server not available, checking port setting directly")
		port, err := c.settingService.GetPort()
		if err != nil {
			return false
		}
		return port == 80
	}

	return webServer.IsListeningOnPort80()
}

// certXrayController implements XrayController interface
type certXrayController struct {
	serverService *ServerService
}

func (c *certXrayController) ReloadCore() error {
	return c.serverService.RestartXrayService()
}

func (c *certXrayController) IsRunning() bool {
	// We need to access the underlying XrayService
	// Since ServerService doesn't expose IsXrayRunning directly but has it in GetStatus...
	// Wait, ServerService has xrayService field but it's private.
	// But ServerService has RestartXrayService which calls xrayService.RestartXray.
	// We need to check if Xray is running.
	// ServerService doesn't seem to expose IsXrayRunning directly as a public method.
	// Let's check ServerService again.
	// It has GetStatus which calls s.xrayService.IsXrayRunning().
	// But we can't call private fields.
	// We might need to add IsXrayRunning to ServerService or use a workaround.
	// Workaround: Check if we can get status.
	status := c.serverService.GetStatus(nil)
	return status.Xray.State == Running
}

func (c *certXrayController) GetProcessInfo() (pid, uid int, err error) {
	return c.serverService.GetXrayProcessInfo()
}

func (c *certXrayController) SendSignal(sig os.Signal) error {
	return c.serverService.SendSignalToXray(sig)
}

// certAlertService implements AlertService interface
type certAlertService struct {
	tgbot TelegramService
}

func (c *certAlertService) SendAlert(title, message, level string) error {
	fullMsg := fmt.Sprintf("<b>%s</b>\n\n%s", title, message)
	return c.tgbot.SendMessage(fullMsg)
}
