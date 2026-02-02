package e2e

import (
	"path/filepath"
	"time"
)

func (s *E2ETestSuite) TestCertApplication() {
	// 1. 注入核心文件 (Mock ACME 资产)
	s.T().Log("Preparing Mock ACME assets...")
	err := s.container.CopyFileToContainer(s.ctx, filepath.Join(s.projectRoot, "tests/e2e/assets/mock_acme.sh"), "/root/mock_acme.sh", 0o755)
	s.Require().NoError(err)

	// 2. 安装 X-Panel (复用之前的 Mock 安装逻辑)
	s.T().Log("Installing X-Panel for Cert Test...")
	s.setupMockInstall()
	exitCode, _, err := s.execCommand([]string{"bash", "-c", "printf '\\nn\\nn\\n' | /root/install.sh v1.0.0"})
	s.Require().Equal(0, exitCode)

	// 3. 配置 Mock ACME
	s.T().Log("Setting up Mock ACME...")
	s.execCommand([]string{"mkdir", "-p", "/root/.acme.sh"})
	s.execCommand([]string{"cp", "/root/mock_acme.sh", "/root/.acme.sh/acme.sh"})

	// 4. 测试 IP 证书申请逻辑
	s.T().Log("Testing IP Cert Application...")
	testIP := "127.0.0.1"
	certDir := "/root/cert/ip"
	s.execCommand([]string{"mkdir", "-p", certDir})

	// 模拟 ACME 签发和安装过程
	s.execCommand([]string{"/root/.acme.sh/acme.sh", "--issue", "-d", testIP, "--standalone", "--server", "letsencrypt", "--force"})
	s.execCommand([]string{
		"/root/.acme.sh/acme.sh", "--installcert", "-d", testIP,
		"--key-file", certDir + "/privkey.pem",
		"--fullchain-file", certDir + "/fullchain.pem",
		"--reloadcmd", "systemctl restart x-ui",
	})

	// 验证证书文件是否存在
	exitCode, _, _ = s.execCommand([]string{"ls", certDir + "/fullchain.pem"})
	s.Equal(0, exitCode, "IP Certificate file should exist")

	// 5. 调用面板 CLI 设置证书并验证数据库
	s.T().Log("Configuring Panel via CLI and verifying DB...")
	s.execCommand([]string{"/usr/local/x-ui/x-ui", "cert", "-webCert", certDir + "/fullchain.pem", "-webCertKey", certDir + "/privkey.pem"})
	s.execCommand([]string{"systemctl", "restart", "x-ui"})
	time.Sleep(2 * time.Second)

	dbPath := "/etc/x-ui/x-ui.db"
	_, dbOutput, _ := s.execCommand([]string{"sqlite3", dbPath, "SELECT value FROM settings WHERE key='webCertFile';"})
	s.Contains(dbOutput, certDir+"/fullchain.pem", "Database settings should contain the new cert path. Output: "+dbOutput)

	s.T().Log("Cert Application E2E Test Passed!")
}

// 辅助方法：封装通用的 Mock 安装环境设置
func (s *E2ETestSuite) setupMockInstall() {
	s.execCommand([]string{"mkdir", "-p", "/root/mock_server/releases/download/v1.0.0"})
	s.container.CopyFileToContainer(s.ctx, filepath.Join(s.projectRoot, "install.sh"), "/root/install.sh", 0o755)
	s.container.CopyFileToContainer(s.ctx, filepath.Join(s.projectRoot, "x-ui-linux-amd64.tar.gz"), "/root/x-ui-linux-amd64.tar.gz", 0o644)
	s.execCommand([]string{"cp", "/root/x-ui-linux-amd64.tar.gz", "/root/mock_server/releases/download/v1.0.0/x-ui-linux-amd64.tar.gz"})

	go func() {
		s.execCommand([]string{"python3", "-m", "http.server", "8080", "--directory", "/root/mock_server"})
	}()
	time.Sleep(2 * time.Second)

	s.execCommand([]string{"sed", "-i", "s|https://github.com/SKIPPINGpetticoatconvent/X-Panel/releases/download|http://127.0.0.1:8080/releases/download|g", "/root/install.sh"})
	s.execCommand([]string{"sed", "-i", "s|last_version=$(curl.*|last_version=\"v1.0.0\"|g", "/root/install.sh"})
}
