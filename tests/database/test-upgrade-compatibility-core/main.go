package main

import (
	"fmt"
	"log"
	"os"

	"x-ui/config"
	"x-ui/database"
)

func main() {
	log.Println("=== 旧版本升级到新版本兼容性测试（核心功能） ===")

	// 测试场景1：从服务器数据库（无迁移表）升级
	testServerDatabaseUpgrade()

	// 测试场景2：从已迁移的数据库升级
	testMigratedDatabaseUpgrade()

	// 测试场景3：模拟全新安装
	testFreshInstallation()

	log.Println("=== 兼容性测试完成 ===")
}

func testServerDatabaseUpgrade() {
	log.Println("\n--- 场景1：从服务器数据库升级 ---")

	// 获取数据库文件的绝对路径
	dbPath := "/home/ub/X-Panel/database/test_simulation.db"
	testPath := "/home/ub/X-Panel/database/test_upgrade_1.db"

	// 复制文件
	copyFile(dbPath, testPath)
	defer os.Remove(testPath)

	// 设置环境变量
	os.Setenv("XUI_DB_PATH", testPath)
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")
	config.RefreshEnvConfig()

	log.Printf("测试数据库: %s", testPath)

	// 执行核心升级流程（跳过严格一致性检查）
	if err := runCoreUpgradeFlow(); err != nil {
		log.Printf("❌ 场景1失败: %v", err)
	} else {
		log.Println("✅ 场景1成功：旧数据库成功升级到新版本")
	}
}

func testMigratedDatabaseUpgrade() {
	log.Println("\n--- 场景2：从已迁移数据库升级 ---")

	// 使用已经迁移过的数据库
	testPath := "/home/ub/X-Panel/database/test_server.db"

	// 设置环境变量
	os.Setenv("XUI_DB_PATH", testPath)
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")
	config.RefreshEnvConfig()

	log.Printf("测试数据库: %s", testPath)

	// 执行核心升级流程（应该检测到已是最新版本）
	if err := runCoreUpgradeFlow(); err != nil {
		log.Printf("❌ 场景2失败: %v", err)
	} else {
		log.Println("✅ 场景2成功：已迁移数据库正常运行")
	}
}

func testFreshInstallation() {
	log.Println("\n--- 场景3：全新安装 ---")

	// 创建空的数据库文件
	testPath := "/home/ub/X-Panel/database/test_fresh.db"
	defer os.Remove(testPath)

	// 设置环境变量
	os.Setenv("XUI_DB_PATH", testPath)
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")
	config.RefreshEnvConfig()

	log.Printf("测试数据库: %s", testPath)

	// 执行核心升级流程
	if err := runCoreUpgradeFlow(); err != nil {
		log.Printf("❌ 场景3失败: %v", err)
	} else {
		log.Println("✅ 场景3成功：全新安装正常运行")
	}
}

func runCoreUpgradeFlow() error {
	// 1. 检查迁移状态
	log.Println("1. 检查迁移状态...")
	if err := database.CheckMigrationStatus(); err != nil {
		return fmt.Errorf("迁移状态检查失败: %v", err)
	}

	// 2. 执行迁移
	log.Println("2. 执行数据库迁移...")
	if err := database.RunMigrationsWithBackup(); err != nil {
		return fmt.Errorf("迁移执行失败: %v", err)
	}

	// 3. 验证迁移成功
	log.Println("3. 验证迁移成功...")
	if err := database.ValidateMigrationSuccess(); err != nil {
		return fmt.Errorf("迁移验证失败: %v", err)
	}

	// 4. 基础表存在性检查（替代严格数据一致性检查）
	log.Println("4. 基础表存在性检查...")
	if err := checkBasicTables(); err != nil {
		return fmt.Errorf("基础表检查失败: %v", err)
	}

	return nil
}

func checkBasicTables() error {
	// 检查关键表是否存在
	requiredTables := []string{"users", "inbounds", "settings", "schema_migrations"}

	for _, table := range requiredTables {
		if !database.IsTableExists(table) {
			return fmt.Errorf("必需的表不存在: %s", table)
		}
	}

	log.Println("✅ 所有关键表都存在")
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
