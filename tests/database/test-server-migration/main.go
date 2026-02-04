package main

import (
	"log"
	"os"
	"path/filepath"

	"x-ui/config"
	"x-ui/database"
)

func main() {
	// 获取数据库文件的绝对路径
	dbPath := "/home/ub/X-Panel/database/test_server.db"
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		log.Fatalf("获取绝对路径失败: %v", err)
	}

	log.Printf("数据库文件路径: %s", absPath)

	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		log.Fatalf("数据库文件不存在: %s", absPath)
	}

	// 设置环境变量使用服务器数据库
	os.Setenv("XUI_DB_PATH", absPath)
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")

	// 刷新配置
	config.RefreshEnvConfig()

	log.Println("=== 使用服务器真实数据库测试 golang-migrate ===")

	// 1. 创建迁移管理器测试
	log.Println("1. 创建迁移管理器测试...")
	manager, err := database.NewMigrationManager(absPath)
	if err != nil {
		log.Printf("创建迁移管理器失败: %v", err)
	} else {
		log.Println("迁移管理器创建成功")
		defer manager.Close()

		// 获取当前状态
		err = manager.Status()
		if err != nil {
			log.Printf("获取状态失败: %v", err)
		} else {
			log.Println("状态获取成功")
		}
	}

	// 2. 检查迁移状态
	log.Println("2. 检查迁移状态...")
	err = database.CheckMigrationStatus()
	if err != nil {
		log.Printf("迁移状态检查: %v", err)
	} else {
		log.Println("迁移状态检查完成")
	}

	// 3. 运行迁移（这会创建schema_migrations表并执行所有迁移）
	log.Println("3. 执行数据库迁移...")
	err = database.RunMigrationsWithBackup()
	if err != nil {
		log.Printf("迁移执行: %v", err)
	} else {
		log.Println("迁移执行完成")
	}

	// 4. 验证迁移成功
	log.Println("4. 验证迁移成功...")
	err = database.ValidateMigrationSuccess()
	if err != nil {
		log.Printf("迁移验证: %v", err)
	} else {
		log.Println("迁移验证完成")
	}

	// 5. 严格数据一致性检查
	log.Println("5. 严格数据一致性检查...")
	err = database.StrictDataConsistencyCheck()
	if err != nil {
		log.Printf("数据一致性检查: %v", err)
	} else {
		log.Println("数据一致性检查通过")
	}

	log.Println("=== 服务器数据库测试完成 ===")
}
