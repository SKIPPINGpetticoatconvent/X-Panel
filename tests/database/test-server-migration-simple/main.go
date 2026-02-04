package main

import (
	"log"
	"os"

	"x-ui/config"
	"x-ui/database"
)

func main() {
	// 获取数据库文件的绝对路径
	dbPath := "/home/ub/X-Panel/database/test_simulation.db"
	absPath, err := os.Stat(dbPath)
	if err != nil {
		log.Fatalf("数据库文件不存在: %v", err)
	}

	log.Printf("=== 使用服务器真实数据库测试 golang-migrate ===")
	log.Printf("数据库文件路径: %s", absPath.Name())

	// 设置环境变量使用服务器数据库
	os.Setenv("XUI_DB_PATH", absPath.Name())
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")

	// 刷新配置
	config.RefreshEnvConfig()

	log.Println("1. 检查迁移状态...")
	err = database.CheckMigrationStatus()
	if err != nil {
		log.Printf("迁移状态检查: %v", err)
	} else {
		log.Println("✅ 迁移状态检查完成")
	}

	log.Println("2. 执行数据库迁移...")
	err = database.RunMigrationsWithBackup()
	if err != nil {
		log.Printf("迁移执行: %v", err)
	} else {
		log.Println("✅ 迁移执行完成")
	}

	log.Println("3. 验证迁移成功...")
	err = database.ValidateMigrationSuccess()
	if err != nil {
		log.Printf("迁移验证: %v", err)
	} else {
		log.Println("✅ 迁移验证完成")
	}

	log.Println("=== 服务器数据库测试完成 ===")
}
