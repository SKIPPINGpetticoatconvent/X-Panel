-- TLS 配置迁移
-- 版本：2
-- 描述：迁移 TLS 配置，移除不安全的设置并更新字段格式

-- 迁移 verifyPeerCertInNames → verifyPeerCertByName
UPDATE inbounds 
SET stream_settings = REPLACE(
    REPLACE(
        REPLACE(stream_settings, '"verifyPeerCertInNames":', '"verifyPeerCertByName":'),
        '["', '"'
    ),
    '"]', '"'
)
WHERE stream_settings LIKE '%verifyPeerCertInNames%';

-- 移除 allowInsecure 字段
UPDATE inbounds 
SET stream_settings = REPLACE(stream_settings, '"allowInsecure":true,', '')
WHERE stream_settings LIKE '%"allowInsecure":true%';

UPDATE inbounds 
SET stream_settings = REPLACE(stream_settings, '"allowInsecure":false,', '')
WHERE stream_settings LIKE '%"allowInsecure":false%';

UPDATE inbounds 
SET stream_settings = REPLACE(stream_settings, '"allowInsecure":true', '')
WHERE stream_settings LIKE '%"allowInsecure":true%';

UPDATE inbounds 
SET stream_settings = REPLACE(stream_settings, '"allowInsecure":false', '')
WHERE stream_settings LIKE '%"allowInsecure":false%';

-- 迁移 pinnedPeerCertSha256 分隔符 ~ → ,
UPDATE inbounds 
SET stream_settings = REPLACE(stream_settings, '"pinnedPeerCertSha256":"', '"pinnedPeerCertSha256":"')
WHERE stream_settings LIKE '%pinnedPeerCertSha256%';

UPDATE inbounds 
SET stream_settings = REPLACE(stream_settings, '~', ',')
WHERE stream_settings LIKE '%pinnedPeerCertSha256%' 
AND stream_settings LIKE '%~%';

-- 清理可能的 JSON 格式问题
UPDATE inbounds 
SET stream_settings = REPLACE(stream_settings, ',,', ',')
WHERE stream_settings LIKE '%,,%';

UPDATE inbounds 
SET stream_settings = REPLACE(stream_settings, ',,', ',')
WHERE stream_settings LIKE '%,,%';

-- 记录迁移执行
INSERT OR IGNORE INTO schema_migrations (version, dirty) 
VALUES (2, false);
