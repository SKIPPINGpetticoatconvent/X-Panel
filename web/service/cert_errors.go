package service

import (
	"errors"
	"fmt"
	"strings"
)

// Certificate Error Codes - 标准化错误码常量
const (
	ErrCodePort80Occupied    = "CERT_E001" // 80 端口被占用
	ErrCodePort80External    = "CERT_E002" // 80 端口被外部进程占用
	ErrCodeCATimeout         = "CERT_E003" // CA 服务器超时
	ErrCodeCARefused         = "CERT_E004" // CA 服务器拒绝
	ErrCodeDNSResolution     = "CERT_E005" // DNS 解析失败
	ErrCodeCertExpired       = "CERT_E006" // 证书已过期
	ErrCodeRenewalFailed     = "CERT_E007" // 续期失败
	ErrCodeXrayReloadFailed  = "CERT_E008" // Xray 重载失败
	ErrCodeFallbackActivated = "CERT_E009" // 回退机制已激活
	ErrCodePermissionDenied  = "CERT_E010" // 权限不足
)

// CertError 标准化证书错误结构
type CertError struct {
	Code      string // 错误码
	Message   string // 用户友好消息（中文）
	MessageEn string // 用户友好消息（英文）
	Technical string // 技术详情
	Cause     error  // 底层错误
}

// Error 实现 error 接口
func (e *CertError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现 error unwrapping
func (e *CertError) Unwrap() error {
	return e.Cause
}

// GetUserMessage 根据语言获取用户消息
func (e *CertError) GetUserMessage(lang string) string {
	if strings.ToLower(lang) == "en" {
		return e.MessageEn
	}
	return e.Message
}

// NewCertError 创建新的证书错误
func NewCertError(code string, message string, messageEn string, technical string, cause error) *CertError {
	return &CertError{
		Code:      code,
		Message:   message,
		MessageEn: messageEn,
		Technical: technical,
		Cause:     cause,
	}
}

// Predefined certificate errors
var (
	// 端口相关错误
	ErrPort80Occupied = &CertError{
		Code:      ErrCodePort80Occupied,
		Message:   "80 端口被占用",
		MessageEn: "Port 80 is occupied",
		Technical: "HTTP-01 challenge requires port 80 to be available",
	}

	ErrPort80External = &CertError{
		Code:      ErrCodePort80External,
		Message:   "80 端口被外部进程占用",
		MessageEn: "Port 80 is occupied by external process",
		Technical: "Port 80 is being used by another application, preventing ACME challenge",
	}

	// CA 服务器相关错误
	ErrCATimeout = &CertError{
		Code:      ErrCodeCATimeout,
		Message:   "CA 服务器超时",
		MessageEn: "CA server timeout",
		Technical: "Let's Encrypt or other ACME server did not respond within timeout period",
	}

	ErrCARefused = &CertError{
		Code:      ErrCodeCARefused,
		Message:   "CA 服务器拒绝",
		MessageEn: "CA server refused",
		Technical: "ACME server rejected the certificate request",
	}

	// 网络相关错误
	ErrDNSResolution = &CertError{
		Code:      ErrCodeDNSResolution,
		Message:   "DNS 解析失败",
		MessageEn: "DNS resolution failed",
		Technical: "Unable to resolve domain/IP for certificate validation",
	}

	// 证书状态错误
	ErrCertExpired = &CertError{
		Code:      ErrCodeCertExpired,
		Message:   "证书已过期",
		MessageEn: "Certificate has expired",
		Technical: "Existing certificate is no longer valid",
	}

	ErrRenewalFailed = &CertError{
		Code:      ErrCodeRenewalFailed,
		Message:   "续期失败",
		MessageEn: "Renewal failed",
		Technical: "Certificate renewal process failed",
	}

	// 系统错误
	ErrXrayReloadFailed = &CertError{
		Code:      ErrCodeXrayReloadFailed,
		Message:   "Xray 重载失败",
		MessageEn: "Xray reload failed",
		Technical: "Failed to reload Xray configuration after certificate update",
	}

	ErrFallbackActivated = &CertError{
		Code:      ErrCodeFallbackActivated,
		Message:   "回退机制已激活",
		MessageEn: "Fallback mechanism activated",
		Technical: "System has switched to self-signed certificate",
	}

	ErrPermissionDenied = &CertError{
		Code:      ErrCodePermissionDenied,
		Message:   "权限不足",
		MessageEn: "Permission denied",
		Technical: "Insufficient permissions to perform certificate operations",
	}
)

// GetCertErrorByCode 根据错误码获取预定义错误
func GetCertErrorByCode(code string) *CertError {
	switch code {
	case ErrCodePort80Occupied:
		return ErrPort80Occupied
	case ErrCodePort80External:
		return ErrPort80External
	case ErrCodeCATimeout:
		return ErrCATimeout
	case ErrCodeCARefused:
		return ErrCARefused
	case ErrCodeDNSResolution:
		return ErrDNSResolution
	case ErrCodeCertExpired:
		return ErrCertExpired
	case ErrCodeRenewalFailed:
		return ErrRenewalFailed
	case ErrCodeXrayReloadFailed:
		return ErrXrayReloadFailed
	case ErrCodeFallbackActivated:
		return ErrFallbackActivated
	case ErrCodePermissionDenied:
		return ErrPermissionDenied
	default:
		return NewCertError(code, "未知错误", "Unknown error", "Unrecognized error code", nil)
	}
}

// WrapError 用证书错误包装底层错误
func WrapError(code string, cause error) error {
	baseErr := GetCertErrorByCode(code)
	return NewCertError(baseErr.Code, baseErr.Message, baseErr.MessageEn, baseErr.Technical, cause)
}

// IsCertError 检查是否为证书错误
func IsCertError(err error) bool {
	var certErr *CertError
	return errors.As(err, &certErr)
}

// GetCertErrorCode 获取证书错误码
func GetCertErrorCode(err error) string {
	var certErr *CertError
	if errors.As(err, &certErr) {
		return certErr.Code
	}
	return ""
}
