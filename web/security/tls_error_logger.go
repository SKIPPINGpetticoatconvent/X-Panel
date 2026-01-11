package security

import (
	"net"
	"strings"
	"time"

	"x-ui/logger"
)

// TLSError TLS错误信息结构体
type TLSError struct {
	ClientIP  string    `json:"client_ip"`
	Time      time.Time `json:"time"`
	ErrorType string    `json:"error_type"`
	RawError  string    `json:"raw_error"`
	IsScanner bool      `json:"is_scanner"`
}

// knownScannerErrors 已知的扫描器错误模式
var knownScannerErrors = []string{
	"tls: client offered only unsupported versions",
	"tls: no cipher suite supported by both client and server",
	"tls: client offered an unsupported, maximum protocol version of",
	"tls: unsupported SSLv2 handshake received",
	"tls: received unexpected handshake message",
	"local error: tls: bad record MAC",
	"remote error: tls: bad certificate",
	"remote error: tls: unknown certificate authority",
}

// LogTLSError 记录TLS错误
func LogTLSError(conn net.Conn, err error) {
	if err == nil {
		return
	}

	clientIP := getClientIP(conn)
	if clientIP == "" {
		clientIP = "unknown"
	}

	rawError := err.Error()
	errorType := classifyTLSError(rawError)
	isScanner := isKnownScannerError(rawError)

	_ = TLSError{
		ClientIP:  clientIP,
		Time:      time.Now(),
		ErrorType: errorType,
		RawError:  rawError,
		IsScanner: isScanner,
	}

	// 根据错误类型选择日志级别
	if isScanner {
		// 扫描器错误使用debug级别，避免日志噪音
		logger.Debugf("TLS扫描器检测 - IP: %s, 错误: %s", clientIP, rawError)
	} else {
		// 其他TLS错误使用warning级别
		logger.Warningf("TLS握手错误 - IP: %s, 类型: %s, 错误: %s", clientIP, errorType, rawError)
	}
}

// classifyTLSError 分类TLS错误类型
func classifyTLSError(errMsg string) string {
	// 检查顺序很重要：从最具体到最通用
	if strings.Contains(errMsg, "certificate") {
		return "certificate"
	}
	if strings.Contains(errMsg, "cipher") {
		return "cipher_suite"
	}
	if strings.Contains(errMsg, "record") {
		return "record"
	}
	if strings.Contains(errMsg, "version") || strings.Contains(errMsg, "SSLv2") || strings.Contains(errMsg, "SSLv3") {
		return "protocol_version"
	}
	if strings.Contains(errMsg, "handshake") {
		return "handshake"
	}
	return "unknown"
}

// isKnownScannerError 检查是否为已知的扫描器错误
func isKnownScannerError(errMsg string) bool {
	for _, pattern := range knownScannerErrors {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// TLSLogger TLS错误记录器
type TLSLogger struct {
	logFunc func(level string, format string, args ...interface{})
}

// NewTLSLogger 创建TLS错误记录器
func NewTLSLogger() *TLSLogger {
	return &TLSLogger{
		logFunc: func(level string, format string, args ...interface{}) {
			switch level {
			case "info":
				logger.Infof(format, args...)
			case "warning":
				logger.Warningf(format, args...)
			case "error":
				logger.Errorf(format, args...)
			case "debug":
				logger.Debugf(format, args...)
			default:
				logger.Infof(format, args...)
			}
		},
	}
}

// LogError 记录TLS错误
func (tl *TLSLogger) LogError(conn net.Conn, err error) {
	if err == nil {
		return
	}

	clientIP := getClientIP(conn)
	if clientIP == "" {
		clientIP = "unknown"
	}

	rawError := err.Error()
	errorType := classifyTLSError(rawError)
	isScanner := isKnownScannerError(rawError)

	// 使用自定义日志函数
	if isScanner {
		tl.logFunc("debug", "TLS扫描器检测 - IP: %s, 错误: %s", clientIP, rawError)
	} else {
		tl.logFunc("warning", "TLS握手错误 - IP: %s, 类型: %s, 错误: %s", clientIP, errorType, rawError)
	}
}

// SetLogFunction 设置自定义日志函数
func (tl *TLSLogger) SetLogFunction(logFunc func(level string, format string, args ...interface{})) {
	tl.logFunc = logFunc
}

// TLSHandshakeMonitor TLS握手监控器
type TLSHandshakeMonitor struct {
	logger *TLSLogger
}

// NewTLSHandshakeMonitor 创建TLS握手监控器
func NewTLSHandshakeMonitor() *TLSHandshakeMonitor {
	return &TLSHandshakeMonitor{
		logger: NewTLSLogger(),
	}
}

// MonitorHandshake 监控TLS握手过程
func (thm *TLSHandshakeMonitor) MonitorHandshake(conn net.Conn, handshakeFunc func() error) error {
	err := handshakeFunc()
	if err != nil {
		thm.logger.LogError(conn, err)
	}
	return err
}