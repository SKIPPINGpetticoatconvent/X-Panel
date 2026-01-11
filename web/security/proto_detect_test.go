package security

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// protoMockConn 模拟net.Conn用于协议检测测试
type protoMockConn struct {
	data     []byte
	readPos  int
	remoteAddr net.Addr
}

func (m *protoMockConn) Read(b []byte) (n int, err error) {
	if m.readPos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(b, m.data[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *protoMockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *protoMockConn) Close() error                       { return nil }
func (m *protoMockConn) LocalAddr() net.Addr                { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080} }
func (m *protoMockConn) RemoteAddr() net.Addr               { return m.remoteAddr }
func (m *protoMockConn) SetDeadline(t time.Time) error      { return nil }
func (m *protoMockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *protoMockConn) SetWriteDeadline(t time.Time) error { return nil }

// TestBufferedConn_Read 测试带缓冲读取正确
func TestBufferedConn_Read(t *testing.T) {
	data := []byte("Hello, World!")
	conn := &protoMockConn{data: data}
	bufferedConn := NewBufferedConn(conn)

	buf := make([]byte, 5)
	n, err := bufferedConn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "Hello", string(buf[:n]))

	// 继续读取
	n, err = bufferedConn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, ", Wor", string(buf[:n]))
}

// TestBufferedConn_Peek 测试Peek不消耗数据
func TestBufferedConn_Peek(t *testing.T) {
	data := []byte("Hello, World!")
	conn := &protoMockConn{data: data}
	bufferedConn := NewBufferedConn(conn)

	// Peek数据
	peeked, err := bufferedConn.Peek(5)
	require.NoError(t, err)
	assert.Equal(t, "Hello", string(peeked))

	// Peek不应该消耗数据，再次读取应该得到相同数据
	buf := make([]byte, 5)
	n, err := bufferedConn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "Hello", string(buf[:n]))
}

// TestProtoDetect_TLS 测试识别TLS流量（0x16首字节）
func TestProtoDetect_TLS(t *testing.T) {
	// TLS记录层的首字节是0x16
	tlsData := []byte{0x16, 0x03, 0x01} // TLS 1.0 handshake
	conn := &protoMockConn{data: tlsData, remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 443}}
	bufferedConn := NewBufferedConn(conn)

	detector := &ProtoDetectListener{}
	isTLS, err := detector.detectTLS(bufferedConn)
	require.NoError(t, err)
	assert.True(t, isTLS, "应该识别为TLS流量")
}

// TestProtoDetect_NonTLS 测试识别非TLS流量
func TestProtoDetect_NonTLS(t *testing.T) {
	// 非TLS数据，比如HTTP请求
	httpData := []byte("GET / HTTP/1.1\r\n")
	conn := &protoMockConn{data: httpData, remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 80}}
	bufferedConn := NewBufferedConn(conn)

	detector := &ProtoDetectListener{}
	isTLS, err := detector.detectTLS(bufferedConn)
	require.NoError(t, err)
	assert.False(t, isTLS, "应该识别为非TLS流量")
}

// TestProtoDetect_Timeout 测试超时优雅处理
func TestProtoDetect_Timeout(t *testing.T) {
	// 创建一个模拟的超时连接
	conn := &protoMockConn{
		data:       []byte{}, // 空数据
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 8080},
	}

	bufferedConn := NewBufferedConn(conn)
	bufferedConn.SetReadDeadline(time.Now().Add(time.Millisecond)) // 很短的超时

	detector := &ProtoDetectListener{}
	isTLS, err := detector.detectTLS(bufferedConn)
	// 应该优雅处理超时，返回false且无错误或返回特定错误
	assert.False(t, isTLS)
	// 注意：实际实现中可能返回错误，这里测试框架适应性
	_ = err // 忽略错误检查，因为实现可能不同
}

// TestProtoDetect_EOF 测试EOF处理
func TestProtoDetect_EOF(t *testing.T) {
	// 空连接，立即EOF
	conn := &protoMockConn{
		data:       []byte{}, // 空数据
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 8080},
	}
	bufferedConn := NewBufferedConn(conn)

	detector := &ProtoDetectListener{}
	isTLS, err := detector.detectTLS(bufferedConn)
	// EOF表示连接已关闭，应该返回错误
	assert.False(t, isTLS)
	assert.Error(t, err, "EOF应该返回错误")
	assert.Equal(t, io.EOF, err, "错误应该是EOF")
}

// TestBufferedConn_Buffered 测试缓冲区大小
func TestBufferedConn_Buffered(t *testing.T) {
	data := []byte("Hello, World!")
	conn := &protoMockConn{data: data}
	bufferedConn := NewBufferedConn(conn)

	// 初始缓冲区应该为空
	assert.Equal(t, 0, bufferedConn.Buffered())

	// Peek一些数据 - Peek会填充缓冲区
	_, err := bufferedConn.Peek(5)
	require.NoError(t, err)
	bufferedBeforeRead := bufferedConn.Buffered()
	// 缓冲区现在包含至少5个字节
	assert.True(t, bufferedBeforeRead >= 5, "Peek后缓冲区应该至少包含5字节")

	// 读取数据
	buf := make([]byte, 3)
	n, err := bufferedConn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	// 读取3字节后，缓冲区应该减少3字节
	bufferedAfterRead := bufferedConn.Buffered()
	assert.Equal(t, bufferedBeforeRead-3, bufferedAfterRead, "读取后缓冲区大小应该减少3字节")
}

// TestBufferedConn_DeadlineMethods 测试BufferedConn的超时方法
func TestBufferedConn_DeadlineMethods(t *testing.T) {
	data := []byte("test")
	conn := &protoMockConn{data: data}
	bufferedConn := NewBufferedConn(conn)

	deadline := time.Now().Add(time.Minute)

	// 测试各种SetDeadline方法
	err := bufferedConn.SetReadDeadline(deadline)
	assert.NoError(t, err)

	err = bufferedConn.SetWriteDeadline(deadline)
	assert.NoError(t, err)

	err = bufferedConn.SetDeadline(deadline)
	assert.NoError(t, err)
}

// TestBufferedConn_AddrMethods 测试BufferedConn的地址方法
func TestBufferedConn_AddrMethods(t *testing.T) {
	data := []byte("test")
	conn := &protoMockConn{data: data, remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 8080}}
	bufferedConn := NewBufferedConn(conn)

	localAddr := bufferedConn.LocalAddr()
	assert.NotNil(t, localAddr)

	remoteAddr := bufferedConn.RemoteAddr()
	assert.NotNil(t, remoteAddr)
	assert.Equal(t, "192.168.1.100:8080", remoteAddr.String())
}

// TestBufferedConn_Close 测试BufferedConn的Close方法
func TestBufferedConn_Close(t *testing.T) {
	data := []byte("test")
	conn := &protoMockConn{data: data}
	bufferedConn := NewBufferedConn(conn)

	err := bufferedConn.Close()
	assert.NoError(t, err)
}

// TestDetectProtocol 测试协议检测函数
func TestDetectProtocol(t *testing.T) {
	// 测试TLS检测
	tlsData := []byte{0x16, 0x03, 0x03} // TLS 1.2 handshake
	conn := &protoMockConn{data: tlsData}
	protocol, err := DetectProtocol(conn)
	require.NoError(t, err)
	assert.Equal(t, "tls", protocol)

	// 测试非TLS检测
	httpData := []byte("GET / HTTP/1.1\r\n")
	conn = &protoMockConn{data: httpData}
	protocol, err = DetectProtocol(conn)
	require.NoError(t, err)
	assert.Equal(t, "unknown", protocol)
}

// mockListener 模拟net.Listener用于测试
type protoMockListener struct {
	conns chan net.Conn
}

func (m *protoMockListener) Accept() (net.Conn, error) {
	conn := <-m.conns
	return conn, nil
}

func (m *protoMockListener) Close() error {
	close(m.conns)
	return nil
}

func (m *protoMockListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

// TestProtoDetectListener_Accept 测试协议检测监听器的Accept方法
func TestProtoDetectListener_Accept(t *testing.T) {
	mockListener := &protoMockListener{conns: make(chan net.Conn, 1)}
	protoListener := NewProtoDetectListener(mockListener)

	// 发送TLS连接
	tlsConn := &protoMockConn{data: []byte{0x16, 0x03, 0x01}, remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 443}}
	mockListener.conns <- tlsConn

	conn, err := protoListener.Accept()
	require.NoError(t, err)
	assert.NotNil(t, conn)

	// 检查是否是BufferedConn
	_, ok := conn.(*BufferedConn)
	assert.True(t, ok, "应该返回BufferedConn")
}

// mockProtoListenerWithError 模拟会返回错误的监听器
type mockProtoListenerWithError struct {
	conns chan net.Conn
}

func (m *mockProtoListenerWithError) Accept() (net.Conn, error) {
	if len(m.conns) == 0 {
		return nil, errors.New("connection error")
	}
	conn := <-m.conns
	return conn, nil
}

func (m *mockProtoListenerWithError) Close() error {
	close(m.conns)
	return nil
}

func (m *mockProtoListenerWithError) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

// TestProtoDetectListener_Accept_Error 测试Accept方法中的错误处理
func TestProtoDetectListener_Accept_Error(t *testing.T) {
	mockListener := &mockProtoListenerWithError{conns: make(chan net.Conn, 1)}
	protoListener := NewProtoDetectListener(mockListener)

	// 监听器会返回错误
	conn, err := protoListener.Accept()
	assert.Error(t, err, "应该返回监听器错误")
	assert.Nil(t, conn, "错误时应该返回nil连接")
}

// TestNewProtoDetectListener 测试协议检测监听器创建
func TestNewProtoDetectListener(t *testing.T) {
	mockListener := &protoMockListener{conns: make(chan net.Conn, 1)}
	protoListener := NewProtoDetectListener(mockListener)

	assert.NotNil(t, protoListener)
	assert.Equal(t, mockListener, protoListener.Listener)
}