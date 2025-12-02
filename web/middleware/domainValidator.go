package middleware

import (
	"net"
	"net/http"
	"strings"

	"x-ui/logger"

	"github.com/gin-gonic/gin"
)

func DomainValidatorMiddleware(domain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		
		// 尝试分离主机名和端口
		if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
			var err error
			host, _, err = net.SplitHostPort(c.Request.Host)
			if err != nil {
				// 如果分离失败，可能是IPv6地址或者格式问题
				// 尝试查找最后一个冒号并处理IPv6地址
				if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
					// IPv6地址格式 [::1]:port
					host = host[1:len(host)-1]
				}
				// 如果仍然有端口，移除它
				if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
					host = host[:colonIndex]
				}
			}
		}

		// 比较域名（忽略大小写）
		if !strings.EqualFold(host, domain) {
			logger.Warningf("Domain validation failed: expected %s, got %s from %s",
				domain, host, c.ClientIP())
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		logger.Debugf("Domain validation passed for %s", host)
		c.Next()
	}
}
