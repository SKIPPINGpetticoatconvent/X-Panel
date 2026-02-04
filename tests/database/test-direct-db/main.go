package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 设置环境变量
	dbPath := "/home/ub/X-Panel/database/test_server.db"

	log.Printf("=== 直接测试数据库连接 ===")
	log.Printf("数据库路径: %s", dbPath)

	// 检查文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("数据库文件不存在: %s", dbPath)
	}

	// 直接使用 database/sql 连接
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// 获取所有表
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		log.Fatalf("查询表失败: %v", err)
	}
	defer rows.Close()

	log.Println("数据库中的表:")
	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Fatalf("扫描表名失败: %v", err)
		}
		tables = append(tables, tableName)
		fmt.Printf("- %s\n", tableName)
	}

	// 检查特定表
	testTables := []string{"users", "inbounds", "settings", "schema_migrations"}
	for _, table := range testTables {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			log.Printf("查询表 %s 失败: %v", table, err)
		} else {
			log.Printf("表 %s 存在: %v", table, count > 0)
		}
	}

	log.Println("=== 测试完成 ===")
}
