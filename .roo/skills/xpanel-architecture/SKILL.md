---
name: xpanel-architecture
description: X-Panel 项目架构说明。在需要理解项目结构、模块职责、数据流或添加新功能时使用。
---

# X-Panel 项目架构

## 目录结构

```
X-Panel/
├── main.go              # 入口，命令行解析，服务启动
├── config/              # 配置管理
├── database/            # 数据库初始化 (SQLite + Gorm)
│   ├── db.go           # 连接池、WAL 模式、迁移
│   └── model/          # 数据模型定义
├── logger/              # 日志系统
├── web/                 # Web 层 (Gin)
│   ├── web.go          # Server 结构体，路由初始化
│   ├── controller/     # HTTP 控制器
│   ├── service/        # 业务逻辑层
│   ├── middleware/     # 中间件
│   ├── html/           # Go Templates
│   ├── assets/         # 静态资源 (JS/CSS)
│   ├── job/            # 定时任务 (cron)
│   └── translation/    # i18n 翻译文件
├── xray/                # Xray-core 集成
│   ├── api.go          # gRPC API 调用
│   ├── process.go      # 进程管理
│   └── config.go       # 配置生成
├── sub/                 # 订阅服务
└── util/                # 通用工具函数
```

## 分层架构

```
┌─────────────────────────────────────────┐
│              Controller                  │  ← HTTP 请求处理、参数绑定
├─────────────────────────────────────────┤
│               Service                    │  ← 业务逻辑、数据处理
├─────────────────────────────────────────┤
│            Database/Model                │  ← 数据持久化 (Gorm)
└─────────────────────────────────────────┘
```

### Controller 层

- 位置: `web/controller/`
- 职责: 处理 HTTP 请求、参数验证、调用 Service、返回响应
- 命名: `<功能>.go` (如 `inbound.go`, `setting.go`)

```go
type InboundController struct {
    BaseController
    inboundService  service.InboundService
}

func (a *InboundController) getInbound(c *gin.Context) {
    // 1. 参数解析
    id, _ := strconv.Atoi(c.Param("id"))
    // 2. 调用 Service
    inbound, err := a.inboundService.Get(id)
    // 3. 返回 JSON
    c.JSON(200, inbound)
}
```

### Service 层

- 位置: `web/service/`
- 职责: 业务逻辑、数据库操作、外部服务调用
- 文件: 按功能划分（`inbound.go`, `setting.go`, `xray.go`）

```go
type InboundService struct{}

func (s *InboundService) GetAll() ([]*model.Inbound, error) {
    db := database.GetDB()
    var inbounds []*model.Inbound
    err := db.Find(&inbounds).Error
    return inbounds, err
}
```

### 依赖注入模式

Server 通过构造函数注入依赖：

```go
// web/web.go
func NewServer(
    serverService *service.ServerService,
    xrayService *service.XrayService,
    inboundService *service.InboundService,
    outboundService *service.OutboundService,
) *Server {
    return &Server{
        serverService:   serverService,
        xrayService:     xrayService,
        inboundService:  inboundService,
        outboundService: outboundService,
    }
}
```

## 核心组件

### Xray 管理

- `xray/process.go`: 启动/停止 Xray 进程
- `xray/api.go`: 通过 gRPC 与 Xray 通信
- `web/service/xray.go`: 封装 Xray 操作

### 定时任务

- 位置: `web/job/`
- 框架: `github.com/robfig/cron/v3`
- 任务: 流量统计、IP 检查、证书监控

### Telegram Bot

- 位置: `web/service/tgbot_*.go`
- 框架: `github.com/mymmrac/telego`
- 功能: 命令处理、回调处理、通知推送

## 新功能开发流程

1. **定义模型** (如需): `database/model/<name>.go`
2. **创建 Service**: `web/service/<name>.go`
3. **创建 Controller**: `web/controller/<name>.go`
4. **注册路由**: Controller 的 `initRouter()` 方法
5. **创建视图** (如需): `web/html/<name>.html`
6. **验证**: `go build ./... && go test ./...`
