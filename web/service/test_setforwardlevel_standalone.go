package service

import (
	"sync"
	"testing"

	"github.com/op/go-logging"
)

// TestLogForwarder_SetForwardLevel_Standalone 独立的 SetForwardLevel 测试
// 这个测试不依赖外部接口，只测试核心功能
func TestLogForwarder_SetForwardLevel_Standalone(t *testing.T) {
	// 创建最小化的 LogForwarder 实例，只初始化需要的字段
	lf := &LogForwarder{
		isEnabled:    true,
		forwardLevel: logging.INFO, // 默认级别
		mu:           sync.RWMutex{},
	}

	tests := []struct {
		name          string
		newLevel      logging.Level
		expectedLevel logging.Level
	}{
		{
			name:          "设置 ERROR 级别",
			newLevel:      logging.ERROR,
			expectedLevel: logging.ERROR,
		},
		{
			name:          "设置 WARNING 级别",
			newLevel:      logging.WARNING,
			expectedLevel: logging.WARNING,
		},
		{
			name:          "设置 INFO 级别",
			newLevel:      logging.INFO,
			expectedLevel: logging.INFO,
		},
		{
			name:          "设置 DEBUG 级别",
			newLevel:      logging.DEBUG,
			expectedLevel: logging.DEBUG,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 调用 SetForwardLevel 方法
			lf.SetForwardLevel(tt.newLevel)

			// 验证级别是否正确设置
			lf.mu.RLock()
			actualLevel := lf.forwardLevel
			lf.mu.RUnlock()

			if actualLevel != tt.expectedLevel {
				t.Errorf("SetForwardLevel() 期望级别 %v，实际得到 %v", tt.expectedLevel, actualLevel)
			}
		})
	}
}

// TestLogForwarder_SetForwardLevel_ThreadSafety 线程安全测试
func TestLogForwarder_SetForwardLevel_ThreadSafety(t *testing.T) {
	lf := &LogForwarder{
		isEnabled:    true,
		forwardLevel: logging.INFO,
		mu:           sync.RWMutex{},
	}

	const numGoroutines = 100
	var wg sync.WaitGroup
	levels := []logging.Level{logging.ERROR, logging.WARNING, logging.INFO, logging.DEBUG}

	// 启动多个 goroutine 同时调用 SetForwardLevel
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			level := levels[id%len(levels)]
			lf.SetForwardLevel(level)
		}(i)
	}

	wg.Wait()

	// 验证最终状态（应该为最后一个设置的级别）
	lf.mu.RLock()
	finalLevel := lf.forwardLevel
	lf.mu.RUnlock()

	// 验证最终级别是有效的
	validLevels := map[logging.Level]bool{
		logging.ERROR:   true,
		logging.WARNING: true,
		logging.INFO:    true,
		logging.DEBUG:   true,
	}

	if !validLevels[finalLevel] {
		t.Errorf("最终级别 %v 不是有效的日志级别", finalLevel)
	}
}

// TestLogForwarder_LevelOrdering_Validation 级别顺序验证测试
func TestLogForwarder_LevelOrdering_Validation(t *testing.T) {
	lf := &LogForwarder{
		isEnabled: true,
		mu:        sync.RWMutex{},
	}

	// 验证日志级别的数值顺序
	// go-logging 库中：CRITICAL(0), ERROR(2), WARNING(3), NOTICE(4), INFO(4), DEBUG(5)
	tests := []struct {
		name          string
		level         logging.Level
		expectedOrder int
	}{
		{"ERROR", logging.ERROR, 2},
		{"WARNING", logging.WARNING, 3},
		{"INFO", logging.INFO, 4},
		{"DEBUG", logging.DEBUG, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lf.SetForwardLevel(tt.level)

			lf.mu.RLock()
			actualLevel := lf.forwardLevel
			lf.mu.RUnlock()

			if actualLevel != tt.level {
				t.Errorf("SetForwardLevel() 期望级别 %v，实际得到 %v", tt.level, actualLevel)
			}

			// 验证级别数值顺序
			if int(actualLevel) != tt.expectedOrder {
				t.Errorf("级别 %v 的数值顺序应为 %d，实际为 %d", actualLevel, tt.expectedOrder, int(actualLevel))
			}
		})
	}
}

// TestLogForwarder_IntegrationWithShouldSkip 集成测试：SetForwardLevel 与 shouldSkipLog 的配合
func TestLogForwarder_IntegrationWithShouldSkip(t *testing.T) {
	lf := &LogForwarder{
		isEnabled:    true,
		forwardLevel: logging.INFO,
		mu:           sync.RWMutex{},
	}

	tests := []struct {
		name         string
		setLevel     logging.Level
		testLogLevel logging.Level
		message      string
		shouldSkip   bool
		description  string
	}{
		{
			name:         "ERROR 级别日志，当转发级别为 INFO 时",
			setLevel:     logging.INFO,
			testLogLevel: logging.ERROR,
			message:      "错误发生",
			shouldSkip:   false,
			description:  "ERROR > INFO，应该转发",
		},
		{
			name:         "INFO 级别日志，当转发级别为 INFO 时",
			setLevel:     logging.INFO,
			testLogLevel: logging.INFO,
			message:      "信息日志",
			shouldSkip:   false,
			description:  "INFO = INFO，应该转发",
		},
		{
			name:         "DEBUG 级别日志，当转发级别为 INFO 时",
			setLevel:     logging.INFO,
			testLogLevel: logging.DEBUG,
			message:      "调试信息",
			shouldSkip:   true,
			description:  "DEBUG < INFO，应该跳过",
		},
		{
			name:         "INFO 级别日志，当转发级别为 WARNING 时",
			setLevel:     logging.WARNING,
			testLogLevel: logging.INFO,
			message:      "信息日志",
			shouldSkip:   true,
			description:  "INFO < WARNING，应该跳过",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置转发级别
			lf.SetForwardLevel(tt.setLevel)

			// 测试 shouldSkipLog 方法
			shouldSkip := lf.shouldSkipLog(tt.message, "", tt.testLogLevel)
			if shouldSkip != tt.shouldSkip {
				t.Errorf("%s: shouldSkipLog() = %v，期望 %v (%s)",
					tt.name, shouldSkip, tt.shouldSkip, tt.description)
			}
		})
	}
}
