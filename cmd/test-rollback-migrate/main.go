package main

import (
	"fmt"
	"log"

	"x-ui/database"
)

func main() {
	// 首先执行迁移
	fmt.Println("首先执行迁移...")
	if err := database.RunMigrationsWithBackup(); err != nil {
		log.Fatalf("迁移失败: %v", err)
	}
	fmt.Println("迁移成功!")

	// 检查迁移状态
	status, err := database.GetMigrationStatus()
	if err != nil {
		log.Fatalf("获取迁移状态失败: %v", err)
	}
	fmt.Printf("迁移后状态: 版本=%d, dirty=%v, 备份=%s\n",
		status.CurrentVersion, status.Dirty, status.LastBackup)

	// 测试回滚
	fmt.Println("测试回滚...")
	if err := database.RollbackMigrations(); err != nil {
		log.Fatalf("回滚失败: %v", err)
	}
	fmt.Println("回滚成功!")

	// 再次检查状态
	status, err = database.GetMigrationStatus()
	if err != nil {
		log.Fatalf("获取迁移状态失败: %v", err)
	}
	fmt.Printf("回滚后状态: 版本=%d, dirty=%v, 备份=%s\n",
		status.CurrentVersion, status.Dirty, status.LastBackup)
}
