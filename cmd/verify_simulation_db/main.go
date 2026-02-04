package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 连接到模拟数据库
	db, err := sql.Open("sqlite3", "/home/ub/X-Panel/database/test_simulation.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("=== 模拟数据库验证 ===")

	// 统计数据
	var count int

	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ 用户数量: %d\n", count)

	err = db.QueryRow("SELECT COUNT(*) FROM inbounds").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ 入站数量: %d\n", count)

	err = db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ 设置数量: %d\n", count)

	err = db.QueryRow("SELECT COUNT(*) FROM client_traffics").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ 客户端流量记录数: %d\n", count)

	err = db.QueryRow("SELECT COUNT(*) FROM outbound_traffics").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ 出站流量记录数: %d\n", count)

	// 检查迁移版本
	var version int
	var dirty bool
	err = db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ 迁移版本: %d, 脏状态: %v\n", version, dirty)

	// 显示一些示例数据
	fmt.Println("\n=== 示例数据 ===")

	rows, err := db.Query("SELECT username, email FROM users LIMIT 3")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("前3个用户:")
	for rows.Next() {
		var username, email string
		err := rows.Scan(&username, &email)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  - %s (%s)\n", username, email)
	}

	rows, err = db.Query("SELECT port, protocol, tag FROM inbounds LIMIT 3")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("前3个入站:")
	for rows.Next() {
		var port int
		var protocol, tag string
		err := rows.Scan(&port, &protocol, &tag)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  - %d (%s) - %s\n", port, protocol, tag)
	}

	fmt.Println("\n✅ 模拟数据库验证完成！")
	fmt.Println("✅ 数据库包含丰富的测试数据，可用于测试和开发")
}
