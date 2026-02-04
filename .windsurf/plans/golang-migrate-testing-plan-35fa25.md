# golang-migrate 测试计划

使用 testify + sqlmock + Docker SQLite 创建全面的测试套件来验证 golang-migrate 的功能，解决当前迁移实例创建卡住的问题。

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

## 测试方案设计

### 技术栈选择
- **testify**: Go 标准测试框架
- **sqlmock**: SQL 数据库模拟
- **testcontainers**: Docker 容器测试
- **SQLite**: 轻量级数据库用于集成测试

### 测试层次结构
```
1. 单元测试 (Unit Tests)
   ├── MigrationManager 测试
   ├── BackupDatabase 测试
   └── 错误处理测试

2. 集成测试 (Integration Tests)
   ├── Docker SQLite 测试
   ├── 迁移文件解析测试
   └── 完整迁移流程测试

3. 端到端测试 (E2E Tests)
   ├── 真实数据库迁移测试
   ├── 回滚场景测试
   └── 错误恢复测试
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

### 步骤3：创建集成测试套件
- 使用 testcontainers 启动 SQLite 容器
- 测试真实的迁移文件解析
- 验证迁移执行和回滚

### 步骤4：创建端到端测试
- 测试完整的迁移流程
- 验证自动备份和回滚机制
- 测试各种错误场景

## 测试用例设计

### 单元测试用例
```go
func TestMigrationManager_Create(t *testing.T)
func TestMigrationManager_Up(t *testing.T)
func TestMigrationManager_Down(t *testing.T)
func TestMigrationManager_Status(t *testing.T)
func TestMigrationManager_Force(t *testing.T)
func TestBackupDatabase_Success(t *testing.T)
func TestBackupDatabase_FileNotFound(t *testing.T)
func TestRollbackMigrations_Success(t *testing.T)
func TestRollbackMigrations_NoDatabase(t *testing.T)
```

### 集成测试用例
```go
func TestMigrationIntegration_FullFlow(t *testing.T)
func TestMigrationIntegration_Rollback(t *testing.T)
func TestMigrationIntegration_ErrorHandling(t *testing.T)
func TestMigrationIntegration_BackupAndRestore(t *testing.T)
```

### 端到端测试用例
```go
func TestE2E_MigrationWithRealFiles(t *testing.T)
func TestE2E_MigrationFailureAndRecovery(t *testing.T)
func TestE2E_MigrationWithCorruptedFiles(t *testing.T)
```

## 测试环境配置

### Docker Compose 配置
```yaml
version: '3.8'
services:
  sqlite-test:
    image: alpine:latest
    command: >
      sh -c "
      apk add --no-cache sqlite &&
      mkdir -p /data &&
      sqlite3 /data/test.db 'CREATE TABLE test (id INTEGER);'
      "
    volumes:
      - ./test-data:/data
```

### 测试配置文件
```go
// test_config.go
package database

import (
    "os"
    "testing"
)

func setupTestConfig(t *testing.T) {
    os.Setenv("XUI_DB_FOLDER", t.TempDir()+"/db")
    os.Setenv("XUI_MIGRATIONS_PATH", t.TempDir()+"/migrations")
}
```

## 模拟策略

### sqlmock 使用示例
```go
func TestMigrationManager_Up_WithMock(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()
    
    // 模拟版本查询
    mock.ExpectQuery("SELECT version, dirty FROM schema_migrations").
        WillReturnError(sql.ErrNoRows)
    
    // 模拟迁移执行
    mock.ExpectExec("CREATE TABLE.*").
        WillReturnResult(sqlmock.NewResult(0, 0))
    
    // 测试迁移执行
    manager := &MigrationManager{db: db}
    err = manager.Up()
    assert.NoError(t, err)
    
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

### testcontainers 使用示例
```go
func TestMigrationIntegration_WithDocker(t *testing.T) {
    ctx := context.Background()
    
    // 启动 SQLite 容器
    req := testcontainers.ContainerRequest{
        Image:        "alpine:latest",
        ExposedPorts: []string{},
        WaitingFor:   wait.ForLog("sqlite3"),
        Cmd:          []string{"sh", "-c", "apk add --no-cache sqlite && mkdir -p /data"},
    }
    
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    // 执行迁移测试
    testMigrationInContainer(t, container)
}
```

## 错误场景测试

### 数据库连接错误
```go
func TestMigrationManager_DatabaseConnectionError(t *testing.T) {
    // 模拟数据库连接失败
    manager, err := NewMigrationManager("/invalid/path")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "创建迁移管理器失败")
}
```

### 迁移文件错误
```go
func TestMigrationManager_InvalidMigrationFile(t *testing.T) {
    // 创建无效的迁移文件
    tempDir := t.TempDir()
    invalidFile := filepath.Join(tempDir, "001_invalid.up.sql")
    os.WriteFile(invalidFile, []byte("INVALID SQL"), 0644)
    
    manager, err := NewMigrationManager(filepath.Join(tempDir, "test.db"))
    assert.Error(t, err)
}
```

### 备份失败场景
```go
func TestBackupDatabase_PermissionError(t *testing.T) {
    // 模拟权限错误
    os.Setenv("XUI_DB_FOLDER", "/root/invalid")
    err := BackupDatabase()
    assert.Error(t, err)
}
```

## 性能测试

### 大量迁移测试
```go
func TestMigrationPerformance_LargeNumberOfMigrations(t *testing.T) {
    // 创建大量迁移文件
    tempDir := t.TempDir()
    for i := 1; i <= 100; i++ {
        createMigrationFile(t, tempDir, i)
    }
    
    // 测试迁移性能
    start := time.Now()
    err := RunMigrationsWithBackup()
    duration := time.Since(start)
    
    assert.NoError(t, err)
    assert.Less(t, duration, 30*time.Second)
}
```

## 测试数据管理

### 测试迁移文件
```
test-data/
├── migrations/
│   ├── 001_test_table.up.sql
│   ├── 001_test_table.down.sql
│   ├── 002_add_column.up.sql
│   ├── 002_add_column.down.sql
│   └── ...
└── databases/
    ├── empty.db
    ├── v1.db
    └── corrupted.db
```

### 测试数据生成
```go
func createTestMigrationFile(t *testing.T, dir string, version int) {
    upFile := filepath.Join(dir, fmt.Sprintf("%03d_test.up.sql", version))
    downFile := filepath.Join(dir, fmt.Sprintf("%03d_test.down.sql", version))
    
    os.WriteFile(upFile, []byte(fmt.Sprintf("CREATE TABLE test_%d (id INTEGER);", version)), 0644)
    os.WriteFile(downFile, []byte(fmt.Sprintf("DROP TABLE IF EXISTS test_%d;", version)), 0644)
}
```

## 预期收益

### 测试覆盖率提升
- **单元测试覆盖率**: 从 30% 提升到 80%
- **集成测试覆盖率**: 从 0% 提升到 60%
- **端到端测试覆盖率**: 从 0% 提升到 40%

### 问题定位能力
- ✅ 快速定位迁移实例创建问题
- ✅ 验证迁移文件解析逻辑
- ✅ 测试错误处理机制
- ✅ 验证备份和回滚功能

### 开发效率提升
- ✅ 自动化测试减少手动验证
- ✅ 快速反馈开发问题
- ✅ 重构信心增强
- ✅ 代码质量保证

## 成功标准

1. 所有单元测试通过，覆盖率达到 80%
2. 集成测试验证真实迁移场景
3. 端到端测试验证完整流程
4. 性能测试满足性能要求
5. 错误场景测试覆盖所有边界条件

## 时间安排

| 步骤 | 时间 | 主要任务 |
|------|------|----------|
| 步骤1 | 0.5天 | 添加测试依赖 |
| 步骤2 | 1天 | 创建单元测试套件 |
| 步骤3 | 1天 | 创建集成测试套件 |
| 步骤4 | 1天 | 创建端到端测试套件 |

**总计：3.5天**
