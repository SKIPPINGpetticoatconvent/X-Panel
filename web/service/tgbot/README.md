# Telegram Bot 架构文档

## 概述

本文档描述了 X-Panel 项目中 Telegram Bot 模块的全新架构设计。该架构采用模块化设计，将原来的单一文件重构为多个独立的组件，以提高代码的可维护性、可测试性和可扩展性。

## 新架构概览

### 目录结构

```
web/service/tgbot/
├── core/                    # 核心组件模块
│   ├── bot.go              # Bot 生命周期管理
│   ├── context.go          # 上下文封装
│   ├── router.go           # 命令和回调路由器
│   ├── router_test.go      # Router 单元测试
│   └── README.go           # 核心模块文档
├── handlers/                # 处理器模块
│   ├── admin.go            # 管理员专用命令
│   ├── inbound.go          # 入站管理命令
│   ├── common.go           # 通用命令
│   ├── integration_example.go # 集成示例
│   └── README.md           # 处理器模块文档
└── README.md               # 本文档
```

### 核心组件

#### 1. Context (context.go)
- **职责**: 封装 Telegram Bot 的上下文信息
- **功能**: 
  - 消息发送和接收
  - 管理员权限检查
  - 国际化支持
  - 查询编码/解码
- **设计特点**: 
  - 实现了 `ContextInterface` 接口，便于测试和模拟
  - 提供了丰富的消息处理方法

#### 2. Router (router.go)
- **职责**: 负责命令和回调查询的注册与分发
- **功能**:
  - 注册命令处理器
  - 注册回调查询处理器
  - 命令分发逻辑
  - 回调分发逻辑
- **设计特点**:
  - 支持外部命令注册
  - 内置管理员权限检查
  - 提供默认命令和回调处理器

#### 3. Bot (bot.go)
- **职责**: Bot 生命周期管理
- **功能**:
  - Bot 启动/停止/重启
  - 配置加载和验证
  - 连接性检查
  - 消息接收和处理
- **设计特点**:
  - 实现了 `Bot` 接口，便于测试和扩展
  - 完整的生命周期管理
  - 错误处理和日志记录

#### 4. Handlers (handlers/)
- **职责**: 具体的业务逻辑处理
- **功能**:
  - 管理员命令处理
  - 入站配置管理
  - 通用功能实现
- **设计特点**:
  - 按功能模块分离
  - 可独立开发和测试
  - 易于扩展和维护

## 如何添加新命令

### 步骤 1: 创建命令处理器

在 `handlers/` 目录下创建或编辑相应的处理器文件：

```go
// handlers/myfeature.go
package handlers

import (
    "github.com/mymmrac/telego"
    "x-ui/web/service/tgbot/core"
)

// MyFeatureHandler 处理我的功能
func MyFeatureHandler(ctx *core.Context, message telego.Message, isAdmin bool) error {
    // 业务逻辑实现
    msg := "这是我的新功能！"
    return ctx.SendMsgToTgbot(message.Chat.ID, msg)
}
```

### 步骤 2: 注册命令

在 `core/router.go` 中的 `RegisterDefaultCommands` 方法中添加：

```go
// 在 RegisterDefaultCommands 方法中添加
r.RegisterCommand("myfeature", func(ctx *core.Context, message telego.Message, isAdmin bool) error {
    return handlers.MyFeatureHandler(ctx, message, isAdmin)
})
```

### 步骤 3: 设置 Bot 命令

在 `core/bot.go` 中的 `setupBotCommands` 方法中添加：

```go
// 在 setupBotCommands 方法中添加
{Command: "myfeature", Description: "我的新功能"},
```

### 步骤 4: 添加回调支持（可选）

如果需要回调查询支持：

```go
// 注册回调查询处理器
r.RegisterCallback("myfeature_action", func(ctx *core.Context, query telego.CallbackQuery, isAdmin bool) error {
    // 回调处理逻辑
    return ctx.AnswerCallbackQuery(query.ID, "操作已执行")
})
```

## 外部集成示例

如果你想在外部模块中添加自定义命令，可以这样使用：

```go
// 导入核心模块
import "x-ui/web/service/tgbot/core"

// 创建 Bot 实例
bot := core.NewBot()
router := bot.GetRouter()

// 注册外部命令
router.RegisterCommandExt("myexternalcmd", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
    return ctx.SendMsgToTgbot(message.GetChat().ID, "外部命令执行成功")
})
```

## 从旧架构迁移

### 迁移概述

新架构保持向后兼容性，旧的 `web/service/tgbot.go` 文件暂时保留，不会影响现有功能。迁移过程可以逐步进行。

### 迁移步骤

#### 步骤 1: 分析现有代码

1. 识别现有的命令和回调处理器
2. 提取业务逻辑
3. 确定依赖关系

#### 步骤 2: 重构为模块化

1. **Context 迁移**:
   - 将原有 Context 相关代码迁移到 `core/context.go`
   - 实现 `ContextInterface` 接口

2. **Router 迁移**:
   - 将命令注册逻辑迁移到 `core/router.go`
   - 实现命令分发机制

3. **Handler 迁移**:
   - 按功能将原有代码分离到 `handlers/` 目录
   - 创建独立的处理器文件

4. **Bot 迁移**:
   - 将生命周期管理迁移到 `core/bot.go`
   - 实现 `Bot` 接口

#### 步骤 3: 测试验证

1. **单元测试**:
   - 为 Router 添加单元测试
   - 为 Context 添加单元测试
   - 为关键处理器添加单元测试

2. **集成测试**:
   - 测试完整的命令处理流程
   - 测试管理员权限控制
   - 测试错误处理

#### 步骤 4: 部署和切换

1. **并行运行**:
   - 新架构与旧架构并行运行
   - 验证功能一致性

2. **逐步切换**:
   - 逐步将流量切换到新架构
   - 监控性能和稳定性

3. **清理旧代码**:
   - 确认新架构稳定后，删除旧代码

## 测试指南

### 运行单元测试

```bash
# 运行核心模块测试
cd web/service/tgbot/core
go test -v

# 运行特定测试
go test -v -run TestRouter_HandleCommand
```

### 测试覆盖率

```bash
# 生成测试覆盖率报告
go test -v -cover
go test -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 最佳实践

### 1. 代码组织

- **单一职责**: 每个组件只负责一个明确的功能
- **依赖注入**: 使用接口解耦依赖关系
- **错误处理**: 统一错误处理机制

### 2. 性能优化

- **异步处理**: 长时间操作使用 goroutine
- **缓存机制**: 适当使用缓存减少重复计算
- **资源管理**: 及时释放不需要的资源

### 3. 安全考虑

- **权限检查**: 所有管理员操作都需要权限验证
- **输入验证**: 严格验证用户输入
- **错误信息安全**: 不暴露敏感信息

### 4. 可维护性

- **文档更新**: 及时更新相关文档
- **日志记录**: 适当的日志记录便于调试
- **配置管理**: 使用配置文件而非硬编码

## 常见问题

### Q: 如何添加新的管理员命令？

A: 在 `handlers/admin.go` 中实现处理器，然后在 `core/router.go` 的 `RegisterDefaultCommands` 方法中注册。

### Q: 如何实现命令的国际化？

A: 使用 `ctx.I18n(key, params...)` 方法，其中 key 对应翻译文件中的键值。

### Q: 如何处理长时间运行的操作？

A: 使用 goroutine 在后台执行，并发送进度更新消息给用户。

### Q: 如何添加新的回调查询？

A: 在相应的处理器中调用 `ctx.EncodeQuery()` 编码查询数据，然后在 `router.go` 中注册对应的回调处理器。

### Q: 如何测试新的命令处理器？

A: 参考 `router_test.go` 中的测试模式，使用 mock 对象模拟 Context 和 Telegram API。

## 总结

新的模块化架构为 X-Panel 的 Telegram Bot 提供了更好的可维护性、可测试性和可扩展性。通过遵循本指南，开发者可以轻松添加新功能、修复问题并保持代码质量。

如有问题，请参考现有的代码实现或联系开发团队。