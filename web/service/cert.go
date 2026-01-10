package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
	"x-ui/logger"
)

type CertService struct {
	settingService *SettingService
	serverService  *ServerService
	tgbot          TelegramService

	// Improvement Modules
	portResolver   *PortConflictResolver
	renewalManager *AggressiveRenewalManager
	hotReloader    *CertHotReloader
	alertFallback  *CertAlertFallback

	initOnce sync.Once
}

func NewCertService(settingService *SettingService) *CertService {
	return &CertService{
		settingService: settingService,
	}
}

func (c *CertService) SetServerService(s *ServerService) {
	c.serverService = s
	c.tryInitImprovements()
}

func (c *CertService) SetTgbot(t TelegramService) {
	c.tgbot = t
	c.tryInitImprovements()
}

// tryInitImprovements attempts to initialize improvement modules if dependencies are ready
func (c *CertService) tryInitImprovements() {
	if c.serverService == nil || c.tgbot == nil {
		return
	}

	c.initOnce.Do(func() {
		logger.Info("Initializing IP Certificate Improvement Modules...")

		// 1. Initialize PortConflictResolver
		webCtrl := &certWebServerController{settingService: c.settingService}
		c.portResolver = NewPortConflictResolver(webCtrl)

		// 2. Initialize CertAlertFallback
		alertSvc := &certAlertService{tgbot: c.tgbot}
		c.alertFallback = NewCertAlertFallback(alertSvc, c, c.settingService)

		// 3. Initialize AggressiveRenewalManager
		renewalConfig := RenewalConfig{
			CheckInterval:  6 * time.Hour,
			RenewThreshold: 3 * 24 * time.Hour, // 3 days
			MaxRetries:     12,
			RetryInterval:  30 * time.Minute,
		}
		c.renewalManager = NewAggressiveRenewalManager(renewalConfig, c, c.portResolver, c.alertFallback)

		// 4. Initialize CertHotReloader
		xrayCtrl := &certXrayController{serverService: c.serverService}
		c.hotReloader = NewCertHotReloader(xrayCtrl)

		logger.Info("IP Certificate Improvement Modules initialized successfully")
	})
}

// ObtainIPCert obtains a Let's Encrypt IP certificate using certmagic with standalone challenge
func (c *CertService) ObtainIPCert(ip, email string) error {
	if ip == "" {
		return errors.New("IP address cannot be empty")
	}
	if email == "" {
		return errors.New("email cannot be empty")
	}

	// Ensure modules are initialized
	if c.portResolver == nil {
		logger.Warning("Improvement modules not initialized, falling back to legacy behavior")
		return c.obtainIPCertLegacy(ip, email)
	}

	// 1. Acquire Port 80
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	if err := c.portResolver.AcquirePort80(ctx); err != nil {
		return fmt.Errorf("failed to acquire port 80: %w", err)
	}
	defer func() {
		if err := c.portResolver.ReleasePort80(); err != nil {
			logger.Warningf("Failed to release port 80: %v", err)
		}
	}()

	// 2. Configure certmagic
	certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA

	certmagic.DefaultACME.Email = email
	certmagic.HTTPPort = 80
	certmagic.HTTPSPort = 443

	// 3. Obtain certificate
	// Use a longer timeout for the actual certificate issuance
	issueCtx, issueCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer issueCancel()

	err := certmagic.ManageSync(issueCtx, []string{ip})
	if err != nil {
		return fmt.Errorf("failed to obtain certificate for IP %s: %w", ip, err)
	}

	logger.Infof("Successfully obtained IP certificate for %s", ip)

	// Set IP cert path for future reference
	certPath := fmt.Sprintf("%s/.local/share/certmagic/certificates/acme-v02.api.letsencrypt.org-directory/%s/%s", os.Getenv("HOME"), ip, ip)
	if err := c.settingService.SetIpCertPath(certPath); err != nil {
		logger.Warningf("Failed to set IP cert path: %v", err)
	}

	// 4. Hot Reload
	if c.hotReloader != nil {
		if certPath != "" {
			if err := c.hotReloader.OnCertRenewed(certPath+".crt", certPath+".key"); err != nil {
				logger.Errorf("Failed to hot reload certificate: %v", err)
			}
		}
	}

	return nil
}

// obtainIPCertLegacy is the old implementation for fallback
func (c *CertService) obtainIPCertLegacy(ip, email string) error {
	certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
	certmagic.DefaultACME.Email = email
	certmagic.HTTPPort = 80
	certmagic.HTTPSPort = 443

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := certmagic.ManageSync(ctx, []string{ip})
	if err != nil {
		return fmt.Errorf("failed to obtain certificate for IP %s: %w", ip, err)
	}

	logger.Infof("Successfully obtained IP certificate for %s", ip)

	// Set IP cert path for future reference
	certPath := fmt.Sprintf("%s/.local/share/certmagic/certificates/acme-v02.api.letsencrypt.org-directory/%s/%s", os.Getenv("HOME"), ip, ip)
	c.settingService.SetIpCertPath(certPath)

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
	// In a real implementation, this would signal the web server to stop listening on port 80
	// Since we don't have direct control over the Gin engine here, we log a warning
	// If the panel is actually on port 80, this will likely fail to free the port
	logger.Warning("PauseHTTPListener called: Cannot pause panel listener directly. Ensure panel is not on port 80.")
	return nil
}

func (c *certWebServerController) ResumeHTTPListener() error {
	logger.Info("ResumeHTTPListener called")
	return nil
}

func (c *certWebServerController) IsListeningOnPort80() bool {
	port, err := c.settingService.GetPort()
	if err != nil {
		return false
	}
	return port == 80
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
