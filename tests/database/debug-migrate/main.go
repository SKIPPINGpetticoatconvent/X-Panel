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

	fmt.Printf("工作目录: %s\n", wd)
	fmt.Printf("迁移路径: %s\n", migrationPath)
	fmt.Printf("数据库路径: %s\n", dbPath)

	// 检查迁移目录是否存在
	if _, err := os.Stat(migrationPath); os.IsNotExist(err) {
		log.Fatalf("迁移目录不存在: %s", migrationPath)
	}

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
		fmt.Sprintf("sqlite3://%s", dbPath),
	)
	if err != nil {
		log.Fatalf("创建迁移实例失败: %v", err)
	}
	defer m.Close()

	// 获取版本信息
	version, dirty, err := m.Version()
	if err != nil {
		if err == migrate.ErrNoChange {
			fmt.Println("没有迁移记录")
		} else {
			fmt.Printf("获取版本失败: %v\n", err)
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
}
