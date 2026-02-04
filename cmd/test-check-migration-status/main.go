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

	// 测试迁移状态检查
	log.Println("测试迁移状态检查...")
	err := database.CheckMigrationStatus()
	if err != nil {
		log.Fatalf("迁移状态检查失败: %v", err)
	}

	log.Println("迁移状态检查成功!")
}
