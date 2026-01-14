package job

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"x-ui/logger"
	"x-ui/web/service"
)

type CertMonitorJob struct {
	settingService  service.SettingService
	panelService    service.PanelService
	telegramService service.TelegramService
}

func NewCertMonitorJob(settingService service.SettingService, telegramService service.TelegramService) *CertMonitorJob {
	return &CertMonitorJob{
		settingService:  settingService,
		telegramService: telegramService,
		panelService:    service.PanelService{},
	}
}

// Run executes the certificate check
func (j *CertMonitorJob) Run() {
	// Check if certificate is configured
	certFile, err := j.settingService.GetCertFile()
	if err != nil || certFile == "" {
		return
	}
	keyFile, err := j.settingService.GetKeyFile()
	if err != nil || keyFile == "" {
		return
	}

	// Read and parse the certificate
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		logger.Warningf("[CertMonitor] Failed to read certificate file: %v", err)
		return
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		logger.Warning("[CertMonitor] Failed to decode PEM block")
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		logger.Warningf("[CertMonitor] Failed to parse certificate: %v", err)
		return
	}

	// Check expiration (warn if < 72 hours / 3 days)
	timeRemaining := time.Until(cert.NotAfter)
	if timeRemaining > 72*time.Hour {
		return
	}

	logger.Infof("[CertMonitor] Certificate for %s is expiring in %v. Initiating failover...", cert.Subject.CommonName, timeRemaining)

	// Avoid loop: if the current cert is arguably already our self-signed IP cert, don't keep regenerating it every check
	// unless it is actually expired.
	// Our self-signed certs usually have Issuer CN == Subject CN (or we can check Issuer)
	if cert.Issuer.CommonName == cert.Subject.CommonName && strings.Contains(cert.Subject.CommonName, ".") {
		// It might be a self-signed IP cert. Check if it's REALLY expired (e.g. < 1 hour) before panic.
		// If it's just < 24h, we might have just generated it (if we generated it with short life? No we use 10 years).
		// If we generated 10 years, it won't be < 24h.
		// So if it is < 24h, it's definitely an old or broken cert.
		logger.Debugf("[CertMonitor] Self-signed cert detected for %s, expiration check continues...", cert.Subject.CommonName)
	}

	// 1. Detect IP
	ip, err := j.detectPublicIP()
	if err != nil {
		logger.Errorf("[CertMonitor] Failed to detect public IP for failover: %v", err)
		j.sendAlert(fmt.Sprintf("⚠️ <b>SSL Failover Failed</b>\n\nCertificate is expiring but could not detect Public IP.\nError: %v", err))
		return
	}

	// 2. Generate new IP Cert
	certPath, keyPath, err := j.generateSelfSignedIPCert(ip)
	if err != nil {
		logger.Errorf("[CertMonitor] Failed to generate IP certificate: %v", err)
		j.sendAlert(fmt.Sprintf("⚠️ <b>SSL Failover Failed</b>\n\nCertificate is expiring but generation failed.\nError: %v", err))
		return
	}

	// 3. Update Settings
	err = j.settingService.SetCertFile(certPath)
	if err != nil {
		logger.Error(err)
	}
	err = j.settingService.SetKeyFile(keyPath)
	if err != nil {
		logger.Error(err)
	}

	// 4. Delete Old Cert Files
	j.cleanupOldCerts(certFile, keyFile)

	// 5. Send Notification
	j.sendAlert(fmt.Sprintf(
		"🚨 <b>SSL Certificate Expired</b>\n\n"+
			"The domain certificate for %s was about to expire.\n\n"+
			"✅ <b>Failover Successful</b>\n"+
			"Switched to Self-Signed IP Certificate.\n"+
			"<b>New Access Address:</b> https://%s:%d\n\n"+
			"🗑️ <b>Cleanup:</b> Old certificate files have been deleted.\n"+
			"Panel is restarting...",
		cert.Subject.CommonName, ip, func() int { p, _ := j.settingService.GetPort(); return p }(),
	))

	// 6. Restart Panel
	_ = j.panelService.RestartPanel(3 * time.Second)
}

func (j *CertMonitorJob) cleanupOldCerts(certPath, keyPath string) {
	// Helper to safely remove file and empty parent dir
	remove := func(path string) {
		if path == "" {
			return
		}

		// 1. Remove the file
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			logger.Warningf("[CertMonitor] Failed to delete old cert file %s: %v", path, err)
			return
		}
		logger.Infof("[CertMonitor] Deleted old cert file: %s", path)

		// 2. Try to remove parent dir if it is a subdir of /root/cert/
		// e.g. /root/cert/example.com/
		dir := filepath.Dir(path)
		absDir, _ := filepath.Abs(dir)

		// Safety check: Ensure we are inside /root/cert and NOT deleting /root/cert itself
		// Assuming standard path /root/cert/<domain>/
		if strings.HasPrefix(absDir, "/root/cert/") && absDir != "/root/cert" {
			// We use Remove, not RemoveAll, to be safe. Only delete if empty (which it should be if we deleted both key and cert)
			// But since we call this twice (once for key, once for cert), the first time dir won't be empty.
			// The second time it might be.
			_ = os.Remove(absDir)
			// We ignore error here because if it's not empty, we shouldn't delete it.
		}
	}

	remove(certPath)
	remove(keyPath)
}

func (j *CertMonitorJob) detectPublicIP() (string, error) {
	urls := []string{
		"https://api.ipify.org",
		"https://ifconfig.me",
		"https://checkip.amazonaws.com",
	}

	client := http.Client{Timeout: 5 * time.Second}

	validIP := func(ip string) bool {
		return net.ParseIP(ip) != nil
	}

	for _, url := range urls {
		resp, err := client.Get(url)
		if err != nil || resp == nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		ip := strings.TrimSpace(string(body))
		if validIP(ip) {
			return ip, nil
		}
	}
	return "", fmt.Errorf("all IP detection services failed")
}

func (j *CertMonitorJob) generateSelfSignedIPCert(ip string) (string, string, error) {
	certDir := fmt.Sprintf("/root/cert/%s", ip)
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		return "", "", err
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   ip,
			Organization: []string{"X-Panel Auto-Gen"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(3650 * 24 * time.Hour), // 10 years

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP(ip)},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	certPath := filepath.Join(certDir, "fullchain.pem")
	certOut, err := os.Create(certPath)
	if err != nil {
		return "", "", err
	}
	_ = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	_ = certOut.Close()

	keyPath := filepath.Join(certDir, "privkey.pem")
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return "", "", err
	}
	_ = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	_ = keyOut.Close()

	return certPath, keyPath, nil
}

func (j *CertMonitorJob) sendAlert(msg string) {
	if j.telegramService != nil {
		_ = j.telegramService.SendMessage(msg)
	}
}
