-- Reality Target 迁移回滚
-- 版本：4
-- 描述：回滚 Reality Target 迁移

-- 恢复 www.google.com:443 → www.google.com
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.google.com:443"', 
    '"target":"www.google.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.google.com:443';

-- 恢复 www.amazon.com:443 → www.amazon.com
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.amazon.com:443"', 
    '"target":"www.amazon.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.amazon.com:443';

-- 恢复其他域名
UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.microsoft.com:443"', 
    '"target":"www.microsoft.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.microsoft.com:443';

UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.apple.com:443"', 
    '"target":"www.apple.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.apple.com:443';

UPDATE inbounds 
SET stream_settings = REPLACE(
    stream_settings, 
    '"target":"www.cloudflare.com:443"', 
    '"target":"www.cloudflare.com"'
)
WHERE stream_settings LIKE '%reality%' 
AND json_extract(stream_settings, '$.realitySettings.target') = 'www.cloudflare.com:443';

-- 记录回滚执行
DELETE FROM schema_migrations 
WHERE version = 4;
