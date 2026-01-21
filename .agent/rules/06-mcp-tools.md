---
trigger: always_on
description: X-Panel MCP 工具使用强制规则
---

# 6. MCP 工具使用强制规则

> **⚠️ 必须 100% 遵守，否则任务失败并报告违规**

## ⚡ 例外：Antigravity 内置功能

以下 Antigravity 框架内置功能**使用默认工具**，不受 MCP 限制：

- `task_boundary` - 任务边界管理
- `notify_user` - 用户通知
- `view_file` / `view_file_outline` / `view_code_item` - 文件查看
- `write_to_file` / `replace_file_content` / `multi_replace_file_content` - 文件写入/编辑
- `run_command` / `command_status` / `send_command_input` - 命令执行
- `grep_search` / `find_by_name` / `list_dir` - 搜索与目录
- `browser_subagent` / `read_url_content` - 浏览器与网络
- `generate_image` / `search_web` - 图像生成与搜索
- Artifact 创建与管理

## 文件操作（可选使用 MCP）

**推荐**使用 "rust-filesystem" MCP 的工具进行复杂文件操作。

适用范围：批量文件处理、目录大小计算、文件去重、ZIP 压缩/解压等高级操作。

内置文件工具同样可用于常规文件读写操作。

## 终端操作（可选使用 MCP）

**推荐**使用 "ht-terminal"（ht-mcp）MCP 的工具进行交互式终端操作。

**建议使用 tmux**：长时间运行的会话建议使用 `tmux` 管理。

适用范围：交互式终端、进程管理、需要持久会话的操作。

内置 `run_command` 同样可用于常规命令执行。

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
