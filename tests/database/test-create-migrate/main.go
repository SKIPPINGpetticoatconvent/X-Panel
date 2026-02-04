package main

import (
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	fmt.Println("开始测试迁移实例创建...")

	// 创建迁移实例
	m, err := migrate.New(
		"file:///home/ub/X-Panel/database/migrations",
		"sqlite3:///tmp/test-x-ui/etc/x-ui/x-ui.db",
	)
	if err != nil {
		log.Fatalf("创建迁移实例失败: %v", err)
	}

	fmt.Println("迁移实例创建成功!")
	defer m.Close()

	fmt.Println("测试完成!")
}
