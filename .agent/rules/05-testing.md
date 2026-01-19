---
trigger: always_on
description: X-Panel 测试规范
---

# 5. 测试规范

- 核心业务逻辑（Service 层）必须编写对应的 `_test.go` 单元测试。
- **强制核心验证流程**: 所有 Go 代码修改后，必须执行以下命令验证：
  1. `.agent/skills/coding-standards/bin/verify`
- 该脚本会自动并行执行 `gofmt`、`go build`、`golangci-lint`、`nilaway` 和 `go test`。
- 提交修改前，必须确保脚本执行结果为 "All checks passed"（0 output）。
- **TOML 文件验证**: 修改 `.toml` 文件后，必须执行 `taplo fmt --check` 并确保无报错。

---

**导航**: [← 版本控制规范](./04-version-control.md) | [返回索引](./main.md) | [下一节: MCP 工具规则 →](./06-mcp-tools.md)
