package security

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRateLimiter_Allow_Normal 测试正常流量允许通过
func TestRateLimiter_Allow_Normal(t *testing.T) {
	limiter := &rateLimiterImpl{
		limiters:    make(map[string]*clientLimiter),
		whitelist:   make(map[string]bool),
		maxConnsSec: 10, // 允许更高的速率用于测试
		burst:       20,
	}

	ip := "192.168.1.100"

	// 应该允许初始连接
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow(ip), "正常流量应该被允许通过")
	}
}

// TestRateLimiter_Allow_Exceed 测试超过阈值被拒绝
func TestRateLimiter_Allow_Exceed(t *testing.T) {
	limiter := &rateLimiterImpl{
		limiters:    make(map[string]*clientLimiter),
		whitelist:   make(map[string]bool),
		maxConnsSec: 1, // 每秒只允许1个连接
		burst:       1,
	}

	ip := "192.168.1.100"

	// 第一个连接应该允许
	assert.True(t, limiter.Allow(ip), "第一个连接应该被允许")

	// 立即再次尝试，应该被拒绝
	assert.False(t, limiter.Allow(ip), "超过速率限制的连接应该被拒绝")

	// 等待一秒后再试
	time.Sleep(time.Second)
	assert.True(t, limiter.Allow(ip), "等待后应该允许新连接")
}

// TestRateLimiter_Whitelist 测试白名单IP始终允许
func TestRateLimiter_Whitelist(t *testing.T) {
	limiter := &rateLimiterImpl{
		limiters:    make(map[string]*clientLimiter),
		whitelist:   make(map[string]bool),
		maxConnsSec: 1,
		burst:       1,
	}

	whitelistedIP := "192.168.1.100"
	normalIP := "192.168.1.101"

	// 添加白名单IP
	limiter.AddWhitelist(whitelistedIP)

	// 白名单IP应该始终允许，即使超过速率限制
	for i := 0; i < 10; i++ {
		assert.True(t, limiter.Allow(whitelistedIP), "白名单IP应该始终被允许")
	}

	// 普通IP仍然受限
	assert.True(t, limiter.Allow(normalIP), "第一个普通IP连接应该被允许")
	assert.False(t, limiter.Allow(normalIP), "普通IP超过限制应该被拒绝")
}

// TestRateLimiter_Concurrent 测试并发场景下的正确性
func TestRateLimiter_Concurrent(t *testing.T) {
	limiter := &rateLimiterImpl{
		limiters:    make(map[string]*clientLimiter),
		whitelist:   make(map[string]bool),
		maxConnsSec: 5,
		burst:       10,
	}

	ip := "192.168.1.100"
	var wg sync.WaitGroup
	results := make(chan bool, 100)

	// 启动多个goroutine并发测试
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := limiter.Allow(ip)
			results <- result
		}()
	}

	wg.Wait()
	close(results)

	allowed := 0
	denied := 0
	for result := range results {
		if result {
			allowed++
		} else {
			denied++
		}
	}

	// 验证结果合理性
	assert.True(t, allowed > 0, "应该有一些连接被允许")
	assert.True(t, denied >= 0, "可能有一些连接被拒绝")
	assert.Equal(t, 50, allowed+denied, "总连接数应该等于并发数")
}

// mockListener 模拟net.Listener用于测试
type mockListener struct {
	conns chan net.Conn
}

func (m *mockListener) Accept() (net.Conn, error) {
	conn := <-m.conns
	return conn, nil
}

func (m *mockListener) Close() error {
	close(m.conns)
	return nil
}

func (m *mockListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

// mockConn 模拟net.Conn用于测试
type mockConn struct {
	remoteAddr net.Addr
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080} }
func (m *mockConn) RemoteAddr() net.Addr               { return m.remoteAddr }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// TestRateLimitListener_Accept 测试监听器包装正确工作
func TestRateLimitListener_Accept(t *testing.T) {
	// 创建模拟监听器
	mockListener := &mockListener{conns: make(chan net.Conn, 10)}

	// 创建带速率限制的监听器
	config := &RateLimitConfig{
		MaxConnsPerSec: 2,
		Burst:          2,
	}
	rateLimitListener := NewRateLimitListener(mockListener, config)

	// 添加白名单IP
	rateLimitListener.AddWhitelist("192.168.1.100")

	// 发送模拟连接
	whitelistedConn := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 12345}}
	normalConn := &mockConn{remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.101"), Port: 12346}}

	go func() {
		mockListener.conns <- whitelistedConn
	}()

	// 测试白名单连接
	conn, err := rateLimitListener.Accept()
	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, "192.168.1.100:12345", conn.RemoteAddr().String())

	// 测试正常连接
	go func() {
		mockListener.conns <- normalConn
	}()

	conn, err = rateLimitListener.Accept()
	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, "192.168.1.101:12346", conn.RemoteAddr().String())
}

// mockNonTCPConn 模拟非TCP连接用于测试
type mockNonTCPConn struct {
	remoteAddr net.Addr
}

func (m *mockNonTCPConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockNonTCPConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockNonTCPConn) Close() error                       { return nil }
func (m *mockNonTCPConn) LocalAddr() net.Addr                { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080} }
func (m *mockNonTCPConn) RemoteAddr() net.Addr               { return m.remoteAddr }
func (m *mockNonTCPConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockNonTCPConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockNonTCPConn) SetWriteDeadline(t time.Time) error { return nil }

// mockUDPAddr 模拟UDP地址
type mockUDPAddr struct {
	ip   string
	port int
}

func (m *mockUDPAddr) Network() string { return "udp" }
func (m *mockUDPAddr) String() string  { return m.ip + ":" + string(rune(m.port)) }

// TestGetClientIP 测试获取客户端IP
func TestGetClientIP(t *testing.T) {
	// 测试TCP连接
	tcpAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 12345}
	conn := &mockConn{remoteAddr: tcpAddr}

	ip := getClientIP(conn)
	assert.Equal(t, "192.168.1.100", ip, "应该正确提取TCP客户端IP")

	// 测试非TCP连接 - 应该返回空字符串
	nonTCPConn := &mockNonTCPConn{remoteAddr: &mockUDPAddr{ip: "192.168.1.101", port: 53}}
	ip = getClientIP(nonTCPConn)
	assert.Equal(t, "", ip, "非TCP连接应该返回空字符串")
}