package service

import (
	"sync"
	"testing"

	"github.com/op/go-logging"
)

// TestLogForwarder_SetForwardLevel_Simple 简单测试 SetForwardLevel 方法
func TestLogForwarder_SetForwardLevel_Simple(t *testing.T) {
	// 创建一个最小化的 LogForwarder 实例
	lf := &LogForwarder{
		isEnabled:    true,
		forwardLevel: logging.INFO,
		mu:           sync.RWMutex{},
	}

	// 测试用例
	tests := []struct {
		name          string
		newLevel      logging.Level
		expectedLevel logging.Level
	}{
		{"ERROR", logging.ERROR, logging.ERROR},
		{"WARNING", logging.WARNING, logging.WARNING},
		{"INFO", logging.INFO, logging.INFO},
		{"DEBUG", logging.DEBUG, logging.DEBUG},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置新的级别
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

// TestLogForwarder_LevelComparison 测试级别比较逻辑
func TestLogForwarder_LevelComparison(t *testing.T) {
	lf := &LogForwarder{
		isEnabled: true,
		mu:        sync.RWMutex{},
	}

	// 测试级别比较：数值越小，级别越高
	tests := []struct {
		name       string
		setLevel   logging.Level
		testLog    logging.Level
		shouldSkip bool
	}{
		{"ERROR > INFO", logging.INFO, logging.ERROR, false},
		{"ERROR > WARNING", logging.WARNING, logging.ERROR, false},
		{"INFO = INFO", logging.INFO, logging.INFO, false},
		{"DEBUG < INFO", logging.INFO, logging.DEBUG, true},
		{"INFO < WARNING", logging.WARNING, logging.INFO, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置转发级别
			lf.SetForwardLevel(tt.setLevel)

			// 测试 shouldSkipLog
			skip := lf.shouldSkipLog("test message", "", tt.testLog)
			if skip != tt.shouldSkip {
				t.Errorf("shouldSkipLog() = %v, want %v", skip, tt.shouldSkip)
			}
		})
	}
}

// TestLogForwarder_ConcurrentSetLevel 测试并发设置级别的安全性
func TestLogForwarder_ConcurrentSetLevel(t *testing.T) {
	lf := &LogForwarder{
		isEnabled:    true,
		forwardLevel: logging.INFO,
		mu:           sync.RWMutex{},
	}

	const numRoutines = 50
	var wg sync.WaitGroup

	// 启动多个 goroutine 并发设置级别
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			levels := []logging.Level{logging.ERROR, logging.WARNING, logging.INFO, logging.DEBUG}
			level := levels[id%len(levels)]
			lf.SetForwardLevel(level)
		}(i)
	}

	wg.Wait()

	// 如果没有 panic 或数据竞争，测试通过
	lf.mu.RLock()
	finalLevel := lf.forwardLevel
	lf.mu.RUnlock()

	// 验证最终级别是有效值
	valid := finalLevel >= logging.ERROR && finalLevel <= logging.DEBUG
	if !valid {
		t.Errorf("Invalid final level: %v", finalLevel)
	}
}
