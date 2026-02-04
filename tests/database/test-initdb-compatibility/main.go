package main

import (
	"fmt"
	"log"
	"os"

	"x-ui/config"
	"x-ui/database"
)

func main() {
	log.Println("=== 完整 InitDB 启动流程兼容性测试 ===")

	// 测试场景1：旧版本数据库启动
	testOldVersionStartup()

	// 测试场景2：已迁移数据库启动
	testMigratedStartup()

	// 测试场景3：全新安装启动
	testFreshStartup()

	log.Println("=== 完整启动流程测试完成 ===")
}

func testOldVersionStartup() {
	log.Println("\n--- 场景1：旧版本数据库启动 ---")

	// 复制原始服务器数据库
	srcPath := "/home/ub/X-Panel/database/test_server.db"
	testPath := "/home/ub/X-Panel/database/test_startup_old.db"

	copyFile(srcPath, testPath)
	defer os.Remove(testPath)

	// 设置环境变量
	os.Setenv("XUI_DB_PATH", testPath)
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")
	config.RefreshEnvConfig()

	log.Printf("测试数据库: %s", testPath)

	// 模拟 InitDB 流程（不包含用户初始化部分）
	if err := simulateInitDB(); err != nil {
		log.Printf("❌ 场景1失败: %v", err)
	} else {
		log.Println("✅ 场景1成功：旧版本数据库启动正常")
	}
}

func testMigratedStartup() {
	log.Println("\n--- 场景2：已迁移数据库启动 ---")

	// 使用已迁移的数据库
	testPath := "/home/ub/X-Panel/database/test_server.db"

	// 设置环境变量
	os.Setenv("XUI_DB_PATH", testPath)
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")
	config.RefreshEnvConfig()

	log.Printf("测试数据库: %s", testPath)

	// 模拟 InitDB 流程
	if err := simulateInitDB(); err != nil {
		log.Printf("❌ 场景2失败: %v", err)
	} else {
		log.Println("✅ 场景2成功：已迁移数据库启动正常")
	}
}

func testFreshStartup() {
	log.Println("\n--- 场景3：全新安装启动 ---")

	// 创建空数据库
	testPath := "/home/ub/X-Panel/database/test_startup_fresh.db"
	defer os.Remove(testPath)

	// 设置环境变量
	os.Setenv("XUI_DB_PATH", testPath)
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")
	config.RefreshEnvConfig()

	log.Printf("测试数据库: %s", testPath)

	// 模拟 InitDB 流程
	if err := simulateInitDB(); err != nil {
		log.Printf("❌ 场景3失败: %v", err)
	} else {
		log.Println("✅ 场景3成功：全新安装启动正常")
	}
}

func simulateInitDB() error {
	log.Println("模拟 InitDB 启动流程...")

	// 1. 检查迁移状态
	log.Println("1. 检查迁移状态...")
	if err := database.CheckMigrationStatus(); err != nil {
		return err
	}

	// 2. 执行数据库迁移（带自动备份和回滚）
	log.Println("2. 执行数据库迁移...")
	if err := database.RunMigrationsWithBackup(); err != nil {
		return err
	}

	// 3. 验证迁移成功
	log.Println("3. 验证迁移成功...")
	if err := database.ValidateMigrationSuccess(); err != nil {
		return err
	}

	// 4. 基础数据一致性检查
	log.Println("4. 基础数据一致性检查...")
	if err := checkBasicDataConsistency(); err != nil {
		return err
	}

	log.Println("✅ InitDB 流程模拟成功")
	return nil
}

func checkBasicDataConsistency() error {
	// 检查关键表是否存在
	requiredTables := []string{"users", "inbounds", "settings", "schema_migrations"}

	for _, table := range requiredTables {
		if !database.IsTableExists(table) {
			return fmt.Errorf("必需的表不存在: %s", table)
		}
	}

	// 检查迁移版本
	status, err := database.GetMigrationStatus()
	if err != nil {
		return fmt.Errorf("获取迁移状态失败: %v", err)
	}

	if status.CurrentVersion != 5 {
		return fmt.Errorf("迁移版本不正确: 期望 5，实际 %d", status.CurrentVersion)
	}

	if status.Dirty {
		return fmt.Errorf("数据库处于脏状态")
	}

	log.Printf("✅ 数据一致性检查通过 - 版本: %d, 状态: 正常", status.CurrentVersion)
	return nil
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		log.Fatalf("读取源文件失败: %v", err)
	}

	err = os.WriteFile(dst, data, 0o644)
	if err != nil {
		log.Fatalf("写入目标文件失败: %v", err)
	}

	log.Printf("文件复制成功: %s -> %s", src, dst)
}
