# 数据库迁移现代化逐步实施计划

基于对当前 X-Panel 数据库迁移实现的深入分析，制定一个分阶段的现代化改造计划，将现有的手动迁移系统升级为专业的数据库迁移管理方案。

## 当前迁移系统分析

### 现有架构概览
```
main.go 
  ↓
bootstrap.Initialize()
  ↓
InitDatabase() → database.InitDB()
  ↓
runSeeders() → 执行4个迁移
```

### 现有迁移列表（远程服务器状态）
1. ✅ `UserPasswordHash` - 用户密码哈希迁移
2. ✅ `TlsConfigMigration` - TLS 配置迁移  
3. ✅ `XhttpFlowMigration` - XHTTP Flow 迁移
4. ❓ `RealityTargetMigration` - Reality target 迁移（本地有，远程没有）

### 现有实现特点
**优点**：
- ✅ 简单直接，与应用启动集成
- ✅ 有基本的迁移状态跟踪（`history_of_seeders` 表）
- ✅ 支持条件执行（检查表是否为空）

**缺点**：
- ❌ 迁移逻辑硬编码在 `database/db.go` 中
- ❌ 缺乏版本控制和顺序管理
- ❌ 无法回滚迁移
- ❌ 测试困难，与应用代码耦合
- ❌ 错误处理不够完善
- ❌ 手动 SQL 操作（如 Reality target 修复）

## 逐步实施计划

### 阶段1：基础设施准备（第1-2天）

#### 步骤1.1：添加依赖和工具
```bash
# 添加 golang-migrate 依赖
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/sqlite3
go get github.com/golang-migrate/migrate/v4/source/file
```

#### 步骤1.2：创建迁移目录结构
```
database/
├── migrations/              # 新的迁移文件目录
│   ├── 000001_init_schema.up.sql
│   ├── 000001_init_schema.down.sql
│   ├── 000002_tls_config.up.sql
│   ├── 000002_tls_config.down.sql
│   ├── 000003_xhttp_flow.up.sql
│   ├── 000003_xhttp_flow.down.sql
│   ├── 000004_reality_target.up.sql
│   ├── 000004_reality_target.down.sql
│   ├── 000005_user_password_hash.up.sql
│   └── 000005_user_password_hash.down.sql
├── migrate.go              # 迁移管理器
├── migrate_test.go         # 迁移测试
└── legacy_migrator.go      # 现有迁移清理
```

#### 步骤1.3：实现迁移管理器
创建 `database/migrate.go`：
```go
type MigrationManager struct {
    db        *gorm.DB
    migrate   *migrate.Migrate
    logger    logger.Logger
}

func (m *MigrationManager) Up() error
func (m *MigrationManager) Down() error
func (m *MigrationManager) Status() error
func (m *MigrationManager) Force(version uint) error
```

### 阶段2：现有迁移转换（第3-5天）

#### 步骤2.1：分析现有迁移逻辑
- `migrateTlsInbounds()` → JSON 处理 + 批量更新
- `migrateXhttpFlow()` → 条件查询 + JSON 修改
- `migrateRealityTarget()` → JSON 解析 + 字符串替换
- `UserPasswordHash` → 用户表遍历 + 密码哈希

#### 步骤2.2：转换为 SQL 迁移文件

**示例：000004_reality_target.up.sql**
```sql
-- Reality Target 迁移：修复缺少端口号的 target
-- 版本：4
-- 描述：为 Reality 配置中的 target 字段添加端口号

-- 修复 www.google.com → www.google.com:443
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.google.com"', 
    '"target":"www.google.com:443"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.google.com';

-- 修复 www.amazon.com → www.amazon.com:443
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.amazon.com"', 
    '"target":"www.amazon.com:443"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.amazon.com';

-- 记录迁移执行
INSERT INTO schema_migrations (version, dirty) 
VALUES (4, false);
```

**示例：000004_reality_target.down.sql**
```sql
-- Reality Target 迁移回滚
-- 恢复 www.google.com:443 → www.google.com
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.google.com:443"', 
    '"target":"www.google.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.google.com';

-- 恢复 www.amazon.com:443 → www.amazon.com
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.amazon.com:443"', 
    '"target":"www.amazon.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.amazon.com';
```

#### 步骤2.3：创建版本表
```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version bigint NOT NULL PRIMARY KEY,
    dirty boolean NOT NULL
);
```

### 阶段3：应用集成（第6-7天）

#### 步骤3.1：修改数据库初始化流程
在 `bootstrap/bootstrap.go` 中：
```go
func InitDatabase() error {
    // 1. 初始化数据库连接
    if err := database.InitDB(config.GetDBPath()); err != nil {
        return err
    }
    
    // 2. 执行数据库迁移
    if err := database.RunMigrations(); err != nil {
        return err
    }
    
    return nil
}
```

#### 步骤3.2：实现数据库迁移执行
在 `database/migrate.go` 中：
```go
func RunMigrations() error {
    m, err := NewMigrationManager(dbPath)
    if err != nil {
        return err
    }
    defer m.Close()
    
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return err
    }
    
    return nil
}
```

#### 步骤3.3：清理现有迁移逻辑
- 保留 `runSeeders()` 但只用于用户初始化
- 移除硬编码的迁移函数
- 保留 `history_of_seeders` 表用于兼容性

### 阶段4：测试和验证（第8-9天）

#### 步骤4.1：创建迁移测试
```go
// migrate_test.go
func TestMigrationUp(t *testing.T)
func TestMigrationDown(t *testing.T)
func TestMigrationRollback(t *testing.T)
func TestExistingDataMigration(t *testing.T)
```

#### 步骤4.2：集成测试
- 在测试数据库中验证完整迁移流程
- 测试从旧版本到新版本的升级
- 验证回滚机制

#### 步骤4.3：生产环境验证
- 在远程服务器上测试迁移
- 验证数据完整性
- 确认服务正常运行

### 阶段5：文档和工具（第10天）

#### 步骤5.1：创建迁移管理工具
```bash
# 添加到 Makefile
migrate-up:     # 执行迁移
migrate-down:   # 回滚迁移
migrate-status: # 查看迁移状态
migrate-force:  # 强制迁移到指定版本
```

#### 步骤5.2：更新文档
- 更新 README.md 中的数据库迁移说明
- 创建迁移开发指南
- 更新部署文档

## 风险控制措施

### 数据安全
1. **自动备份**：迁移前自动备份数据库
2. **分步验证**：每个迁移后验证数据完整性
3. **回滚机制**：提供完整的回滚能力

### 服务可用性
1. **停机时间最小化**：迁移在应用启动时执行
2. **错误处理**：迁移失败时提供清晰的错误信息
3. **监控告警**：迁移过程的日志记录

### 兼容性保证
1. **向后兼容**：保持现有 API 不变
2. **渐进式迁移**：支持从任意旧版本升级
3. **测试覆盖**：全面的测试确保稳定性

## 成功标准

### 技术指标
- ✅ 所有现有迁移成功转换为新的迁移系统
- ✅ 新迁移系统支持 Up/Down 操作
- ✅ 应用启动时自动执行迁移
- ✅ 提供迁移状态查询接口

### 质量指标
- ✅ 100% 测试覆盖率
- ✅ 零数据丢失风险
- ✅ 完整的文档和工具支持

### 运维指标
- ✅ 迁移执行时间 < 30秒
- ✅ 支持从任意版本升级
- ✅ 提供迁移回滚能力

## 时间安排

| 阶段 | 天数 | 主要任务 | 交付物 |
|------|------|----------|--------|
| 阶段1 | 2天 | 基础设施准备 | 迁移管理器框架 |
| 阶段2 | 3天 | 现有迁移转换 | SQL 迁移文件 |
| 阶段3 | 2天 | 应用集成 | 集成后的启动流程 |
| 阶段4 | 2天 | 测试验证 | 测试报告 |
| 阶段5 | 1天 | 文档工具 | 完整的迁移系统 |

**总计：10天**

## 后续优化建议

1. **CI/CD 集成**：在部署流程中自动检查迁移状态
2. **监控仪表板**：提供迁移状态的可视化界面
3. **自动化测试**：在 CI/CD 中自动运行迁移测试
4. **性能优化**：优化大数据量迁移的执行效率
