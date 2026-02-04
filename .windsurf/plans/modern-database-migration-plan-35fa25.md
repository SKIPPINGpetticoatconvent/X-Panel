# 现代化数据库迁移方案

本计划将 X-Panel 项目从当前的手动迁移方式升级为专业的数据库迁移管理系统，提高可维护性、可靠性和可测试性。

## 当前问题分析

### 现有迁移方式的缺陷
1. **硬编码迁移逻辑**：迁移函数直接写在 `database/db.go` 中，与应用代码耦合
2. **缺乏版本控制**：使用 `HistoryOfSeeders` 表手动跟踪迁移状态
3. **无法回滚**：没有提供回滚机制，迁移失败后难以恢复
4. **测试困难**：迁移逻辑与应用逻辑混合，难以独立测试
5. **手动 SQL 操作**：如修复 Reality target 时需要手动执行 SQL

### 现有迁移列表
- `TlsConfigMigration`: TLS 配置迁移
- `XhttpFlowMigration`: XHTTP Flow 迁移  
- `RealityTargetMigration`: Reality target 端口修复
- `UserPasswordHash`: 用户密码哈希迁移

## 目标方案设计

### 技术选型：golang-migrate
选择理由：
- ✅ Go 生态官方推荐
- ✅ 支持 SQLite（项目当前使用）
- ✅ 完善的版本控制和回滚机制
- ✅ 社区活跃，文档完善
- ✅ 支持嵌入到应用中

### 目录结构设计
```
database/
├── migrations/           # 迁移文件目录
│   ├── 000001_init_schema.up.sql
│   ├── 000001_init_schema.down.sql
│   ├── 000002_tls_config.up.sql
│   ├── 000002_tls_config.down.sql
│   ├── 000003_xhttp_flow.up.sql
│   ├── 000003_xhttp_flow.down.sql
│   ├── 000004_reality_target.up.sql
│   ├── 000004_reality_target.down.sql
│   └── 000005_user_password_hash.up.sql
│       └── 000005_user_password_hash.down.sql
├── migrate.go          # 迁移管理器
├── migrate_test.go     # 迁移测试
└── legacy_migrator.go  # 现有迁移清理
```

### 迁移管理器设计
```go
type MigrationManager struct {
    db        *gorm.DB
    migrate   *migrate.Migrate
    migrations []*Migration
}

type Migration struct {
    Version     uint64
    Name        string
    UpFunc      func(*gorm.DB) error
    DownFunc    func(*gorm.DB) error
    Description string
}
```

## 实施计划

### 阶段1：基础设施搭建
1. **添加依赖**：引入 `github.com/golang-migrate/migrate/v4`
2. **创建迁移目录结构**：建立标准的迁移文件组织
3. **实现迁移管理器**：创建统一的迁移执行和回滚机制
4. **版本表设计**：使用标准的 `schema_migrations` 表

### 阶段2：现有迁移转换
1. **分析现有迁移**：梳理当前的 4 个迁移逻辑
2. **转换为 SQL 文件**：将 Go 逻辑转换为可执行的 SQL
3. **创建回滚脚本**：为每个迁移提供回滚能力
4. **数据迁移验证**：确保数据完整性

### 阶段3：集成到应用
1. **启动时执行迁移**：在应用启动前自动执行迁移
2. **错误处理**：完善迁移失败的处理机制
3. **日志记录**：详细记录迁移过程和结果
4. **健康检查**：添加迁移状态检查接口

### 阶段4：测试和文档
1. **单元测试**：为每个迁移编写测试
2. **集成测试**：测试完整的迁移流程
3. **回滚测试**：验证回滚机制的正确性
4. **文档更新**：更新部署和维护文档

## 关键迁移文件设计

### Reality Target 迁移示例
```sql
-- 000004_reality_target.up.sql
-- 修复 Reality 配置中缺少端口号的 target
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.google.com"', 
    '"target":"www.google.com:443"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.google.com';

UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.amazon.com"', 
    '"target":"www.amazon.com:443"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.amazon.com';
```

```sql
-- 000004_reality_target.down.sql
-- 回滚 Reality target 端口修复
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.google.com:443"', 
    '"target":"www.google.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.google.com';

UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.amazon.com:443"', 
    '"target":"www.amazon.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.amazon.com';
```

## 风险评估和缓解

### 潜在风险
1. **数据丢失风险**：迁移过程中的数据操作
2. **服务中断风险**：迁移执行期间的服务可用性
3. **回滚复杂性**：复杂迁移的回滚难度
4. **兼容性问题**：新旧版本数据库结构的兼容性

### 缓解措施
1. **备份策略**：迁移前自动备份数据库
2. **分阶段执行**：小步骤、可验证的迁移
3. **测试环境验证**：在测试环境充分验证后再生产执行
4. **监控和告警**：迁移过程的实时监控

## 预期收益

### 技术收益
- ✅ **标准化流程**：遵循行业最佳实践
- ✅ **可维护性**：清晰的迁移历史和版本控制
- ✅ **可测试性**：独立的迁移测试
- ✅ **可回滚性**：安全的回滚机制

### 业务收益
- ✅ **部署可靠性**：减少部署过程中的数据库问题
- ✅ **开发效率**：标准化迁移流程提高开发效率
- ✅ **运维便利**：清晰的迁移状态和操作日志

## 时间估算

- **阶段1**：2-3天（基础设施）
- **阶段2**：3-4天（迁移转换）
- **阶段3**：2-3天（应用集成）
- **阶段4**：2-3天（测试文档）

**总计**：9-13天

## 成功标准

1. 所有现有迁移成功转换为新的迁移系统
2. 新的迁移系统支持完整的 Up/Down 操作
3. 应用启动时自动执行待处理的迁移
4. 提供迁移状态查询和管理接口
5. 完善的测试覆盖和文档
