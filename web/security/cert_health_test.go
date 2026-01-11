package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestCert 创建测试证书和密钥文件
func createTestCert(t *testing.T, certPath, keyPath string, validDays int, isExpired bool) {
	// 生成RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// 创建证书模板
	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, validDays)
	if isExpired {
		notAfter = notBefore.AddDate(0, 0, -1) // 过期证书
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	// 创建自签名证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// 编码为PEM格式
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	// 写入文件
	err = ioutil.WriteFile(certPath, certPEM, 0644)
	require.NoError(t, err)
	err = ioutil.WriteFile(keyPath, keyPEM, 0600)
	require.NoError(t, err)
}

// cleanupTestFiles 清理测试文件
func cleanupTestFiles(certPath, keyPath string) {
	os.Remove(certPath)
	os.Remove(keyPath)
}

// TestCertHealthChecker_Check_Valid 测试有效证书检查
func TestCertHealthChecker_Check_Valid(t *testing.T) {
	certPath := "/tmp/test_cert.pem"
	keyPath := "/tmp/test_key.pem"
	defer cleanupTestFiles(certPath, keyPath)

	// 创建有效证书
	createTestCert(t, certPath, keyPath, 365, false)

	checker := NewCertHealthChecker()
	expiry, err := checker.Check(certPath, keyPath)
	require.NoError(t, err)

	// 验证到期时间在未来
	assert.True(t, expiry.After(time.Now()), "证书应该未过期")
	assert.True(t, expiry.After(time.Now().AddDate(0, 0, 360)), "证书应该有效期超过360天")
}

// TestCertHealthChecker_Check_Expired 测试过期证书检测
func TestCertHealthChecker_Check_Expired(t *testing.T) {
	certPath := "/tmp/test_expired_cert.pem"
	keyPath := "/tmp/test_expired_key.pem"
	defer cleanupTestFiles(certPath, keyPath)

	// 创建过期证书
	createTestCert(t, certPath, keyPath, -1, true)

	checker := NewCertHealthChecker()
	expiry, err := checker.Check(certPath, keyPath)
	require.NoError(t, err)

	// 验证证书已过期
	assert.True(t, expiry.Before(time.Now()), "证书应该已过期")
}

// TestCertHealthChecker_ValidateIP 测试IP SAN验证
func TestCertHealthChecker_ValidateIP(t *testing.T) {
	certPath := "/tmp/test_ip_cert.pem"
	defer os.Remove(certPath)

	// 创建包含IP地址的证书
	createTestCert(t, certPath, "/tmp/dummy_key.pem", 365, false)

	checker := NewCertHealthChecker()

	// 测试有效的IP
	err := checker.ValidateIP(certPath, "127.0.0.1")
	assert.NoError(t, err, "应该验证通过包含的IP地址")

	// 测试无效的IP
	err = checker.ValidateIP(certPath, "192.168.1.100")
	assert.Error(t, err, "应该拒绝不包含的IP地址")

	// 测试无效的IP格式
	err = checker.ValidateIP(certPath, "invalid-ip")
	assert.Error(t, err, "应该拒绝无效的IP格式")

	// 清理
	os.Remove("/tmp/dummy_key.pem")
}

// TestCertHealthChecker_ChainValidation 测试证书链验证
func TestCertHealthChecker_ChainValidation(t *testing.T) {
	certPath := "/tmp/test_chain_cert.pem"
	caPath := "/tmp/test_ca_cert.pem"
	defer func() {
		os.Remove(certPath)
		os.Remove(caPath)
		os.Remove("/tmp/dummy_key.pem")
	}()

	// 创建CA证书
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	require.NoError(t, err)

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	err = ioutil.WriteFile(caPath, caPEM, 0644)
	require.NoError(t, err)

	// 创建由CA签名的证书
	certPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	certTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 365),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &caTemplate, &certPrivateKey.PublicKey, caPrivateKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	err = ioutil.WriteFile(certPath, certPEM, 0644)
	require.NoError(t, err)

	checker := NewCertHealthChecker()
	err = checker.ValidateChain(certPath, caPath)
	assert.NoError(t, err, "证书链验证应该成功")
}

// TestCertMonitor 测试证书监控器
func TestCertMonitor(t *testing.T) {
	certPath := "/tmp/test_monitor_cert.pem"
	keyPath := "/tmp/test_monitor_key.pem"
	defer cleanupTestFiles(certPath, keyPath)

	// 创建有效证书
	createTestCert(t, certPath, keyPath, 365, false)

	monitor := NewCertMonitor()
	err := monitor.MonitorCert(certPath, keyPath)
	assert.NoError(t, err, "证书监控应该成功")
}

// TestIsCertValid 测试证书有效性检查函数
func TestIsCertValid(t *testing.T) {
	certPath := "/tmp/test_valid_cert.pem"
	keyPath := "/tmp/test_valid_key.pem"
	defer cleanupTestFiles(certPath, keyPath)

	// 创建有效证书
	createTestCert(t, certPath, keyPath, 365, false)

	valid := IsCertValid(certPath, keyPath)
	assert.True(t, valid, "有效证书应该返回true")
}

// TestCertHealthChecker_Check_InvalidFiles 测试无效文件处理
func TestCertHealthChecker_Check_InvalidFiles(t *testing.T) {
	checker := NewCertHealthChecker()

	// 测试不存在的文件
	_, err := checker.Check("/nonexistent/cert.pem", "/nonexistent/key.pem")
	assert.Error(t, err, "应该处理不存在的文件")

	// 测试无效的PEM格式
	invalidCertPath := "/tmp/invalid_cert.pem"
	invalidKeyPath := "/tmp/invalid_key.pem"
	defer func() {
		os.Remove(invalidCertPath)
		os.Remove(invalidKeyPath)
	}()

	err = ioutil.WriteFile(invalidCertPath, []byte("invalid pem"), 0644)
	require.NoError(t, err)
	err = ioutil.WriteFile(invalidKeyPath, []byte("invalid pem"), 0600)
	require.NoError(t, err)

	_, err = checker.Check(invalidCertPath, invalidKeyPath)
	assert.Error(t, err, "应该处理无效的PEM格式")
}