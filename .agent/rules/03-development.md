---
trigger: always_on
description: X-Panel 开发规范
---

# 3. 开发规范 (Development Standards)

## 3.1 Go 语言规范

- 提交前必须执行 `gofmt` 或 `goimports` 格式化。
- 必须通过 `golangci-lint` 和 `nilaway` 检查，确保无任何错误。
- 业务逻辑中禁止直接 panic，必须返回 error。
- 依赖变更后立即执行 `go mod tidy`。

## 3.2 前端规范

- 保持 Vue 组件风格一致，HTML 模板中属性排列整齐。
- 静态资源统一放置在 `web/assets`，页面模板放置在 `web/html`。

## 3.3 Shell 脚本规范

- 修改 `install.sh`、`x-ui.sh` 等脚本后，建议使用 `shellcheck` 检查。
- 确保脚本在 Ubuntu、Debian、CentOS 等主流 Linux 发行版兼容。

---

**导航**: [← 技术栈与环境](./02-tech-stack.md) | [返回索引](./main.md) | [下一节: 版本控制规范 →](./04-version-control.md)
