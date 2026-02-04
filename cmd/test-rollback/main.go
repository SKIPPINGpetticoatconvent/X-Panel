package main

import (
	"fmt"
	"log"

	"x-ui/database"
)

func main() {
	// 使用临时数据库进行测试
	testDBPath := "/tmp/test-x-ui/test.db"

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(testDBPath)
	if err != nil {
		log.Fatalf("创建迁移管理器失败: %v", err)
	}
	defer manager.Close()

	// 检查当前状态
	fmt.Println("检查迁移状态...")
	if err := manager.Status(); err != nil {
		fmt.Printf("状态检查: %v\n", err)
	}

	// 回滚一个迁移
	fmt.Println("回滚迁移...")
	if err := manager.Down(); err != nil {
		log.Fatalf("回滚失败: %v", err)
	}
	fmt.Println("回滚完成！")

	// 再次检查状态
	fmt.Println("检查迁移状态...")
	if err := manager.Status(); err != nil {
		fmt.Printf("状态检查: %v\n", err)
	}
}
