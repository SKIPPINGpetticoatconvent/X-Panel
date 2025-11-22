package common

import (
	"context"
	"errors"
	"fmt"

	"x-ui/logger"
)

// AppError 统一的应用错误类型
type AppError struct {
	Code    string
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError 创建应用错误
func NewAppError(code, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// 预定义错误码
const (
	ErrCodeValidation    = "VALIDATION_ERROR"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeConflict      = "CONFLICT"
	ErrCodeInternal      = "INTERNAL_ERROR"
	ErrCodeTimeout       = "TIMEOUT"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
)

// 便捷的错误创建函数
func NewValidationError(message string, cause error) *AppError {
	return NewAppError(ErrCodeValidation, message, cause)
}

func NewNotFoundError(message string, cause error) *AppError {
	return NewAppError(ErrCodeNotFound, message, cause)
}

func NewConflictError(message string, cause error) *AppError {
	return NewAppError(ErrCodeConflict, message, cause)
}

func NewInternalError(message string, cause error) *AppError {
	return NewAppError(ErrCodeInternal, message, cause)
}

// 向后兼容的函数
func NewErrorf(format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	return errors.New(msg)
}

func NewError(a ...any) error {
	msg := fmt.Sprintln(a...)
	return errors.New(msg)
}

func Recover(msg string) any {
	panicErr := recover()
	if panicErr != nil {
		if msg != "" {
			logger.Error(msg, "panic:", panicErr)
		}
	}
	return panicErr
}

// Context相关的工具函数
func WithTimeout(ctx context.Context, timeoutSeconds int) (context.Context, context.CancelFunc) {
	if timeoutSeconds <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
}

func IsContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
