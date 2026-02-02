---
trigger: always_on
description: X-Panel 测试规范
---

# 5. 测试规范

- 核心业务逻辑（Service 层）必须编写对应的 `_test.go` 单元测试。
- **强制核心验证流程**: 所有 Go 代码修改后，必须在 `screen` 会话中执行以下命令验证：
  `go build ./... && go test ./... && nilaway -test=false ./...`
- 提交修改前，必须确保上述检查全部通过。
- **TOML 文件验证**: 修改 `.toml` 文件后，必须执行 `taplo fmt --check` 并确保无报错。
- **Shell 脚本验证**: 修改或新建 `.sh` 文件后，必须执行 `shfmt -i 2 -w -s .` 格式化，并通过 `shellcheck` 检查。
- **Makefile 验证**: 修改或新建 `Makefile` 后，必须执行 `checkmake Makefile` 并确保无报错。
- **强制 E2E 验证**: 发布新版本或修改安装脚本前，必须执行 `make e2e` 并通过 Docker 容器内的端到端安装测试。

---

**导航**: [← 版本控制规范](./04-version-control.md) | [返回索引](./main.md) | [下一节: MCP 工具规则 →](./06-mcp-tools.md)
