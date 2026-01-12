# 注意事项 (Project Notes)

## 安全注意事项

### 敏感信息处理

> [!CAUTION]
> **绝对禁止**将以下敏感信息提交到代码仓库：
> - 数据库文件 (`x-ui.db`)
> - 私钥和证书文件 (`*.key`, `*.crt`, `*.pem`)
> - API Keys 和 Tokens
> - 用户密码（明文或哈希）

**正确做法**:
1. 使用 `.gitignore` 排除敏感文件
2. 敏感配置通过环境变量传递
3. 使用 `.env` 文件管理本地开发配置（不提交）

### 认证与授权

- **密码存储**: 必须使用 bcrypt 或类似算法哈希存储
- **Session 管理**: 设置合理的过期时间，支持强制登出
- **访问控制**: API 端点必须验证用户权限

```go
// ✅ 正确: 使用 bcrypt 存储密码
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// ❌ 错误: 明文存储
user.Password = password // 危险！
```

### HTTPS 要求

> [!IMPORTANT]
> 生产环境**必须**使用 HTTPS 或 SSH 隧道访问面板，HTTP 明文传输会导致凭证泄露。

**推荐方案**:
1. 申请 Let's Encrypt 证书 (菜单选项 18)
2. 使用 SSH 端口转发: `ssh -L 15208:127.0.0.1:端口 root@IP`
3. 配置反向代理 (Nginx/Caddy) 终止 SSL

### API 安全

- 所有 API 请求必须验证 Session 或 Token
- 敏感操作需要二次确认或 CAPTCHA
- 实现请求频率限制防止暴力破解

## 部署注意事项

### 端口配置

| 用途 | 默认端口 | 说明 |
|------|----------|------|
| 面板 Web | 54321 | 管理界面端口 |
| Xray API | 随机 | Xray 核心 API |
| 入站协议 | 用户配置 | 代理服务端口 |

> [!WARNING]
> 部署后务必使用菜单选项 **22** 放行面板端口和所有入站端口。

### 防火墙配置

```bash
# UFW (Ubuntu/Debian)
sudo ufw allow 54321/tcp    # 面板端口
sudo ufw allow 443/tcp      # HTTPS
sudo ufw allow 入站端口/tcp  # 代理端口

# firewalld (CentOS)
sudo firewall-cmd --permanent --add-port=54321/tcp
sudo firewall-cmd --reload
```

### 资源限制

- **内存**: 建议至少 512MB 空闲内存
- **磁盘**: 数据库和日志会持续增长，建议定期清理
- **CPU**: 高并发场景下考虑限制连接数

### 备份策略

> [!TIP]
> 启用 Telegram 机器人自动备份功能，可在面板意外损坏时快速恢复。

**关键备份文件**:
| 文件 | 路径 | 说明 |
|------|------|------|
| 数据库 | `/etc/x-ui/x-ui.db` | 所有配置和用户数据 |
| Xray 配置 | `/usr/local/x-ui/bin/config.json` | Xray 核心配置 |
| 证书文件 | `/root/cert/` | SSL 证书 |

**手动备份命令**:
```bash
# 备份数据库
cp /etc/x-ui/x-ui.db ~/x-ui-backup-$(date +%Y%m%d).db

# 完整备份
tar -czf x-ui-backup.tar.gz /etc/x-ui /usr/local/x-ui/bin/config.json
```

## 开发注意事项

### 数据库迁移

- 新增字段必须设置默认值或允许 NULL
- 修改表结构前备份数据库
- 使用 GORM AutoMigrate 自动处理简单迁移

```go
// ✅ 正确: 新字段设置默认值
type Inbound struct {
    // ...
    NewField string `gorm:"default:'default_value'"`
}
```

### Xray 配置生成

- 配置变更后必须调用 Xray 重载
- 验证生成的 JSON 配置有效性
- 处理 Xray 进程启动失败的情况

### 并发安全

- 数据库操作使用事务保证一致性
- 共享状态使用互斥锁保护
- 避免在 HTTP Handler 中执行长时间阻塞操作

### 日志记录

- 使用项目统一的 Logger 模块
- 生产环境避免记录敏感信息
- 设置合理的日志级别和轮转策略

```go
// ✅ 正确
logger.Info("User logged in", "user_id", userID)

// ❌ 错误: 记录敏感信息
logger.Info("User logged in", "password", password)
```

## 常见问题

### 服务启动失败 (exit-code 2)

1. 检查日志: `journalctl -u x-ui -e`
2. 常见原因: 端口占用、配置错误、权限问题
3. 尝试重置配置: `/usr/local/x-ui/x-ui setting -reset`

### 无法访问面板

1. 确认服务运行中: `systemctl status x-ui`
2. 检查端口是否放行
3. 尝试 SSH 端口转发测试

### IP 证书申请失败

1. 确保 80 端口对外开放
2. 验证 IP 地址正确可达
3. 检查邮箱格式有效

## 版本兼容性

| X-Panel 版本 | Go 版本 | Xray 版本 |
|--------------|---------|-----------|
| v25.x | 1.21+ | 1.8.x |
| v24.x | 1.20+ | 1.7.x |
