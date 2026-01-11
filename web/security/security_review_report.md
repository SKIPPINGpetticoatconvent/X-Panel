# Security Review Report: web/security

## 1. Executive Summary
A comprehensive security review was performed on the `web/security` package. The package implements essential security features including rate limiting, protocol detection, TLS error logging, and certificate health monitoring. 

Overall, the code quality is good, with modular design and clear separation of concerns. A critical memory leak vulnerability was identified in the Rate Limiter module and has been patched during this review. Several other medium and low severity issues were identified with recommended mitigations.

## 2. Scope
The following files were reviewed:
- `web/security/rate_limiter.go`
- `web/security/proto_detect.go`
- `web/security/tls_error_logger.go`
- `web/security/cert_health.go`
- `web/security/security.go`
- Associated test files.

## 3. Findings

### 3.1 High Severity

#### [FIXED] Memory Leak in Rate Limiter
- **Description**: The `rateLimiterImpl` struct used a map `limiters` to store rate limiters for each client IP. Prior to the fix, there was no mechanism to remove old entries, causing the map to grow indefinitely as new IPs connected. This could lead to memory exhaustion (DoS) under a distributed attack.
- **Fix Applied**: Implemented `cleanupExpiredLimiters` to remove IPs inactive for over 1 hour. Added a background goroutine in `NewRateLimitListener` to trigger cleanup every 10 minutes.

### 3.2 Medium Severity

#### IP Rate Limiting Bypass (Reverse Proxy)
- **Description**: The rate limiter uses `conn.RemoteAddr()` to identify clients. If the server is deployed behind a reverse proxy (e.g., Nginx, Cloudflare), all connections will appear to come from the proxy's IP.
- **Impact**: The rate limiter will block the proxy, affecting all legitimate users, or if the proxy is whitelisted, rate limiting is effectively bypassed.
- **Recommendation**: If proxy support is required, implement PROXY protocol support or trust `X-Forwarded-For` headers (requires parsing HTTP/TLS first, which might be complex at this layer). Alternatively, document that this rate limiter is for direct connections only.

#### Log Injection Risk
- **Description**: `tls_error_logger.go` logs `rawError` returned by the TLS library. While usually safe, if an attacker can manipulate the error string (e.g., via a custom TLS implementation), they might inject control characters into the logs.
- **Impact**: Log spoofing or corruption.
- **Recommendation**: Sanitize `rawError` before logging, replacing newlines and control characters with safe alternatives.

### 3.3 Low Severity

#### Hardcoded Scanner Errors
- **Description**: `knownScannerErrors` in `tls_error_logger.go` contains a hardcoded list of error strings.
- **Impact**: Maintenance burden; new scanner patterns won't be detected without code changes.
- **Recommendation**: Move these patterns to an external configuration file.

#### Magic Byte Protocol Detection
- **Description**: `proto_detect.go` identifies TLS by checking if the first byte is `0x16`.
- **Impact**: While standard for TLS ClientHello, it's a heuristic. Non-TLS protocols might coincidentally start with `0x16`.
- **Recommendation**: Acceptable for current logging/routing purposes, but consider inspecting more bytes (e.g., version major/minor) for stricter detection if needed.

## 4. Code Quality & Modular Boundaries

- **File Size**: All files are well under 500 lines, adhering to the requirement.
- **Coupling**: The package has low coupling, primarily depending on `x-ui/logger` and standard libraries. `security.go` serves as a clean facade.
- **Secrets**: No hardcoded secrets (passwords, keys) were found.
- **Concurrency**: `sync.RWMutex` is correctly used to protect shared maps in `rate_limiter.go`.

## 5. Conclusion
The `web/security` package is well-structured. The critical memory leak has been resolved. The remaining issues are primarily related to deployment architecture (proxy support) and minor improvements.

**Next Steps:**
1.  Verify the rate limiter fix in a staging environment.
2.  Decide on the strategy for Reverse Proxy support (documentation vs. implementation).
3.  Consider sanitizing log inputs.
