package middleware

import (
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"x-ui/config"
	"x-ui/logger"

	"github.com/gin-gonic/gin"
)

// RecoveryMiddleware 捕获所有 panic，防止服务崩溃
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 检查是否是连接断开导致的 panic (Broken pipe)
				// 这种情况通常不需要打印堆栈，只需记录
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				if brokenPipe {
					logger.Errorf("[PANIC RECOVER] Broken pipe: %v", err)
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				// 真正的 panic 处理
				if config.IsDebug() {
					// 开发模式：打印完整堆栈
					stack := string(debug.Stack())
					logger.Errorf("[PANIC RECOVER] panic recovered:\nError: %v\nStack: %s", err, stack)
				} else {
					// 生产模式：仅记录错误信息，避免日志爆炸，但保留关键标记
					logger.Errorf("[PANIC RECOVER] panic recovered: %v", err)
				}

				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
