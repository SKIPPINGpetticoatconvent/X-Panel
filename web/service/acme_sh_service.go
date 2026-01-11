package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"x-ui/logger"
)

// AcmeShService 封装 acme.sh 证书管理功能
type AcmeShService struct {
	installPath string
	certDir     string
}

// NewAcmeShService 创建新的 AcmeShService 实例
func NewAcmeShService() *AcmeShService {
	return &AcmeShService{
		installPath: "/root/.acme.sh/acme.sh", // 默认安装路径
		certDir:     "/root/.acme.sh",         // 默认证书目录
	}
}

// isValidIP 验证 IP 地址格式，防止命令注入
func (a *AcmeShService) isValidIP(ip string) bool {
	ipRegex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if !ipRegex.MatchString(ip) {
		return false
	}

	// 检查每个段是否在 0-255 范围内
	parts := strings.Split(ip, ".")
	for _, part := range parts {
		if len(part) > 3 || (len(part) > 1 && part[0] == '0') {
			return false
		}
		var num int
		fmt.Sscanf(part, "%d", &num)
		if num < 0 || num > 255 {
			return false
		}
	}
	return true
}

// EnsureInstalled 检查并安装 acme.sh
func (a *AcmeShService) EnsureInstalled() error {
	logger.Infof("Checking acme.sh at path: %s", a.installPath)
	// 检查 acme.sh 是否已安装
	if _, err := os.Stat(a.installPath); err == nil {
		logger.Info("acme.sh is already installed")
		return nil
	}
	// 也检查 /usr/local/bin/acme.sh
	if _, err := os.Stat("/usr/local/bin/acme.sh"); err == nil {
		logger.Info("acme.sh found at /usr/local/bin/acme.sh, using it")
		a.installPath = "/usr/local/bin/acme.sh"
		return nil
	}
	// 检查用户目录 ~/.acme.sh/acme.sh
	if homeDir, err := os.UserHomeDir(); err == nil {
		userPath := filepath.Join(homeDir, ".acme.sh", "acme.sh")
		if _, err := os.Stat(userPath); err == nil {
			logger.Info("acme.sh found at user home directory, using it")
			a.installPath = userPath
			return nil
		}
	}
	logger.Warningf("acme.sh not found at %s, attempting to install", a.installPath)

	logger.Info("Installing acme.sh...")

	// 下载并安装 acme.sh
	installCmd := exec.Command("wget", "-O", "-", "https://get.acme.sh")
	installOutput, err := installCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to download acme.sh install script: %w", err)
	}

	// 执行安装脚本
	installScript := string(installOutput)
	installScriptCmd := exec.Command("bash", "-c", installScript)
	installScriptCmd.Env = append(os.Environ(), "HOME=/root")

	output, err := installScriptCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install acme.sh: %w, output: %s", err, string(output))
	}

	logger.Info("acme.sh installed successfully")

	// 检查安装后的路径是否存在
	if _, err := os.Stat(a.installPath); err != nil {
		logger.Errorf("acme.sh installation completed but executable not found at %s: %v", a.installPath, err)
		return fmt.Errorf("acme.sh installation failed: executable not found at %s", a.installPath)
	}
	return nil
}

// IssueIPCert 执行 IP 证书申请
func (a *AcmeShService) IssueIPCert(ip, email string) error {
	if !a.isValidIP(ip) {
		return fmt.Errorf("invalid IP address format: %s", ip)
	}
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	logger.Infof("Issuing IP certificate for %s with email %s", ip, email)

	// 确保 acme.sh 已安装
	if err := a.EnsureInstalled(); err != nil {
		return fmt.Errorf("failed to ensure acme.sh installation: %w", err)
	}

	// 构建证书申请命令
	args := []string{
		"--issue",
		"--standalone",
		"-d", ip,
		"--server", "letsencrypt",
		"--certificate-profile", "shortlived",
		"--email", email,
	}
	logger.Infof("Executing acme.sh command: %s %v", a.installPath, args)

	cmd := exec.Command(a.installPath, args...)
	cmd.Env = append(os.Environ(), "HOME=/root")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Failed to issue certificate for IP %s: %v, output: %s", ip, err, string(output))
		return fmt.Errorf("failed to issue certificate: %w", err)
	}

	logger.Infof("Successfully issued IP certificate for %s", ip)
	return nil
}

// InstallCert 将证书安装到指定路径
func (a *AcmeShService) InstallCert(ip, certPath, keyPath string) error {
	if !a.isValidIP(ip) {
		return fmt.Errorf("invalid IP address format: %s", ip)
	}
	if certPath == "" || keyPath == "" {
		return fmt.Errorf("certificate and key paths cannot be empty")
	}

	logger.Infof("Installing certificate for IP %s to %s and %s", ip, certPath, keyPath)

	// 构建证书安装命令
	args := []string{
		"--install-cert",
		"-d", ip,
		"--cert-file", certPath,
		"--key-file", keyPath,
	}

	cmd := exec.Command(a.installPath, args...)
	cmd.Env = append(os.Environ(), "HOME=/root")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Failed to install certificate for IP %s: %v, output: %s", ip, err, string(output))
		return fmt.Errorf("failed to install certificate: %w", err)
	}

	// 设置私钥文件权限为 0600
	if err := os.Chmod(keyPath, 0o600); err != nil {
		logger.Warningf("Failed to set key file permissions: %v", err)
	}

	logger.Infof("Successfully installed certificate for IP %s", ip)
	return nil
}

// GetCertExpiry 获取证书过期时间
func (a *AcmeShService) GetCertExpiry(ip string) (time.Time, error) {
	if !a.isValidIP(ip) {
		return time.Time{}, fmt.Errorf("invalid IP address format: %s", ip)
	}

	// 获取证书信息
	args := []string{
		"--info",
		"-d", ip,
	}

	cmd := exec.Command(a.installPath, args...)
	cmd.Env = append(os.Environ(), "HOME=/root")

	output, err := cmd.Output()
	if err != nil {
		logger.Errorf("Failed to get certificate info for IP %s: %v", ip, err)
		return time.Time{}, fmt.Errorf("failed to get certificate info: %w", err)
	}

	// 解析输出以获取过期时间
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Le_NextRenewTime") {
			// 解析日期格式，通常是 Unix 时间戳
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				timestampStr := strings.TrimSpace(parts[1])
				if timestamp, err := time.Parse("Jan 2 15:04:05 2006 MST", timestampStr); err == nil {
					return timestamp, nil
				}
				// 如果不是标准格式，尝试 Unix 时间戳
				if timestamp, err := time.Parse(time.RFC3339, timestampStr); err == nil {
					return timestamp, nil
				}
			}
		}
	}

	// 如果无法从 info 中获取，尝试直接检查证书文件
	certFile := filepath.Join(a.certDir, ip, ip+".cer")
	if _, err := os.Stat(certFile); err == nil {
		// 从证书文件中解析过期时间
		if expiry, err := a.parseCertExpiryFromFile(certFile); err == nil {
			return expiry, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to determine certificate expiry time")
}

// parseCertExpiryFromFile 从证书文件中解析过期时间
func (a *AcmeShService) parseCertExpiryFromFile(certPath string) (time.Time, error) {
	cmd := exec.Command("openssl", "x509", "-in", certPath, "-noout", "-enddate")
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate: %w", err)
	}

	outputStr := string(output)
	if strings.HasPrefix(outputStr, "notAfter=") {
		dateStr := strings.TrimPrefix(outputStr, "notAfter=")
		dateStr = strings.TrimSpace(dateStr)

		// OpenSSL 日期格式: Dec 31 23:59:59 2024 GMT
		expiry, err := time.Parse("Jan 02 15:04:05 2006 MST", dateStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse expiry date: %w", err)
		}
		return expiry, nil
	}

	return time.Time{}, fmt.Errorf("invalid certificate expiry format")
}

// GetCertPath 获取证书路径
func (a *AcmeShService) GetCertPath(ip string) (certPath, keyPath string) {
	basePath := filepath.Join(a.certDir, ip)
	certPath = filepath.Join(basePath, ip+".cer")
	keyPath = filepath.Join(basePath, ip+".key")
	return
}
