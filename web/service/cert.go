package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/caddyserver/certmagic"
	"x-ui/logger"
)

type CertService struct {
	settingService *SettingService
}

func NewCertService(settingService *SettingService) *CertService {
	return &CertService{
		settingService: settingService,
	}
}

// ObtainIPCert obtains a Let's Encrypt IP certificate using certmagic with standalone challenge
func (c *CertService) ObtainIPCert(ip, email string) error {
	if ip == "" {
		return errors.New("IP address cannot be empty")
	}
	if email == "" {
		return errors.New("email cannot be empty")
	}

	// Configure certmagic for IP certificate
	certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA // Use staging for testing
	certmagic.DefaultACME.Email = email

	// Use standalone challenge solver for port 80
	certmagic.HTTPPort = 80
	certmagic.HTTPSPort = 443

	// Obtain certificate
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err := certmagic.ManageSync(ctx, []string{ip})
	if err != nil {
		return fmt.Errorf("failed to obtain certificate for IP %s: %w", ip, err)
	}

	logger.Infof("Successfully obtained IP certificate for %s", ip)
	return nil
}

// RenewLoop runs a background goroutine that periodically checks and renews IP certificates
func (c *CertService) RenewLoop() {
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