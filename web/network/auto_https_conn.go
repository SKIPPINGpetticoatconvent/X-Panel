package network

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"x-ui/config"
	"x-ui/logger"
)

type AutoHttpsConn struct {
	net.Conn

	firstBuf []byte
	bufStart int
	bufLen   int
	isHttps  bool

	closed bool

	readRequestOnce sync.Once
}

func NewAutoHttpsConn(conn net.Conn) net.Conn {
	return &AutoHttpsConn{
		Conn: conn,
	}
}

func (c *AutoHttpsConn) detectProtocol() bool {
	if c.closed {
		return false
	}

	// 设置读取超时，避免阻塞 - 增加超时时间以提高兼容性
	_ = c.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer func() { _ = c.Conn.SetReadDeadline(time.Time{}) }()

	// 尝试读取少量数据来判断协议
	c.firstBuf = make([]byte, 512) // 减小缓冲区大小
	n, err := c.Conn.Read(c.firstBuf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			logger.Warning("Read timeout during protocol detection, treating as HTTPS")
		} else {
			logger.Warning("Failed to read initial data for protocol detection:", err)
		}
		// 无法读取数据，默认视为HTTPS
		c.isHttps = true
		return true
	}

	if n == 0 {
		// 没有数据，默认视为HTTPS
		c.isHttps = true
		return true
	}

	c.firstBuf = c.firstBuf[:n]
	c.bufLen = n
	c.bufStart = 0

	// 检查是否是HTTPS (TLS handshake starts with 0x16 for TLS 1.0-1.2, 0x17 for TLS 1.3)
	if n >= 1 && (c.firstBuf[0] == 0x16 || c.firstBuf[0] == 0x17) {
		// 这确实是TLS握手，这是HTTPS连接
		c.isHttps = true
		logger.Debug("Detected HTTPS connection via TLS handshake")
		return true
	}

	// 尝试解析为HTTP请求
	reader := bytes.NewReader(c.firstBuf)
	bufReader := bufio.NewReader(reader)
	request, err := http.ReadRequest(bufReader)
	if err != nil {
		// 无法解析为HTTP请求，检查是否是TLS握手（可能有额外数据）
		if n >= 3 && c.firstBuf[0] == 0x16 {
			c.isHttps = true
			logger.Debug("Detected HTTPS connection (TLS protocol)")
			return true
		}

		// 无法确定协议，默认为HTTPS避免连接关闭
		logger.Warning("Unable to determine connection protocol, treating as HTTPS to prevent connection closure")
		c.isHttps = true
		return true
	}

	// 成功解析HTTP请求，发送重定向但不关闭连接
	c.sendRedirect(request)
	return true
}

func (c *AutoHttpsConn) sendRedirect(request *http.Request) {
	if c.closed {
		return
	}

	resp := http.Response{
		Header: make(http.Header),
	}
	resp.StatusCode = http.StatusTemporaryRedirect
	location := fmt.Sprintf("https://%v%v", request.Host, request.RequestURI)
	resp.Header.Set("Location", location)

	// 设置响应头
	resp.Header.Set("Connection", "close")
	resp.Header.Set("Content-Length", "0")

	// 发送重定向响应
	if err := resp.Write(c.Conn); err != nil {
		logger.Warning("Failed to send redirect response:", err)
	}

	// 使用配置的延时，给客户端时间接收响应
	time.Sleep(config.HTTPSRedirectDelay)
	_ = c.Close()
	logger.Info("HTTP request redirected to HTTPS")
}

func (c *AutoHttpsConn) Read(buf []byte) (int, error) {
	if c.closed {
		return 0, io.EOF
	}

	c.readRequestOnce.Do(func() {
		c.detectProtocol()
	})

	// 优先返回缓冲区中的数据
	if c.firstBuf != nil && c.bufStart < c.bufLen {
		n := copy(buf, c.firstBuf[c.bufStart:c.bufLen])
		c.bufStart += n
		if c.bufStart >= c.bufLen {
			// 缓冲区数据全部读取完，清空缓冲区
			c.firstBuf = nil
			c.bufStart = 0
			c.bufLen = 0
		}
		return n, nil
	}

	// 缓冲区数据已全部读取，从底层连接读取
	// 设置超时避免阻塞 - 与协议检测超时保持一致
	_ = c.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	n, err := c.Conn.Read(buf)
	_ = c.Conn.SetReadDeadline(time.Time{})

	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			logger.Debug("Read timeout in AutoHttpsConn.Read")
			return 0, io.EOF
		}
		return n, err
	}

	return n, nil
}

func (c *AutoHttpsConn) Write(buf []byte) (int, error) {
	if c.closed {
		return 0, io.EOF
	}

	// 设置写超时 - 增加超时时间提高兼容性
	_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer func() { _ = c.Conn.SetWriteDeadline(time.Time{}) }()

	return c.Conn.Write(buf)
}

func (c *AutoHttpsConn) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.Conn.Close()
}

func (c *AutoHttpsConn) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

func (c *AutoHttpsConn) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

func (c *AutoHttpsConn) SetDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *AutoHttpsConn) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *AutoHttpsConn) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}
