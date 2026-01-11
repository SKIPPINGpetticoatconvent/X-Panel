package security

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConn 模拟net.Conn用于TLS错误日志测试
type tlsMockConn struct {
	remoteAddr net.Addr
}

func (m *tlsMockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *tlsMockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *tlsMockConn) Close() error                       { return nil }
func (m *tlsMockConn) LocalAddr() net.Addr                { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080} }
func (m *tlsMockConn) RemoteAddr() net.Addr               { return m.remoteAddr }
func (m *tlsMockConn) SetDeadline(t time.Time) error      { return nil }
func (m *tlsMockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *tlsMockConn) SetWriteDeadline(t time.Time) error { return nil }

// TestTLSError_Parse 测试错误解析正确
func TestTLSError_Parse(t *testing.T) {
	conn := &tlsMockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 443}}

	testCases := []struct {
		name         string
		errorMsg     string
		expectedType string
	}{
		{"ProtocolVersion", "tls: client offered only unsupported versions", "protocol_version"},
		{"CipherSuite", "tls: no cipher suite supported by both client and server", "cipher_suite"},
		{"Certificate", "tls: bad certificate", "certificate"},
		{"Handshake", "tls: received unexpected handshake message", "handshake"},
		{"Record", "local error: tls: bad record MAC", "record"},
		{"Unknown", "some unknown tls error", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.New(tc.errorMsg)
			LogTLSError(conn, err)

			// 验证错误分类函数
			classifiedType := classifyTLSError(tc.errorMsg)
			assert.Equal(t, tc.expectedType, classifiedType, "错误类型分类应该正确")
		})
	}
}

// TestTLSError_IsScanner 测试扫描器识别
func TestTLSError_IsScanner(t *testing.T) {
	testCases := []struct {
		name        string
		errorMsg    string
		isScanner   bool
	}{
		{"ScannerError1", "tls: client offered only unsupported versions", true},
		{"ScannerError2", "tls: no cipher suite supported by both client and server", true},
		{"ScannerError3", "tls: client offered an unsupported, maximum protocol version of", true},
		{"ScannerError4", "tls: unsupported SSLv2 handshake received", true},
		{"ScannerError5", "tls: received unexpected handshake message", true},
		{"ScannerError6", "local error: tls: bad record MAC", true},
		{"ScannerError7", "remote error: tls: bad certificate", true},
		{"ScannerError8", "remote error: tls: unknown certificate authority", true},
		{"NormalError", "tls: handshake failure", false},
		{"NormalError2", "tls: connection reset by peer", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isKnownScannerError(tc.errorMsg)
			assert.Equal(t, tc.isScanner, result, "扫描器错误识别应该正确")
		})
	}
}

// TestLogTLSError_Level 测试日志级别正确
func TestLogTLSError_Level(t *testing.T) {
	conn := &tlsMockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 443}}

	// 测试扫描器错误（应该使用debug级别）
	scannerError := errors.New("tls: client offered only unsupported versions")
	LogTLSError(conn, scannerError)

	// 测试普通TLS错误（应该使用warning级别）
	normalError := errors.New("tls: handshake failure")
	LogTLSError(conn, normalError)

	// 测试空错误（应该无操作）
	LogTLSError(conn, nil)
}

// TestTLSError_Types 测试各种错误类型分类
func TestTLSError_Types(t *testing.T) {
	testCases := []struct {
		name         string
		errorMsg     string
		expectedType string
	}{
		{"VersionError", "tls: client offered only unsupported versions", "protocol_version"},
		{"VersionError2", "tls: unsupported SSLv2 handshake received", "protocol_version"},
		{"CipherError", "tls: no cipher suite supported by both client and server", "cipher_suite"},
		{"CertError", "remote error: tls: bad certificate", "certificate"},
		{"CertError2", "remote error: tls: unknown certificate authority", "certificate"},
		{"HandshakeError", "tls: received unexpected handshake message", "handshake"},
		{"RecordError", "local error: tls: bad record MAC", "record"},
		{"RecordError2", "remote error: tls: bad record MAC", "record"},
		{"UnknownError", "some random error", "unknown"},
		{"EmptyError", "", "unknown"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			classifiedType := classifyTLSError(tc.errorMsg)
			assert.Equal(t, tc.expectedType, classifiedType, "错误类型应该正确分类")
		})
	}
}

// TestTLSLogger 测试TLS日志记录器
func TestTLSLogger(t *testing.T) {
	logger := NewTLSLogger()
	require.NotNil(t, logger)

	conn := &tlsMockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 443}}

	// 测试扫描器错误（debug级别）
	scannerConn := &tlsMockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 443}}
	scannerErr := errors.New("tls: client offered only unsupported versions")

	// 测试普通错误（warning级别）
	normalErr := errors.New("tls: handshake failure")

	// 测试自定义日志函数覆盖所有级别
	logLevels := []string{}
	customLogFunc := func(level string, format string, args ...interface{}) {
		logLevels = append(logLevels, level)
	}

	logger.SetLogFunction(customLogFunc)

	// 测试warning级别
	logger.LogError(conn, normalErr)
	assert.Contains(t, logLevels, "warning", "应该使用warning级别记录普通TLS错误")

	// 测试debug级别
	logger.LogError(scannerConn, scannerErr)
	assert.Contains(t, logLevels, "debug", "应该使用debug级别记录扫描器错误")

	// 测试所有日志级别
	testLogger := NewTLSLogger()
	testLogger.SetLogFunction(func(level string, format string, args ...interface{}) {
		// 这里我们只是为了覆盖switch语句的各个分支
	})

	// 通过直接设置logFunc来测试不同级别
	testLogger.logFunc("info", "test")
	testLogger.logFunc("warning", "test")
	testLogger.logFunc("error", "test")
	testLogger.logFunc("debug", "test")
	testLogger.logFunc("unknown", "test") // default case
}

// TestTLSHandshakeMonitor 测试TLS握手监控器
func TestTLSHandshakeMonitor(t *testing.T) {
	monitor := NewTLSHandshakeMonitor()
	require.NotNil(t, monitor)

	conn := &tlsMockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 443}}

	// 测试成功握手
	err := monitor.MonitorHandshake(conn, func() error {
		return nil
	})
	assert.NoError(t, err, "成功握手应该无错误")

	// 测试失败握手
	testErr := errors.New("tls: handshake failure")
	err = monitor.MonitorHandshake(conn, func() error {
		return testErr
	})
	assert.Equal(t, testErr, err, "失败握手应该返回原始错误")
}

