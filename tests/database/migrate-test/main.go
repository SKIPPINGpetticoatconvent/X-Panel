package main

import (
	"fmt"
	"log"
	"os"

	"x-ui/database"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法:")
		fmt.Println("  migrate-test up     - 执行迁移")
		fmt.Println("  migrate-test down   - 回滚迁移")
		fmt.Println("  migrate-test status - 查看迁移状态")
		fmt.Println("  migrate-test backup - 备份数据库")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "up":
		fmt.Println("执行数据库迁移...")
		if err := database.RunMigrations(); err != nil {
			log.Fatalf("迁移失败: %v", err)
		}
		fmt.Println("迁移完成！")

	case "down":
		fmt.Println("回滚数据库迁移...")
		dbPath := "/etc/x-ui/x-ui.db"
		manager, err := database.NewMigrationManager(dbPath)
		if err != nil {
			log.Fatalf("创建迁移管理器失败: %v", err)
		}
		defer manager.Close()

		if err := manager.Down(); err != nil {
			log.Fatalf("回滚失败: %v", err)
		}
		fmt.Println("回滚完成！")

	case "status":
		fmt.Println("检查迁移状态...")
		dbPath := "/etc/x-ui/x-ui.db"
		manager, err := database.NewMigrationManager(dbPath)
		if err != nil {
			log.Fatalf("创建迁移管理器失败: %v", err)
		}
		defer manager.Close()

		if err := manager.Status(); err != nil {
			log.Fatalf("检查状态失败: %v", err)
		}
		fmt.Println("状态检查完成！")

	case "backup":
		fmt.Println("备份数据库...")
		if err := database.BackupDatabase(); err != nil {
			log.Fatalf("备份失败: %v", err)
		}
		fmt.Println("备份完成！")

	default:
		fmt.Printf("未知命令: %s\n", command)
		os.Exit(1)
	}
}
