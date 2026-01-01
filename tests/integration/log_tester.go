package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// LogTester 日志测试器
type LogTester struct {
	logFilePath string
}

// NewLogTester 创建新的日志测试器
func NewLogTester(logFilePath string) *LogTester {
	return &LogTester{
		logFilePath: logFilePath,
	}
}

// TestLogCollection 测试日志收集
func (lt *LogTester) TestLogCollection(t *testing.T) error {
	// 等待一些日志生成
	time.Sleep(10 * time.Second)

	// 检查日志文件是否存在
	if _, err := os.Stat(lt.logFilePath); os.IsNotExist(err) {
		return fmt.Errorf("日志文件不存在: %s", lt.logFilePath)
	}

	// 读取日志文件
	logData, err := os.ReadFile(lt.logFilePath)
	if err != nil {
		return fmt.Errorf("读取日志文件失败: %v", err)
	}

	logContent := string(logData)

	// 检查日志是否包含启动信息
	if !strings.Contains(logContent, "Starting") {
		return fmt.Errorf("日志不包含启动信息")
	}

	// 检查是否有错误日志（不应该有启动错误）
	if strings.Contains(logContent, "Error starting") {
		return fmt.Errorf("日志包含启动错误")
	}

	t.Log("日志收集测试通过")
	return nil
}

// TestLogRotation 测试日志轮转（如果适用）
func (lt *LogTester) TestLogRotation(t *testing.T) error {
	// 检查日志文件大小
	info, err := os.Stat(lt.logFilePath)
	if err != nil {
		return fmt.Errorf("获取日志文件信息失败: %v", err)
	}

	// 如果日志文件太大，可能需要轮转
	maxSize := int64(10 * 1024 * 1024) // 10MB
	if info.Size() > maxSize {
		t.Logf("日志文件较大 (%d bytes)，可能需要轮转", info.Size())
	}

	return nil
}
