#!/bin/bash

echo "=== 创建 golang-migrate 集成提交 ==="

# 添加所有修改的文件
echo "添加修改的文件..."
git add database/migrate.go
git add database/db.go
git add database/migrate_startup_test.go
git add database/migrations/
git add cmd/test-*.go

# 检查状态
echo "检查 git 状态..."
git status --short

# 创建提交
echo "创建提交..."
git commit -m "feat: 集成 golang-migrate 数据库迁移系统

- 实现完整的数据库迁移管理器
- 添加迁移状态检查和报告功能
- 实现自动备份和回滚机制
- 添加迁移成功验证功能
- 实现严格数据一致性检查
- 添加智能日志记录系统
- 支持环境变量配置
- 完善错误处理和恢复机制
- 添加全面的单元测试
- 验证旧版本数据库升级兼容性

主要功能:
- MigrationManager: 数据库迁移管理器
- CheckMigrationStatus: 迁移状态检查
- RunMigrationsWithBackup: 带备份的迁移执行
- ValidateMigrationSuccess: 迁移成功验证
- StrictDataConsistencyCheck: 严格数据一致性检查
- 智能日志记录: 生产/调试模式自适应

测试覆盖:
- 25+ 单元测试覆盖所有新功能
- 兼容性测试验证旧版本升级
- 完整启动流程测试
- 服务器真实数据库测试

技术特性:
- 支持 XUI_DB_PATH 和 XUI_MIGRATIONS_PATH 环境变量
- 自动检测数据库状态并执行相应操作
- 完善的错误处理和自动回滚
- 生产环境日志优化和调试模式详细信息
- 数据完整性验证和关系一致性检查

BREAKING CHANGE: 数据库初始化流程更新，但保持向后兼容"

echo "=== 提交完成 ==="
