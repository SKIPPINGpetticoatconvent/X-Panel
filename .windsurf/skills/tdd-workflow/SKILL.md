---
name: tdd-workflow
description: X-Panel 项目 TDD 工作流。在编写新功能、修复 Bug 或重构代码时使用。强制执行测试驱动开发，包含单元测试、集成测试和 E2E 测试。
---

# X-Panel 测试驱动开发工作流

本技能确保所有代码开发遵循 TDD 原则和项目测试规范。

## 激活条件

- 编写新功能或特性
- 修复 Bug 或问题
- 重构现有代码
- 添加 API 端点
- 修改 Service 层逻辑
- 修改安装脚本 (`install.sh`, `x-ui.sh`)

## 核心原则

### 1. 测试优先
始终先编写测试，再实现代码使测试通过。

### 2. 强制验证流程
所有 Go 代码修改后，**必须**执行以下命令验证：

```bash
go build ./... && go test -race -short ./... && golangci-lint run --timeout=10m && nilaway -test=false ./...
```

### 3. 测试类型

#### 单元测试 (`*_test.go`)
- 核心业务逻辑（Service 层）
- 工具函数 (`util/` 目录)
- 数据模型和仓库操作
- 配置解析和验证

#### 集成测试
- API 端点 (`web/controller/`)
- 数据库操作 (`database/repository/`)
- Xray 核心交互 (`xray/`)

#### E2E 测试 (`tests/e2e/` 和 `QA/e2e/`)
- 安装流程测试 (Docker 容器)
- 服务状态检查
- Web API 完整流程

## TDD 工作流步骤

### 步骤 1: 定义用户场景
```
作为 [角色]，我希望 [操作]，以便 [收益]

示例:
作为管理员，我希望为入站连接选择 SNI 域名，
以便为不同地区的用户提供最优的连接配置。
```

### 步骤 2: 编写测试用例
为每个场景创建完整测试用例：

```go
func TestInboundService_GetInbounds(t *testing.T) {
    setupTestDB(t)
    
    db := database.GetDB()
    s := &InboundService{}
    
    // Arrange: 准备测试数据
    user := &model.User{Username: "testuser", Password: "password"}
    db.Create(user)
    
    inbound := &model.Inbound{
        UserId:   user.Id,
        Tag:      "test-inbound",
        Protocol: model.VMESS,
        Port:     10001,
    }
    db.Create(inbound)
    
    // Act: 执行被测方法
    inbounds, err := s.GetInbounds(user.Id)
    
    // Assert: 验证结果
    if err != nil {
        t.Fatalf("GetInbounds failed: %v", err)
    }
    if len(inbounds) != 1 {
        t.Errorf("Expected 1 inbound, got %d", len(inbounds))
    }
}
```

### 步骤 3: 运行测试（预期失败）
```bash
go test -v ./web/service/... -run TestInboundService
# 测试应该失败 - 尚未实现功能
```

### 步骤 4: 实现代码
编写最小代码使测试通过：

```go
func (s *InboundService) GetInbounds(userId int) ([]*model.Inbound, error) {
    db := database.GetDB()
    var inbounds []*model.Inbound
    err := db.Model(model.Inbound{}).Where("user_id = ?", userId).Find(&inbounds).Error
    return inbounds, err
}
```

### 步骤 5: 再次运行测试
```bash
go test -v ./web/service/... -run TestInboundService
# 测试应该通过
```

### 步骤 6: 执行完整验证
```bash
go build ./... && go test -race -short ./... && golangci-lint run --timeout=10m && nilaway -test=false ./...
```

### 步骤 7: 重构（保持测试绿色）
- 消除重复代码
- 改进命名
- 优化性能
- 提升可读性

## 测试模式

### 单元测试模式 (Go)
```go
package service

import (
    "testing"
    "x-ui/database"
    "x-ui/database/model"
)

// 测试辅助函数 - 设置测试数据库
func setupTestDB(t *testing.T) {
    t.Helper()
    database.InitTestDB()
    t.Cleanup(func() {
        database.CloseDB()
    })
}

func TestServiceMethod_Success(t *testing.T) {
    setupTestDB(t)
    s := &MyService{}
    
    result, err := s.Method()
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("expected %v, got %v", expected, result)
    }
}

func TestServiceMethod_EdgeCase(t *testing.T) {
    setupTestDB(t)
    s := &MyService{}
    
    _, err := s.Method()
    
    if err == nil {
        t.Error("expected error, got nil")
    }
}
```

### 表驱动测试模式
```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Config
        wantErr bool
    }{
        {
            name:  "valid config",
            input: `{"port": 8080}`,
            want:  &Config{Port: 8080},
        },
        {
            name:    "invalid json",
            input:   `{invalid}`,
            wantErr: true,
        },
        {
            name:  "empty config",
            input: `{}`,
            want:  &Config{},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParseConfig() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### API 测试模式 (Gin)
```go
func TestAPIEndpoint(t *testing.T) {
    setupTestDB(t)
    
    gin.SetMode(gin.TestMode)
    router := gin.New()
    controller := NewController()
    router.GET("/api/test", controller.GetTest)
    
    req := httptest.NewRequest("GET", "/api/test", nil)
    w := httptest.NewRecorder()
    
    router.ServeHTTP(w, req)
    
    if w.Code != http.StatusOK {
        t.Errorf("expected status 200, got %d", w.Code)
    }
    
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    
    if response["success"] != true {
        t.Error("expected success: true")
    }
}
```

### E2E 测试模式 (Go + Docker)
```go
// tests/e2e/install_test.go
func (s *E2ETestSuite) TestInstallation() {
    t := s.T()
    
    // 在 Docker 容器内执行安装
    output, err := s.ExecInContainer("bash /app/install.sh")
    require.NoError(t, err, "installation should succeed")
    
    // 验证服务已启动
    assert.Contains(t, output, "x-ui started successfully")
    
    // 验证端口监听
    _, err = s.ExecInContainer("nc -z localhost 54321")
    require.NoError(t, err, "port 54321 should be listening")
}
```

## 测试文件组织

```
X-Panel/
├── config/
│   ├── config.go
│   └── config_test.go           # 配置单元测试
├── database/
│   ├── repository/
│   │   ├── inbound_repository.go
│   │   └── inbound_repository_test.go
│   └── db_test.go               # 数据库单元测试
├── web/
│   ├── controller/
│   │   ├── api.go
│   │   └── api_interface_test.go # API 接口测试
│   └── service/
│       ├── inbound.go
│       └── inbound_test.go      # Service 单元测试
├── xray/
│   ├── adaptation.go
│   └── adaptation_test.go       # Xray 适配测试
├── tests/
│   └── e2e/
│       ├── e2e_test.go          # E2E 测试套件
│       ├── install_test.go      # 安装测试
│       └── cert_test.go         # 证书测试
└── QA/
    ├── e2e/
    │   ├── 1_install/test.sh    # Shell 安装测试
    │   ├── 2_service/test.sh    # 服务状态测试
    │   └── 3_api/               # Python API 测试
    ├── unit/run.sh              # 单元测试脚本
    └── run_qa.sh                # 完整 QA 流程
```

## Mock 模式

### 数据库 Mock (Gorm)
```go
func setupTestDB(t *testing.T) {
    t.Helper()
    // 使用内存 SQLite 进行测试
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatalf("failed to open test database: %v", err)
    }
    
    // 自动迁移测试所需的表
    db.AutoMigrate(&model.User{}, &model.Inbound{})
    
    database.SetDB(db)
}
```

### HTTP Client Mock
```go
type MockHTTPClient struct {
    DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
    return m.DoFunc(req)
}

func TestExternalAPICall(t *testing.T) {
    mockClient := &MockHTTPClient{
        DoFunc: func(req *http.Request) (*http.Response, error) {
            return &http.Response{
                StatusCode: 200,
                Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
            }, nil
        },
    }
    
    service := &MyService{client: mockClient}
    result, err := service.CallAPI()
    
    require.NoError(t, err)
    assert.Equal(t, "ok", result.Status)
}
```

## Shell 脚本测试

修改 Shell 脚本后必须执行：

```bash
# 格式化
shfmt -i 2 -w -s .

# 静态检查
shellcheck install.sh x-ui.sh

# E2E 验证
make e2e
```

## 验证命令汇总

### Go 代码验证
```bash
# 完整验证（必须全部通过）
go build ./... && go test -race -short ./... && golangci-lint run --timeout=10m && nilaway -test=false ./...

# 单独运行单元测试
go test -v ./...

# 运行特定包测试
go test -v ./web/service/...

# 运行特定测试
go test -v ./web/service/... -run TestInboundService

# 带覆盖率测试
go test -cover ./...
```

### E2E 测试
```bash
# 标准 E2E 测试
make e2e

# 仅安装测试
make e2e-install

# 完整 QA 流程
./QA/run_qa.sh
```

### Shell 脚本验证
```bash
shfmt -i 2 -w -s .
shellcheck install.sh x-ui.sh
```

### TOML 验证
```bash
taplo fmt --check
```

### Makefile 验证
```bash
checkmake Makefile
```

## 常见错误避免

### ❌ 错误: 业务逻辑中使用 panic
```go
func GetUser(id int) *User {
    if id <= 0 {
        panic("invalid id") // 禁止!
    }
}
```

### ✅ 正确: 返回 error
```go
func GetUser(id int) (*User, error) {
    if id <= 0 {
        return nil, errors.New("invalid id")
    }
    // ...
}
```

### ❌ 错误: 测试间共享状态
```go
var globalUser *User // 测试间共享

func TestCreate(t *testing.T) {
    globalUser = createUser()
}

func TestUpdate(t *testing.T) {
    updateUser(globalUser) // 依赖上一个测试
}
```

### ✅ 正确: 测试独立
```go
func TestCreate(t *testing.T) {
    setupTestDB(t)
    user := createUser()
    // 测试逻辑
}

func TestUpdate(t *testing.T) {
    setupTestDB(t)
    user := createUser() // 每个测试创建自己的数据
    updateUser(user)
}
```

### ❌ 错误: 硬编码测试数据
```go
func TestPort(t *testing.T) {
    // 端口可能被占用
    inbound := &Inbound{Port: 8080}
}
```

### ✅ 正确: 使用动态值
```go
func TestPort(t *testing.T) {
    port := getAvailablePort()
    inbound := &Inbound{Port: port}
}
```

## CI/CD 集成

### GitHub Actions 配置
```yaml
# .github/workflows/qa.yml
name: QA

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      
      - name: Build
        run: go build ./...
      
      - name: Test
        run: go test -race -short ./...
      
      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

## 最佳实践

1. **测试优先** - 始终遵循 TDD
2. **Arrange-Act-Assert** - 清晰的测试结构
3. **表驱动测试** - 覆盖多种输入场景
4. **测试隔离** - 每个测试独立运行
5. **测试命名** - 描述性函数名 `TestFunction_Scenario`
6. **边界条件** - 测试空值、零值、边界值
7. **错误路径** - 测试失败场景
8. **快速执行** - 单元测试 < 100ms
9. **清理资源** - 使用 `t.Cleanup()`
10. **持续验证** - 修改后立即运行验证命令

## 成功指标

- 所有测试通过（绿色）
- 无跳过或禁用的测试
- 快速测试执行（单元测试 < 30s）
- E2E 测试覆盖关键安装和服务流程
- 验证命令全部通过
- 无 golangci-lint 和 nilaway 警告

---

**记住**: 测试不是可选的。它们是实现可靠重构、快速开发和生产稳定性的安全网。
