# Telegram Bot Handlers

这个目录包含了 Telegram Bot 的处理器模块，实现了模块化的命令处理架构。

## 架构说明

### 核心组件

1. **CommonHandlers**: 通用命令处理器，处理基本的 bot 命令
2. **AdminHandlers**: 管理员命令处理器，处理系统管理相关命令
3. **InboundHandlers**: 入站管理处理器，处理入站配置和客户端管理
4. **ContextInterface**: 上下文接口，定义了处理器需要的核心方法
5. **ExternalCommandRegistry**: 外部命令注册接口，用于注册处理器

### 目录结构

```
handlers/
├── common.go              # 通用命令处理器实现
├── admin.go               # 管理员命令处理器实现
├── inbound.go             # 入站管理处理器实现
├── integration_example.go # 集成示例代码
└── README.md              # 使用说明
```

## 使用方法

### 1. 创建服务实例

首先需要创建各种服务实例：

```go
// 在 main.go 或服务初始化代码中
inboundService := service.NewInboundService(db)
settingService := service.NewSettingService(db)
serverService := service.NewServerService()
xrayService := service.NewXrayService()
```

### 2. 创建 Context 和 Router

```go
// 创建 Context 实例
ctx := core.NewContext()
ctx.SetBot(bot) // 设置 bot 实例
ctx.SetAdminIds(adminIds) // 设置管理员 ID 列表

// 创建 Router 实例
router := core.NewRouter(ctx)
```

### 3. 创建和注册各种处理器

```go
// 创建各种处理器实例
commonHandlers := handlers.NewCommonHandlers(ctx)
adminHandlers := handlers.NewAdminHandlers(ctx, serverService, xrayService, inboundService, settingService)
inboundHandlers := handlers.NewInboundHandlers(ctx, inboundService, serverService, xrayService)

// 注册所有处理器到路由器
commonHandlers.RegisterCommonCommands(router)
adminHandlers.RegisterAdminCommands(router)
inboundHandlers.RegisterInboundCommands(router)
```

### 4. 在 Bot 中使用

在现有的 bot 初始化代码中：

```go
func (b *TgBot) Start(i18nFS embed.FS, settingService interface{}) error {
    // ... 现有的初始化代码 ...
    
    // 创建 Context 和 Router
    ctx := core.NewContext()
    ctx.SetBot(b.bot)
    ctx.SetAdminIds(b.adminIds)
    router := core.NewRouter(ctx)
    
    // 创建各种处理器（需要传入实际的服务实例）
    commonHandlers := handlers.NewCommonHandlers(ctx)
    adminHandlers := handlers.NewAdminHandlers(ctx, serverService, xrayService, inboundService, settingService)
    inboundHandlers := handlers.NewInboundHandlers(ctx, inboundService, serverService, xrayService)
    
    // 注册所有处理器
    commonHandlers.RegisterCommonCommands(router)
    adminHandlers.RegisterAdminCommands(router)
    inboundHandlers.RegisterInboundCommands(router)
    
    // 设置处理器到 bot handler
    router.SetupHandlers(b.botHandler)
    
    // ... 其余代码 ...
}
```

## 支持的命令

### 通用命令 (CommonHandlers)
- `/start` - 欢迎消息（管理员显示主机名）
- `/help` - 帮助信息（根据用户类型显示不同内容）
- `/id` - 显示用户 Telegram ID
- `/version` - 显示 X-Panel 版本信息

### 管理员命令 (AdminHandlers)
- `/status` - 显示系统状态（CPU, 内存, 磁盘, Xray 状态）
- `/restart` - 重启面板
- `/restartx` - 重启 Xray 服务
- `/stop` - 停止 Xray 服务
- `/startx` - 启动 Xray 服务
- `/log` - 获取日志（可选参数：行数 日志级别）
- `/backup` - 备份数据库

### 入站管理命令 (InboundHandlers)
- `/inbound` - 入站管理
  - 不带参数：显示所有入站列表
  - 带端口号：显示指定入站详细信息
- `/clients` - 查看入站客户端列表
  - 参数：`<端口号>`
- `/toggle` - 启用/禁用入站
  - 参数：`<端口号>`

## 处理器详解

### CommonHandlers (通用处理器)
处理基本的 bot 命令，适合所有用户使用。提供欢迎消息、帮助信息和用户 ID 查询等功能。

### AdminHandlers (管理员处理器)
专门处理系统管理相关的命令，需要管理员权限。包含系统状态监控、服务控制、日志查看和数据库备份等功能。

### InboundHandlers (入站管理处理器)
处理入站配置和客户端管理的相关命令。需要管理员权限，提供入站列表查看、客户端管理和入站启停控制等功能。

## 扩展开发

### 添加新的命令处理器

1. 在 `handlers` 目录下创建新的处理器文件
2. 实现你的命令逻辑
3. 在 bot 初始化时注册你的处理器

### 示例：自定义命令处理器

```go
package handlers

import (
    "fmt"
    "x-ui/web/service/tgbot/core"
    "github.com/mymmrac/telego"
)

type CustomHandlers struct {
    ctx core.ContextInterface
}

func NewCustomHandlers(ctx core.ContextInterface) *CustomHandlers {
    return &CustomHandlers{ctx: ctx}
}

func (h *CustomHandlers) HandlePing(message telego.Message) error {
    return h.ctx.SendMsgToTgbot(message.Chat.ID, "pong!")
}

func (h *CustomHandlers) RegisterCommands(router core.ExternalCommandRegistry) {
    router.RegisterCommandExt("ping", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
        return h.HandlePing(message)
    })
}
```

## 设计原则

1. **模块化**: 每个处理器都是独立的模块，职责清晰
2. **接口分离**: 使用接口解耦不同组件，便于测试和维护
3. **向后兼容**: 不破坏现有的 bot 架构
4. **易于扩展**: 可以轻松添加新的命令处理器
5. **依赖注入**: 通过构造函数注入所需的服务依赖
6. **权限控制**: 支持管理员和普通用户的权限区分

## 注意事项

- 确保在注册处理器之前正确初始化 Context
- Router 必须实现 ExternalCommandRegistry 接口
- 所有处理器都应该使用 ContextInterface 来访问 bot 功能
- 管理员命令需要在处理函数中检查 `isAdmin` 参数
- 服务实例通过构造函数注入，不要在处理器内部创建服务实例

## 依赖说明

各处理器需要的服务依赖：

- **CommonHandlers**: 仅需要 `core.ContextInterface`
- **AdminHandlers**: 需要 `ServerService`, `XrayService`, `InboundService`, `SettingService`
- **InboundHandlers**: 需要 `InboundService`, `ServerService`, `XrayService`

确保在使用前正确初始化这些服务实例。