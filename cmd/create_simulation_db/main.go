package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 创建新的模拟数据库
	dbPath := "/home/ub/X-Panel/database/test_simulation.db"

	// 删除现有文件（如果存在）
	os.Remove(dbPath)

	// 连接到数据库
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("=== 创建模拟数据库 ===")
	fmt.Printf("数据库路径: %s\n", dbPath)

	// 创建表结构
	createTables(db)

	// 插入模拟数据
	insertTestData(db)

	fmt.Println("✅ 模拟数据库创建完成")
}

func createTables(db *sql.DB) {
	fmt.Println("\n=== 创建表结构 ===")

	// 创建 users 表
	_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			email TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_active BOOLEAN DEFAULT 1
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 users 表")

	// 创建 inbounds 表
	_, err = db.Exec(`
		CREATE TABLE inbounds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			port INTEGER NOT NULL UNIQUE,
			protocol TEXT NOT NULL,
			settings TEXT,
			tag TEXT,
			remark TEXT,
			enable BOOLEAN DEFAULT 1,
			expiry_time INTEGER,
			device_limit INTEGER DEFAULT 0,
			listen TEXT,
			stream_settings TEXT,
			sniffing TEXT,
			up INTEGER DEFAULT 0,
			down INTEGER DEFAULT 0,
			total INTEGER DEFAULT 0,
			all_time INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 inbounds 表")

	// 创建 settings 表
	_, err = db.Exec(`
		CREATE TABLE settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT UNIQUE NOT NULL,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 settings 表")

	// 创建 client_traffics 表
	_, err = db.Exec(`
		CREATE TABLE client_traffics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			inbound_id INTEGER NOT NULL,
			enable BOOLEAN DEFAULT 1,
			email TEXT,
			up INTEGER DEFAULT 0,
			down INTEGER DEFAULT 0,
			all_time INTEGER DEFAULT 0,
			expiry_time INTEGER,
			total INTEGER DEFAULT 0,
			reset INTEGER DEFAULT 0,
			last_online INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (inbound_id) REFERENCES inbounds(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 client_traffics 表")

	// 创建 outbound_traffics 表
	_, err = db.Exec(`
		CREATE TABLE outbound_traffics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tag TEXT NOT NULL,
			up INTEGER DEFAULT 0,
			down INTEGER DEFAULT 0,
			total INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 outbound_traffics 表")

	// 创建 inbound_client_ips 表
	_, err = db.Exec(`
		CREATE TABLE inbound_client_ips (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_email TEXT,
			ips TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 inbound_client_ips 表")

	// 创建 history_of_seeders 表
	_, err = db.Exec(`
		CREATE TABLE history_of_seeders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			seeder_name TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 history_of_seeders 表")

	// 创建 link_histories 表
	_, err = db.Exec(`
		CREATE TABLE link_histories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type VARCHAR(255) NOT NULL,
			link TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 link_histories 表")

	// 创建 schema_migrations 表
	_, err = db.Exec(`
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			dirty BOOLEAN NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建 schema_migrations 表")

	// 创建索引
	_, err = db.Exec(`
		CREATE INDEX idx_inbounds_user_id ON inbounds(user_id);
		CREATE INDEX idx_inbounds_port ON inbounds(port);
		CREATE INDEX idx_client_traffics_inbound_id ON client_traffics(inbound_id);
		CREATE INDEX idx_client_traffics_email ON client_traffics(email);
		CREATE INDEX idx_settings_key ON settings(key);
	`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 创建索引")
}

func insertTestData(db *sql.DB) {
	fmt.Println("\n=== 插入模拟数据 ===")

	// 插入用户数据
	users := [][3]string{
		{"admin", "$2a$10$92IXUNpkjOzOxxQqyUP1IuV.z/kEaj2Fqs8jI9MkMjI3Esl3RTFdd", "admin@example.com"},
		{"user1", "$2a$10$N.zmdr9k7uOCQJ379N17O1NedW9n8QI8I9MkMjI3Esl3RTFdd", "user1@example.com"},
		{"user2", "$2a$10$N.zmdr9k7uOCQJ379N17O1NedW9n8QI8I9MkMjI3Esl3RTFdd", "user2@example.com"},
		{"user3", "$2a$10$N.zmdr9k7uOCQJ379N17O1NedW9n8QI8I9MkMjI3Esl3RTFdd", "user3@example.com"},
		{"user4", "$2a$10$N.zmdr9k7uOCQJ379N17O1NedW9n8QI8I9MkMjI3Esl3RTFdd", "user4@example.com"},
		{"user5", "$2a$10$N.zmdr9k7uOCQJ379N17O1NedW9n8QI8I9MkMjI3Esl3RTFdd", "user5@example.com"},
	}

	for _, user := range users {
		_, err := db.Exec(`
			INSERT INTO users (username, password, email, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?)
		`, user[0], user[1], user[2], time.Now(), time.Now())
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("✅ 插入 %d 个用户\n", len(users))

	// 插入设置数据
	settings := [][2]string{
		{"webListen", "0.0.0.0:54321"},
		{"webPort", "54321"},
		{"webCertFile", ""},
		{"webKeyFile", ""},
		{"webBasePath", "/"},
		{"webSessionTimeout", "30"},
		{"maxUser", "5"},
		{"xrayTemplateConfig", ""},
		{"telegramBotToken", ""},
		{"telegramBotChatId", ""},
		{"telegramBotNotify", "false"},
		{"telegramBotNotifyInterval", "60"},
	}

	for _, setting := range settings {
		_, err := db.Exec(`
			INSERT INTO settings (key, value, created_at, updated_at) 
			VALUES (?, ?, ?, ?)
		`, setting[0], setting[1], time.Now(), time.Now())
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("✅ 插入 %d 个设置\n", len(settings))

	// 插入入站数据
	inbounds := [][7]interface{}{
		{1, 10080, "vmess", `{"id": "1", "level": "5"}`, "vmess-10080", "VMess 10080", "0.0.0.0"},
		{1, 10081, "vless", `{"id": "2", "level": "5"}`, "vless-10081", "VLESS 10081", "0.0.0.0"},
		{1, 10082, "trojan", `{"id": "3", "level": "5"}`, "trojan-10082", "Trojan 10082", "0.0.0.0"},
		{1, 10083, "shadowsocks", `{"id": "4", "level": "5"}`, "ss-10083", "Shadowsocks 10083", "0.0.0.0"},
		{1, 10084, "dokodemo-door", `{"id": "5", "level": "5"}`, "dd-10084", "Dokodemo-door 10084", "0.0.0.0"},
		{2, 20080, "vmess", `{"id": "6", "level": "5"}`, "vmess-20080", "VMess 20080", "0.0.0.0"},
		{2, 20081, "vless", `{"id": "7", "level": "5"}`, "vless-20081", "VLESS 20081", "0.0.0.0"},
		{2, 20082, "trojan", `{"id": "8", "level": "5"}`, "trojan-20082", "Trojan 20082", "0.0.0.0"},
		{3, 30080, "vmess", `{"id": "9", "level": "5"}`, "vmess-30080", "VMess 30080", "0.0.0.0"},
		{3, 30081, "vless", `{"id": "10", "level": "5"}`, "vless-30081", "VLESS 30081", "0.0.0.0"},
		{4, 40080, "vmess", `{"id": "11", "level": "5"}`, "vmess-40080", "VMess 40080", "0.0.0.0"},
		{4, 40081, "vless", `{"id": "12", "level": "5"}`, "vless-40081", "VLESS 40081", "0.0.0.0"},
		{5, 50080, "vmess", `{"id": "13", "level": "5"}`, "vmess-50080", "VMess 50080", "0.0.0.0"},
		{5, 50081, "vless", `{"id": "14", "level": "5"}`, "vless-50081", "VLESS 50081", "0.0.0.0"},
	}

	for _, inbound := range inbounds {
		_, err := db.Exec(`
			INSERT INTO inbounds (user_id, port, protocol, settings, tag, remark, listen, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, inbound[0], inbound[1], inbound[2], inbound[3], inbound[4], inbound[5], inbound[6], time.Now(), time.Now())
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("✅ 插入 %d 个入站\n", len(inbounds))

	// 插入客户端流量数据
	clientTraffics := [][5]interface{}{
		{1, "client1@example.com", 1024, 2048, 3072},
		{2, "client2@example.com", 2048, 4096, 6144},
		{3, "client3@example.com", 512, 1024, 1536},
		{4, "client4@example.com", 1024, 2048, 3072},
		{5, "client5@example.com", 256, 512, 768},
		{6, "client6@example.com", 128, 256, 384},
		{7, "client7@example.com", 64, 128, 192},
		{8, "client8@example.com", 32, 64, 96},
		{9, "client9@example.com", 16, 32, 48},
		{10, "client10@example.com", 8, 16, 24},
	}

	for _, traffic := range clientTraffics {
		_, err := db.Exec(`
			INSERT INTO client_traffics (inbound_id, email, up, down, total, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, traffic[0], traffic[1], traffic[2], traffic[3], traffic[4], time.Now(), time.Now())
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("✅ 插入 %d 个客户端流量记录\n", len(clientTraffics))

	// 插入出站流量数据
	outboundTraffics := [][4]interface{}{
		{"vmess-10080", 2048, 4096, 6144},
		{"vless-10081", 1024, 2048, 3072},
		{"trojan-10082", 512, 1024, 1536},
		{"ss-10083", 256, 512, 768},
		{"dd-10084", 128, 256, 384},
		{"vmess-20080", 4096, 8192, 12288},
		{"vless-20081", 2048, 4096, 6144},
		{"trojan-20082", 1024, 2048, 3072},
		{"vmess-30080", 1024, 2048, 3072},
		{"vless-30081", 512, 1024, 1536},
		{"vmess-40080", 256, 512, 768},
		{"vless-40081", 128, 256, 384},
		{"vmess-50080", 64, 128, 192},
		{"vless-50081", 32, 64, 96},
	}

	for _, traffic := range outboundTraffics {
		_, err := db.Exec(`
			INSERT INTO outbound_traffics (tag, up, down, total, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?)
		`, traffic[0], traffic[1], traffic[2], traffic[3], time.Now(), time.Now())
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("✅ 插入 %d 个出站流量记录\n", len(outboundTraffics))

	// 插入历史记录
	historyRecords := []string{
		"seeder1",
		"seeder2",
		"seeder3",
	}

	for _, record := range historyRecords {
		_, err := db.Exec(`
			INSERT INTO history_of_seeders (seeder_name, created_at) 
			VALUES (?, ?)
		`, record, time.Now())
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("✅ 插入 %d 个历史记录\n", len(historyRecords))

	// 插入链接历史记录
	linkHistories := [][2]string{
		{"subscription", "https://example.com/sub/test1"},
		{"subscription", "https://example.com/sub/test2"},
		{"subscription", "https://example.com/sub/test3"},
		{"share", "https://example.com/share/test1"},
		{"share", "https://example.com/share/test2"},
	}

	for _, record := range linkHistories {
		_, err := db.Exec(`
			INSERT INTO link_histories (type, link, created_at, updated_at) 
			VALUES (?, ?, ?, ?)
		`, record[0], record[1], time.Now(), time.Now())
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("✅ 插入 %d 个链接历史记录\n", len(linkHistories))

	// 插入迁移版本
	_, err := db.Exec(`
		INSERT INTO schema_migrations (version, dirty) 
		VALUES (?, ?)
	`, 5, false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ 插入迁移版本 5\n")

	fmt.Printf("\n=== 数据统计 ===\n")

	// 统计数据
	var count int

	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("用户数量: %d\n", count)

	err = db.QueryRow("SELECT COUNT(*) FROM inbounds").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("入站数量: %d\n", count)

	err = db.QueryRow("SELECT COUNT(*) FROM client_traffics").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("客户端流量记录数: %d\n", count)

	err = db.QueryRow("SELECT COUNT(*) FROM outbound_traffics").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("出站流量记录数: %d\n", count)

	err = db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("设置数量: %d\n", count)

	fmt.Printf("✅ 模拟数据库创建完成，包含丰富的测试数据\n")
}
