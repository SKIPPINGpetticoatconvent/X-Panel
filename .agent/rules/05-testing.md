---
trigger: always_on
description: X-Panel 测试规范
---

# 5. 测试规范

- 核心业务逻辑（Service 层）必须编写对应的 `_test.go` 单元测试。
- **强制核心验证流程**: 所有 Go 代码修改后，必须依次执行以下命令并确保全部通过：
  1. `go build ./...` (编译检查 - 优先确保语法正确)
  2. `golangci-lint run ./...` (静态分析)
  3. `nilaway -test=false ./...` (空指针检查)
  4. `go test ./...` (单元测试)
- 提交修改前，必须确保以上四个步骤均无报错（0 issues）。
- **TOML 文件验证**: 修改 `.toml` 文件后，必须执行 `taplo fmt --check` 并确保无报错。

---

**导航**: [← 版本控制规范](./04-version-control.md) | [返回索引](./main.md) | [下一节: MCP 工具规则 →](./06-mcp-tools.md)
