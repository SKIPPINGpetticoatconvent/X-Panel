-- XHTTP Flow 迁移
-- 版本：3
-- 描述：为 VLESS + XHTTP + TLS/Reality 的客户端添加 flow 字段

-- 为 VLESS 协议的 XHTTP + TLS/Reality 入站添加 flow 字段
UPDATE inbounds 
SET settings = REPLACE(
    REPLACE(
        settings,
        '"id":"', 
        '"id":"'
    ),
    '"email":"', 
    '"flow":"xtls-rprx-vision","email":"'
)
WHERE protocol = 'vless' 
AND stream_settings LIKE '%"network":"xhttp"%' 
AND (stream_settings LIKE '%"security":"tls"%' OR stream_settings LIKE '%"security":"reality"%')
AND settings NOT LIKE '%"flow":"xtls-rprx-vision"%';

-- 处理没有 email 字段的情况
UPDATE inbounds 
SET settings = REPLACE(
    settings,
    '"id":"', 
    '"id":"'
)
WHERE protocol = 'vless' 
AND stream_settings LIKE '%"network":"xhttp"%' 
AND (stream_settings LIKE '%"security":"tls"%' OR stream_settings LIKE '%"security":"reality"%')
AND settings NOT LIKE '%"flow":"xtls-rprx-vision"%';

-- 为没有 flow 字段的客户端添加 flow
UPDATE inbounds 
SET settings = REPLACE(
    settings,
    '}', 
    ',"flow":"xtls-rprx-vision"}'
)
WHERE protocol = 'vless' 
AND stream_settings LIKE '%"network":"xhttp"%' 
AND (stream_settings LIKE '%"security":"tls"%' OR stream_settings LIKE '%"security":"reality"%')
AND settings LIKE '"clients":['
AND settings NOT LIKE '%"flow"%';

-- 记录迁移执行
INSERT OR IGNORE INTO schema_migrations (version, dirty) 
VALUES (3, false);
