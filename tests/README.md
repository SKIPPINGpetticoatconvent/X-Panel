# X-UI 测试环境

本测试环境用于验证 X-UI 项目的各项功能。

## 目录结构

```
tests/
├── docker/                      # Docker测试环境
│   ├── Dockerfile.ubuntu       # Ubuntu测试镜像
│   ├── Dockerfile.centos       # CentOS测试镜像
│   ├── ubuntu-test-entrypoint.sh # Ubuntu测试入口脚本
│   └── centos-test-entrypoint.sh # CentOS测试入口脚本
├── run_podman_tests.sh         # Podman测试运行脚本
├── run_test.go                 # 主测试文件
├── run_oneclick_test.go        # 一键安装测试
├── run_oneclick.go             # 一键安装逻辑
├── test_oneclick_menu.go       # 一键安装菜单测试
└── README.md                   # 本文档
```

## 容器化测试环境

### Ubuntu测试环境 (`Dockerfile.ubuntu`)
- 基于 Ubuntu 22.04
- 安装 Go 1.21.5
- 配置 SSH 服务 (端口 2222)
- 安装必要系统工具
- 设置测试环境变量

### CentOS测试环境 (`Dockerfile.centos`)
- 基于 CentOS 7
- 安装 Go 1.21.5
- 配置 SSH 服务 (端口 2222)
- 安装必要系统工具
- 设置测试环境变量

## 自动化测试脚本 (`run_podman_tests.sh`)

- **多平台支持**: 自动构建并测试 Ubuntu 和 CentOS 环境
- **灵活运行**: 支持运行所有测试或特定测试
- **特权模式**: 支持在特权模式下运行以测试 systemd 服务
- **清理功能**: 提供完整的清理功能
- **结果记录**: 自动保存测试结果到 `test-results` 目录

## 使用方法

### 快速开始

```bash
# 查看帮助信息
./tests/run_podman_tests.sh --help

# 列出所有可用的测试
./tests/run_podman_tests.sh --list

# 运行所有测试
./tests/run_podman_tests.sh

# 使用特权模式运行测试（用于测试systemd服务）
./tests/run_podman_tests.sh --privileged

# 只运行Ubuntu测试
./tests/run_podman_tests.sh --ubuntu-only

# 只运行CentOS测试
./tests/run_podman_tests.sh --centos-only

# 清理所有测试容器和镜像
./tests/run_podman_tests.sh --cleanup
```

### 本地测试

```bash
# 运行主测试
go test -v ./tests/

# 运行一键安装测试
go test -v ./tests/ -run TestOneClick

# 运行性能测试
go test -bench=. ./tests/
```

## 支持的测试

### 单元测试

- 主功能测试
- 一键安装测试
- 菜单交互测试

### 集成测试

- Docker容器测试
- 多操作系统环境测试

## 测试策略

### 1. 容器隔离性

- 每个测试运行在独立的容器中
- 测试之间互不影响
- 测试完成后自动清理

### 2. 特权模式

某些测试需要特权模式来运行 systemd 服务：

```bash
./tests/run_podman_tests.sh --privileged
```

## 故障排除

### 常见问题

1. **Podman未安装**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install podman
   
   # CentOS/RHEL
   sudo yum install podman
   ```

2. **Go版本不兼容**
   - 确保使用 Go 1.21 或更高版本
   ```bash
   go version
   ```

3. **容器构建失败**
   - 检查Docker/Podman网络连接
   - 确认有足够的磁盘空间

4. **测试执行失败**
   - 查看测试日志: `tests/test-results/`

### 调试模式

```bash
# 启用详细输出
./tests/run_podman_tests.sh -v TestSpecificTest

# 进入调试容器
podman run -it --rm x-ui-test-ubuntu /bin/bash
```

## 贡献指南

1. 添加新的测试用例时，请遵循现有的命名约定
2. 确保新测试在所有支持的操作系统上都能运行
3. 更新此文档以反映新功能
4. 所有测试都应该通过 CI/CD 流水线

## 许可证

本测试环境遵循与 X-UI 主项目相同的许可证。