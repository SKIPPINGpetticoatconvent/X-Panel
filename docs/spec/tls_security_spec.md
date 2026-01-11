# TLS Security Package Specification

## Overview
The `web/security` package provides a robust layer for handling TLS connections, mitigating abuse, and diagnosing handshake issues. It is designed to sit between the raw network listener and the application's TLS handler.

## 1. Connection Rate Limit Module (`RateLimitListener`)

### Functional Specification
- **Goal**: Protect the server from SYN floods and aggressive scanning by limiting the rate of accepted connections per source IP.
- **Mechanism**: Token Bucket algorithm per IP address.
- **Configuration**:
  - `MaxConnsPerSec`: Rate at which tokens are added (default: 5).
  - `Burst`: Maximum capacity of the bucket (default: 10).
  - `CleanupInterval`: Interval to remove stale IP entries (default: 1m).
- **Behavior**:
  - If `Allow(ip)` returns true, accept connection.
  - If `Allow(ip)` returns false, close connection immediately (or reject accept).
  - Whitelisted IPs bypass rate limiting.

### Pseudocode

```go
package security

import (
    "net"
    "sync"
    "time"
    "golang.org/x/time/rate" // Standard rate limiter
)

// TDD Anchor: TestRateLimit_Allow
// TDD Anchor: TestRateLimit_Burst
// TDD Anchor: TestRateLimit_Cleanup
// TDD Anchor: TestRateLimit_Whitelist

type RateLimitConfig struct {
    MaxConnsPerSec int
    Burst          int
    CleanupInterval time.Duration
}

type RateLimitListener struct {
    net.Listener
    config    RateLimitConfig
    limiters  map[string]*rate.Limiter
    lastSeen  map[string]time.Time
    whitelist map[string]bool
    mu        sync.RWMutex
    stopCleanup chan struct{}
}

func NewRateLimitListener(l net.Listener, config RateLimitConfig) *RateLimitListener {
    // Initialize defaults if zero
    // Start cleanup goroutine
    return &RateLimitListener{
        Listener: l,
        // ... init maps
    }
}

func (l *RateLimitListener) Accept() (net.Conn, error) {
    conn, err := l.Listener.Accept()
    if err != nil {
        return nil, err
    }

    ip := getRemoteIP(conn.RemoteAddr())

    if !l.allow(ip) {
        conn.Close()
        // TDD Anchor: Ensure rejected connections are closed and not returned (or returned with specific error if preferred, but usually we just drop or error log)
        // For this implementation, we might want to return a specific error or just loop to the next Accept
        // Let's decide to loop and drop silently to the caller, or return a specific error type.
        // Better approach for a Listener wrapper: Loop until a valid connection is found or error occurs.
        return l.Accept() // Recursive call or loop
    }

    return conn, nil
}

func (l *RateLimitListener) allow(ip string) bool {
    l.mu.Lock()
    defer l.mu.Unlock()

    if l.whitelist[ip] {
        return true
    }

    limiter, exists := l.limiters[ip]
    if !exists {
        limiter = rate.NewLimiter(rate.Limit(l.config.MaxConnsPerSec), l.config.Burst)
        l.limiters[ip] = limiter
    }

    l.lastSeen[ip] = time.Now()
    return limiter.Allow()
}

func (l *RateLimitListener) AddWhitelist(ip string) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.whitelist[ip] = true
}

// cleanupLoop runs periodically to remove old entries from limiters map
func (l *RateLimitListener) cleanupLoop() {
    // Ticker loop
    // Lock, iterate lastSeen, delete old entries, Unlock
}

func getRemoteIP(addr net.Addr) string {
    // Split host/port
    // Return host
    return "" // implementation detail
}
```

## 2. Protocol Detection Optimization Module (`ProtoDetectListener`)

### Functional Specification
- **Goal**: Reduce log noise and resource usage from non-TLS traffic (e.g., HTTP requests to HTTPS port) and slow-loris attacks during handshake.
- **Mechanism**: Peek at the first byte of the connection.
- **Logic**:
  - TLS Handshake starts with `0x16` (Handshake Record).
  - If first byte != `0x16`, it's likely not TLS (could be HTTP `G`, `P`, etc.).
  - Set a strict read deadline for the first byte (e.g., 2 seconds).
- **Error Handling**:
  - Timeout -> `ErrHandshakeTimeout`
  - EOF -> `ErrClientClosed`
  - Non-TLS -> `ErrNotTLS`

### Pseudocode

```go
package security

import (
    "bufio"
    "errors"
    "net"
    "time"
)

// TDD Anchor: TestProtoDetect_TLS
// TDD Anchor: TestProtoDetect_NonTLS
// TDD Anchor: TestProtoDetect_Timeout

var (
    ErrNotTLS = errors.New("protocol detection: not TLS")
)

type ProtoDetectListener struct {
    net.Listener
    detectTimeout time.Duration
}

func NewProtoDetectListener(l net.Listener) *ProtoDetectListener {
    return &ProtoDetectListener{
        Listener:      l,
        detectTimeout: 2 * time.Second,
    }
}

func (l *ProtoDetectListener) Accept() (net.Conn, error) {
    conn, err := l.Listener.Accept()
    if err != nil {
        return nil, err
    }

    // Wrap connection to support peeking
    bufferedConn := newBufferedConn(conn)

    // Set deadline for detection
    conn.SetReadDeadline(time.Now().Add(l.detectTimeout))

    // Peek first byte
    firstByte, err := bufferedConn.Peek(1)
    
    // Reset deadline so handshake can proceed normally (or let caller set it)
    conn.SetReadDeadline(time.Time{})

    if err != nil {
        conn.Close()
        // Log debug if needed, but return error to caller to handle logging
        return nil, err 
    }

    if len(firstByte) > 0 && firstByte[0] != 0x16 {
        conn.Close()
        return nil, ErrNotTLS
    }

    return bufferedConn, nil
}

// bufferedConn wraps net.Conn with bufio.Reader to allow Peeking
type bufferedConn struct {
    net.Conn
    r *bufio.Reader
}

func newBufferedConn(c net.Conn) *bufferedConn {
    return &bufferedConn{
        Conn: c,
        r:    bufio.NewReader(c),
    }
}

func (b *bufferedConn) Read(p []byte) (n int, err error) {
    return b.r.Read(p)
}
```

## 3. Enhanced Error Log Module (`TLSErrorLogger`)

### Functional Specification
- **Goal**: Provide actionable insights into TLS failures without flooding logs with scanner noise.
- **Data Structure**:
  - `ClientIP`: Source IP.
  - `Time`: Timestamp.
  - `ErrorType`: Enum (Timeout, BadCert, NonTLS, Reset, Unknown).
  - `RawError`: Original error string.
  - `IsScanner`: Boolean flag (heuristic based).
- **Heuristics**:
  - `remote error: bad certificate` -> Client rejected our cert.
  - `tls: first record does not look like a TLS handshake` -> Non-TLS traffic.
  - `i/o timeout` during handshake -> Slow connection or scanner.

### Pseudocode

```go
package security

import (
    "log"
    "strings"
    "time"
)

// TDD Anchor: TestLogger_Categorize_BadCert
// TDD Anchor: TestLogger_Categorize_Scanner
// TDD Anchor: TestLogger_Format

type ErrorType string

const (
    ErrorTypeTimeout  ErrorType = "TIMEOUT"
    ErrorTypeBadCert  ErrorType = "BAD_CERT"
    ErrorTypeNonTLS   ErrorType = "NON_TLS"
    ErrorTypeReset    ErrorType = "CONN_RESET"
    ErrorTypeUnknown  ErrorType = "UNKNOWN"
)

type TLSErrorLog struct {
    ClientIP  string
    Time      time.Time
    ErrorType ErrorType
    RawError  string
    IsScanner bool
}

type TLSErrorLogger struct {
    // dependencies like a structured logger interface
}

func (l *TLSErrorLogger) AnalyzeAndLog(ip string, err error) {
    if err == nil {
        return
    }

    logEntry := TLSErrorLog{
        ClientIP: ip,
        Time:     time.Now(),
        RawError: err.Error(),
    }

    logEntry.ErrorType = categorizeError(err)
    logEntry.IsScanner = isScanner(logEntry.ErrorType)

    l.emit(logEntry)
}

func categorizeError(err error) ErrorType {
    msg := strings.ToLower(err.Error())
    
    switch {
    case strings.Contains(msg, "timeout"):
        return ErrorTypeTimeout
    case strings.Contains(msg, "bad certificate"), strings.Contains(msg, "unknown certificate"):
        return ErrorTypeBadCert
    case strings.Contains(msg, "first record does not look like a tls handshake"), strings.Contains(msg, "http request"):
        return ErrorTypeNonTLS
    case strings.Contains(msg, "connection reset"), strings.Contains(msg, "broken pipe"):
        return ErrorTypeReset
    default:
        return ErrorTypeUnknown
    }
}

func isScanner(t ErrorType) bool {
    // Scanners often disconnect abruptly or send non-TLS data
    return t == ErrorTypeNonTLS || t == ErrorTypeReset || t == ErrorTypeTimeout
}

func (l *TLSErrorLogger) emit(entry TLSErrorLog) {
    // If IsScanner, maybe log at Debug level or aggregate
    // If BadCert, log at Warning
    // Implementation depends on the project's logging infrastructure
}
```

## 4. Certificate Health Check Module (`CertHealthChecker`)

### Functional Specification
- **Goal**: Verify that the certificates on disk are valid, match the expected IP/Domain, and are not expired.
- **Interface**: `Check(certPath, keyPath string) (time.Time, error)`
- **Checks**:
  - File existence and permission.
  - PEM parsing.
  - X.509 parsing.
  - Expiry check (`NotAfter`).
  - (Optional) IP SAN match if running on a specific public IP.

### Pseudocode

```go
package security

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "time"
)

// TDD Anchor: TestCertCheck_Valid
// TDD Anchor: TestCertCheck_Expired
// TDD Anchor: TestCertCheck_InvalidFile

type CertHealthChecker struct {
    // Optional: ExpectedIP string
}

func (c *CertHealthChecker) Check(certPath, keyPath string) (time.Time, error) {
    // 1. Load KeyPair to validate matching key/cert
    cert, err := tls.LoadX509KeyPair(certPath, keyPath)
    if err != nil {
        return time.Time{}, fmt.Errorf("failed to load keypair: %w", err)
    }

    // 2. Parse Leaf Certificate
    // tls.LoadX509KeyPair doesn't parse the leaf by default in all versions/contexts, 
    // but we can parse the bytes manually.
    if len(cert.Certificate) == 0 {
        return time.Time{}, fmt.Errorf("no certificate found")
    }

    x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
    if err != nil {
        return time.Time{}, fmt.Errorf("failed to parse x509: %w", err)
    }

    // 3. Check Expiry
    now := time.Now()
    if now.After(x509Cert.NotAfter) {
        return x509Cert.NotAfter, fmt.Errorf("certificate expired on %s", x509Cert.NotAfter)
    }

    if now.Before(x509Cert.NotBefore) {
        return x509Cert.NotAfter, fmt.Errorf("certificate not valid until %s", x509Cert.NotBefore)
    }

    // 4. (Optional) Check IP SANs if needed
    // if c.ExpectedIP != "" { ... }

    return x509Cert.NotAfter, nil
}
```

## Integration Strategy

To integrate these modules into the existing `web` package:

1.  **Initialization**:
    In `web/controller/server.go` (or equivalent entry point), initialize the `RateLimitListener` and `ProtoDetectListener` when setting up the TLS listener.

    ```go
    // Conceptual usage
    baseListener, _ := net.Listen("tcp", addr)
    
    // 1. Rate Limit
    rlListener := security.NewRateLimitListener(baseListener, config)
    
    // 2. Proto Detect
    pdListener := security.NewProtoDetectListener(rlListener)
    
    // 3. TLS
    tlsListener := tls.NewListener(pdListener, tlsConfig)
    ```

2.  **Error Logging**:
    Use `TLSErrorLogger` within the `http.Server.ErrorLog` or by wrapping the `tls.Config.GetCertificate` callback to capture handshake errors if possible, or simply rely on the `ProtoDetectListener` to catch early failures. Note that standard `http.Server` logs TLS handshake errors to its `ErrorLog`. We might need to parse those logs or use a custom `net.Listener` that logs accept errors before passing them up.

3.  **Health Check**:
    Expose an API endpoint or a startup check that runs `CertHealthChecker.Check()` to ensure the panel is serving valid certificates.
