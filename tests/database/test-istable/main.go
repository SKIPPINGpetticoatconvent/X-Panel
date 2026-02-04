package main

import (
	"log"
	"os"

	"x-ui/database"
)

func main() {
	// 设置环境变量
	os.Setenv("XUI_DB_PATH", "/home/ub/X-Panel/database/test_server.db")

	// 测试 isTableExists 函数
	log.Println("=== 测试 isTableExists 函数 ===")

	tables := []string{"users", "inbounds", "settings", "schema_migrations", "nonexistent"}

	for _, table := range tables {
		exists := database.IsTableExists(table)
		log.Printf("表 %s 存在: %v", table, exists)
	}

	log.Println("=== 测试完成 ===")
}
