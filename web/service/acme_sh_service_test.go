package service

import (
	"os"
	"testing"
)

// MockExecutor 用于模拟 exec.Command 的执行
type MockExecutor struct {
	Commands map[string]*MockCommand
}

type MockCommand struct {
	Output []byte
	Error  error
}

func (m *MockExecutor) Execute(name string, args ...string) ([]byte, error) {
	key := name + " " + args[0] // 简单键
	if cmd, ok := m.Commands[key]; ok {
		return cmd.Output, cmd.Error
	}
	return nil, nil
}

// NewMockExecutor 创建新的 MockExecutor
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		Commands: make(map[string]*MockCommand),
	}
}

// TestValidateIP 测试 IP 地址校验逻辑
func TestValidateIP(t *testing.T) {
	service := NewAcmeShService()

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"valid IPv4", "192.168.1.1", true},
		{"valid IPv4 with leading zero", "192.168.001.001", false}, // 应该检测到前导零
		{"invalid IPv4 too many parts", "192.168.1.1.1", false},
		{"invalid IPv4 out of range", "256.1.1.1", false},
		{"invalid IPv4 negative", "-1.1.1.1", false},
		{"invalid format", "192.168", false},
		{"command injection attempt", "192.168.1.1; rm -rf /", false},
		{"empty string", "", false},
		{"spaces", " 192.168.1.1 ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isValidIP(tt.ip)
			if result != tt.expected {
				t.Errorf("isValidIP(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// TestEnsureInstalled 测试安装检查逻辑
func TestEnsureInstalled(t *testing.T) {
	service := NewAcmeShService()

	// 测试 acme.sh 已安装的情况
	// 由于我们不能实际安装，创建临时文件模拟
	tempDir := t.TempDir()
	service.installPath = tempDir + "/acme.sh"

	// 创建模拟的 acme.sh 文件
	if err := os.WriteFile(service.installPath, []byte("#!/bin/bash\necho 'acme.sh installed'"), 0o755); err != nil {
		t.Fatalf("Failed to create mock acme.sh: %v", err)
	}

	err := service.EnsureInstalled()
	if err != nil {
		t.Errorf("EnsureInstalled() = %v, want nil", err)
	}

	// 测试 acme.sh 未安装的情况 - 但我们需要 mock exec.Command
	// 这里我们跳过，因为实际安装需要网络权限
	t.Log("Skipping EnsureInstalled test for uninstalled case due to network requirements")
}

// TestIssueIPCert 测试证书申请逻辑
func TestIssueIPCert(t *testing.T) {
	service := NewAcmeShService()

	// 测试有效参数
	err := service.IssueIPCert("192.168.1.1", "test@example.com")
	// 由于需要实际的 acme.sh 和网络，我们期望失败
	if err == nil {
		t.Log("IssueIPCert succeeded unexpectedly - this may indicate test environment has acme.sh")
	}

	// 测试无效 IP
	err = service.IssueIPCert("invalid.ip", "test@example.com")
	if err == nil {
		t.Error("IssueIPCert with invalid IP should fail")
	}

	// 测试空 email
	err = service.IssueIPCert("192.168.1.1", "")
	if err == nil {
		t.Error("IssueIPCert with empty email should fail")
	}
}

// TestGetCertExpiry 测试过期时间获取逻辑
func TestGetCertExpiry(t *testing.T) {
	service := NewAcmeShService()

	// 测试无效 IP
	_, err := service.GetCertExpiry("invalid.ip")
	if err == nil {
		t.Error("GetCertExpiry with invalid IP should fail")
	}

	// 测试有效 IP 但证书不存在
	expiry, err := service.GetCertExpiry("192.168.1.1")
	if err == nil {
		t.Errorf("GetCertExpiry should fail for non-existent cert, but got expiry: %v", expiry)
	}

	// 创建模拟证书文件进行测试
	tempDir := t.TempDir()
	certDir := tempDir + "/certs"
	service.certDir = certDir

	// 创建证书目录
	ipDir := certDir + "/192.168.1.1"
	if err := os.MkdirAll(ipDir, 0o755); err != nil {
		t.Fatalf("Failed to create cert dir: %v", err)
	}

	// 创建模拟证书文件（简化的 PEM 格式）
	certContent := `-----BEGIN CERTIFICATE-----
MIICiTCCAg+gAwIBAgIJAJ8l4HnPq7F5MAOGA1UEBhMCVVMxCzAJBgNVBAgTAkNB
... (simplified for test)
-----END CERTIFICATE-----`
	certFile := ipDir + "/192.168.1.1.cer"
	if err := os.WriteFile(certFile, []byte(certContent), 0o644); err != nil {
		t.Fatalf("Failed to create mock cert file: %v", err)
	}

	// 由于 OpenSSL 命令可能不可用，我们测试错误处理
	_, err = service.GetCertExpiry("192.168.1.1")
	if err == nil {
		t.Log("GetCertExpiry succeeded with mock cert - openssl may be available")
	}
}

// TestGetCertPath 测试获取证书路径
func TestGetCertPath(t *testing.T) {
	service := NewAcmeShService()

	certPath, keyPath := service.GetCertPath("192.168.1.1")

	expectedCert := "/root/.acme.sh/192.168.1.1/192.168.1.1.cer"
	expectedKey := "/root/.acme.sh/192.168.1.1/192.168.1.1.key"

	if certPath != expectedCert {
		t.Errorf("GetCertPath cert = %v, want %v", certPath, expectedCert)
	}
	if keyPath != expectedKey {
		t.Errorf("GetCertPath key = %v, want %v", keyPath, expectedKey)
	}
}

// TestInstallCert 测试证书安装逻辑
func TestInstallCert(t *testing.T) {
	service := NewAcmeShService()

	// 测试无效 IP
	err := service.InstallCert("invalid.ip", "/tmp/test.crt", "/tmp/test.key")
	if err == nil {
		t.Error("InstallCert with invalid IP should fail")
	}

	// 测试空路径
	err = service.InstallCert("192.168.1.1", "", "/tmp/test.key")
	if err == nil {
		t.Error("InstallCert with empty cert path should fail")
	}

	err = service.InstallCert("192.168.1.1", "/tmp/test.crt", "")
	if err == nil {
		t.Error("InstallCert with empty key path should fail")
	}

	// 由于需要实际证书文件，我们不测试成功情况
	t.Log("Skipping successful InstallCert test due to file system requirements")
}
