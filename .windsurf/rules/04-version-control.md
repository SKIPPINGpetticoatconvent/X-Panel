---
trigger: always_on
description: X-Panel 版本控制规范
---

# 4. 版本控制规范

## Commit Message 规范

必须遵循 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)：

| 类型 | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | 修补 Bug |
| `docs` | 文档修改 |
| `style` | 代码格式修改（无运行影响） |
| `refactor` | 重构（非新增功能、非修 Bug） |
| `chore` | 构建/辅助工具变动 |

## Git Hooks (自动检查)

项目已配置 Git Hooks，位于 `.githooks/` 目录：

- **pre-commit**: 提交前自动检查 `gofmt`、`go vet`、`golangci-lint`
- **commit-msg**: 验证 Commit Message 是否符合 Conventional Commits 格式

```bash
# 首次克隆后安装 hooks
make hooks
```

## CHANGELOG 自动生成

使用 `git-cliff` 基于 Conventional Commits 自动生成变更日志：

```bash
# 生成/更新 CHANGELOG.md
make changelog
```

## 分支命名

- 功能开发: `feat/`
- 修复: `fix/`

---

**导航**: [← 开发规范](./03-development.md) | [返回索引](./main.md) | [下一节: 测试规范 →](./05-testing.md)
