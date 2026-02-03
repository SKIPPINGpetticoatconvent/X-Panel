---
trigger: always_on
description: X-Panel 项目全局开发、安全及沟通规范 - 索引文件
---

# X-Panel 项目全局规范

本规范已模块化拆分，请点击下方链接查看各部分详细内容：

| 模块 | 说明 |
|------|------|
| [1. 沟通规范](./01-communication.md) | 中文沟通、进度反馈要求 |
| [2. 技术栈与环境](./02-tech-stack.md) | Go、Vue、SQLite、Bash 技术栈说明 |
| [3. 开发规范](./03-development.md) | Go/前端/Shell/TOML 编码标准 |
| [4. 版本控制规范](./04-version-control.md) | Git Hooks、Commit Message、CHANGELOG |
| [5. 测试规范](./05-testing.md) | 单元测试与构建验证 |
| [6. MCP 工具规则](./06-mcp-tools.md) | 文件/终端操作强制规则 |
| [7. 其他强制要求](./07-misc.md) | 中文输出与违规处理 |

## 常用 Makefile 命令

| 命令 | 说明 |
|------|------|
| `make build` | 编译 x-ui 二进制 |
| `make test` | 运行单元测试 |
| `make lint` | 运行 golangci-lint |
| `make fmt` | 格式化代码 |
| `make hooks` | 安装 git hooks |
| `make changelog` | 生成 CHANGELOG.md |

---

> **快速导航**: 所有模块文件均位于 `.windsurf/rules/` 目录下。
