package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("获取工作目录失败: %v", err)
	}

	migrationPath := filepath.Join(wd, "database", "migrations")
	dbPath := "/tmp/test-x-ui/test.db"

	// 创建迁移实例
	m, err := migrate.New(
		"file://"+migrationPath,
		fmt.Sprintf("sqlite3://%s", dbPath),
	)
	if err != nil {
		log.Fatalf("创建迁移实例失败: %v", err)
	}
	defer m.Close()

	// 获取当前版本
	version, dirty, err := m.Version()
	if err != nil {
		log.Fatalf("获取版本失败: %v", err)
	}

	fmt.Printf("当前版本: %d, dirty: %v\n", version, dirty)

	// 执行回滚
	fmt.Println("执行回滚...")
	if err := m.Down(); err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("无需回滚")
		} else {
			log.Fatalf("回滚失败: %v", err)
		}
	}

	fmt.Println("回滚完成!")

	// 再次检查版本
	version, dirty, err = m.Version()
	if err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("没有迁移记录")
		} else {
			fmt.Printf("获取版本失败: %v\n", err)
		}
	} else {
		fmt.Printf("回滚后版本: %d, dirty: %v\n", version, dirty)
	}
}
