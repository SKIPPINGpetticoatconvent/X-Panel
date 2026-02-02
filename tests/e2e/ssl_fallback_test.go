package e2e

import (
	"time"
)

func (s *E2ETestSuite) TestSSLFallback() {
	// 1. 安装 X-Panel
	s.T().Log("Installing X-Panel for SSL Fallback Test...")
	s.setupMockInstall()
	s.execCommand([]string{"bash", "-c", "printf '\\nn\\nn\\n' | /root/install.sh v1.0.0"})

	// 2. 生成一个即将过期的伪造证书 (2天有效期)
	s.T().Log("Generating short-lived certificate...")
	certDir := "/root/cert/test"
	s.execCommand([]string{"mkdir", "-p", certDir})
	certPath := certDir + "/fullchain.pem"
	keyPath := certDir + "/privkey.pem"

	s.execCommand([]string{"openssl", "req", "-x509", "-newkey", "rsa:2048", "-keyout", keyPath, "-out", certPath, "-days", "2", "-nodes", "-subj", "/CN=test.example.com"})

	// 3. 配置面板使用此证书
	s.T().Log("Configuring panel to use short-lived cert...")
	s.execCommand([]string{"/usr/local/x-ui/x-ui", "cert", "-webCert", certPath, "-webCertKey", keyPath})
	s.execCommand([]string{"systemctl", "restart", "x-ui"})
	time.Sleep(5 * time.Second)

	// 4. Trigger CertMonitorJob immediately via SIGUSR2 signal
	s.T().Log("Triggering CertMonitorJob via SIGUSR2 signal...")
	_, _, err := s.execCommand([]string{"bash", "-c", "kill -SIGUSR2 $(pgrep x-ui)"})
	s.Require().NoError(err, "Failed to send SIGUSR2 to x-ui")

	// 5. Wait a short time for the job to complete and the panel to potentially restart
	s.T().Log("Waiting for job execution and potential restart...")
	time.Sleep(5 * time.Second)

	// 6. Verify fallback
	s.T().Log("Verifying fallback results...")
	dbPath := "/etc/x-ui/x-ui.db"
	_, newCertPath, _ := s.execCommand([]string{"sqlite3", dbPath, "SELECT value FROM settings WHERE key='webCertFile';"})

	s.NotContains(newCertPath, certPath, "Database should NOT point to old expiring cert")
	s.Contains(newCertPath, "/root/cert/", "New cert path should be in the managed cert directory")

	// 验证旧文件是否被清理
	exitCode, _, _ := s.execCommand([]string{"ls", certPath})
	s.NotEqual(0, exitCode, "Old certificate file should have been deleted")

	s.T().Log("SSL Fallback E2E Test Passed!")
}
