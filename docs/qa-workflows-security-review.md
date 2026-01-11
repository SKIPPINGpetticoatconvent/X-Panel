# QA 工作流安全审查报告

**日期**: 2024-05-23
**审查对象**: `.github/workflows/` 下的 QA 工作流文件
**审查者**: Security Reviewer

## 1. 摘要

本次审查覆盖了新创建的 6 个 QA 工作流文件。整体而言，工作流结构清晰，使用了官方或知名的 Actions，未发现严重的硬编码 Secrets 或注入漏洞。但存在权限配置缺失、供应链安全加固空间等问题。

## 2. 详细发现

### 2.1 权限控制 (Permissions)
- **风险等级**: 中
- **问题描述**: 所有 6 个工作流文件均未配置 `permissions` 字段。
- **影响**: 默认情况下，`GITHUB_TOKEN` 拥有读写权限。如果工作流被攻破，攻击者可能利用此 Token 修改仓库代码或发布恶意包。
- **涉及文件**: 所有 QA 工作流文件。
- **建议**: 遵循最小权限原则，在每个工作流的顶级添加 `permissions` 配置。
  ```yaml
  # 示例：对于只需要读取代码的工作流
  permissions:
    contents: read
  ```

### 2.2 Secrets 管理
- **风险等级**: 低
- **问题描述**:
  - 未发现硬编码 Secrets。
  - `qa-unit-test.yml` 和 `qa-integration-test.yml` 使用了 `codecov/codecov-action`，但未显式传递 `CODECOV_TOKEN`。
- **影响**: 对于公共仓库，Codecov 可能不需要 Token，但对于私有仓库或为了避免速率限制，建议配置 Token。
- **建议**: 确认是否需要配置 `CODECOV_TOKEN` Secret。

### 2.3 供应链安全
- **风险等级**: 低
- **问题描述**: 所有 Actions 均使用主版本号（如 `@v4`）而非 SHA 哈希。
- **影响**: 如果 Action 的某个 Tag 被恶意篡改，工作流可能执行恶意代码。
- **建议**: 在高安全要求的场景下，建议使用 SHA 哈希锁定 Action 版本，并使用 Dependabot 进行更新。

### 2.4 工具安装与环境
- **风险等级**: 低
- **问题描述**:
  - `qa-security.yml` 中安装 Trivy 使用了 `wget ... | sudo apt-key add -` 的方式。虽然这是官方文档推荐的方式，但直接管道执行网络内容存在一定风险。
  - Go 版本 `1.25.5` 在多个文件中硬编码，维护成本较高。
- **建议**:
  - 考虑使用校验和验证下载的脚本/密钥。
  - 考虑将 Go 版本提取为环境变量或使用统一的配置。

### 2.5 Podman 安全
- **风险等级**: 安全
- **观察**:
  - 工作流正确安装并使用了 Podman。
  - Podman 默认以非 root 方式运行容器，符合安全最佳实践。
  - 镜像构建基于仓库内的 `Dockerfile`，来源可控。

## 3. 修复建议清单

### 3.1 添加权限配置 (High Priority)

建议对所有工作流应用以下权限配置：

**qa-lint.yml, qa-unit-test.yml, qa-integration-test.yml, qa-e2e-test.yml, qa-security.yml**:
```yaml
permissions:
  contents: read
  # 如果需要上传测试报告或 artifacts，可能需要 write 权限，但通常 actions/upload-artifact 不需要 write 权限
```

**qa-build.yml**:
```yaml
permissions:
  contents: read
```

### 3.2 优化 Codecov 配置 (Medium Priority)

在 `qa-unit-test.yml` 和 `qa-integration-test.yml` 中：
```yaml
- name: Upload coverage reports
  uses: codecov/codecov-action@v4
  with:
    token: ${{ secrets.CODECOV_TOKEN }} # 建议添加
    # ...
```

### 3.3 固化 Trivy 安装 (Low Priority)

在 `qa-security.yml` 中，虽然目前方式可接受，但建议关注 Trivy 官方 Action 是否能替代手动安装，以简化维护。

## 4. 结论

QA 工作流整体安全性良好，主要改进点在于显式声明权限以通过最小权限原则加固系统。建议优先实施权限配置的修复。
