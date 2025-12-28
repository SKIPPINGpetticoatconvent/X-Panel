# E2E 测试文档

本文档说明如何运行 X-Panel 项目的端到端（E2E）测试。

## 前置条件

在运行 E2E 测试之前，请确保您的系统已安装以下软件：

1. **Go** (版本 1.18 或更高)
   - 安装包：https://golang.org/dl/
   - 验证安装：`go version`

2. **Podman** (版本 3.0 或更高)
   - Windows/macOS：https://podman.io/getting-started/installation
   - Linux：使用包管理器安装（如 `sudo apt install podman` 或 `sudo yum install podman`）
   - 验证安装：`podman version`

## 运行测试

在项目根目录执行以下命令运行 E2E 测试：

```bash
go test -v ./tests/e2e/...
```

### 测试流程

测试将执行以下步骤：

1. **清理环境**：删除可能存在的旧测试容器
2. **构建镜像**：使用当前 Dockerfile 构建 Docker 镜像 `x-panel-e2e:latest`
3. **启动容器**：在后台启动 X-Panel 容器，映射端口 13688
4. **健康检查**：等待服务启动并响应 HTTP 请求（最多等待 30 秒）
5. **功能验证**：验证 Web 面板是否可以正常访问
6. **清理环境**：测试完成后自动清理测试容器

### 预期输出

成功的测试应该输出类似以下内容：

```
=== RUN   TestPodmanE2E
=== RUN   TestPodmanE2E
    podman_test.go:20: Building Docker image: x-panel-e2e:latest...
    podman_test.go:26: Starting container: x-panel-e2e-container...
    podman_test.go:38: Waiting for service to be ready at http://localhost:13688...
    podman_test.go:51: Service is ready!
    podman_test.go:56: E2E Test Passed Successfully!
--- PASS: TestPodmanE2E (35.42s)
PASS
ok      github.com/your-org/x-panel/tests/e2e        35.424s
```

## 故障排除

### 常见问题

1. **端口冲突**
   - 如果端口 13688 已被占用，测试将失败
   - 解决方案：停止占用该端口的服务或修改测试配置

2. **Podman 权限问题**
   - 在 Linux 上，可能需要 sudo 权限运行 Podman
   - 解决方案：确保用户有 Podman 运行权限或使用 `sudo`

3. **网络问题**
   - 如果无法下载 Docker 镜像或访问外网，测试可能超时
   - 解决方案：检查网络连接和防火墙设置

4. **Go 模块问题**
   - 如果出现 Go 模块相关错误
   - 解决方案：在项目根目录运行 `go mod tidy`

### 手动清理

如果测试异常退出，可以手动清理测试环境：

```bash
# 删除测试容器
podman rm -f x-panel-e2e-container

# 删除测试镜像
podman rmi x-panel-e2e:latest
```

## 测试配置

可以通过修改 `tests/e2e/podman_test.go` 中的常量来调整测试参数：

- `imageName`: 测试镜像名称
- `containerName`: 测试容器名称
- `hostPort`: 主机端口映射
- `maxRetries`: 最大重试次数
- `retryInterval`: 重试间隔

## 注意事项

- E2E 测试需要较长时间（通常 30-60 秒）
- 测试过程中会消耗网络带宽下载依赖
- 请确保有足够的磁盘空间用于存储 Docker 镜像
- 测试完成后会自动清理容器，但镜像会保留以供后续使用