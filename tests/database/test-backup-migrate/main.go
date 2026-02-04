package main

import (
	"fmt"
	"log"

	"x-ui/database"
)

func main() {
	// 测试带备份的迁移
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
