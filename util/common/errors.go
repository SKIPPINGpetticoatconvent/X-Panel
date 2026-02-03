package common

import (
	"errors"
	"fmt"
	"strings"
)

// =================================================================
// 错误码常量
// =================================================================

const (
	ErrCodeNotFound     = "NOT_FOUND"
	ErrCodeInvalidInput = "INVALID_INPUT"
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeInternal     = "INTERNAL"
	ErrCodeConflict     = "CONFLICT"
	ErrCodeExternal     = "EXTERNAL"
)

// =================================================================
// ServiceError 服务层错误包装
// =================================================================

type ServiceError struct {
	Op      string         // 操作名称，如 "InboundService.GetInbound"
	Code    string         // 错误码，如 "NOT_FOUND"
	Err     error          // 原始错误
	Context map[string]any // 上下文信息
}

func (e *ServiceError) Error() string {
	var sb strings.Builder
	if e.Op != "" {
		sb.WriteString("[")
		sb.WriteString(e.Op)
		sb.WriteString("] ")
	}
	if e.Code != "" {
		sb.WriteString("(")
		sb.WriteString(e.Code)
		sb.WriteString(") ")
	}
	if e.Err != nil {
		sb.WriteString(e.Err.Error())
	}
	return sb.String()
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewServiceError 创建服务层错误
func NewServiceError(op string, err error) *ServiceError {
	return &ServiceError{
		Op:  op,
		Err: err,
	}
}

// WithCode 添加错误码
func (e *ServiceError) WithCode(code string) *ServiceError {
	e.Code = code
	return e
}

// WithContext 添加上下文信息
func (e *ServiceError) WithContext(key string, val any) *ServiceError {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	e.Context[key] = val
	return e
}

// Wrap 快速包装错误
func Wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return NewServiceError(op, err)
}

// Wrapf 带格式化消息包装错误
func Wrapf(op string, err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf(format, args...)
	return NewServiceError(op, fmt.Errorf("%s: %w", msg, err))
}

// =================================================================
// 通用错误定义
// =================================================================

var (
	// ErrNotFound 资源未找到
	ErrNotFound = errors.New("资源未找到")

	// ErrInvalidInput 无效输入
	ErrInvalidInput = errors.New("无效输入")

	// ErrUnauthorized 未授权
	ErrUnauthorized = errors.New("未授权访问")

	// ErrInternal 内部错误
	ErrInternal = errors.New("内部服务器错误")
)

// =================================================================
// 数据库相关错误
// =================================================================

var (
	// ErrDBConnection 数据库连接错误
	ErrDBConnection = errors.New("数据库连接失败")

	// ErrDBQuery 数据库查询错误
	ErrDBQuery = errors.New("数据库查询失败")

	// ErrDBTransaction 数据库事务错误
	ErrDBTransaction = errors.New("数据库事务失败")
)

// =================================================================
// Inbound 相关错误
// =================================================================

var (
	// ErrInboundNotFound 入站规则未找到
	ErrInboundNotFound = errors.New("入站规则未找到")

	// ErrPortExists 端口已存在
	ErrPortExists = errors.New("端口已被占用")

	// ErrEmailExists 邮箱已存在
	ErrEmailExists = errors.New("客户端邮箱已存在")

	// ErrClientNotFound 客户端未找到
	ErrClientNotFound = errors.New("客户端未找到")

	// ErrInvalidProtocol 无效协议
	ErrInvalidProtocol = errors.New("不支持的协议类型")
)

// =================================================================
// Telegram Bot 相关错误
// =================================================================

var (
	// ErrTelegramNotRunning Telegram Bot 未运行
	ErrTelegramNotRunning = errors.New("telegram bot 未运行")

	// ErrTelegramInvalidToken 无效的 Telegram Token
	ErrTelegramInvalidToken = errors.New("无效的 Telegram Bot Token")

	// ErrTelegramInvalidChatID 无效的 Chat ID
	ErrTelegramInvalidChatID = errors.New("无效的 Telegram Chat ID")

	// ErrTelegramNoAdmins 未配置管理员
	ErrTelegramNoAdmins = errors.New("未配置 Telegram 管理员")
)

// =================================================================
// Xray 相关错误
// =================================================================

var (
	// ErrXrayNotRunning Xray 未运行
	ErrXrayNotRunning = errors.New("xray 服务未运行")

	// ErrXrayConfigInvalid Xray 配置无效
	ErrXrayConfigInvalid = errors.New("xray 配置无效")

	// ErrXrayAPIFailed Xray API 调用失败
	ErrXrayAPIFailed = errors.New("xray API 调用失败")
)

// =================================================================
// 用户认证相关错误
// =================================================================

var (
	// ErrUserNotFound 用户未找到
	ErrUserNotFound = errors.New("用户未找到")

	// ErrInvalidCredentials 凭证无效
	ErrInvalidCredentials = errors.New("用户名或密码错误")

	// ErrTwoFactorRequired 需要两步验证
	ErrTwoFactorRequired = errors.New("需要两步验证码")

	// ErrTwoFactorInvalid 两步验证码无效
	ErrTwoFactorInvalid = errors.New("两步验证码无效")
)

// =================================================================
// 辅助函数
// =================================================================

// WrapError 包装错误，添加上下文信息
func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return NewErrorf("%s: %v", context, err)
}

// IsNotFoundError 检查是否为未找到错误
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrInboundNotFound) ||
		errors.Is(err, ErrClientNotFound) ||
		errors.Is(err, ErrUserNotFound)
}
