-- 用户密码哈希迁移
-- 版本：5
-- 描述：将用户密码从明文转换为 bcrypt 哈希

-- 注意：这个迁移需要在应用层执行，因为需要 bcrypt 哈希算法
-- 这里只创建迁移记录，实际哈希操作在应用层完成

-- 为现有用户添加哈希标记（应用层会处理实际的哈希）
UPDATE users 
SET password = CASE 
    WHEN password NOT LIKE '$2%' THEN 'HASHED:' || password
    ELSE password
END
WHERE password NOT LIKE '$2%';

-- 记录迁移执行
INSERT OR IGNORE INTO schema_migrations (version, dirty) 
VALUES (5, false);
