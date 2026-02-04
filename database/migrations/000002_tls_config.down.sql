-- TLS 配置迁移回滚
-- 版本：2
-- 描述：回滚 TLS 配置迁移

-- 注意：由于 TLS 配置迁移涉及复杂的 JSON 操作，完整回滚比较困难
-- 这里提供基本的回滚操作，但某些数据可能无法完全恢复到原始状态

-- 恢复 allowInsecure 字段（添加默认值 false）
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"tlsSettings":{', 
    '"tlsSettings":{"allowInsecure":false,'
)
WHERE stream_settings LIKE '%tlsSettings%' 
AND stream_settings NOT LIKE '%allowInsecure%';

-- 记录回滚执行
DELETE FROM schema_migrations 
WHERE version = 2;
