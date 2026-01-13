---
trigger: always_on
description: X-Panel 项目全局开发、安全及沟通规范
---

# X-Panel 项目全局规范

## 1. 沟通规范 (Communication Standards)
- **语言要求**: 所有向用户提交的汇报、说明、文档及回复必须使用**中文**。
- **任务更新**: 每次任务执行过程中，需通过 `task_boundary` 和 `notify_user` 保持及时的中文进度反馈。

## 2. 技术栈与环境 (Tech Stack)
- **Backend**: Go (Gin, Gorm, Xray-core).
- **Frontend**: Go Templates + Vue.js (Ant Design Vue).
- **Database**: SQLite.
- **Scripts**: Bash.

## 3. 开发规范 (Development Standards)

### 3.1 Go 语言规范
- **格式化**: 提交前必须执行 `gofmt` 或 `goimports`。
- **Linting**: 尽量通过 `golangci-lint` 检查，消除不必要的 lint 错误。
- **错误处理**: 避免在业务逻辑中直接 panic，应返回 error。
- **模块管理**: 依赖变更需及时执行 `go mod tidy`。

### 3.2 前端规范 (Frontend)
- **风格**: 保持 Vue 组件风格一致，HTML 模板中属性排列整齐。
- **资源**: 静态资源放于 `web/assets`，页面模板放于 `web/html`。

### 3.3 Shell 脚本规范
- **检查**: 修改 shell 脚本 (`install.sh`, `x-ui.sh`) 后，建议通过 `shellcheck` 检查潜在问题。
- **兼容性**: 确保脚本在主流 Linux 发行版 (Ubuntu, Debian, CentOS) 下的兼容性。

## 4. 版本控制规范 (Git & Version Control)
- **Commit Message**: 遵循 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/) 规范。
    - `feat`: 新功能
    - `fix`: 修补 Bug
    - `docs`: 文档修改
    - `style`: 代码格式修改 (不影响代码运行的变动)
    - `refactor`: 重构 (即不是新增功能，也不是修改 bug 的代码变动)
    - `chore`: 构建过程或辅助工具的变动
- **Branch**: 功能开发建议使用 `feat/` 分支，修复使用 `fix/` 分支。

## 5. 测试规范 (Testing)
- **单元测试**: 核心业务逻辑 (Service 层) 需编写对应的 `_test.go`。
- **验证**: 提交修改前，需在本地进行编译 (`go build`) 及基础功能验证。
