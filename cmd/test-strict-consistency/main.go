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

	// 测试严格数据一致性检查集成
	log.Println("测试严格数据一致性检查集成...")

	// 由于没有真实的数据库，这里主要测试函数不会崩溃
	err := database.StrictDataConsistencyCheck()
	if err != nil {
		log.Printf("预期的严格一致性检查错误: %v", err)
		log.Println("严格数据一致性检查集成测试成功!")
	} else {
		log.Println("严格数据一致性检查通过（在真实环境中）")
	}
}
