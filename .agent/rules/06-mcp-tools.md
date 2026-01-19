---
trigger: always_on
description: X-Panel MCP 工具使用强制规则
---

# 6. MCP 工具使用强制规则

> **⚠️ 必须 100% 遵守，否则任务失败并报告违规**

## 文件操作

**必须且只能**使用 "rust-filesystem" MCP 的工具。

适用范围：读取、写入、追加、搜索、目录列表、创建/移动/删除文件或目录。

❌ 严禁使用任何内置 file read/write、file system tools 或其他 MCP 的文件工具。

## 终端操作

**必须且只能**使用 "ht-terminal"（ht-mcp）MCP 的工具。

**必须强制使用 screen**：所有终端会话必须使用 `screen`。严禁直接运行 `bash`或其他 shell，必须封装在 `screen` 中。

适用范围：命令执行、Shell 操作、进程管理、交互式终端。

❌ 严禁使用内置 run_command 或其他终端工具。

## 工具调用声明

在思考过程中必须明确声明：

- "严格遵守规则，使用 rust-filesystem 执行文件操作"
- "严格遵守规则，使用 ht-terminal 执行终端命令"

## 路径规范

**必须使用绝对路径**，从项目根目录开始：

```
✅ /home/ub/X-Panel/main.go
✅ /home/ub/X-Panel/install.sh
❌ 严禁使用相对路径或超出项目目录的访问
```

## 安全要求

- 始终遵守白名单限制（`--root` 指定）
- 优先使用只读模式测试
- 禁止不必要的写操作

---

**导航**: [← 测试规范](./05-testing.md) | [返回索引](./main.md) | [下一节: 其他强制要求 →](./07-misc.md)
