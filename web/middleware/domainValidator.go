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
		
		// 清理和标准化主机名处理
		cleanHost := host
		
		// 首先尝试使用标准的net.SplitHostPort来分离主机和端口
		if strings.Contains(host, ":") {
			// 检查是否是IPv6地址格式 [::1]:port 或 IPv4:port
			if strings.HasPrefix(host, "[") && strings.Contains(host, "]:") {
				// IPv6地址格式 [::1]:port
				// 提取括号内的IPv6地址
				if endBracket := strings.Index(host, "]"); endBracket != -1 {
					cleanHost = host[1:endBracket] // 去掉方括号
				}
			} else {
				// IPv4:port 或域名:port 格式
				var err error
				cleanHost, _, err = net.SplitHostPort(host)
				if err != nil {
					// 如果SplitHostPort失败，直接使用原始host
					// 这可能是没有端口的情况或者是其他格式
					cleanHost = host
				}
			}
		}
		
		// 对于IPv6地址，确保它是标准格式
		if strings.Contains(cleanHost, ":") && !strings.HasPrefix(cleanHost, "[") {
			cleanHost = "[" + cleanHost + "]"
		}

		// 比较域名（忽略大小写）
		if !strings.EqualFold(cleanHost, domain) {
			logger.Warningf("Domain validation failed: expected %s, got %s from %s",
				domain, cleanHost, c.ClientIP())
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		logger.Debugf("Domain validation passed for %s", cleanHost)
		c.Next()
	}
}
