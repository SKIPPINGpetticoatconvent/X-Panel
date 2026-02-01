package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RedirectMiddleware(basePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Redirect from old '/xui' path to '/panel'
		// Redirect from old '/xui' path to '/panel'
		// Use slice to ensure order (specific paths first)
		redirects := []struct {
			from string
			to   string
		}{
			{"panel/API", "panel/api"},
			{"xui/API", "panel/api"},
			{"xui", "panel"},
		}

		path := c.Request.URL.Path
		for _, r := range redirects {
			from, to := basePath+r.from, basePath+r.to

			if strings.HasPrefix(path, from) {
				newPath := to + path[len(from):]

				c.Redirect(http.StatusMovedPermanently, newPath)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
