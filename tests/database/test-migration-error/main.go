package main

import (
	"log"
	"os"

	"x-ui/config"
	"x-ui/database"
)

func main() {
	// 设置环境变量
	os.Setenv("XUI_DB_FOLDER", "/tmp/test-x-ui/etc/x-ui")
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")

	// 刷新配置
	config.RefreshEnvConfig()

	// 测试迁移错误处理
	log.Println("测试迁移错误处理...")

	// 模拟迁移失败（通过设置无效的数据库路径）
	os.Setenv("XUI_DB_PATH", "/invalid/path/test.db")

	err := database.RunMigrationsWithBackup()
	if err != nil {
		log.Printf("预期的错误处理: %v", err)
		log.Println("迁移错误处理测试成功!")
	} else {
		log.Fatal("应该返回错误，但没有")
	}
}
