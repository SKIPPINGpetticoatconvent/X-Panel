# 分支保护策略配置指南

本文档说明如何为 X-Panel 项目配置分支保护策略，以确保代码质量和安全性。

## GitHub 分支保护规则配置

### 主分支 (main) 保护规则

在 GitHub 仓库设置中，导航到 **Settings > Branches > Branch protection rules**，添加以下规则：

#### 基本设置
- **Branch name pattern**: `main`
- **Require a pull request before merging**
  - ☑️ Require approvals: **1** (至少需要1个审批)
  - ☑️ Dismiss stale pull request approvals when new commits are pushed
  - ☑️ Require review from Code Owners (如果有 CODEOWNERS 文件)
  - ☑️ Restrict who can dismiss pull request reviews: 选择仓库管理员或维护者

#### 状态检查
- ☑️ Require status checks to pass before merging
- ☑️ Require branches to be up to date before merging
- **Status checks that are required**:
  - `build-linux` (Linux构建)
  - `build-windows` (Windows构建)
  - `update-deps` (依赖更新检查)
  - `vulnerability-check` (安全漏洞检查)
  - `test` (如果有单独的测试工作流)

#### 其他限制
- ☑️ Include administrators
- ☑️ Restrict pushes that create matching branches
- ☐ Allow force pushes (保持未选中)
- ☐ Allow deletions (保持未选中)

### 依赖更新分支的特殊要求

对于带有 `dependencies` 标签的 PR，需要额外审查：

#### 手动审查流程
1. 依赖更新 PR 创建后，自动运行测试
2. 维护者需要手动验证：
   - 所有测试通过
   - 没有安全漏洞报告
   - 更新日志合理
   - 功能没有回归
3. 只有在额外审查通过后才能合并

#### 依赖更新PR的识别
- PR 标题包含: `chore: update Go dependencies`
- PR 包含标签: `dependencies` 和 `automated`
- PR 来源分支: `deps/go-updates`

## 配置步骤

### 1. 启用分支保护

1. 进入仓库 **Settings** 标签
2. 点击左侧菜单 **Branches**
3. 点击 **Add rule** 按钮
4. 配置上述规则

### 2. 验证配置

创建测试 PR 来验证保护规则是否正常工作：

```bash
# 创建测试分支
git checkout -b test-protection
echo "test" > test.txt
git add test.txt
git commit -m "test: add test file"
git push origin test-protection

# 在 GitHub 上创建 PR
# 验证是否需要审批和状态检查
```

### 3. 依赖更新的额外检查

对于依赖更新，确保：

- 自动化测试覆盖所有关键功能
- 有回滚计划（可以回退到上一个稳定版本）
- 监控生产环境是否有异常

## 安全考虑

- **最小权限原则**: 只给必要的协作者推送权限
- **定期审查**: 每季度审查分支保护规则
- **紧急情况**: 为紧急修复保留管理员覆盖选项
- **审计日志**: 监控分支保护规则的绕过情况

## 故障排除

### 常见问题

**Q: PR 无法合并，即使所有检查都通过了**  
A: 检查是否满足最低审批数量要求

**Q: 依赖更新 PR 被阻止**  
A: 确保 PR 包含必要的标签和通过了所有状态检查

**Q: 需要紧急部署但分支保护阻止了**  
A: 管理员可以临时覆盖，但应在事后记录原因

### 联系方式

如果在配置过程中遇到问题，请参考：
- [GitHub 分支保护文档](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule)
- 项目维护者