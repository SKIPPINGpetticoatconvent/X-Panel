package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"x-ui/logger"
	"x-ui/web/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RecoveryMiddleware())

	r.GET("/panic", func(c *gin.Context) {
		panic("intentional test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)

	// Verifying status code
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Verifying logs
	// Note: logger.GetLogs returns formatted strings: "Time Level - Message"
	logs := logger.GetLogs(50, "ERROR")
	found := false
	for _, logMsg := range logs {
		if strings.Contains(logMsg, "[PANIC RECOVER]") && strings.Contains(logMsg, "intentional test panic") {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected panic log not found in logger buffer. Captured logs: %v", logs)
}
