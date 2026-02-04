package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"x-ui/config"
	"x-ui/logger"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrationManager 数据库迁移管理器
type MigrationManager struct {
	migrate       *migrate.Migrate
	dbPath        string
	migrationPath string
}

// NewMigrationManager 创建新的迁移管理器
func NewMigrationManager(dbPath string) (*MigrationManager, error) {
	// 优先使用环境变量指定的迁移路径
	migrationPath := os.Getenv("XUI_MIGRATIONS_PATH")
	if migrationPath == "" {
		// 使用项目目录下的迁移文件夹
		// 获取当前工作目录（项目根目录）
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("获取工作目录失败: %v", err)
		}
		migrationPath = filepath.Join(wd, "database", "migrations")
	}

	if err := os.MkdirAll(migrationPath, 0o755); err != nil {
		return nil, fmt.Errorf("创建迁移目录失败: %v", err)
	}

	// 创建迁移实例
	m, err := migrate.New(
		"file://"+migrationPath,
		fmt.Sprintf("sqlite3://%s", dbPath),
	)
	if err != nil {
		return nil, fmt.Errorf("创建迁移实例失败: %v", err)
	}

	return &MigrationManager{
		migrate:       m,
		dbPath:        dbPath,
		migrationPath: migrationPath,
	}, nil
}

// Up 执行所有待执行的迁移
func (m *MigrationManager) Up() error {
	logger.Info("开始执行数据库迁移...")

	// 获取当前版本
	version, dirty, err := m.migrate.Version()
	if err != nil && err != migrate.ErrNoChange {
		logger.Errorf("获取当前版本失败: %v", err)
		return fmt.Errorf("获取当前版本失败: %v", err)
	}

	if err == migrate.ErrNoChange {
		logger.Info("没有迁移记录，开始执行迁移")
	} else {
		logger.Infof("当前迁移版本: %d, dirty: %v", version, dirty)
	}

	if err := m.migrate.Up(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("数据库已是最新状态，无需迁移")
			return nil
		}
		logger.Errorf("迁移执行失败: %v", err)
		return fmt.Errorf("迁移执行失败: %v", err)
	}

	logger.Info("数据库迁移执行完成")
	return nil
}

// Down 回滚最后一个迁移
func (m *MigrationManager) Down() error {
	logger.Warning("开始回滚数据库迁移...")

	if err := m.migrate.Down(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("数据库已是初始状态，无需回滚")
			return nil
		}
		return fmt.Errorf("迁移回滚失败: %v", err)
	}

	logger.Warning("数据库迁移回滚完成")
	return nil
}

// Status 查看迁移状态
func (m *MigrationManager) Status() error {
	_, dirty, err := m.migrate.Version()
	if err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("没有迁移记录")
			return nil
		}
		return fmt.Errorf("获取迁移状态失败: %v", err)
	}

	if dirty {
		logger.Errorf("数据库迁移状态异常 (dirty=true)")
	} else {
		logger.Info("数据库迁移状态正常")
	}

	return nil
}

// Force 强制设置迁移版本（用于修复状态）
func (m *MigrationManager) Force(version int) error {
	logger.Warningf("强制设置迁移版本为: %d", version)

	if err := m.migrate.Force(version); err != nil {
		return fmt.Errorf("强制设置迁移版本失败: %v", err)
	}

	logger.Infof("迁移版本已强制设置为: %d", version)
	return nil
}

// Steps 返回待执行的迁移步数
func (m *MigrationManager) Steps() (int, error) {
	// 简化实现，直接返回 0 表示没有待执行的迁移
	// 实际使用时可以通过 Status() 检查状态
	return 0, nil
}

// Close 关闭迁移管理器
func (m *MigrationManager) Close() error {
	sourceErr, dbErr := m.migrate.Close()
	if sourceErr != nil {
		return sourceErr
	}
	return dbErr
}

// RunMigrations 执行数据库迁移的主要入口函数
func RunMigrations() error {
	dbPath := config.GetDBPath()

	// 检查数据库文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		logger.Info("数据库文件不存在，将在初始化时创建")
		return nil
	}

	// 创建迁移管理器
	manager, err := NewMigrationManager(dbPath)
	if err != nil {
		return fmt.Errorf("创建迁移管理器失败: %v", err)
	}
	defer manager.Close()

	// 检查迁移状态
	if err := manager.Status(); err != nil {
		logger.Warningf("检查迁移状态失败: %v", err)
		return err
	}

	// 执行迁移
	if err := manager.Up(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("数据库已是最新状态，无需迁移")
			return nil
		}
		logger.Errorf("迁移执行失败: %v", err)
		return err
	}

	logger.Info("数据库迁移执行完成")
	return nil
}

// RunMigrationsWithBackup 执行带自动备份的数据库迁移
func RunMigrationsWithBackup() error {
	logger.Info("开始执行数据库迁移（带自动备份）...")

	// 1. 迁移前备份
	if err := BackupDatabase(); err != nil {
		return fmt.Errorf("数据库备份失败: %v", err)
	}

	// 2. 执行迁移
	if err := RunMigrations(); err != nil {
		// 3. 迁移失败时自动回滚
		logger.Errorf("迁移失败，开始自动回滚: %v", err)
		if rollbackErr := RollbackMigrations(); rollbackErr != nil {
			return fmt.Errorf("迁移失败且回滚也失败: %v (回滚错误: %v)", err, rollbackErr)
		}
		return fmt.Errorf("迁移失败但已成功回滚: %v", err)
	}

	logger.Info("数据库迁移执行完成")
	return nil
}

// RollbackMigrations 回滚数据库迁移
func RollbackMigrations() error {
	dbPath := config.GetDBPath()

	// 检查数据库文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		logger.Info("数据库文件不存在，无需回滚")
		return nil
	}

	manager, err := NewMigrationManager(dbPath)
	if err != nil {
		return fmt.Errorf("创建迁移管理器失败: %v", err)
	}
	defer manager.Close()

	logger.Warning("开始回滚数据库迁移...")

	if err := manager.Down(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("数据库已是初始状态，无需回滚")
			return nil
		}
		return fmt.Errorf("迁移回滚失败: %v", err)
	}

	logger.Warning("数据库迁移回滚完成")
	return nil
}

// MigrationStatus 迁移状态信息
type MigrationStatus struct {
	CurrentVersion uint   `json:"current_version"`
	Dirty          bool   `json:"dirty"`
	PendingCount   int    `json:"pending_count"`
	LastBackup     string `json:"last_backup"`
}

// GetMigrationStatus 获取迁移状态
func GetMigrationStatus() (*MigrationStatus, error) {
	dbPath := config.GetDBPath()

	// 检查数据库文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return &MigrationStatus{
			CurrentVersion: 0,
			Dirty:          false,
			PendingCount:   0,
			LastBackup:     "",
		}, nil
	}

	manager, err := NewMigrationManager(dbPath)
	if err != nil {
		return nil, fmt.Errorf("创建迁移管理器失败: %v", err)
	}
	defer manager.Close()

	version, dirty, err := manager.migrate.Version()
	if err != nil && err != migrate.ErrNoChange {
		return nil, fmt.Errorf("获取迁移版本失败: %v", err)
	}

	if err == migrate.ErrNoChange {
		version = 0
		dirty = false
	}

	// 获取最后备份时间
	backupPath := fmt.Sprintf("%s.backup", dbPath)
	var lastBackup string
	if info, err := os.Stat(backupPath); err == nil {
		lastBackup = info.ModTime().Format("2006-01-02 15:04:05")
	}

	return &MigrationStatus{
		CurrentVersion: version,
		Dirty:          dirty,
		PendingCount:   0, // TODO: 计算待执行的迁移数量
		LastBackup:     lastBackup,
	}, nil
}

// BackupDatabase 备份数据库
func BackupDatabase() error {
	// 优先使用环境变量指定的数据库路径
	dbPath := os.Getenv("XUI_DB_PATH")
	if dbPath == "" {
		dbPath = config.GetDBPath()
	}

	// 检查数据库文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		logger.Infof("数据库文件不存在，跳过备份: %s", dbPath)
		return nil
	}

	backupPath := fmt.Sprintf("%s.backup.%d", dbPath, time.Now().Unix())

	// 读取原数据库文件
	data, err := os.ReadFile(dbPath)
	if err != nil {
		return fmt.Errorf("读取数据库文件失败: %v", err)
	}

	// 写入备份文件
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return fmt.Errorf("创建数据库备份失败: %v", err)
	}

	logger.Infof("数据库已备份到: %s", backupPath)
	return nil
}
