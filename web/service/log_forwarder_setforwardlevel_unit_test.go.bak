package service

import (
	"sync"
	"testing"

	"github.com/op/go-logging"
)

// TestLogForwarder_SetForwardLevel_UnitTest 单元测试：仅测试 SetForwardLevel 方法的核心逻辑
func TestLogForwarder_SetForwardLevel_UnitTest(t *testing.T) {
	// 创建一个简化的 LogForwarder 实例用于测试
	lf := &LogForwarder{
		isEnabled:    true,
		forwardLevel: logging.INFO, // 初始级别
		mu:           sync.RWMutex{},
	}

	tests := []struct {
		name          string
		newLevel      logging.Level
		expectedLevel logging.Level
	}{
		{
			name:          "Set to ERROR level",
			newLevel:      logging.ERROR,
			expectedLevel: logging.ERROR,
		},
		{
			name:          "Set to WARNING level",
			newLevel:      logging.WARNING,
			expectedLevel: logging.WARNING,
		},
		{
			name:          "Set to INFO level",
			newLevel:      logging.INFO,
			expectedLevel: logging.INFO,
		},
		{
			name:          "Set to DEBUG level",
			newLevel:      logging.DEBUG,
			expectedLevel: logging.DEBUG,
		},
		{
			name:          "Set to same level",
			newLevel:      logging.INFO,
			expectedLevel: logging.INFO,
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
				t.Errorf("SetForwardLevel() = %v, want %v", actualLevel, tt.expectedLevel)
			}
		})
	}
}

// TestLogForwarder_SetForwardLevel_ThreadSafety_UnitTest 测试 SetForwardLevel 的线程安全性
func TestLogForwarder_SetForwardLevel_ThreadSafety_UnitTest(t *testing.T) {
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

	// 验证最终状态
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
		t.Errorf("Final level %v is not a valid logging level", finalLevel)
	}
}

// TestLogForwarder_LevelOrdering_UnitTest 测试日志级别的数值顺序
func TestLogForwarder_LevelOrdering_UnitTest(t *testing.T) {
	lf := &LogForwarder{
		isEnabled: true,
		mu:        sync.RWMutex{},
	}

	// 验证级别顺序：ERROR(2) > WARNING(3) > INFO(4) > DEBUG(5)
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
				t.Errorf("SetForwardLevel() = %v, want %v", actualLevel, tt.level)
			}

			// 验证级别数值顺序
			if int(actualLevel) != tt.expectedOrder {
				t.Errorf("Level %v has order %d, expected %d", actualLevel, int(actualLevel), tt.expectedOrder)
			}
		})
	}
}

// TestLogForwarder_IntegrationLevelCheck 测试 SetForwardLevel 与 shouldSkipLog 的集成
func TestLogForwarder_IntegrationLevelCheck(t *testing.T) {
	lf := &LogForwarder{
		isEnabled:    true,
		forwardLevel: logging.INFO,
		mu:           sync.RWMutex{},
	}

	tests := []struct {
		name         string
		setLevel     logging.Level
		testLevel    logging.Level
		message      string
		shouldSkip   bool
	}{
		{
			name:       "DEBUG < INFO (should skip)",
			setLevel:   logging.INFO,
			testLevel:  logging.DEBUG,
			message:    "Debug message",
			shouldSkip: true,
		},
		{
			name:       "INFO = INFO (should not skip)",
			setLevel:   logging.INFO,
			testLevel:  logging.INFO,
			message:    "Info message",
			shouldSkip: false,
		},
		{
			name:       "ERROR > INFO (should not skip)",
			setLevel:   logging.INFO,
			testLevel:  logging.ERROR,
			message:    "Error message",
			shouldSkip: false,
		},
		{
			name:       "INFO < WARNING (should skip)",
			setLevel:   logging.WARNING,
			testLevel:  logging.INFO,
			message:    "Info message",
			shouldSkip: true,
		},
		{
			name:       "ERROR > WARNING (should not skip)",
			setLevel:   logging.WARNING,
			testLevel:  logging.ERROR,
			message:    "Error message",
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置转发级别
			lf.SetForwardLevel(tt.setLevel)

			// 测试 shouldSkipLog 方法
			shouldSkip := lf.shouldSkipLog(tt.message, "", tt.testLevel)
			if shouldSkip != tt.shouldSkip {
				t.Errorf("shouldSkipLog() with forward level %v and test level %v = %v, want %v",
					tt.setLevel, tt.testLevel, shouldSkip, tt.shouldSkip)
			}
		})
	}
}