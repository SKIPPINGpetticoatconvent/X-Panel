# Shell 脚本 E2E 测试指南

本目录包含用于测试 `install.sh` 和 `x-ui.sh` 的端到端 (E2E) 测试套件。测试使用 Python 的 `pytest` 框架，结合 `docker` 和 `pytest-testinfra` 来模拟真实的 Linux 环境。

## 目录结构

- `Dockerfile`: 用于构建测试用的 Docker 镜像（包含 mocked systemd）。
- `test_installer.py`: 主要的测试脚本，定义了测试用例。
- `.venv/`: (自动生成) Python 虚拟环境。

## 前置要求

在运行测试之前，请确保您的环境已安装以下工具：

1. **Docker**: 用于运行测试容器。
   - 确保 Docker 守护进程正在运行。
   - 当前用户应有权运行 Docker 命令（无需 sudo）。
2. **uv**: 快速的 Python 包和项目管理器。
   - 安装方法: `curl -LsSf https://astral.sh/uv/install.sh | sh` (或参考官方文档)

## 快速开始

### 1. 初始化环境

首次运行时，使用 `uv` 同步依赖环境：

```bash
cd shell_test
# 自动创建环境并安装依赖
uv sync
```

### 2. 运行测试

使用 `uv run` 执行测试，无需手动激活虚拟环境：

```bash
# 运行所有测试
uv run pytest

# 运行所有测试并显示详细输出 (stdout/stderr)
uv run pytest -s

# 运行特定测试文件
uv run pytest test_installer.py
```

## 测试原理

1. **构建镜像**: 测试开始时，`pytest` 会根据 `Dockerfile` 构建一个名为 `x-ui-test-image` 的本地镜像。该镜像基于 Ubuntu 22.04，并预置了 `systemctl`、`service` 等命令的 mock 版本，以便在容器中运行依赖 systemd 的安装脚本。
2. **启动容器**: 每个测试模块会启动一个临时的 Docker 容器。
3. **文件注入**: `install.sh` 和 `x-ui.sh` 会被复制到容器中。
4. **执行验证**: `test_installer.py` 控制容器执行安装脚本，并通过检查文件存在性、命令输出等方式验证安装是否成功。
5. **清理**: 测试结束后，临时容器会被自动删除。

## 常见问题

- **Docker 权限错误**: 如果提示 `permission denied`，请尝试将用户加入 docker 组 (`sudo usermod -aG docker $USER`) 或使用 `sudo` 运行 (不推荐)。
- **网络问题**: 安装脚本需要从 GitHub 下载资源。如果测试容器网络受限，测试可能会失败。
