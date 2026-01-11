# QA Workflows Specification

## 1. 项目技术栈概述 (Project Overview)

X-Panel 是一个基于 Go 语言的 Xray 面板项目，集成了 Web 管理界面、数据库和 Xray 核心服务。

- **Core Language**: Go (v1.25.5 defined in `go.mod`)
- **Web Framework**: Gin
- **Database**: SQLite (via GORM)
- **Proxy Core**: Xray-core
- **Frontend/Assets**: HTML/JS (Linted by Biome)
- **Container Runtime**: Docker (Strict Requirement)
- **Testing Framework**:
  - Unit/Integration: `testify`, `go test`
  - E2E: Custom Docker-based tests in `tests/e2e`

## 2. QA 工作流职责划分 (Workflow Responsibilities)

建议将 QA 流程拆分为以下模块化 Job，以实现快速反馈和全面覆盖：

| Workflow / Job | 职责 (Responsibility) | 触发条件 (Trigger) | 关键工具 (Tools) |
| :--- | :--- | :--- | :--- |
| **Lint & Static Analysis** | 代码风格检查、静态分析、依赖安全扫描 | PR, Push | `golangci-lint`, `biome`, `govulncheck` |
| **Unit Tests** | 快速验证核心逻辑，无外部依赖 | PR, Push | `go test -short` |
| **Build Verification** | 验证二进制文件和容器镜像能否成功构建 | PR, Push | `go build`, `docker build` |
| **Integration Tests** | 验证数据库交互、API 接口、系统集成 | PR (Merge Queue), Nightly | `go test ./tests/integration/...` |
| **E2E Tests (Docker)** | 在隔离容器环境中验证完整业务流程 | PR (Critical), Release | `go test ./tests/e2e/...`, `docker` |

## 3. 功能需求 (Functional Requirements)

### 3.1 环境一致性
- 所有 Go 相关任务必须使用 `go.mod` 中定义的 Go 版本 (1.25.5)。
- 容器操作必须使用 **Docker**，严禁使用 Podman。

### 3.2 模块化与缓存
- 使用 GitHub Actions Cache 缓存 `~/.cache/go-build` 和 `~/go/pkg/mod`。
- 各个 Job 独立运行，通过 `needs` 关键字管理依赖关系（如 E2E 依赖 Build）。

### 3.3 测试报告
- 测试失败时必须输出详细日志。
- 建议生成 JUnit 格式的测试报告以便在 CI 界面展示。

## 4. Docker 配置需求 (Docker Configuration)

由于项目强制要求使用 Docker，CI 环境需满足：
1.  **Installation**: 确保 Runner 安装了 Docker (Ubuntu-latest 通常预装，但需验证版本)。
2.  **Daemon**: 确保 Docker Daemon 在后台运行。
3.  **Privilege**: E2E 测试可能需要 `--privileged` 模式或特定的用户权限配置。
4.  **Network**: 确保 Docker 容器间网络互通。

## 5. High-Level Pseudocode & TDD Anchors

以下是 GitHub Actions Workflow 的伪代码结构。

```yaml
name: QA Workflow
on: [push, pull_request]

jobs:
  # ------------------------------------------------------------------
  # Job 1: Static Analysis & Linting
  # ------------------------------------------------------------------
  lint:
    name: Lint & Static Analysis
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.5' # TDD Anchor: Verify Go version matches go.mod

      - name: Setup Biome
        uses: biomejs/setup-biome@v2

      - name: Run Biome Lint
        run: biome ci .

      - name: Run GolangCI-Lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest

      - name: Security Scan (Govulncheck)
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...

  # ------------------------------------------------------------------
  # Job 2: Unit & Integration Tests
  # ------------------------------------------------------------------
  test-unit:
    name: Unit Tests
    runs-on: ubuntu-latest
    steps:
      - name: Checkout & Setup Go
        uses: ./.github/actions/setup-go-env # Modular Action

      - name: Run Unit Tests
        # TDD Anchor: Ensure -short flag is used to skip long running tests
        run: go test -v -short -race ./...

  test-integration:
    name: Integration Tests
    needs: lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout & Setup Go
        uses: ./.github/actions/setup-go-env

      - name: Run Integration Tests
        # TDD Anchor: Target specific integration directory
        run: go test -v -coverprofile=coverage.txt ./tests/integration/...

  # ------------------------------------------------------------------
  # Job 3: Build Verification
  # ------------------------------------------------------------------
  build:
    name: Build Binary & Container
    runs-on: ubuntu-latest
    steps:
      - name: Checkout & Setup Go
        uses: ./.github/actions/setup-go-env

      - name: Build Binary
        run: go build -v -o x-ui main.go

      - name: Setup Docker
        run: |
          sudo apt-get update
          sudo apt-get -y install docker.io
          # TDD Anchor: Verify Docker version
          docker version

      - name: Build Container Image
        # TDD Anchor: Use Docker build
        run: docker build -t x-panel:test .

  # ------------------------------------------------------------------
  # Job 4: E2E Tests (Docker)
  # ------------------------------------------------------------------
  test-e2e:
    name: E2E Tests
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout & Setup Go
        uses: ./.github/actions/setup-go-env

      - name: Setup Docker Environment
        run: |
          # Ensure Docker daemon is running
          sudo systemctl start docker
          # TDD Anchor: Check Docker availability
          ls -l /run/user/$(id -u)/podman/podman.sock

      - name: Run E2E Tests
        # TDD Anchor: Execute tests in tests/e2e that depend on Docker
        # Environment variables might be needed for test config
        env:
          TEST_CONTAINER_RUNTIME: docker
        run: go test -v -timeout 20m ./tests/e2e/...
```
