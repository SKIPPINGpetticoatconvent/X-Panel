package network

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"x-ui/logger"
)

type AutoHttpsConn struct {
	net.Conn

	firstBuf    []byte
	bufStart    int
	isHttps     bool
	initialized bool

	readRequestOnce sync.Once
}

func NewAutoHttpsConn(conn net.Conn) net.Conn {
	return &AutoHttpsConn{
		Conn: conn,
	}
}

func (c *AutoHttpsConn) detectProtocol() bool {
	// 尝试读取初始数据来判断是HTTP还是HTTPS
	c.firstBuf = make([]byte, 1024) // 增加缓冲区大小以确保能读取完整的TLS握手
	n, err := c.Conn.Read(c.firstBuf)
	
	if err != nil {
		logger.Warning("Failed to read initial data for protocol detection:", err)
		return false
	}
	
	c.firstBuf = c.firstBuf[:n]
	
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
		// 无法解析为HTTP请求
		// 检查是否是TLS握手（可能有额外数据）
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
	
	// 延迟关闭连接，给客户端时间接收响应
	time.Sleep(100 * time.Millisecond)
	c.Close()
	logger.Info("HTTP request redirected to HTTPS")
	return true
}

func (c *AutoHttpsConn) Read(buf []byte) (int, error) {
	c.readRequestOnce.Do(func() {
		c.detectProtocol()
	})

	if c.firstBuf != nil && !c.isHttps {
		// 只在HTTP连接时处理缓冲数据
		n := copy(buf, c.firstBuf[c.bufStart:])
		c.bufStart += n
		if c.bufStart >= len(c.firstBuf) {
			c.firstBuf = nil
		}
		return n, nil
	}

	// 对于HTTPS连接，直接转发到原始连接
	return c.Conn.Read(buf)
}

func (c *AutoHttpsConn) Write(buf []byte) (int, error) {
	return c.Conn.Write(buf)
}

func (c *AutoHttpsConn) Close() error {
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
