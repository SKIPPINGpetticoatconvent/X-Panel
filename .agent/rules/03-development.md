---
trigger: always_on
description: X-Panel 开发规范
---

# 3. 开发规范 (Development Standards)

## 3.1 Go 语言规范

- 提交前必须执行 `gofmt` 或 `goimports` 格式化。
- **验证要求**: 必须遵守测试规范中的 "强制核心验证流程" (执行 `.agent/skills/coding-standards/bin/verify`)。
- **运行方式**: 本地启动服务进行调试或验证运行时行为时，默认使用 `go run main.go [args...]`。
- 业务逻辑中禁止直接 panic，必须返回 error。
- 依赖变更后立即执行 `go mod tidy`。

## 3.2 前端规范

- 保持 Vue 组件风格一致，HTML 模板中属性排列整齐。
- 静态资源统一放置在 `web/assets`，页面模板放置在 `web/html`。

## 3.3 Shell 脚本规范

- 修改 `install.sh`、`x-ui.sh` 等脚本后，建议使用 `shellcheck` 检查。
- 确保脚本在 Ubuntu、Debian、CentOS 等主流 Linux 发行版兼容。

## 3.4 TOML 配置文件规范

- 修改 `.toml` 文件后，必须使用 `taplo fmt --check` 进行格式检查。
- 确保配置文件结构清晰，注释准确。

---

**导航**: [← 技术栈与环境](./02-tech-stack.md) | [返回索引](./main.md) | [下一节: 版本控制规范 →](./04-version-control.md)
