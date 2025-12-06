package database

import (
	"fmt"
	"os"
	"path/filepath"

	"x-ui/database/model"
	"x-ui/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CreateTestDB 创建一个临时测试数据库
func CreateTestDB(dbPath string) (*gorm.DB, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 创建新的数据库连接
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 测试时静默模式
	})
	if err != nil {
		return nil, fmt.Errorf("创建测试数据库失败: %w", err)
	}

	return db, nil
}

// InitTestModels 初始化测试模型
func InitTestModels(db *gorm.DB) error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.Client{},        // Client 数据库表
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&LinkHistory{},
	}
	
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("自动迁移模型失败: %w", err)
		}
	}
	
	return nil
}

// CleanupTestDB 清理测试数据库
func CleanupTestDB(db *gorm.DB) error {
	if db != nil {
		// 获取底层 sql.DB 对象
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("获取数据库实例失败: %w", err)
		}
		
		// 关闭连接
		if err := sqlDB.Close(); err != nil {
			return fmt.Errorf("关闭数据库连接失败: %w", err)
		}
	}
	
	return nil
}