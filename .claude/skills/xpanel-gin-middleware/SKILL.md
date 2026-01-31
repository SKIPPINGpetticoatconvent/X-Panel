---
name: xpanel-gin-middleware
description: X-Panel Gin 中间件开发模式。在创建认证、日志、错误恢复、域名验证等中间件时使用。
---

# X-Panel Gin 中间件模式

## 中间件位置

`web/middleware/`

## 现有中间件

| 文件 | 功能 |
|------|------|
| `recovery.go` | Panic 恢复，防止服务崩溃 |
| `domainValidator.go` | 域名白名单验证 |
| `redirect.go` | URL 重定向 |

## 中间件模板

```go
package middleware

import "github.com/gin-gonic/gin"

func MyMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 前置处理
        
        c.Next()  // 调用后续处理器
        
        // 后置处理
    }
}
```

## 项目中间件示例

### Recovery 中间件

```go
func RecoveryMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if err := recover(); err != nil {
                // 检查连接断开 (Broken pipe)
                var brokenPipe bool
                if ne, ok := err.(*net.OpError); ok {
                    if se, ok := ne.Err.(*os.SyscallError); ok {
                        if strings.Contains(strings.ToLower(se.Error()), "broken pipe") {
                            brokenPipe = true
                        }
                    }
                }

                if brokenPipe {
                    logger.Errorf("[PANIC RECOVER] Broken pipe: %v", err)
                    c.Abort()
                    return
                }

                // 生产模式: 简洁日志
                logger.Errorf("[PANIC RECOVER] panic recovered: %v", err)
                c.AbortWithStatus(http.StatusInternalServerError)
            }
        }()
        c.Next()
    }
}
```

### 域名验证中间件

```go
func DomainValidatorMiddleware(allowedDomain string) gin.HandlerFunc {
    return func(c *gin.Context) {
        host := c.Request.Host
        // 移除端口号
        if idx := strings.Index(host, ":"); idx != -1 {
            host = host[:idx]
        }
        
        if host != allowedDomain {
            c.AbortWithStatus(http.StatusForbidden)
            return
        }
        c.Next()
    }
}
```

## 注册中间件

在 `web/web.go` 的 `initRouter()` 中注册：

```go
func (s *Server) initRouter() (*gin.Engine, error) {
    engine := gin.New()
    
    // 全局中间件
    engine.Use(gin.Logger())
    engine.Use(middleware.RecoveryMiddleware())
    
    // 条件中间件
    if webDomain != "" {
        engine.Use(middleware.DomainValidatorMiddleware(webDomain))
    }
    
    // 路由组中间件
    g := engine.Group(basePath)
    g.Use(middleware.AuthMiddleware())
    
    return engine, nil
}
```

## 认证中间件模式

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 从 session 获取用户信息
        session := sessions.Default(c)
        user := session.Get("login_user")
        
        if user == nil {
            c.Redirect(http.StatusTemporaryRedirect, basePath)
            c.Abort()
            return
        }
        
        c.Set("user", user)
        c.Next()
    }
}
```

## 日志中间件模式

```go
func LoggerMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path
        
        c.Next()
        
        latency := time.Since(start)
        status := c.Writer.Status()
        
        logger.Infof("[%d] %s %s %v", status, c.Request.Method, path, latency)
    }
}
```

## 最佳实践

1. **中间件职责单一**: 每个中间件只做一件事
2. **错误处理**: 使用 `c.AbortWithStatus()` 或 `c.AbortWithStatusJSON()`
3. **上下文传递**: 使用 `c.Set()` 和 `c.Get()` 传递数据
4. **性能考虑**: 避免在中间件中进行耗时操作
5. **日志规范**: 使用项目的 `x-ui/logger` 包
