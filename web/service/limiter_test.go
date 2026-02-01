package service

import (
	"sync"
	"testing"
	"time"
)

// newTestLimiter 创建独立的 LoginLimiter 实例用于测试，避免使用全局单例
func newTestLimiter(maxAttempts int, blockDuration time.Duration) *LoginLimiter {
	return &LoginLimiter{
		attempts:      make(map[string]int),
		blocked:       make(map[string]time.Time),
		maxAttempts:   maxAttempts,
		blockDuration: blockDuration,
	}
}

func TestLoginLimiter_RecordFailure_BlockAfterThreshold(t *testing.T) {
	limiter := newTestLimiter(5, 15*time.Minute)
	ip := "192.168.1.1"

	// 前 4 次失败不应被封锁
	for i := 0; i < 4; i++ {
		limiter.RecordFailure(ip)
		if limiter.IsBlocked(ip) {
			t.Errorf("IP should not be blocked after %d failures", i+1)
		}
	}

	// 第 5 次失败应触发封锁
	limiter.RecordFailure(ip)
	if !limiter.IsBlocked(ip) {
		t.Error("IP should be blocked after 5 failures")
	}
}

func TestLoginLimiter_IsBlocked_NotBlocked(t *testing.T) {
	limiter := newTestLimiter(5, 15*time.Minute)

	if limiter.IsBlocked("10.0.0.1") {
		t.Error("Unknown IP should not be blocked")
	}
}

func TestLoginLimiter_Reset(t *testing.T) {
	limiter := newTestLimiter(5, 15*time.Minute)
	ip := "192.168.1.2"

	// 触发封锁
	for i := 0; i < 5; i++ {
		limiter.RecordFailure(ip)
	}
	if !limiter.IsBlocked(ip) {
		t.Fatal("IP should be blocked")
	}

	// 重置后应解除封锁
	limiter.Reset(ip)
	if limiter.IsBlocked(ip) {
		t.Error("IP should not be blocked after reset")
	}
}

func TestLoginLimiter_Reset_ClearsAttempts(t *testing.T) {
	limiter := newTestLimiter(5, 15*time.Minute)
	ip := "192.168.1.3"

	// 累积 4 次失败
	for i := 0; i < 4; i++ {
		limiter.RecordFailure(ip)
	}

	// 重置后重新累积应需要 5 次
	limiter.Reset(ip)
	for i := 0; i < 4; i++ {
		limiter.RecordFailure(ip)
		if limiter.IsBlocked(ip) {
			t.Errorf("IP should not be blocked after reset + %d failures", i+1)
		}
	}
}

func TestLoginLimiter_BlockExpiry(t *testing.T) {
	// 使用极短的封锁时间测试过期
	limiter := newTestLimiter(1, 1*time.Millisecond)
	ip := "192.168.1.4"

	limiter.RecordFailure(ip)
	if !limiter.IsBlocked(ip) {
		t.Fatal("IP should be blocked immediately after failure")
	}

	// 等待封锁过期
	time.Sleep(5 * time.Millisecond)
	if limiter.IsBlocked(ip) {
		t.Error("IP block should have expired")
	}
}

func TestLoginLimiter_ConcurrentSafety(t *testing.T) {
	limiter := newTestLimiter(100, 15*time.Minute)

	var wg sync.WaitGroup
	// 并发记录失败
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ip := "10.0.0.1"
			limiter.RecordFailure(ip)
			limiter.IsBlocked(ip)
		}(i)
	}

	// 并发重置
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Reset("10.0.0.1")
		}()
	}

	wg.Wait()
	// 只要不 panic 或 data race 即测试通过
}

func TestLoginLimiter_MultipleIPs(t *testing.T) {
	limiter := newTestLimiter(3, 15*time.Minute)

	// 封锁 IP1
	for i := 0; i < 3; i++ {
		limiter.RecordFailure("ip1")
	}

	// IP2 不应受影响
	if limiter.IsBlocked("ip2") {
		t.Error("ip2 should not be blocked")
	}

	// IP1 应被封锁
	if !limiter.IsBlocked("ip1") {
		t.Error("ip1 should be blocked")
	}

	// 重置 IP1 不影响其他
	limiter.Reset("ip1")
	if limiter.IsBlocked("ip1") {
		t.Error("ip1 should not be blocked after reset")
	}
}
