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

	// 测试生产环境日志记录
	log.Println("=== 生产环境日志记录测试 ===")

	// 模拟迁移开始
	database.LogMigrationProgress("migration_start", map[string]interface{}{
		"action":  "backup_and_migrate",
		"db_path": config.GetDBPath(),
	})

	// 模拟非关键步骤
	database.LogMigrationProgress("non_critical_step", map[string]interface{}{
		"step": "test",
	})

	// 模拟迁移完成
	database.LogMigrationProgress("migration_complete", map[string]interface{}{
		"version": 5,
		"backup":  "backup.file",
	})

	log.Println("生产环境日志记录测试完成!")
}
