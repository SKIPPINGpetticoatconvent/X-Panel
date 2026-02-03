---
trigger: always_on
description: X-Panel 测试规范 - TDD 驱动
---

# 5. 测试规范

> **⚡ 技能引用**: 详细的测试模式、代码示例请使用 `/tdd-workflow` skill。

## 核心原则

所有开发必须遵循 **测试驱动开发 (TDD)** 原则：先写测试，再实现功能。

## 强制验证流程

### Go 代码（必须通过）
```bash
go build ./... && go test -race -short ./... && golangci-lint run --timeout=10m && nilaway -test=false ./...
```

### 其他文件类型
| 文件类型 | 验证命令 |
|---------|---------|
| `.toml` | `taplo fmt --check` |
| `.sh` | `shfmt -i 2 -w -s .` && `shellcheck` |
| `Makefile` | `checkmake Makefile` |

### E2E 测试（发布/脚本修改前）
```bash
make e2e
```

## 测试覆盖要求

- **Service 层**: 必须编写 `_test.go` 单元测试
- **API 端点**: 使用 `httptest` 进行接口测试
- **安装脚本**: Docker 容器内 E2E 测试

## 激活 TDD Skill

在以下场景**必须**调用 `/tdd-workflow` skill 获取详细指导：
- 编写新功能或特性
- 修复 Bug
- 重构代码
- 添加 API 端点
- 修改 Service 层逻辑

---

**导航**: [← 版本控制规范](./04-version-control.md) | [返回索引](./main.md) | [下一节: MCP 工具规则 →](./06-mcp-tools.md)
