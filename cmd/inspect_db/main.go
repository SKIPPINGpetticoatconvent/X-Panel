package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 连接到数据库
	db, err := sql.Open("sqlite3", "/home/ub/X-Panel/database/test_simulation.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 获取所有表名
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("=== 数据库表结构 ===")
	var tables []string
	for rows.Next() {
		var tableName string
		err := rows.Scan(&tableName)
		if err != nil {
			log.Fatal(err)
		}
		tables = append(tables, tableName)
		fmt.Printf("表: %s\n", tableName)
	}

	// 获取每个表的结构
	for _, table := range tables {
		fmt.Printf("\n=== 表 %s 的结构 ===\n", table)
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name, dataType string
			var notNull, pk int
			var defaultValue interface{}
			err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("  %d | %-20s | %-10s | %d | %d | %v\n", cid, name, dataType, notNull, pk, defaultValue)
		}
	}

	// 获取一些示例数据
	fmt.Println("\n=== 示例数据 ===")

	// 如果有 users 表，显示一些数据
	rows, err = db.Query("SELECT COUNT(*) FROM users")
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			var count int
			rows.Scan(&count)
			fmt.Printf("users 表记录数: %d\n", count)

			if count > 0 {
				rows, err = db.Query("SELECT * FROM users LIMIT 3")
				if err == nil {
					defer rows.Close()
					fmt.Println("前3条用户记录:")
					for rows.Next() {
						var id int
						var name, email string
						rows.Scan(&id, &name, &email)
						fmt.Printf("  %d | %s | %s\n", id, name, email)
					}
				}
			}
		}
	}

	// 如果有 inbounds 表，显示一些数据
	rows, err = db.Query("SELECT COUNT(*) FROM inbounds")
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			var count int
			rows.Scan(&count)
			fmt.Printf("inbounds 表记录数: %d\n", count)

			if count > 0 {
				rows, err = db.Query("SELECT id, port, protocol FROM inbounds LIMIT 3")
				if err == nil {
					defer rows.Close()
					fmt.Println("前3条入站记录:")
					for rows.Next() {
						var id int
						var port int
						var protocol string
						rows.Scan(&id, &port, &protocol)
						fmt.Printf("  %d | %d | %s\n", id, port, protocol)
					}
				}
			}
		}
	}
}
