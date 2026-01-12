package security

import (
	"net"
	"sync"

	"time"

	"x-ui/logger"

	"golang.org/x/time/rate"
)

// RateLimiter 接口定义连接速率限制器
type RateLimiter interface {
	Allow(ip string) bool
	AddWhitelist(ip string)
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiterImpl 令牌桶速率限制器的实现
type rateLimiterImpl struct {
	limiters    map[string]*clientLimiter
	whitelist   map[string]bool
	mutex       sync.RWMutex
	maxConnsSec rate.Limit
	burst       int
}

// Allow 检查IP是否允许建立连接
func (rl *rateLimiterImpl) Allow(ip string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// 白名单IP直接允许
	if rl.whitelist[ip] {
		return true
	}

	// 获取或创建该IP的限速器
	client, exists := rl.limiters[ip]
	if !exists {
		client = &clientLimiter{
			limiter:  rate.NewLimiter(rl.maxConnsSec, rl.burst),
			lastSeen: time.Now(),
		}
		rl.limiters[ip] = client
	} else {
		client.lastSeen = time.Now()
	}

	// 检查是否允许
	return client.limiter.Allow()
}

// AddWhitelist 添加IP到白名单
func (rl *rateLimiterImpl) AddWhitelist(ip string) {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	rl.whitelist[ip] = true
}

// RateLimitListener 包装的监听器，添加速率限制
type RateLimitListener struct {
	net.Listener
	limiter   *rateLimiterImpl
	closeChan chan struct{}
}

// NewRateLimitListener 创建带速率限制的监听器
func NewRateLimitListener(listener net.Listener, config *RateLimitConfig) *RateLimitListener {
	if config == nil {
		config = &RateLimitConfig{
			MaxConnsPerSec: 5,
			Burst:          10,
		}
	}

	limiter := &rateLimiterImpl{
		limiters:    make(map[string]*clientLimiter),
		whitelist:   make(map[string]bool),
		maxConnsSec: rate.Limit(config.MaxConnsPerSec),
		burst:       config.Burst,
	}

	rl := &RateLimitListener{
		Listener:  listener,
		limiter:   limiter,
		closeChan: make(chan struct{}),
	}

	// 启动清理goroutine
	go rl.cleanupLoop()

	return rl
}

// Accept 实现net.Listener接口，添加速率检查
func (rl *RateLimitListener) Accept() (net.Conn, error) {
	for {
		conn, err := rl.Listener.Accept()
		if err != nil {
			return nil, err
		}

		// 获取客户端IP
		clientIP := getClientIP(conn)
		if clientIP == "" {
			logger.Warning("无法获取客户端IP，拒绝连接")
			conn.Close()
			continue
		}

		// 检查速率限制
		if !rl.limiter.Allow(clientIP) {
			logger.Warningf("连接速率限制：拒绝来自 %s 的连接", clientIP)
			conn.Close()
			continue
		}

		return conn, nil
	}
}

// AddWhitelist 添加IP到白名单
func (rl *RateLimitListener) AddWhitelist(ip string) {
	rl.limiter.AddWhitelist(ip)
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	MaxConnsPerSec int
	Burst          int
}

// getClientIP 从连接中提取客户端IP
func getClientIP(conn net.Conn) string {
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

// cleanupExpiredLimiters 清理过期的限速器
func (rl *rateLimiterImpl) cleanupExpiredLimiters() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// 清理超过1小时未活动的IP
	threshold := time.Now().Add(-1 * time.Hour)
	for ip, client := range rl.limiters {
		if client.lastSeen.Before(threshold) {
			delete(rl.limiters, ip)
		}
	}
}

// cleanupLoop 定期清理过期限速器
func (rl *RateLimitListener) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.limiter.cleanupExpiredLimiters()
		case <-rl.closeChan:
			return
		}
	}
}

// Close 关闭监听器并停止清理任务
func (rl *RateLimitListener) Close() error {
	close(rl.closeChan)
	return rl.Listener.Close()
}
