package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- DomainValidator 测试 ---

func TestDomainValidatorMiddleware_ValidDomain(t *testing.T) {
	tests := []struct {
		name       string
		domain     string
		host       string
		wantStatus int
	}{
		{"精确匹配", "example.com", "example.com", http.StatusOK},
		{"带端口匹配", "example.com", "example.com:8080", http.StatusOK},
		{"忽略大小写", "Example.COM", "example.com", http.StatusOK},
		{"IPv4", "192.168.1.1", "192.168.1.1:443", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(DomainValidatorMiddleware(tt.domain))
			r.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Host = tt.host
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestDomainValidatorMiddleware_InvalidDomain(t *testing.T) {
	r := gin.New()
	r.Use(DomainValidatorMiddleware("allowed.com"))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "evil.com"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestDomainValidatorMiddleware_IPv6(t *testing.T) {
	r := gin.New()
	r.Use(DomainValidatorMiddleware("[::1]"))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "[::1]:8080"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("IPv6 validation failed: status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- RedirectMiddleware 测试 ---

func TestRedirectMiddleware_XUIToPanel(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCode int
		wantLoc  string
	}{
		{"xui -> panel", "/xui", http.StatusMovedPermanently, "/panel"},
		{"xui/ -> panel/", "/xui/", http.StatusMovedPermanently, "/panel/"},
		{"xui/path -> panel/path", "/xui/settings", http.StatusMovedPermanently, "/panel/settings"},
		{"panel/API -> panel/api", "/panel/API", http.StatusMovedPermanently, "/panel/api"},
		{"xui/API -> panel/api", "/xui/API", http.StatusMovedPermanently, "/panel/api"},
		{"正常路径不重定向", "/other", http.StatusOK, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(RedirectMiddleware("/"))
			r.GET("/other", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}
			if tt.wantLoc != "" {
				loc := w.Header().Get("Location")
				if loc != tt.wantLoc {
					t.Errorf("Location = %q, want %q", loc, tt.wantLoc)
				}
			}
		})
	}
}

func TestRedirectMiddleware_WithBasePath(t *testing.T) {
	r := gin.New()
	r.Use(RedirectMiddleware("/base/"))

	req := httptest.NewRequest("GET", "/base/xui", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMovedPermanently)
	}

	loc := w.Header().Get("Location")
	if loc != "/base/panel" {
		t.Errorf("Location = %q, want /base/panel", loc)
	}
}

// --- RecoveryMiddleware 测试 ---

func TestRecoveryMiddleware_PanicRecovery(t *testing.T) {
	r := gin.New()
	r.Use(RecoveryMiddleware())
	r.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d after panic", w.Code, http.StatusInternalServerError)
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	r := gin.New()
	r.Use(RecoveryMiddleware())
	r.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
