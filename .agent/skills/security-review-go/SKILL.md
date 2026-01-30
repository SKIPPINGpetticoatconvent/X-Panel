---
name: security-review-go
description: Go 语言安全审查技能。在实现认证、处理用户输入、管理密钥、创建 API 端点或实现敏感功能时使用。提供 Go/Gin/Gorm 专属的安全检查清单和模式。
---

# Go 安全审查技能

确保所有 Go 代码遵循安全最佳实践，识别潜在漏洞。

## 触发场景

- 实现认证或授权
- 处理用户输入或文件上传
- 创建新的 API 端点
- 处理密钥或凭证
- 存储或传输敏感数据
- 集成第三方 API

## 安全检查清单

### 1. 密钥管理

#### ❌ 禁止

```go
const apiKey = "sk-proj-xxxxx"  // 硬编码密钥
var dbPassword = "password123"   // 源码中的密码
```

#### ✅ 必须

```go
import "os"

func getConfig() (*Config, error) {
    apiKey := os.Getenv("API_KEY")
    if apiKey == "" {
        return nil, errors.New("API_KEY 环境变量未配置")
    }
    return &Config{APIKey: apiKey}, nil
}
```

#### 验证项
- [ ] 无硬编码 API 密钥、令牌或密码
- [ ] 所有密钥通过环境变量配置
- [ ] `.env` 文件已加入 `.gitignore`
- [ ] Git 历史中无密钥泄露

### 2. 输入验证 (Gin)

#### 使用结构体绑定验证

```go
type CreateUserRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Name     string `json:"name" binding:"required,min=1,max=100"`
    Age      int    `json:"age" binding:"gte=0,lte=150"`
    Password string `json:"password" binding:"required,min=8"`
}

func (a *UserController) CreateUser(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "验证失败", "details": err.Error()})
        return
    }
    // 处理已验证的数据
}
```

#### 文件上传验证

```go
func validateFileUpload(file *multipart.FileHeader) error {
    // 大小检查 (5MB)
    const maxSize = 5 * 1024 * 1024
    if file.Size > maxSize {
        return errors.New("文件过大 (最大 5MB)")
    }

    // 类型检查
    allowedTypes := map[string]bool{
        "image/jpeg": true,
        "image/png":  true,
        "image/gif":  true,
    }

    contentType := file.Header.Get("Content-Type")
    if !allowedTypes[contentType] {
        return errors.New("无效的文件类型")
    }

    // 扩展名检查
    ext := strings.ToLower(filepath.Ext(file.Filename))
    allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true}
    if !allowedExts[ext] {
        return errors.New("无效的文件扩展名")
    }

    return nil
}
```

#### 验证项
- [ ] 所有用户输入使用结构体绑定验证
- [ ] 文件上传有大小、类型、扩展名限制
- [ ] 使用白名单验证（非黑名单）
- [ ] 错误消息不泄露敏感信息

### 3. SQL 注入防护 (Gorm)

#### ❌ 危险：字符串拼接

```go
// 危险 - SQL 注入漏洞
query := fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", userEmail)
db.Raw(query).Scan(&user)
```

#### ✅ 安全：参数化查询

```go
// 安全 - Gorm 自动处理参数化
db.Where("email = ?", userEmail).First(&user)

// 或使用命名参数
db.Where("email = @email AND status = @status", 
    sql.Named("email", userEmail), 
    sql.Named("status", "active")).Find(&users)
```

#### 验证项
- [ ] 所有数据库查询使用参数化查询
- [ ] 禁止 SQL 字符串拼接
- [ ] Gorm 查询正确使用占位符

### 4. 认证与授权

#### JWT 令牌处理

```go
import "github.com/golang-jwt/jwt/v5"

type Claims struct {
    UserID string `json:"user_id"`
    Role   string `json:"role"`
    jwt.RegisteredClaims
}

func verifyToken(tokenString string) (*Claims, error) {
    secret := []byte(os.Getenv("JWT_SECRET"))
    
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, errors.New("无效的签名方法")
        }
        return secret, nil
    })
    
    if err != nil || !token.Valid {
        return nil, errors.New("无效的令牌")
    }
    
    return token.Claims.(*Claims), nil
}
```

#### 授权中间件

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.AbortWithStatusJSON(401, gin.H{"error": "未提供认证令牌"})
            return
        }
        
        token = strings.TrimPrefix(token, "Bearer ")
        claims, err := verifyToken(token)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"error": "无效的认证令牌"})
            return
        }
        
        c.Set("user_id", claims.UserID)
        c.Set("role", claims.Role)
        c.Next()
    }
}

func AdminOnly() gin.HandlerFunc {
    return func(c *gin.Context) {
        role, _ := c.Get("role")
        if role != "admin" {
            c.AbortWithStatusJSON(403, gin.H{"error": "权限不足"})
            return
        }
        c.Next()
    }
}
```

#### 验证项
- [ ] 令牌存储安全（httpOnly cookies 优于 localStorage）
- [ ] 敏感操作前有授权检查
- [ ] 实现基于角色的访问控制
- [ ] 会话管理安全

### 5. XSS 防护

#### HTML 转义

```go
import "html/template"

// 始终转义用户输入
func sanitizeHTML(input string) string {
    return template.HTMLEscapeString(input)
}

// 在模板中使用
// {{ .UserContent }} - 自动转义
// {{ .UserContent | safeHTML }} - 仅在确认安全时使用
```

#### Content-Security-Policy 头

```go
func securityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Next()
    }
}
```

### 6. CSRF 防护

```go
import "github.com/gin-contrib/csrf"

func setupCSRF(r *gin.Engine) {
    r.Use(csrf.Middleware(csrf.Options{
        Secret: os.Getenv("CSRF_SECRET"),
        ErrorFunc: func(c *gin.Context) {
            c.JSON(403, gin.H{"error": "CSRF 令牌验证失败"})
            c.Abort()
        },
    }))
}

// 在表单中添加 CSRF 令牌
// <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">
```

### 7. 速率限制

```go
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters sync.Map
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
    return &RateLimiter{rate: r, burst: b}
}

func (rl *RateLimiter) GetLimiter(key string) *rate.Limiter {
    limiter, exists := rl.limiters.Load(key)
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters.Store(key, limiter)
    }
    return limiter.(*rate.Limiter)
}

func RateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        limiter := rl.GetLimiter(ip)
        
        if !limiter.Allow() {
            c.AbortWithStatusJSON(429, gin.H{"error": "请求过于频繁"})
            return
        }
        c.Next()
    }
}
```

### 8. 敏感数据处理

#### 日志安全

```go
// ❌ 错误：记录敏感数据
logger.Info("用户登录", "email", email, "password", password)

// ✅ 正确：脱敏处理
logger.Info("用户登录", "email", maskEmail(email), "user_id", userID)

func maskEmail(email string) string {
    parts := strings.Split(email, "@")
    if len(parts) != 2 {
        return "***"
    }
    if len(parts[0]) <= 2 {
        return "**@" + parts[1]
    }
    return parts[0][:2] + "***@" + parts[1]
}
```

#### 错误处理

```go
// ❌ 错误：暴露内部细节
func handler(c *gin.Context) {
    _, err := db.Query(...)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()}) // 可能暴露 SQL 结构
    }
}

// ✅ 正确：通用错误消息
func handler(c *gin.Context) {
    _, err := db.Query(...)
    if err != nil {
        logger.Error("数据库查询失败", "error", err) // 仅在日志中记录
        c.JSON(500, gin.H{"error": "服务器内部错误，请稍后重试"})
    }
}
```

### 9. 密码存储

```go
import "golang.org/x/crypto/bcrypt"

func hashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func checkPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

### 10. 依赖安全

```bash
# 检查已知漏洞
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# 更新依赖
go get -u ./...
go mod tidy
```

## 部署前安全检查清单

- [ ] **密钥**: 无硬编码密钥，全部使用环境变量
- [ ] **输入验证**: 所有用户输入已验证
- [ ] **SQL 注入**: 所有查询参数化
- [ ] **XSS**: 用户内容已转义
- [ ] **CSRF**: 启用保护
- [ ] **认证**: 令牌处理正确
- [ ] **授权**: 角色检查到位
- [ ] **速率限制**: 已在所有端点启用
- [ ] **HTTPS**: 生产环境强制使用
- [ ] **安全头**: CSP、X-Frame-Options 已配置
- [ ] **错误处理**: 不向用户暴露敏感信息
- [ ] **日志**: 不记录敏感数据
- [ ] **依赖**: 已更新，无已知漏洞

---

**记住**: 安全不可妥协。一个漏洞可能危及整个平台。有疑问时，采取更安全的做法。
