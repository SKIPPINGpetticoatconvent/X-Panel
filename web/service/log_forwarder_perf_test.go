package service

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"
)

// MockSettingService 模拟设置服务，用于测试
type MockSettingService struct {
	logLevel string
}

func (m *MockSettingService) GetTgLogLevel() (string, error) {
	return m.logLevel, nil
}

func (m *MockSettingService) GetTgLogForwardEnabled() (bool, error) {
	return true, nil
}

// MockTelegramService 模拟 Telegram 服务，用于测试
type MockTelegramService struct {
	sendCount int
	mu        sync.Mutex
}

func (m *MockTelegramService) SendMessage(message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCount++
	return nil
}

func (m *MockTelegramService) IsRunning() bool {
	return true
}

func (m *MockTelegramService) GetSendCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sendCount
}

// BenchmarkLogForwarder_OnLog 高并发日志写入基准测试
func BenchmarkLogForwarder_OnLog(b *testing.B) {
	mockSetting := &MockSettingService{logLevel: "warn"}
	mockTelegram := &MockTelegramService{}
	lf := NewLogForwarder(mockSetting, mockTelegram)

	// 启动转发器
	err := lf.Start()
	if err != nil {
		b.Fatal(err)
	}
	defer lf.Stop()

	// 重置基准测试计数器
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 模拟不同级别的日志
		levels := []logging.Level{logging.DEBUG, logging.INFO, logging.WARNING, logging.ERROR}
		level := levels[i%len(levels)]
		message := fmt.Sprintf("Test log message %d", i)
		formatted := fmt.Sprintf("%s - %s", level, message)

		lf.OnLog(level, message, formatted)
	}
}

// TestLogForwarder_PerformanceTest 性能测试：验证非阻塞写入和高并发
func TestLogForwarder_PerformanceTest(t *testing.T) {
	mockSetting := &MockSettingService{logLevel: "info"}
	mockTelegram := &MockTelegramService{}
	lf := NewLogForwarder(mockSetting, mockTelegram)

	err := lf.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Stop()

	const numGoroutines = 100
	const logsPerGoroutine = 1000
	var wg sync.WaitGroup

	start := time.Now()
	startMem := getMemStats()

	// 启动多个 goroutine 并发写入日志
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < logsPerGoroutine; i++ {
				level := logging.INFO
				message := fmt.Sprintf("Goroutine %d: log %d", goroutineID, i)
				formatted := fmt.Sprintf("INFO - %s", message)

				// 非阻塞写入，不应该阻塞
				lf.OnLog(level, message, formatted)
			}
		}(g)
	}

	wg.Wait()
	elapsed := time.Since(start)
	endMem := getMemStats()

	// 验证性能指标
	t.Logf("Total logs sent: %d", numGoroutines*logsPerGoroutine)
	t.Logf("Time taken: %v", elapsed)
	t.Logf("Logs per second: %.2f", float64(numGoroutines*logsPerGoroutine)/elapsed.Seconds())
	t.Logf("Memory used: %d KB", (endMem-endMem)/1024)

	// 等待一小段时间让 worker 处理完
	time.Sleep(100 * time.Millisecond)

	sentCount := mockTelegram.GetSendCount()
	t.Logf("Messages actually sent to Telegram: %d", sentCount)

	// 验证没有阻塞（时间应该合理）
	if elapsed > 5*time.Second {
		t.Errorf("Test took too long: %v, possible blocking", elapsed)
	}

	// 验证内存使用合理（粗略检查）
	if endMem-startMem > 50*1024*1024 { // 50MB
		t.Errorf("Memory usage too high: %d bytes", endMem-startMem)
	}
}

// TestLogForwarder_LevelFilteringTest 测试日志级别过滤
func TestLogForwarder_LevelFilteringTest(t *testing.T) {
	tests := []struct {
		name         string
		logLevel     string
		logs         []struct{ level logging.Level; shouldSend bool }
		expectedSent int
	}{
		{
			name:     "Error level - only errors",
			logLevel: "error",
			logs: []struct {
				level      logging.Level
				shouldSend bool
			}{
				{logging.ERROR, true},
				{logging.WARNING, false},
				{logging.INFO, false},
			},
			expectedSent: 1,
		},
		{
			name:     "Warn level - warnings and errors",
			logLevel: "warn",
			logs: []struct {
				level      logging.Level
				shouldSend bool
			}{
				{logging.ERROR, true},
				{logging.WARNING, true},
				{logging.INFO, false},
			},
			expectedSent: 2,
		},
		{
			name:     "Info level - all levels",
			logLevel: "info",
			logs: []struct {
				level      logging.Level
				shouldSend bool
			}{
				{logging.ERROR, true},
				{logging.WARNING, true},
				{logging.INFO, true}, // 假设是重要 INFO
			},
			expectedSent: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSetting := &MockSettingService{logLevel: tt.logLevel}
			mockTelegram := &MockTelegramService{}
			lf := NewLogForwarder(mockSetting, mockTelegram)

			err := lf.Start()
			if err != nil {
				t.Fatal(err)
			}
			defer lf.Stop()

			// 发送各种级别的日志
			for i, log := range tt.logs {
				message := fmt.Sprintf("Test message %d", i)
				formatted := fmt.Sprintf("%s - %s", log.level, message)
				lf.OnLog(log.level, message, formatted)
			}

			// 等待处理
			time.Sleep(50 * time.Millisecond)

			sentCount := mockTelegram.GetSendCount()
			if sentCount != tt.expectedSent {
				t.Errorf("Expected %d messages sent, got %d", tt.expectedSent, sentCount)
			}
		})
	}
}

// TestLogForwarder_BatchProcessingTest 测试批处理功能
func TestLogForwarder_BatchProcessingTest(t *testing.T) {
	mockSetting := &MockSettingService{logLevel: "warn"}
	mockTelegram := &MockTelegramService{}
	lf := NewLogForwarder(mockSetting, mockTelegram)

	err := lf.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Stop()

	// 发送多个 WARNING 消息，应该被批处理
	for i := 0; i < 7; i++ { // 超过 batchSize (5)
		message := fmt.Sprintf("Warning message %d", i)
		formatted := fmt.Sprintf("WARNING - %s", message)
		lf.OnLog(logging.WARNING, message, formatted)
	}

	// 等待批处理完成
	time.Sleep(50 * time.Millisecond)

	sentCount := mockTelegram.GetSendCount()
	t.Logf("Messages sent after batch processing: %d", sentCount)

	// 应该发送 2 次：第一次 5 条，第二次 2 条（因为定时器也可能触发）
	if sentCount < 1 {
		t.Error("Expected at least 1 message to be sent")
	}
	if sentCount > 3 {
		t.Errorf("Too many messages sent: %d, batching not working properly", sentCount)
	}
}

// getMemStats 获取当前内存统计
func getMemStats() uint64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return m.Alloc
}