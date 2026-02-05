# Git Hooks 文档

## Prek Git Hooks 功能说明

本项目的 Git hooks 使用 prek 管理，提供了全面的代码质量检查，确保提交的代码符合项目规范。

### 📋 检查项目

#### ✅ Go 代码检查
- **golangci-lint**: 高级代码质量检查
- **gofmt**: 自动检查Go代码格式
- **go vet**: 静态分析Go代码
- **go mod tidy**: 清理 Go 模块依赖

#### ✅ Shell 脚本检查
- **shfmt**: 自动格式化Shell脚本
- **shellcheck**: 语法和最佳实践检查

#### ✅ Commit Message 检查
- **Conventional Commits**: 验证提交消息格式规范

### 🚀 技术特性

- **Rust 实现**: 高性能、现代化架构
- **YAML 配置**: 标准化配置管理
- **生态兼容**: 兼容 pre-commit hooks 生态
- **并发执行**: 多检查器并行运行
- **自动修复**: Shell脚本格式问题自动修复
- **彩色输出**: 友好的用户界面

### 🔧 工具安装

#### Go 工具
```bash
# 安装 golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 安装 shfmt
go install mvdan.cc/sh/v3/cmd/shfmt@latest
```

#### Shell 工具
```bash
# Ubuntu/Debian
sudo apt install shellcheck

# CentOS/RHEL
sudo yum install ShellCheck

# macOS
brew install shellcheck
```

#### Prek 安装
```bash
curl --proto '=https' --tlsv1.2 -LsSf https://github.com/j178/prek/releases/download/v0.2.23/prek-installer.sh | sh
```

### 🚀 工作流程

1. **开发代码**: 正常编写Go和Shell脚本
2. **添加到暂存区**: `git add <files>`
3. **提交代码**: `git commit`
4. **自动检查**: prek 自动运行配置的 hooks
5. **自动修复**: 
   - Shell脚本格式问题自动修复并重新暂存
   - Go和Shell语法错误需要手动修复
6. **提交成功**: 所有检查通过后完成提交

### 📝 检查详情

#### Shell 脚本自动格式化
- 检测文件: `*.sh`, `*.bash`
- 格式化工具: `shfmt -i 2 -w -s`
- 自动重新暂存格式化后的文件
- 使用2空格缩进，简化语法

#### Shell 语法检查
- 检测常见语法错误
- 检查最佳实践违规
- 阻止有问题的脚本提交
- 提供详细的错误信息

#### Go 代码检查
- golangci-lint 高级检查（阻塞）
- gofmt格式检查（阻塞）
- go vet静态分析（阻塞）
- go mod tidy 依赖清理

#### Commit Message 检查
- 支持类型: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
- 格式要求: `<type>(<scope>): <description>`
- 阻止不符合规范的提交消息

### ⚠️ 注意事项

1. **工具依赖**: 确保安装了所需的检查工具
2. **自动修复**: 只有Shell格式问题会自动修复
3. **语法错误**: Go和Shell语法错误需要手动修复
4. **性能**: 检查过程通常很快，大型项目可能需要几秒钟

### 🐛 故障排除

#### prek 未安装
```bash
curl --proto '=https' --tlsv1.2 -LsSf https://github.com/j178/prek/releases/download/v0.2.23/prek-installer.sh | sh
```

#### shfmt 未安装
```bash
go install mvdan.cc/sh/v3/cmd/shfmt@latest
```

#### shellcheck 未安装
```bash
# Ubuntu/Debian
sudo apt install shellcheck

# CentOS/RHEL
sudo yum install ShellCheck
```

#### golangci-lint 未安装
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

#### 跳过检查（不推荐）
```bash
git commit --no-verify
```

### 📊 示例输出

#### 成功输出
```
golangci-lint............................................................Passed
gofmt....................................................................Passed
go vet...................................................................Passed
shellcheck...............................................................Passed
shfmt....................................................................Passed
Check commit message.....................................................Passed
```

#### 格式修复输出
```
shfmt....................................................................Failed
- hook id: shfmt
- files were modified by this hook
```

#### 错误输出
```
shellcheck...............................................................Failed
- hook id: shellcheck
- exit code: 1

  In script.sh line 10:
  if [ $? -eq 0 ]; then
       ^-- SC2181 (style): Check exit code directly with e.g. 'if mycmd;', not indirectly with $?.
```

### 🔧 配置管理

#### 配置文件位置
`.pre-commit-config.yaml`

#### 添加新 hook
```yaml
- repo: local
  hooks:
    - id: new-hook
      name: New Hook
      entry: command
      language: system
      types: [file-type]
```

#### 排除文件/目录
```yaml
exclude: |
  (?x)^(
    vendor/|
    \.git/|
    node_modules/|
    build/|
    dist/
  )$
```

### 🔄 维护命令

#### 重新安装 hooks
```bash
prek install --install-hooks --overwrite
```

#### 验证配置
```bash
prek validate-config
```

#### 列出所有 hooks
```bash
prek list
```

#### 手动运行所有检查
```bash
prek run --all-files
```

#### 运行特定 hook
```bash
prek run golangci-lint shellcheck
```

### 🎯 性能优势

- **启动速度**: Rust 实现的高性能
- **并发执行**: 支持多检查器并行运行
- **智能缓存**: 避免重复检查
- **增量检查**: 只检查变更的文件

这个基于 prek 的 Git hooks 系统确保了代码质量和一致性，同时提供了更好的性能和开发体验！
