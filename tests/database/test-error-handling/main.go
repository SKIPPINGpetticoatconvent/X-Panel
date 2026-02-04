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

	// 测试错误处理
	log.Println("测试错误处理集成...")

	// 模拟迁移状态检查失败
	// 这里我们故意设置一个无效的迁移路径来触发错误
	os.Setenv("XUI_MIGRATIONS_PATH", "/invalid/path")

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Printf("预期的错误处理: %v", err)
		log.Println("错误处理测试成功!")
	} else {
		log.Fatal("应该返回错误，但没有")
	}
}
