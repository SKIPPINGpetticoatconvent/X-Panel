# 项目指令 (Project Instructions)

## 开发环境设置

### 环境要求

- Go 1.21 或更高版本
- Node.js 18+ (用于前端工具)
- SQLite3

### 克隆项目

```bash
git clone https://github.com/SKIPPINGpetticoatconvent/X-Panel.git
cd X-Panel
```

### 安装依赖

```bash
# Go 依赖
go mod download

# 前端工具 (可选)
npm install
```

## 常用开发指令

### 运行应用

```bash
# 开发模式运行
go run main.go

# 指定配置文件
go run main.go -config /path/to/config
```

### 编译构建

```bash
# 编译当前平台
go build -o x-ui main.go

# 交叉编译 Linux amd64
GOOS=linux GOARCH=amd64 go build -o x-ui-linux-amd64 main.go

# 使用构建脚本
bash build/build.sh
```

### 代码格式化

```bash
# Go 代码格式化
dprint fmt

# 或使用 gofmt
gofmt -w .

# 前端代码格式化 (Biome)
npx biome format --write .
```

## 测试指令

### 运行单元测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./web/service/...

# 带覆盖率报告
go test -cover ./...

# 生成覆盖率 HTML 报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 运行 E2E 测试

```bash
# 编译测试
go test -c -o e2e.test ./tests/e2e

# 运行 E2E 测试
./e2e.test

# 使用环境变量配置
source .env.e2e && go test ./tests/e2e/...
```

## 安装与部署

### 一键安装脚本

```bash
# 在线安装最新版本
bash <(curl -Ls https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/main/install.sh)

# 安装指定版本
VERSION=v25.10.25 bash <(curl -Ls https://raw.githubusercontent.com/SKIPPINGpetticoatconvent/X-Panel/$VERSION/install.sh) $VERSION
```

### 管理菜单脚本

安装后可使用 `x-ui` 命令进入管理菜单：

```bash
x-ui
```

**常用菜单选项**:

| 选项 | 功能 |
|------|------|
| 1 | 安装 X-UI |
| 2 | 更新 X-UI |
| 3 | 卸载 X-UI |
| 5 | 启动服务 |
| 6 | 停止服务 |
| 7 | 重启服务 |
| 8 | 查看状态 |
| 14 | 查看面板日志 |
| 18 | 申请 SSL 证书 |
| 22 | 放行面板端口 |
| 26 | 申请 IP 证书 |

### Docker 部署

```bash
# 拉取镜像
docker pull ghcr.io/skippingpetticoatconvent/x-panel:latest

# 运行容器
docker run -d \
  --name x-ui \
  -p 54321:54321 \
  -v /etc/x-ui:/etc/x-ui \
  ghcr.io/skippingpetticoatconvent/x-panel:latest
```

## 服务管理

### systemd 命令

```bash
# 启动服务
sudo systemctl start x-ui

# 停止服务
sudo systemctl stop x-ui

# 重启服务
sudo systemctl restart x-ui

# 查看服务状态
sudo systemctl status x-ui

# 查看服务日志
journalctl -u x-ui -f

# 设置开机自启
sudo systemctl enable x-ui
```

## 调试技巧

### 查看日志

```bash
# 实时查看面板日志
journalctl -u x-ui.service -f

# 查看最近 100 行日志
journalctl -u x-ui.service -n 100

# 查看 Xray 核心日志
tail -f /usr/local/x-ui/bin/xray.log
```

### 数据库操作

```bash
# SQLite 交互模式
sqlite3 /etc/x-ui/x-ui.db

# 常用查询
# 查看所有入站规则
SELECT * FROM inbounds;

# 查看用户信息
SELECT * FROM users;
```

## 开发工具配置

### VSCode 推荐配置

`.vscode/settings.json`:
```json
{
  "go.formatTool": "gofmt",
  "go.lintTool": "golangci-lint",
  "editor.formatOnSave": true,
  "[go]": {
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  }
}
```

### 推荐 VSCode 插件

- Go (golang.go)
- Biome
- GitLens
