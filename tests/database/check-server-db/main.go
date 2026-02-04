package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 打开数据库
	// 获取数据库文件的绝对路径
	dbPath := "/home/ub/X-Panel/database/test_simulation.db"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 获取所有表
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("数据库中的表:")
	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Fatal(err)
		}
		tables = append(tables, tableName)
		fmt.Printf("- %s\n", tableName)
	}

	// 检查schema_migrations表
	fmt.Println("\n检查schema_migrations表:")
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		log.Printf("查询schema_migrations失败: %v", err)
	} else {
		fmt.Printf("schema_migrations表中有 %d 条记录\n", count)
	}

	// 获取迁移版本
	fmt.Println("\n迁移记录:")
	rows, err = db.Query("SELECT version, dirty FROM schema_migrations ORDER BY version")
	if err != nil {
		log.Printf("查询迁移记录失败: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var version int
			var dirty bool
			if err := rows.Scan(&version, &dirty); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("- 版本: %d, 脏状态: %v\n", version, dirty)
		}
	}

	// 检查用户表
	fmt.Println("\n用户表记录数:")
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Printf("查询用户表失败: %v", err)
	} else {
		fmt.Printf("用户表中有 %d 条记录\n", count)
	}

	// 检查入站表
	fmt.Println("\n入站表记录数:")
	err = db.QueryRow("SELECT COUNT(*) FROM inbounds").Scan(&count)
	if err != nil {
		log.Printf("查询入站表失败: %v", err)
	} else {
		fmt.Printf("入站表中有 %d 条记录\n", count)
	}

	fmt.Println("\n数据库检查完成!")
}
