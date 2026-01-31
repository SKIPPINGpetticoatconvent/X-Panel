---
name: xpanel-tgbot
description: X-Panel Telegram Bot 开发模式。在添加 Bot 命令、回调处理、消息通知或扩展 Bot 功能时使用。
---

# X-Panel Telegram Bot 开发

## 文件结构

```
web/service/
├── tgbot_core.go      # Bot 初始化、启动、停止
├── tgbot_cmds.go      # 命令定义
├── tgbot_callback.go  # 回调处理
└── tgbot_utils.go     # 工具函数
```

## 框架

使用 `github.com/mymmrac/telego`

## Bot 核心结构

```go
type Tgbot struct {
    bot         *telego.Bot
    chatId      int64
    running     bool
    hashStorage *HashStorage
    
    // 服务依赖
    settingService SettingService
    xrayService    *XrayService
    inboundService *InboundService
}
```

## 添加新命令

### 1. 定义命令

在 `tgbot_cmds.go` 中：

```go
var commands = []telego.BotCommand{
    {Command: "status", Description: "查看系统状态"},
    {Command: "usage", Description: "查看流量统计"},
    {Command: "mycommand", Description: "我的新命令"},  // 新增
}
```

### 2. 实现处理器

在 `tgbot_callback.go` 中添加 case：

```go
func (t *Tgbot) handleMessage(message telego.Message) {
    text := message.Text
    
    switch {
    case strings.HasPrefix(text, "/status"):
        t.handleStatus(message)
    case strings.HasPrefix(text, "/mycommand"):
        t.handleMyCommand(message)
    }
}

func (t *Tgbot) handleMyCommand(message telego.Message) {
    // 获取数据
    data, err := t.inboundService.GetAll()
    if err != nil {
        t.sendError(message.Chat.ID, err)
        return
    }
    
    // 发送响应
    t.sendMessage(message.Chat.ID, formatData(data))
}
```

## 回调按钮处理

### 创建带按钮的消息

```go
func (t *Tgbot) sendWithButtons(chatId int64, text string) {
    keyboard := &telego.InlineKeyboardMarkup{
        InlineKeyboard: [][]telego.InlineKeyboardButton{
            {
                {Text: "确认", CallbackData: "confirm_action"},
                {Text: "取消", CallbackData: "cancel_action"},
            },
        },
    }
    
    t.bot.SendMessage(&telego.SendMessageParams{
        ChatID:      telego.ChatID{ID: chatId},
        Text:        text,
        ReplyMarkup: keyboard,
    })
}
```

### 处理回调

```go
func (t *Tgbot) handleCallback(callback telego.CallbackQuery) {
    data := callback.Data
    
    switch {
    case data == "confirm_action":
        t.handleConfirm(callback)
    case data == "cancel_action":
        t.handleCancel(callback)
    case strings.HasPrefix(data, "inbound_"):
        t.handleInboundCallback(callback)
    }
    
    // 应答回调 (移除加载动画)
    t.bot.AnswerCallbackQuery(&telego.AnswerCallbackQueryParams{
        CallbackQueryID: callback.ID,
    })
}
```

## 消息格式化

### Markdown 格式

```go
func formatInboundInfo(inbound *model.Inbound) string {
    return fmt.Sprintf(
        "*入站信息*\n"+
        "📍 端口: `%d`\n"+
        "📡 协议: `%s`\n"+
        "📊 流量: ↑ %s ↓ %s",
        inbound.Port,
        inbound.Protocol,
        formatBytes(inbound.Up),
        formatBytes(inbound.Down),
    )
}
```

### 发送消息

```go
func (t *Tgbot) sendMessage(chatId int64, text string) {
    _, _ = t.bot.SendMessage(&telego.SendMessageParams{
        ChatID:    telego.ChatID{ID: chatId},
        Text:      text,
        ParseMode: "Markdown",
    })
}
```

## 权限控制

```go
func (t *Tgbot) isAdmin(userId int64) bool {
    return userId == t.chatId
}

func (t *Tgbot) handleAdminCommand(message telego.Message) {
    if !t.isAdmin(message.From.ID) {
        t.sendMessage(message.Chat.ID, "⛔ 无权限执行此操作")
        return
    }
    // 执行管理员操作
}
```

## 通知推送

```go
func (t *Tgbot) SendNotification(text string) error {
    if !t.running {
        return errors.New("bot not running")
    }
    
    _, err := t.bot.SendMessage(&telego.SendMessageParams{
        ChatID:    telego.ChatID{ID: t.chatId},
        Text:      text,
        ParseMode: "Markdown",
    })
    return err
}
```

## 最佳实践

1. **错误处理**: 发送友好的错误消息给用户
2. **权限检查**: 敏感操作验证用户身份
3. **消息长度**: Telegram 单条消息限制 4096 字符
4. **速率限制**: 避免短时间内发送过多消息
5. **回调应答**: 始终调用 `AnswerCallbackQuery` 移除加载动画
