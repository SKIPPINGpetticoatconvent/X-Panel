package common

import (
	"errors"
	"fmt"

	"x-ui/logger"
)

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

// =================================================================
// 统一错误处理辅助函数
// =================================================================

// HandleError 统一错误处理，记录日志并包装错误
func HandleError(op string, err error) error {
	if err == nil {
		return nil
	}
	logger.Warningf("[%s] %v", op, err)
	return NewServiceError(op, err)
}

// HandleErrorWithCode 带错误码的统一错误处理
func HandleErrorWithCode(op string, code string, err error) error {
	if err == nil {
		return nil
	}
	logger.Warningf("[%s] (%s) %v", op, code, err)
	return NewServiceError(op, err).WithCode(code)
}

// LogAndReturn 仅记录日志并返回原始错误
func LogAndReturn(op string, err error) error {
	if err == nil {
		return nil
	}
	logger.Warningf("[%s] %v", op, err)
	return err
}

// MustNotError 断言无错误，有错误则 panic
func MustNotError(err error) {
	if err != nil {
		panic(err)
	}
}

// IgnoreError 忽略错误，仅记录警告日志
func IgnoreError(op string, err error) {
	if err != nil {
		logger.Warningf("[%s] ignored error: %v", op, err)
	}
}

// GetErrorCode 从错误中提取错误码
func GetErrorCode(err error) string {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.Code
	}
	// 根据预定义错误返回对应错误码
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrInboundNotFound),
		errors.Is(err, ErrClientNotFound), errors.Is(err, ErrUserNotFound):
		return ErrCodeNotFound
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidProtocol):
		return ErrCodeInvalidInput
	case errors.Is(err, ErrUnauthorized), errors.Is(err, ErrInvalidCredentials):
		return ErrCodeUnauthorized
	case errors.Is(err, ErrPortExists), errors.Is(err, ErrEmailExists):
		return ErrCodeConflict
	default:
		return ErrCodeInternal
	}
}
