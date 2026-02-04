-- 回滚初始化数据库架构
-- 版本：1
-- 描述：删除所有创建的表

DROP TABLE IF EXISTS "link_histories";
DROP TABLE IF EXISTS "history_of_seeders";
DROP TABLE IF EXISTS "client_traffics";
DROP TABLE IF EXISTS "inbound_client_ips";
DROP TABLE IF EXISTS "settings";
DROP TABLE IF EXISTS "outbound_traffics";
DROP TABLE IF EXISTS "inbounds";
DROP TABLE IF EXISTS "users";
