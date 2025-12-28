# Telegram Bot 功能测试

本目录包含 X-Panel Telegram Bot 的功能测试套件。

## 测试覆盖范围

### 1. 命令响应测试 (TestCommandResponse)
- 测试各种命令的解析和处理
- 验证命令参数提取
- 检查不同命令类型的响应

### 2. Webhook 处理测试 (TestWebhookHandling)
- 模拟 Telegram Webhook 请求
- 验证更新消息的正确解析
- 测试消息路由和处理

### 3. 权限验证测试 (TestPermissionValidation)
- 管理员 ID 验证
- 非管理员用户的访问控制
- 权限级别的功能隔离

## 测试文件结构

```
tests/telegram/
├── tgbot_functional_test.go  # 主要功能测试
└── README.md                 # 测试文档
```

## 运行测试

### 运行所有测试
```bash
go test ./tests/telegram/ -v
```

### 运行基准测试
```bash
go test ./tests/telegram/ -bench=.
```

### 生成覆盖率报告
```bash
go test ./tests/telegram/ -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 测试用例说明

### TestCommandParsing
测试 Telegram 命令解析功能：
- 简单命令（如 `/start`）
- 带参数的命令（如 `/usage email@example.com`）
- 多参数命令
- 特殊字符处理

### TestBotTokenValidation
验证 Bot Token 格式：
- 有效的 Token 格式检查
- 无效 Token 的拒绝
- 边界情况处理

### TestAdminIdParsing
管理员 ID 解析测试：
- 单个管理员 ID
- 多个管理员 ID（逗号分隔）
- 带空格的 ID 列表
- 无效 ID 的处理

### TestInlineKeyboard & TestReplyKeyboard
键盘创建功能测试：
- 内联键盘按钮布局
- 回复键盘配置
- 键盘属性验证

### TestMessageFormatting
消息格式化测试：
- 长消息分页逻辑
- 消息长度限制处理
- 空消息处理

## 基准测试

- `BenchmarkCommandParsing`: 命令解析性能测试
- `BenchmarkTokenValidation`: Token 验证性能测试
- `BenchmarkAdminIdParsing`: 管理员 ID 解析性能测试

## 注意事项

1. 这些测试主要验证 Telegram Bot 的外围功能和工具函数
2. 核心业务逻辑测试需要与实际的服务实例集成
3. 网络相关的测试使用模拟对象，避免依赖外部服务
4. 所有测试都是单元测试级别，不涉及实际的 Telegram API 调用

## 扩展测试

如需添加更多测试，可以：

1. 集成测试：测试与实际 Telegram API 的交互
2. 端到端测试：完整的用户交互流程
3. 压力测试：高并发情况下的性能表现
4. 安全测试：输入验证和权限控制的深入测试