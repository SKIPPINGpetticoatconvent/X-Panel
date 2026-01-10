# X-Panel 测试套件

本文档介绍了X-Panel项目的完整测试套件，包括新添加的测试类型和使用方法。

## 📋 测试概览

X-Panel项目现在包含以下测试类型：

### 🆕 新增测试类型

1. **Web界面功能测试** (`web/controller/web_interface_test.go`)
   - 用户认证和授权
   - 面板设置管理
   - 入站配置操作
   - 数据验证和边界条件

2. **API接口测试** (`web/controller/api_interface_test.go`)
   - 响应格式验证
   - 权限验证
   - 数据验证
   - 安全头和速率限制
   - 错误处理

3. **数据库测试** (`web/service/database_test.go`)
   - 连接池管理
   - 事务处理
   - CRUD操作
   - 并发安全
   - 数据迁移

4. **Xray核心集成测试** (`web/service/xray_integration_test.go`)
   - 配置生成
   - 进程管理
   - 流量统计
   - 动态策略生成
   - 崩溃检测

### 现有测试类型

5. **安全测试** (`tests/security_test.go`)
   - SQL注入防护
   - XSS攻击防护
   - CSRF保护验证
   - 输入验证测试
   - 安全头检查

6. **性能稳定性测试** (`tests/performance_stability_test.go`)
7. **端到端测试** (`tests/e2e/`)
8. **集成测试** (`web/service/*_test.go`)

## 🚀 快速开始

### 运行所有测试

```bash
# 运行所有测试
go test -v ./...

# 运行特定测试包
go test -v ./web/controller/
go test -v ./web/service/
go test -v ./tests/
```

### 生成覆盖率报告

```bash
# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 查看覆盖率统计
go tool cover -func=coverage.out
```

### 运行基准测试

```bash
# 运行所有基准测试
go test -bench=. -benchmem

# 运行特定基准测试
go test -bench=BenchmarkDatabase_CreateInbound -benchmem
go test -bench=BenchmarkXrayService_GetXrayConfig -benchmem
```

### 并发安全测试

```bash
# 运行并发安全测试
go test -race -v ./...
```

## 📁 测试文件结构

```
tests/
├── README.md                          # 本文档
├── security_test.go                   # 安全测试
├── integration/
│   └── comprehensive_test.go         # 综合测试运行器
├── performance_stability_test.go      # 性能稳定性测试
├── e2e/                              # 端到端测试
│   ├── podman_test.go
│   └── README.md
└── tools/                            # 测试工具
    ├── run_oneclick.go
    └── test_oneclick_menu.go

web/
├── controller/
│   ├── web_interface_test.go         # Web界面功能测试
│   └── api_interface_test.go         # API接口测试
└── service/
    ├── database_test.go              # 数据库测试
    ├── xray_integration_test.go      # Xray核心集成测试
    └── *_test.go                     # 其他集成测试
```

## 🧪 详细测试说明

### 安全测试

**测试文件**: `tests/security_test.go`

**测试内容**:

- `TestSQLInjection` - SQL注入攻击防护测试
  - 备注字段SQL注入测试
  - 邮箱字段SQL注入测试
- `TestXSS` - 跨站脚本攻击防护测试
  - 备注字段XSS payload测试
  - HTML响应转义测试
- `TestCSRF` - 跨站请求伪造防护测试
  - 缺少CSRF令牌的请求测试
  - Referer头检查测试
- `TestInputValidation` - 输入验证测试
  - 端口号验证
  - 协议验证
  - 备注长度验证
  - 流量限制验证
  - JSON格式验证
- `TestSecurityHeaders` - 安全头测试
  - X-Content-Type-Options
  - X-Frame-Options
  - X-XSS-Protection
  - Content-Security-Policy

**运行方法**:

```bash
go test -v ./tests/ -run TestSQLInjection
go test -v ./tests/ -run TestXSS
go test -v ./tests/ -run TestCSRF
go test -v ./tests/ -run TestInputValidation
go test -v ./tests/ -run TestSecurityHeaders
```

### Web界面功能测试

**测试文件**: `web/controller/web_interface_test.go`

**测试内容**:

- `TestInboundController_GetInbounds` - 获取入站列表
- `TestInboundController_AddInbound` - 添加入站配置
- `TestInboundController_ValidateInboundData` - 入站数据验证
- `TestSettingController_GetAllSetting` - 获取面板设置
- `TestSettingController_UpdateUser` - 更新用户信息
- `TestBaseController_CheckLogin` - 登录状态检查
- `TestProtocolValidation` - 协议验证

**运行方法**:

```bash
go test -v ./web/controller/ -run TestInbound
go test -v ./web/controller/ -run TestSetting
go test -v ./web/controller/ -run TestBase
```

### API接口测试

**测试文件**: `web/controller/api_interface_test.go`

**测试内容**:

- `TestInboundAPI_ResponseFormat` - API响应格式验证
- `TestInboundAPI_DataValidation` - API数据验证
- `TestInboundAPI_PermissionValidation` - API权限验证
- `TestSettingAPI_Configuration` - 设置API配置
- `TestAPI_SecurityHeaders` - API安全头测试
- `TestAPI_RateLimiting` - API速率限制测试
- `TestAPI_JSONResponseFormat` - JSON响应格式测试
- `TestAPI_ContentTypeValidation` - 内容类型验证

**运行方法**:

```bash
go test -v ./web/controller/ -run TestInboundAPI
go test -v ./web/controller/ -run TestSettingAPI
go test -v ./web/controller/ -run TestAPI
```

### 数据库测试

**测试文件**: `web/service/database_test.go`

**测试内容**:

- `TestUserService_CreateUser` - 用户创建
- `TestUserService_GetUserByUsername` - 用户查询
- `TestUserService_UpdateUser` - 用户更新
- `TestInboundService_CreateInbound` - 入站创建
- `TestInboundService_GetInbounds` - 入站查询
- `TestInboundService_UpdateInbound` - 入站更新
- `TestInboundService_DeleteInbound` - 入站删除
- `TestSettingService_CreateSetting` - 设置创建
- `TestSettingService_GetSetting` - 设置查询
- `TestDatabase_Transaction` - 数据库事务
- `TestDatabase_Concurrency` - 并发操作

**运行方法**:

```bash
go test -v ./web/service/ -run TestUserService
go test -v ./web/service/ -run TestInboundService
go test -v ./web/service/ -run TestSettingService
go test -v ./web/service/ -run TestDatabase
```

### Xray核心集成测试

**测试文件**: `web/service/xray_integration_test.go`

**测试内容**:

- `TestXrayService_GetXrayConfig` - Xray配置生成
- `TestXrayService_StartStopXray` - Xray进程管理
- `TestXrayService_GetXrayTraffic` - 流量获取
- `TestXrayService_PolicyGeneration` - 策略生成
- `TestXrayService_ConfigValidation` - 配置验证
- `TestXrayService_GracefulShutdown` - 优雅关闭
- `TestXrayService_NeedRestartFlag` - 重启标志
- `TestXrayService_ClientFiltering` - 客户端过滤
- `TestXrayService_XrayCrashDetection` - 崩溃检测

**运行方法**:

```bash
go test -v ./web/service/ -run TestXrayService
go test -v ./web/service/ -run TestXray
```

## 📊 测试覆盖率

### 当前覆盖率目标

- **Web控制器**: >80%
- **服务层**: >75%
- **数据库层**: >85%
- **Xray集成**: >70%

### 查看覆盖率报告

```bash
# 生成覆盖率报告
go test -coverprofile=coverage.out ./...

# 查看详细覆盖率
go tool cover -html=coverage.out -o coverage.html
open coverage.html  # 在浏览器中打开

# 查看函数级覆盖率
go tool cover -func=coverage.out
```

## ⚡ 性能测试

### 基准测试

项目包含多个基准测试，用于监控性能：

```bash
# 运行所有基准测试
go test -bench=. -benchmem

# 运行特定基准测试
go test -bench=BenchmarkDatabase_CreateInbound -benchmem
go test -bench=BenchmarkXrayService_GetXrayConfig -benchmem
go test -bench=BenchmarkInboundController_ValidateInboundData -benchmem
```

### 性能测试场景

1. **数据库操作性能**
   - 用户创建/查询/更新
   - 入站配置CRUD操作
   - 设置管理操作

2. **API响应性能**
   - 入站列表获取
   - 配置生成
   - 流量统计查询

3. **Xray配置生成性能**
   - 复杂配置生成
   - 策略计算
   - 客户端过滤

4. **并发处理性能**
   - 多用户同时操作
   - 高并发配置生成
   - 数据库并发访问

## 🔒 安全测试

### 安全测试场景

1. **输入验证**
   - SQL注入防护
   - XSS攻击防护
   - 命令注入防护

2. **权限控制**
   - 用户身份验证
   - 角色权限验证
   - 跨用户访问控制

3. **会话安全**
   - 会话劫持防护
   - CSRF攻击防护
   - 会话超时处理

### 运行安全测试

```bash
# 运行所有安全测试
go test -v ./tests/ -run "TestSQLInjection|TestXSS|TestCSRF|TestInputValidation|TestSecurityHeaders"

# 运行特定安全测试
go test -v ./tests/ -run TestSQLInjection
go test -v ./tests/ -run TestXSS
go test -v ./tests/ -run TestCSRF
go test -v ./tests/ -run TestInputValidation
go test -v ./tests/ -run TestSecurityHeaders

# 运行并发安全测试
go test -race -v ./tests/

# 运行安全相关测试（包括其他模块）
go test -v ./... -run "Security\|Auth\|Permission"
```

## 🐛 故障排除

### 常见问题

1. **测试依赖缺失**
   ```bash
   # 确保所有依赖已安装
   go mod tidy
   go mod download
   ```

2. **数据库锁定**
   ```bash
   # 清理测试数据库
   rm -f test.db
   go test ./...
   ```

3. **Xray进程未安装**
   ```bash
   # 跳过Xray相关测试
   go test -v ./... -skip="Xray\|xray"
   ```

4. **端口占用**
   ```bash
   # 检查端口占用
   lsof -i :8080
   # 杀死占用进程
   kill -9 <PID>
   ```

### 测试环境变量

```bash
# 设置测试环境
export GO_ENV=test
export TEST_DB_PATH=/tmp/x-ui-test.db
export TEST_CONFIG_DIR=/tmp/x-ui-test-config
```

## 📈 持续集成

### GitHub Actions配置示例

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.21

      - name: Run tests
        run: |
          go test -v -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

## 🤝 贡献指南

### 添加新测试

1. **遵循命名约定**
   - 测试文件: `*_test.go`
   - 测试函数: `TestFunctionName`
   - 基准测试: `BenchmarkFunctionName`

2. **使用 testify 断言**
   ```go
   import "github.com/stretchr/testify/assert"

   func TestExample(t *testing.T) {
       result := SomeFunction()
       assert.Equal(t, expected, result)
       assert.NotNil(t, result)
   }
   ```

3. **添加测试文档**
   ```go
   // TestExample 测试示例函数
   // 验证特定场景下的行为
   func TestExample(t *testing.T) {
       // 测试代码
   }
   ```

4. **更新此README**
   - 添加新测试说明
   - 更新运行命令
   - 添加使用示例

## 📞 支持

如果遇到测试相关问题：

1. 查看本文档的故障排除部分
2. 检查测试日志输出
3. 确认环境配置正确
4. 在项目仓库中提交Issue

---

**注意**: 本测试套件会持续更新和完善，建议定期运行测试以确保代码质量。
