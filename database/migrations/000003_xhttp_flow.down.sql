-- XHTTP Flow 迁移回滚
-- 版本：3
-- 描述：回滚 XHTTP Flow 迁移

-- 移除添加的 flow 字段
UPDATE inbounds 
SET settings = REPLACE(settings, '"flow":"xtls-rprx-vision",', '')
WHERE protocol = 'vless' 
AND stream_settings LIKE '%"network":"xhttp"%' 
AND (stream_settings LIKE '%"security":"tls"%' OR stream_settings LIKE '%"security":"reality"%');

UPDATE inbounds 
SET settings = REPLACE(settings, ',"flow":"xtls-rprx-vision"', '')
WHERE protocol = 'vless' 
AND stream_settings LIKE '%"network":"xhttp"%' 
AND (stream_settings LIKE '%"security":"tls"%' OR stream_settings LIKE '%"security":"reality"%');

UPDATE inbounds 
SET settings = REPLACE(settings, '"flow":"xtls-rprx-vision"', '')
WHERE protocol = 'vless' 
AND stream_settings LIKE '%"network":"xhttp"%' 
AND (stream_settings LIKE '%"security":"tls"%' OR stream_settings LIKE '%"security":"reality"%');

-- 记录回滚执行
DELETE FROM schema_migrations 
WHERE version = 3;
