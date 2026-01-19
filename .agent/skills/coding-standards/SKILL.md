---
name: coding-standards
description: X-Panel development, technology stack, version control, and testing standards.
---

# 2. 技术栈与环境 (Tech Stack)

- **Backend**: Go (Gin, Gorm, Xray-core)
- **Frontend**: Go Templates + Vue.js (Ant Design Vue)
- **Database**: SQLite
- **Scripts**: Bash
- **Configuration**: TOML (Taplo)

---

# 3. 开发规范 (Development Standards)

## 3.1 Go 语言规范

- 提交前必须执行 `gofmt` 或 `goimports` 格式化。
- **验证要求**: 必须遵守测试规范中的 "强制核心验证流程"。
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

## 分支命名

- 功能开发: `feat/`
- 修复: `fix/`

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
