-- 初始化数据库架构
-- 版本：1
-- 描述：创建所有必要的数据表

-- 用户表
CREATE TABLE IF NOT EXISTS "users" (
    "id" integer PRIMARY KEY AUTOINCREMENT,
    "username" text NOT NULL UNIQUE,
    "password" text NOT NULL,
    "created_at" datetime DEFAULT CURRENT_TIMESTAMP,
    "updated_at" datetime DEFAULT CURRENT_TIMESTAMP
);

-- 入站表
CREATE TABLE IF NOT EXISTS "inbounds" (
    "id" integer PRIMARY KEY AUTOINCREMENT,
    "user_id" integer,
    "up" integer DEFAULT 0,
    "down" integer DEFAULT 0,
    "total" integer DEFAULT 0,
    "all_time" integer DEFAULT 0,
    "remark" text,
    "enable" numeric DEFAULT 1,
    "expiry_time" integer DEFAULT 0,
    "device_limit" integer DEFAULT 0,
    "listen" text,
    "port" integer,
    "protocol" text,
    "settings" text,
    "stream_settings" text,
    "tag" text,
    "sniffing" text,
    CONSTRAINT "uni_inbounds_tag" UNIQUE ("tag")
);

-- 出站流量表
CREATE TABLE IF NOT EXISTS "outbound_traffics" (
    "id" integer PRIMARY KEY AUTOINCREMENT,
    "user_id" integer,
    "up" integer DEFAULT 0,
    "down" integer DEFAULT 0,
    "created_at" datetime DEFAULT CURRENT_TIMESTAMP
);

-- 设置表
CREATE TABLE IF NOT EXISTS "settings" (
    "key" text PRIMARY KEY,
    "value" text
);

-- 入站客户端IP表
CREATE TABLE IF NOT EXISTS "inbound_client_ips" (
    "id" integer PRIMARY KEY AUTOINCREMENT,
    "inbound_id" integer,
    "client_ip" text,
    "client_time" datetime DEFAULT CURRENT_TIMESTAMP
);

-- 客户端流量表
CREATE TABLE IF NOT EXISTS "client_traffics" (
    "id" integer PRIMARY KEY AUTOINCREMENT,
    "user_id" integer,
    "inbound_id" integer,
    "email" text,
    "upload" integer DEFAULT 0,
    "download" integer DEFAULT 0,
    "total" integer DEFAULT 0,
    "expiry_time" integer DEFAULT 0,
    "device_limit" integer DEFAULT 0,
    "created_at" datetime DEFAULT CURRENT_TIMESTAMP
);

-- 历史记录表
CREATE TABLE IF NOT EXISTS "history_of_seeders" (
    "id" integer PRIMARY KEY AUTOINCREMENT,
    "seeder_name" text
);

-- 链接历史表
CREATE TABLE IF NOT EXISTS "link_histories" (
    "id" integer PRIMARY KEY AUTOINCREMENT,
    "user_id" integer,
    "link_type" text,
    "link_content" text,
    "created_at" datetime DEFAULT CURRENT_TIMESTAMP
);
