---
name: mcp-guidelines
description: Strict rules for using MCP tools in X-Panel.
---

# 6. MCP 工具使用强制规则

> **⚠️ 必须 100% 遵守，否则任务失败并报告违规**

## 文件操作

**必须且只能**使用 "rust-filesystem" MCP 的工具。

适用范围：读取、写入、追加、搜索、目录列表、创建/移动/删除文件或目录。

❌ 严禁使用任何内置 file read/write、file system tools 或其他 MCP 的文件工具。

## 终端操作

**必须且只能**使用 "ht-terminal"（ht-mcp）MCP 的工具。

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
