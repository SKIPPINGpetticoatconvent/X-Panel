package main

import (
	"fmt"
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

	// 测试配置加载
	fmt.Printf("数据库路径: %s\n", config.GetDBPath())

	// 现在测试迁移
	fmt.Println("测试带自动备份的迁移...")
	if err := database.RunMigrationsWithBackup(); err != nil {
		log.Fatalf("迁移失败: %v", err)
	}
	fmt.Println("迁移成功!")

	// 检查迁移状态
	status, err := database.GetMigrationStatus()
	if err != nil {
		log.Fatalf("获取迁移状态失败: %v", err)
	}
	fmt.Printf("迁移状态: 版本=%d, dirty=%v, 备份=%s\n",
		status.CurrentVersion, status.Dirty, status.LastBackup)
}
