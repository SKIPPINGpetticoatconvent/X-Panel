package main

import (
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	// 设置环境变量
	migrationPath := "/home/ub/X-Panel/database/migrations"
	dbPath := "sqlite3:///tmp/test-x-ui/etc/x-ui/x-ui.db"

	fmt.Printf("迁移路径: %s\n", migrationPath)
	fmt.Printf("数据库路径: %s\n", dbPath)

	// 列出迁移文件
	files, err := os.ReadDir(migrationPath)
	if err != nil {
		log.Fatalf("读取迁移目录失败: %v", err)
	}

	fmt.Println("迁移文件:")
	for _, file := range files {
		fmt.Printf("  %s\n", file.Name())
	}

	// 创建迁移实例
	m, err := migrate.New(
		"file://"+migrationPath,
		dbPath,
	)
	if err != nil {
		log.Fatalf("创建迁移实例失败: %v", err)
	}
	defer m.Close()

	// 获取版本信息
	version, dirty, err := m.Version()
	if err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("没有迁移记录，开始执行迁移")
		} else {
			log.Fatalf("获取版本失败: %v", err)
		}
	} else {
		fmt.Printf("当前版本: %d, dirty: %v\n", version, dirty)
	}

	// 执行迁移
	fmt.Println("执行迁移...")
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("无需迁移")
		} else {
			log.Fatalf("迁移失败: %v", err)
		}
	}

	fmt.Println("迁移完成!")

	// 再次检查版本
	version, dirty, err = m.Version()
	if err != nil {
		log.Fatalf("获取版本失败: %v", err)
	}
	fmt.Printf("迁移后版本: %d, dirty: %v\n", version, dirty)
}
