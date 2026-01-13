package service

import (
	"sync"
	"time"
)

type LoginLimiter struct {
	attempts map[string]int
	blocked  map[string]time.Time
	mu       sync.RWMutex

	maxAttempts   int
	blockDuration time.Duration
}

var (
	loginLimiter *LoginLimiter
	once         sync.Once
)

func GetLoginLimiter() *LoginLimiter {
	once.Do(func() {
		loginLimiter = &LoginLimiter{
			attempts:      make(map[string]int),
			blocked:       make(map[string]time.Time),
			maxAttempts:   5,
			blockDuration: 15 * time.Minute,
		}
		// Start cleanup routine
		go loginLimiter.cleanupLoop()
	})
	return loginLimiter
}

func (l *LoginLimiter) IsBlocked(ip string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if unblockTime, exists := l.blocked[ip]; exists {
		if time.Now().Before(unblockTime) {
			return true
		}
	}
	return false
}

func (l *LoginLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.attempts[ip]++
	if l.attempts[ip] >= l.maxAttempts {
		l.blocked[ip] = time.Now().Add(l.blockDuration)
		delete(l.attempts, ip) // Reset attempts once blocked
	}
}

func (l *LoginLimiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, ip)
	delete(l.blocked, ip)
}

func (l *LoginLimiter) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for ip, unblockTime := range l.blocked {
			if now.After(unblockTime) {
				delete(l.blocked, ip)
			}
		}
		// Optional: Clear old attempts that haven't reached block threshold
		// For simplicity, we just clear blocked IPs here.
		l.mu.Unlock()
	}
}
