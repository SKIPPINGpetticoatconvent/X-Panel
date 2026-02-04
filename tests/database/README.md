# 数据库测试

本目录包含 X-Panel 数据库迁移系统的所有测试文件。

## 测试分类

### 🧪 单元测试
- `test-check-migration-status/` - 测试迁移状态检查功能
- `test-error-handling/` - 测试错误处理机制
- `test-migration-validation/` - 测试迁移验证功能
- `test-production-logging/` - 测试生产环境日志记录
- `test-strict-consistency/` - 测试严格数据一致性检查

### 🔧 集成测试
- `test-server-migration/` - 测试服务器数据库迁移
- `test-server-migration-simple/` - 简化的服务器迁移测试
- `test-initdb-compatibility/` - 测试 InitDB 启动流程兼容性
- `test-upgrade-compatibility/` - 测试升级兼容性
- `test-upgrade-compatibility-core/` - 核心升级兼容性测试

### 🛠️ 功能测试
- `test-backup-migrate/` - 测试备份和迁移功能
- `test-create-migrate/` - 测试迁移管理器创建
- `test-debug-migrate/` - 调试模式迁移测试
- `test-direct-db/` - 直接数据库连接测试
- `test-istable/` - 表存在性检查测试
- `test-migrate/` - 基础迁移测试
- `test-migration-error/` - 迁移错误处理测试
- `test-rollback/` - 迁移回滚测试
- `test-rollback-migrate/` - 回滚迁移测试
- `test-simple-migrate/` - 简单迁移测试

### 🔍 验证测试
- `check-migrated-db/` - 检查迁移后的数据库状态
- `check-server-db/` - 检查服务器数据库状态
- `debug-migrate/` - 调试迁移过程
- `debug-rollback/` - 调试回滚过程

## 运行测试

### 运行单个测试
```bash
# 运行服务器迁移测试
cd tests/database/test-server-migration
go run main.go

# 运行兼容性测试
cd tests/database/test-upgrade-compatibility
go run main.go
```

### 运行所有测试
```bash
# 运行所有数据库测试
for dir in tests/database/*/; do
  echo "Running test in $dir"
  cd "$dir"
  if [ -f "main.go" ]; then
    go run main.go
  fi
  cd - > /dev/null
done
```

## 测试说明

### 兼容性测试
- **场景1**: 从旧版本数据库升级到新版本
- **场景2**: 已迁移数据库正常运行
- **场景3**: 全新安装流程

### 功能验证
- **迁移状态检查**: 验证数据库迁移状态检测
- **自动备份**: 验证迁移前自动备份功能
- **迁移执行**: 验证迁移过程正确执行
- **迁移验证**: 验证迁移成功后的状态检查
- **数据一致性**: 验证数据完整性检查

### 错误处理
- **迁移失败**: 测试迁移失败时的处理
- **回滚机制**: 测试自动回滚功能
- **错误恢复**: 测试错误恢复流程

## 环境变量

测试使用以下环境变量：
- `XUI_DB_PATH`: 数据库文件路径
- `XUI_MIGRATIONS_PATH`: 迁移文件路径
- `XUI_DB_FOLDER`: 数据库文件夹路径

## 注意事项

1. 测试文件仅用于开发和验证，不应在生产环境中运行
2. 某些测试会创建临时数据库文件，测试完成后会自动清理
3. 运行测试前请确保相关依赖已正确安装
4. 测试过程中可能会创建备份文件，请定期清理

## 贡献

如需添加新的测试，请遵循以下规范：
1. 在相应的分类目录下创建测试
2. 使用描述性的目录名称
3. 在 README 中更新测试说明
4. 确保测试具有适当的错误处理和清理逻辑
