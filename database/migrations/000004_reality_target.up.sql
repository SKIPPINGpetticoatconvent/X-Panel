-- Reality Target 迁移
-- 版本：4
-- 描述：修复 Reality 配置中缺少端口号的 target 字段

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

-- 修复其他常见的域名（如果缺少端口）
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.microsoft.com"', 
    '"target":"www.microsoft.com:443"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.microsoft.com';

UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.apple.com"', 
    '"target":"www.apple.com:443"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.apple.com';

UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.cloudflare.com"', 
    '"target":"www.cloudflare.com:443"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.cloudflare.com';

-- 记录迁移执行
INSERT OR IGNORE INTO schema_migrations (version, dirty) 
VALUES (4, false);
