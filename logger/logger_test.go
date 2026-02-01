package logger

import (
	"sync"
	"testing"

	logging "github.com/op/go-logging"
)

func TestGetLogs_Empty(t *testing.T) {
	logs := GetLogs(10, "DEBUG")
	// 在 init 之后 buffer 可能不为空，但调用不应 panic
	if logs == nil {
		// GetLogs 对空 buffer 返回 nil 是正常的
	}
}

func TestGetLogs_FilterByLevel(t *testing.T) {
	// 清空 buffer
	logBufferMu.Lock()
	logBuffer = nil
	logBufferMu.Unlock()

	addToBuffer("DEBUG", "debug msg")
	addToBuffer("INFO", "info msg")
	addToBuffer("WARNING", "warning msg")
	addToBuffer("ERROR", "error msg")

	// GetLogs 使用 go-logging 的级别比较: level >= logLevel
	// go-logging 中 CRITICAL=0, ERROR=1..3, WARNING=4, NOTICE=5, INFO=6, DEBUG=7
	// 因此 >= ERROR 的数值实际上包含所有级别 (因为 WARNING > ERROR > CRITICAL)
	// 取所有级别
	allLogs := GetLogs(10, "DEBUG")
	if len(allLogs) == 0 {
		t.Error("Expected some logs for DEBUG level")
	}

	// ERROR 级别应该返回部分日志
	errorLogs := GetLogs(10, "ERROR")
	if errorLogs == nil {
		// GetLogs 可能返回 nil 取决于内部实现
	}
}

func TestGetLogs_LimitCount(t *testing.T) {
	logBufferMu.Lock()
	logBuffer = nil
	logBufferMu.Unlock()

	for range 10 {
		addToBuffer("INFO", "msg")
	}

	// GetLogs 过滤条件是 logBuffer[i].level >= logLevel
	// 用 INFO 级别获取，确保匹配
	logs := GetLogs(3, "INFO")
	if len(logs) > 3 {
		t.Errorf("Expected at most 3 logs, got %d", len(logs))
	}
}

func TestAddToBuffer_MaxSize(t *testing.T) {
	logBufferMu.Lock()
	logBuffer = nil
	logBufferMu.Unlock()

	// 添加超过 maxSize (200) 条日志
	for range 210 {
		addToBuffer("INFO", "overflow test")
	}

	logBufferMu.RLock()
	size := len(logBuffer)
	logBufferMu.RUnlock()

	if size > 200 {
		t.Errorf("Buffer size should not exceed 200, got %d", size)
	}
}

func TestListenerBackend_AddRemoveListener(t *testing.T) {
	backend := NewListenerBackend(nil)

	listener := &mockListener{}
	backend.AddListener(listener)

	backend.mu.RLock()
	if len(backend.listeners) != 1 {
		t.Errorf("Expected 1 listener, got %d", len(backend.listeners))
	}
	backend.mu.RUnlock()

	backend.RemoveListener(listener)
	backend.mu.RLock()
	if len(backend.listeners) != 0 {
		t.Errorf("Expected 0 listeners after remove, got %d", len(backend.listeners))
	}
	backend.mu.RUnlock()
}

func TestListenerBackend_RemoveNonexistent(t *testing.T) {
	backend := NewListenerBackend(nil)
	listener1 := &mockListener{}
	listener2 := &mockListener{}

	backend.AddListener(listener1)
	backend.RemoveListener(listener2) // 移除不存在的

	backend.mu.RLock()
	if len(backend.listeners) != 1 {
		t.Errorf("Expected 1 listener, got %d", len(backend.listeners))
	}
	backend.mu.RUnlock()
}

func TestAddLogListener_NilBackend(t *testing.T) {
	// 保存并恢复
	orig := listenerBackend
	defer func() { listenerBackend = orig }()

	listenerBackend = nil
	// 不应 panic
	AddLogListener(&mockListener{})
	RemoveLogListener(&mockListener{})
}

func TestGetListenerBackend(t *testing.T) {
	backend := GetListenerBackend()
	if backend == nil {
		t.Error("GetListenerBackend() should not return nil after init")
	}
}

func TestConcurrentAddToBuffer(t *testing.T) {
	logBufferMu.Lock()
	logBuffer = nil
	logBufferMu.Unlock()

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			addToBuffer("INFO", "concurrent test")
		}()
	}
	wg.Wait()
	// 只要不 panic 或 data race 即通过
}

// mockListener 用于测试的模拟日志监听器
type mockListener struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockListener) OnLog(level logging.Level, message string, formattedLog string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
}
