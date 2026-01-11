package security

import (
	"bufio"
	"io"
	"net"
	"time"

	"x-ui/logger"
)

// ProtoDetectListener 协议检测监听器
type ProtoDetectListener struct {
	net.Listener
}

// NewProtoDetectListener 创建协议检测监听器
func NewProtoDetectListener(listener net.Listener) *ProtoDetectListener {
	return &ProtoDetectListener{
		Listener: listener,
	}
}

// Accept 实现net.Listener接口，检测协议类型
func (pdl *ProtoDetectListener) Accept() (net.Conn, error) {
	for {
		conn, err := pdl.Listener.Accept()
		if err != nil {
			return nil, err
		}

		// 创建带缓冲的连接用于协议检测
		bufferedConn := NewBufferedConn(conn)

		// 检测是否为TLS连接
		isTLS, err := pdl.detectTLS(bufferedConn)
		if err != nil {
			logger.Warningf("协议检测失败：%v", err)
			bufferedConn.Close()
			continue
		}

		// 这里可以根据检测结果进行不同处理
		// 目前只记录检测结果，实际应用中可以路由到不同处理器
		if isTLS {
			logger.Debugf("检测到TLS连接来自 %s", conn.RemoteAddr())
		} else {
			logger.Debugf("检测到非TLS连接来自 %s", conn.RemoteAddr())
		}

		return bufferedConn, nil
	}
}

// detectTLS 检测连接是否为TLS协议
func (pdl *ProtoDetectListener) detectTLS(conn *BufferedConn) (bool, error) {
	// 设置2秒读取超时
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// 读取首字节
	buf := make([]byte, 1)
	n, err := conn.Read(buf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// 超时，可能是客户端未发送数据
			return false, nil
		}
		if err == io.EOF {
			// 连接已关闭
			return false, err
		}
		return false, err
	}

	if n == 0 {
		// 未读取到数据
		return false, nil
	}

	// TLS记录层的首字节是0x16
	return buf[0] == 0x16, nil
}

// BufferedConn 带缓冲的连接包装器
type BufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

// NewBufferedConn 创建带缓冲的连接
func NewBufferedConn(conn net.Conn) *BufferedConn {
	return &BufferedConn{
		Conn:   conn,
		reader: bufio.NewReader(conn),
	}
}

// Read 实现io.Reader接口，使用缓冲读取
func (bc *BufferedConn) Read(b []byte) (int, error) {
	return bc.reader.Read(b)
}

// Peek 查看缓冲区中的数据而不消费
func (bc *BufferedConn) Peek(n int) ([]byte, error) {
	return bc.reader.Peek(n)
}

// Buffered 返回底层缓冲读取器
func (bc *BufferedConn) Buffered() int {
	return bc.reader.Buffered()
}

// SetReadDeadline 设置读取超时
func (bc *BufferedConn) SetReadDeadline(t time.Time) error {
	return bc.Conn.SetReadDeadline(t)
}

// SetWriteDeadline 设置写入超时
func (bc *BufferedConn) SetWriteDeadline(t time.Time) error {
	return bc.Conn.SetWriteDeadline(t)
}

// SetDeadline 设置读写超时
func (bc *BufferedConn) SetDeadline(t time.Time) error {
	return bc.Conn.SetDeadline(t)
}

// Close 关闭连接
func (bc *BufferedConn) Close() error {
	return bc.Conn.Close()
}

// LocalAddr 返回本地地址
func (bc *BufferedConn) LocalAddr() net.Addr {
	return bc.Conn.LocalAddr()
}

// RemoteAddr 返回远程地址
func (bc *BufferedConn) RemoteAddr() net.Addr {
	return bc.Conn.RemoteAddr()
}

// ProtocolDetector 协议检测器接口
type ProtocolDetector interface {
	Detect(conn net.Conn) (string, error)
}

// TLSDetector TLS协议检测器
type TLSDetector struct{}

// Detect 检测是否为TLS协议
func (td *TLSDetector) Detect(conn net.Conn) (string, error) {
	bufferedConn := NewBufferedConn(conn)

	// 设置读取超时
	bufferedConn.SetReadDeadline(time.Now().Add(2 * time.Second))

	buf := make([]byte, 1)
	n, err := bufferedConn.Read(buf)
	if err != nil {
		return "", err
	}

	if n == 0 {
		return "unknown", nil
	}

	if buf[0] == 0x16 {
		return "tls", nil
	}

	return "unknown", nil
}

// DetectProtocol 检测协议类型
func DetectProtocol(conn net.Conn) (string, error) {
	detector := &TLSDetector{}
	return detector.Detect(conn)
}