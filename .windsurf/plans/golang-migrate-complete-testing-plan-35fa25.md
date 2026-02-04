# golang-migrate 完整测试计划

使用 testify + sqlmock + Docker SQLite 创建全面的测试套件来验证 golang-migrate 的功能，测试所有 5 个迁移文件的完整迁移流程，解决当前迁移实例创建卡住的问题。

## 当前问题分析

### 现有问题
1. **迁移实例创建卡住**：golang-migrate 实例创建时无响应
2. **测试覆盖不足**：现有测试无法验证真实迁移场景
3. **依赖外部文件**：测试依赖实际的迁移文件和数据库
4. **缺乏模拟测试**：无法模拟各种迁移场景

### 现有测试状态
- ✅ `TestMigrationManager` - 基本功能测试
- ✅ `TestMigrationManagerStatus` - 状态检查测试
- ⚠️ `TestBackupDatabase` - 跳过（依赖生产配置）
- ✅ `TestRunMigrations` - 基本运行测试
- ❌ 缺乏真实迁移场景测试
- ❌ 缺乏错误场景测试
- ❌ 缺乏 Docker 集成测试

## 5个迁移文件分析

### 迁移文件概览
```
000001_init_schema.up.sql     - 初始化数据库架构（9个表）
000002_tls_config.up.sql       - TLS 配置迁移（JSON字符串替换）
000003_xhttp_flow.up.sql       - XHTTP Flow 迁移（JSON字符串替换）
000004_reality_target.up.sql   - Reality target 迁移（JSON字符串替换）
000005_user_password_hash.up.sql - 用户密码哈希迁移（应用层处理）
```

### 迁移复杂度分析
| 版本 | 类型 | 复杂度 | 测试重点 |
|------|------|--------|----------|
| **1** | DDL | 中等 | 表创建、约束验证 |
| **2** | DML | 高 | JSON字符串替换、多条件更新 |
| **3** | DML | 高 | JSON字符串替换、复杂条件 |
| **4** | DML | 高 | JSON字符串替换、JSON函数 |
| **5** | DML | 中等 | 条件更新、应用层集成 |

## 测试方案设计

### 技术栈选择
- **testify**: Go 标准测试框架 ✅ (已存在)
- **sqlmock**: SQL 数据库模拟 (需要添加)
- **testcontainers**: Docker 容器测试 ✅ (已存在)
- **SQLite**: 轻量级数据库 (需要配置)

### 测试层次结构
```
1. 单元测试 (Unit Tests)
   ├── MigrationManager 核心功能测试
   ├── BackupDatabase 功能测试
   ├── 错误处理和边界条件测试
   └── 迁移文件解析测试

2. 集成测试 (Integration Tests)
   ├── Docker SQLite 真实环境测试
   ├── 5个迁移文件完整测试
   ├── 迁移顺序和依赖测试
   └── JSON操作验证测试

3. 端到端测试 (E2E Tests)
   ├── 完整迁移流程测试
   ├── 回滚和恢复测试
   ├── 错误场景和边界测试
   └── 性能和并发测试
```

## 实施步骤

### 步骤1：添加测试依赖
```bash
go get github.com/DATA-DOG/go-sqlmock
go get github.com/stretchr/testify/assert
go get github.com/stretchr/testify/suite
go get github.com/testcontainers/testcontainers-go/modules/sqlite
```

### 步骤2：创建单元测试套件
- 使用 sqlmock 模拟数据库操作
- 测试 MigrationManager 的各个方法
- 测试错误处理和边界条件
- 测试迁移文件解析逻辑

### 步骤3：创建集成测试套件
- 使用 testcontainers 启动 SQLite 容器
- 测试所有 5 个迁移文件的完整执行
- 验证迁移顺序和依赖关系
- 测试 JSON 操作和数据修改

### 步骤4：创建端到端测试套件
- 测试完整的迁移流程（1→5）
- 测试回滚流程（5→1）
- 测试部分迁移和错误恢复
- 测试并发迁移和性能

## 详细测试用例设计

### 单元测试用例
```go
// MigrationManager 核心功能测试
func TestMigrationManager_Create(t *testing.T)
func TestMigrationManager_Up(t *testing.T)
func TestMigrationManager_Down(t *testing.T)
func TestMigrationManager_Status(t *testing.T)
func TestMigrationManager_Force(t *testing.T)

// 备份功能测试
func TestBackupDatabase_Success(t *testing.T)
func TestBackupDatabase_FileNotFound(t *testing.T)
func TestBackupDatabase_PermissionError(t *testing.T)

// 回滚功能测试
func TestRollbackMigrations_Success(t *testing.T)
func TestRollbackMigrations_NoDatabase(t *testing.T)
func TestRollbackMigrations_NoMigration(t *testing.T)

// 迁移状态测试
func TestGetMigrationStatus_NoDatabase(t *testing.T)
func TestGetMigrationStatus_WithMigrations(t *testing.T)
func TestGetMigrationStatus_DirtyState(t *testing.T)
```

### 集成测试用例
```go
// 完整迁移流程测试
func TestMigrationIntegration_FullFlow_1To5(t *testing.T)
func TestMigrationIntegration_PartialMigration(t *testing.T)
func TestMigrationIntegration_SkipVersions(t *testing.T)

// 单个迁移文件测试
func TestMigration001_InitSchema(t *testing.T)
func TestMigration002_TlsConfig(t *testing.T)
func TestMigration003_XhttpFlow(t *testing.T)
func TestMigration004_RealityTarget(t *testing.T)
func TestMigration005_UserPasswordHash(t *testing.T)

// JSON操作验证测试
func TestJsonOperations_TlsConfig(t *testing.T)
func TestJsonOperations_XhttpFlow(t *testing.T)
func TestJsonOperations_RealityTarget(t *testing.T)

// 错误处理测试
func TestMigrationIntegration_ErrorHandling(t *testing.T)
func TestMigrationIntegration_CorruptedFiles(t *testing.T)
func TestMigrationIntegration_BackupAndRestore(t *testing.T)
```

### 端到端测试用例
```go
// 完整流程测试
func TestE2E_MigrationWithAllFiles(t *testing.T)
func TestE2E_MigrationRollback(t *testing.T)
func TestE2E_MigrationPartialRollback(t *testing.T)

// 错误恢复测试
func TestE2E_MigrationFailureAndRecovery(t *testing.T)
func TestE2E_MigrationWithCorruptedFiles(t *testing.T)
func TestE2E_MigrationWithInvalidData(t *testing.T)

// 性能测试
func TestE2E_MigrationPerformance(t *testing.T)
func TestE2E_LargeDatasetMigration(t *testing.T)
func TestE2E_ConcurrentMigration(t *testing.T)
```

## 测试数据准备

### 测试数据库状态
```go
// 测试数据生成器
type TestDataGenerator struct {
    tempDir string
    dbPath  string
}

func (g *TestDataGenerator) CreateEmptyDatabase() error
func (g *TestDataGenerator) CreateDatabaseWithInbounds() error
func (g *TestDataGenerator) CreateDatabaseWithUsers() error
func (g *TestDataGenerator) CreateCorruptedDatabase() error
```

### 测试迁移文件验证
```go
// 验证迁移文件执行结果
func ValidateMigration001(t *testing.T, db *sql.DB)
func ValidateMigration002(t *testing.T, db *sql.DB)
func ValidateMigration003(t *testing.T, db *sql.DB)
func ValidateMigration004(t *testing.T, db *sql.DB)
func ValidateMigration005(t *testing.T, db *sql.DB)
```

## 关键测试场景

### 场景1：完整迁移流程（1→5）
```go
func TestCompleteMigrationFlow(t *testing.T) {
    // 1. 创建空数据库
    db := setupTestDatabase(t)
    
    // 2. 执行所有迁移
    err := RunMigrationsWithBackup()
    assert.NoError(t, err)
    
    // 3. 验证所有表存在
    validateAllTablesExist(t, db)
    
    // 4. 验证迁移版本
    validateMigrationVersion(t, db, 5)
    
    // 5. 验证数据完整性
    validateDataIntegrity(t, db)
}
```

### 场景2：JSON操作验证
```go
func TestJsonOperationsValidation(t *testing.T) {
    // 1. 创建测试数据
    db := setupDatabaseWithInbounds(t)
    
    // 2. 执行 TLS 配置迁移
    err := executeMigration(t, db, 2)
    assert.NoError(t, err)
    
    // 3. 验证 JSON 字段修改
    validateTlsConfigChanges(t, db)
    
    // 4. 执行 XHTTP Flow 迁移
    err = executeMigration(t, db, 3)
    assert.NoError(t, err)
    
    // 5. 验证 JSON 字段修改
    validateXhttpFlowChanges(t, db)
}
```

### 场景3：回滚测试
```go
func TestRollbackScenario(t *testing.T) {
    // 1. 执行所有迁移
    db := setupTestDatabase(t)
    err := RunMigrationsWithBackup()
    assert.NoError(t, err)
    
    // 2. 验证迁移状态
    validateMigrationVersion(t, db, 5)
    
    // 3. 回滚到版本3
    err = rollbackToVersion(t, 3)
    assert.NoError(t, err)
    
    // 4. 验证回滚结果
    validateMigrationVersion(t, db, 3)
    validateRollbackResults(t, db)
}
```

## Docker 测试环境

### SQLite 容器配置
```yaml
# docker-compose.test.yml
version: '3.8'
services:
  sqlite-test:
    image: alpine:latest
    command: >
      sh -c "
      apk add --no-cache sqlite3 &&
      mkdir -p /data &&
      sqlite3 /data/test.db 'PRAGMA journal_mode=WAL;'
      "
    volumes:
      - ./test-data:/data
      - ./database/migrations:/migrations
    environment:
      - SQLITE_DB=/data/test.db
```

### 测试容器管理
```go
type TestContainer struct {
    container testcontainers.Container
    db        *sql.DB
    tempDir   string
}

func setupTestContainer(t *testing.T) *TestContainer {
    ctx := context.Background()
    
    // 启动 SQLite 容器
    req := testcontainers.ContainerRequest{
        Image:        "alpine:latest",
        ExposedPorts: []string{},
        WaitingFor:   wait.ForLog("sqlite3"),
        Cmd: []string{"sh", "-c", "apk add --no-cache sqlite3 && mkdir -p /data"},
    }
    
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    
    // 创建数据库连接
    db, err := sql.Open("sqlite3", "/data/test.db")
    require.NoError(t, err)
    
    return &TestContainer{
        container: container,
        db:        db,
        tempDir:   t.TempDir(),
    }
}
```

## 性能测试要求

### 性能基准
```go
func BenchmarkMigrationExecution(b *testing.B) {
    for i := 0; i < b.N; i++ {
        setupFreshDatabase()
        RunMigrationsWithBackup()
    }
}

func BenchmarkLargeDatasetMigration(b *testing.B) {
    // 创建大量测试数据
    setupLargeDataset()
    
    b.ResetTimer()
    RunMigrationsWithBackup()
}
```

### 性能指标
- **迁移执行时间**: < 30秒（5个迁移）
- **内存使用**: < 100MB
- **数据库大小**: < 50MB（迁移后）
- **并发安全**: 支持单线程执行

## 错误场景测试

### 数据库连接错误
```go
func TestMigrationManager_DatabaseConnectionError(t *testing.T) {
    manager, err := NewMigrationManager("/invalid/path")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "创建迁移管理器失败")
}
```

### 迁移文件错误
```go
func TestMigrationManager_InvalidMigrationFile(t *testing.T) {
    tempDir := t.TempDir()
    invalidFile := filepath.Join(tempDir, "001_invalid.up.sql")
    os.WriteFile(invalidFile, []byte("INVALID SQL SYNTAX"), 0644)
    
    manager, err := NewMigrationManager(filepath.Join(tempDir, "test.db"))
    assert.Error(t, err)
}
```

### JSON操作错误
```go
func TestMigration002_InvalidJsonData(t *testing.T) {
    db := setupDatabaseWithInvalidJson(t)
    
    err := executeMigration(t, db, 2)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "JSON解析失败")
}
```

## 预期收益

### 测试覆盖率目标
- **单元测试覆盖率**: 85%
- **集成测试覆盖率**: 70%
- **端到端测试覆盖率**: 50%
- **迁移文件覆盖率**: 100%（5/5）

### 问题解决能力
- ✅ 快速定位迁移实例创建问题
- ✅ 验证所有 5 个迁移文件的正确性
- ✅ 测试 JSON 操作和数据修改逻辑
- ✅ 验证备份和回滚功能的完整性
- ✅ 测试错误处理和边界条件

### 开发效率提升
- ✅ 自动化测试减少手动验证时间
- ✅ 快速反馈开发问题
- ✅ 重构信心增强
- ✅ 代码质量保证
- ✅ 文档化测试结果

## 成功标准

1. 所有 5 个迁移文件的单元测试通过
2. 集成测试验证完整迁移流程（1→5）
3. 端到端测试验证回滚和错误恢复
4. 性能测试满足要求（< 30秒）
5. 错误场景测试覆盖所有边界条件
6. 测试覆盖率达到目标指标

## 时间安排

| 步骤 | 时间 | 主要任务 |
|------|------|----------|
| 步骤1 | 0.5天 | 添加测试依赖 |
| 步骤2 | 1.5天 | 创建单元测试套件（包含5个迁移文件） |
| 步骤3 | 1.5天 | 创建集成测试套件（Docker SQLite） |
| 步骤4 | 1.5天 | 创建端到端测试套件（完整流程） |

**总计：5天**
