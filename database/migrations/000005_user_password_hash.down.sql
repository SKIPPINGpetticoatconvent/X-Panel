-- 用户密码哈希迁移回滚
-- 版本：5
-- 描述：回滚用户密码哈希迁移

-- 注意：由于 bcrypt 是单向哈希，无法完全回滚到原始明文密码
-- 这里只能移除哈希标记，实际密码需要用户重新设置

-- 移除哈希标记（注意：这不会恢复原始密码）
UPDATE users 
SET password = REPLACE(password, 'HASHED:', '')
WHERE password LIKE 'HASHED:%';

-- 记录回滚执行
-- 注意：这里使用正确的表名 schema_migrations
DELETE FROM schema_migrations 
WHERE version = 5;
