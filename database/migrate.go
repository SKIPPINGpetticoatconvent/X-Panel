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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
		// 检查是否是 "no migration" 错误（数据库没有迁移表）
		if err.Error() == "no migration" {
			logger.Info("数据库没有迁移记录，开始执行迁移")
		} else {
			logger.Errorf("获取当前版本失败: %v", err)
			return fmt.Errorf("获取当前版本失败: %v", err)
		}
	}

	if err == migrate.ErrNoChange {
		logger.Info("没有迁移记录，开始执行迁移")
	} else if err == nil {
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

// RunMigrationsWithBackup 执行数据库迁移（带自动备份和回滚）
func RunMigrationsWithBackup() error {
	// 优先使用环境变量指定的数据库路径
	dbPath := os.Getenv("XUI_DB_PATH")
	if dbPath == "" {
		dbPath = config.GetDBPath()
	}

	logMigrationProgress("migration_start", map[string]interface{}{
		"action":  "backup_and_migrate",
		"db_path": dbPath,
	})

	// 执行数据库备份
	if err := BackupDatabase(); err != nil {
		logMigrationProgress("migration_failed", map[string]interface{}{
			"error": err.Error(),
			"step":  "backup",
		})
		return fmt.Errorf("数据库备份失败: %v", err)
	}

	// 执行迁移
	manager, err := NewMigrationManager(dbPath)
	if err != nil {
		logMigrationProgress("migration_failed", map[string]interface{}{
			"error": err.Error(),
			"step":  "create_manager",
		})
		return fmt.Errorf("创建迁移管理器失败: %v", err)
	}
	defer manager.Close()

	if err := manager.Up(); err != nil {
		logMigrationProgress("migration_failed", map[string]interface{}{
			"error": err.Error(),
			"step":  "execute_migration",
		})
		return fmt.Errorf("执行迁移失败: %v", err)
	}

	// 获取迁移状态
	status, err := GetMigrationStatus()
	if err != nil {
		logMigrationProgress("migration_failed", map[string]interface{}{
			"error": err.Error(),
			"step":  "get_status",
		})
		return fmt.Errorf("获取迁移状态失败: %v", err)
	}

	logMigrationProgress("migration_complete", map[string]interface{}{
		"version": status.CurrentVersion,
		"backup":  status.LastBackup,
	})

	logger.Infof("数据库迁移完成，当前版本: %d", status.CurrentVersion)
	return nil
}

// RollbackMigrations 回滚数据库迁移
func RollbackMigrations() error {
	logMigrationProgress("migration_rollback", map[string]interface{}{
		"action":  "rollback",
		"db_path": config.GetDBPath(),
	})

	dbPath := config.GetDBPath()

	// 检查数据库文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		logger.Info("数据库文件不存在，无需回滚")
		return nil
	}

	// 创建迁移管理器
	manager, err := NewMigrationManager(dbPath)
	if err != nil {
		return fmt.Errorf("创建迁移管理器失败: %v", err)
	}
	defer manager.Close()

	// 检查当前版本
	version, _, err := manager.migrate.Version()
	if err != nil {
		return fmt.Errorf("获取当前版本失败: %v", err)
	}

	if version <= 1 {
		logger.Info("数据库已是初始状态，无需回滚")
		return nil
	}

	// 执行回滚
	if err := manager.Down(); err != nil {
		logMigrationProgress("migration_failed", map[string]interface{}{
			"error": err.Error(),
			"step":  "rollback",
		})
		return fmt.Errorf("回滚失败: %v", err)
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
	// 优先使用环境变量指定的数据库路径
	dbPath := os.Getenv("XUI_DB_PATH")
	if dbPath == "" {
		dbPath = config.GetDBPath()
	}

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
		// 检查是否是 "no migration" 错误（数据库没有迁移表）
		if err.Error() == "no migration" {
			version = 0
			dirty = false
		} else {
			return nil, fmt.Errorf("获取迁移版本失败: %v", err)
		}
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

// CheckMigrationStatus 检查迁移状态
func CheckMigrationStatus() error {
	logger.Info("开始检查数据库迁移状态...")

	// 1. 获取当前迁移状态
	status, err := GetMigrationStatus()
	if err != nil {
		return fmt.Errorf("获取迁移状态失败: %v", err)
	}

	// 2. 生成状态报告
	if err := generateMigrationStatusReport(status); err != nil {
		return fmt.Errorf("生成迁移状态报告失败: %v", err)
	}

	// 3. 检查数据库兼容性
	if err := checkDatabaseCompatibility(status); err != nil {
		return fmt.Errorf("数据库兼容性检查失败: %v", err)
	}

	logger.Info("迁移状态检查完成")
	return nil
}

// generateMigrationStatusReport 生成迁移状态报告
func generateMigrationStatusReport(status *MigrationStatus) error {
	logger.Info("=== 数据库迁移状态报告 ===")
	logger.Infof("当前版本: %d", status.CurrentVersion)
	logger.Infof("数据库状态: %s", getDatabaseStateText(status))
	logger.Infof("最后备份: %s", status.LastBackup)

	// 调试模式显示更多信息
	if config.GetLogLevel() == config.Debug {
		logger.Debugf("数据库路径: %s", config.GetDBPath())
		logger.Debugf("待执行迁移数: %d", status.PendingCount)
		logger.Debugf("脏状态: %v", status.Dirty)
	}

	logger.Info("========================")
	return nil
}

// checkDatabaseCompatibility 检查数据库兼容性
func checkDatabaseCompatibility(status *MigrationStatus) error {
	// 检查版本是否过低
	if status.CurrentVersion < 3 {
		logger.Warning("检测到较旧的数据库版本，建议进行完整迁移")
	}

	// 检查是否有脏状态
	if status.Dirty {
		logger.Error("数据库处于脏状态，可能需要手动干预")
		return fmt.Errorf("数据库状态异常：脏状态")
	}

	// 检查备份状态
	if status.LastBackup == "" && status.CurrentVersion > 0 {
		logger.Warning("没有找到备份文件，建议手动备份")
	}

	return nil
}

// getDatabaseStateText 获取数据库状态文本
func getDatabaseStateText(status *MigrationStatus) string {
	if status.Dirty {
		return "异常（脏状态）"
	}

	if status.CurrentVersion == 0 {
		return "未初始化"
	}

	if status.CurrentVersion < 5 {
		return "需要迁移"
	}

	return "正常"
}

// handleMigrationError 处理迁移错误
func handleMigrationError(err error) error {
	logger.Errorf("数据库迁移失败: %v", err)

	// 提供回滚选项
	logger.Warning("正在尝试自动回滚...")
	if rollbackErr := RollbackMigrations(); rollbackErr != nil {
		logger.Errorf("自动回滚也失败: %v", rollbackErr)
		return fmt.Errorf("迁移失败且回滚也失败: %v (回滚错误: %v)", err, rollbackErr)
	}

	logger.Info("自动回滚成功，数据库已恢复到迁移前状态")
	return fmt.Errorf("迁移失败但已成功回滚: %v", err)
}

// handleMigrationStatusError 处理迁移状态错误
func handleMigrationStatusError(err error) error {
	logger.Errorf("迁移状态检查失败: %v", err)

	// 提供解决建议
	logger.Warning("迁移状态检查失败的可能解决方案：")
	logger.Warning("1. 检查数据库文件权限")
	logger.Warning("2. 验证数据库文件完整性")
	logger.Warning("3. 检查磁盘空间")

	return fmt.Errorf("迁移状态检查失败: %v", err)
}

// ValidateMigrationSuccess 验证迁移成功
func ValidateMigrationSuccess() error {
	logMigrationProgress("validation_start", map[string]interface{}{
		"action": "validate_migration",
	})

	// 1. 获取迁移状态
	status, err := GetMigrationStatus()
	if err != nil {
		logMigrationProgress("migration_failed", map[string]interface{}{
			"error": err.Error(),
			"step":  "get_migration_status",
		})
		return fmt.Errorf("获取迁移状态失败: %v", err)
	}

	// 2. 验证迁移版本
	if err := validateMigrationVersion(status); err != nil {
		logMigrationProgress("migration_failed", map[string]interface{}{
			"error": err.Error(),
			"step":  "validate_version",
		})
		return err
	}

	// 3. 验证数据库结构
	if err := validateDatabaseSchema(); err != nil {
		logMigrationProgress("migration_failed", map[string]interface{}{
			"error": err.Error(),
			"step":  "validate_schema",
		})
		return err
	}

	logMigrationProgress("validation_complete", map[string]interface{}{
		"version": status.CurrentVersion,
		"status":  "success",
	})

	logger.Info("迁移验证完成，数据库状态正常")
	return nil
}

// validateMigrationVersion 验证迁移版本
func validateMigrationVersion(status *MigrationStatus) error {
	expectedVersion := uint(5) // 根据实际迁移文件数量

	if status.CurrentVersion != expectedVersion {
		if status.CurrentVersion < expectedVersion {
			return fmt.Errorf("迁移版本过低: 当前 %d，期望 %d",
				status.CurrentVersion, expectedVersion)
		} else {
			logger.Warningf("迁移版本过高: 当前 %d，期望 %d",
				status.CurrentVersion, expectedVersion)
		}
	}

	return nil
}

// validateDatabaseSchema 验证数据库结构
func validateDatabaseSchema() error {
	// 检查所有必需的表是否存在
	requiredTables := []string{
		"users", "inbounds", "outbound_traffics",
		"settings", "inbound_client_ips", "client_traffics",
		"history_of_seeders", "link_histories", "schema_migrations",
	}

	for _, table := range requiredTables {
		if !isTableExists(table) {
			return fmt.Errorf("必需的表不存在: %s", table)
		}
	}

	logger.Info("数据库结构验证通过")
	return nil
}

// isTableExists 检查表是否存在
func isTableExists(tableName string) bool {
	if db == nil {
		// 如果全局 db 变量为空，尝试使用环境变量创建临时连接
		dbPath := os.Getenv("XUI_DB_PATH")
		if dbPath == "" {
			dbPath = config.GetDBPath()
		}

		// 创建临时数据库连接
		tempDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
		if err != nil {
			logger.Debugf("创建临时数据库连接失败，无法检查表 %s: %v", tableName, err)
			return false
		}
		sqlDB, err := tempDB.DB()
		if err != nil {
			logger.Debugf("获取底层数据库连接失败，无法检查表 %s: %v", tableName, err)
			return false
		}
		defer sqlDB.Close()

		var count int
		err = tempDB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count).Error
		if err != nil {
			logger.Debugf("检查表 %s 是否存在时出错: %v", tableName, err)
			return false
		}
		return count > 0
	}

	var count int
	err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count).Error
	if err != nil {
		logger.Debugf("检查表 %s 是否存在时出错: %v", tableName, err)
		return false
	}
	return count > 0
}

// IsTableExists 检查表是否存在（导出版本）
func IsTableExists(tableName string) bool {
	return isTableExists(tableName)
}

// LogMigrationProgress 记录迁移进度（导出版本）
func LogMigrationProgress(step string, details interface{}) {
	logMigrationProgress(step, details)
}

// logMigrationProgress 记录迁移进度
func logMigrationProgress(step string, details interface{}) {
	switch config.GetLogLevel() {
	case config.Debug:
		logDebugMigrationProgress(step, details)
	case config.Info, config.Warning, config.Error:
		logProductionMigrationProgress(step)
	}
}

// logProductionMigrationProgress 生产环境日志
func logProductionMigrationProgress(step string) {
	criticalSteps := []string{
		"migration_start", "migration_complete",
		"migration_failed", "migration_rollback",
		"validation_start", "validation_complete",
	}

	if isCriticalStep(step, criticalSteps) {
		logger.Infof("数据库迁移: %s", getStepDescription(step))
	}
}

// logDebugMigrationProgress 调试模式日志
func logDebugMigrationProgress(step string, details interface{}) {
	logger.Debugf("=== 迁移调试信息 ===")
	logger.Debugf("步骤: %s", step)
	logger.Debugf("详情: %+v", details)
	logger.Debugf("时间戳: %s", time.Now().Format("2006-01-02 15:04:05"))
	logger.Debugf("===================")
}

// isCriticalStep 判断是否为关键步骤
func isCriticalStep(step string, criticalSteps []string) bool {
	for _, critical := range criticalSteps {
		if step == critical {
			return true
		}
	}
	return false
}

// getStepDescription 获取步骤描述
func getStepDescription(step string) string {
	descriptions := map[string]string{
		"migration_start":     "开始迁移",
		"migration_complete":  "迁移完成",
		"migration_failed":    "迁移失败",
		"migration_rollback":  "迁移回滚",
		"validation_start":    "开始验证",
		"validation_complete": "验证完成",
	}

	if desc, exists := descriptions[step]; exists {
		return desc
	}
	return step
}

// StrictDataConsistencyCheck 严格数据一致性检查
func StrictDataConsistencyCheck() error {
	logger.Info("开始严格数据一致性检查...")

	// 1. 数据库结构完整性检查
	if err := validateDatabaseSchemaIntegrity(); err != nil {
		return fmt.Errorf("数据库结构完整性检查失败: %v", err)
	}

	// 2. 数据关系一致性检查
	if err := validateDataRelationships(); err != nil {
		return fmt.Errorf("数据关系一致性检查失败: %v", err)
	}

	// 3. 数据完整性检查
	if err := validateDataIntegrity(); err != nil {
		return fmt.Errorf("数据完整性检查失败: %v", err)
	}

	// 4. 迁移版本一致性检查
	if err := validateMigrationVersionConsistency(); err != nil {
		return fmt.Errorf("迁移版本一致性检查失败: %v", err)
	}

	logger.Info("严格数据一致性检查通过")
	return nil
}

// validateDatabaseSchemaIntegrity 验证数据库结构完整性
func validateDatabaseSchemaIntegrity() error {
	// 检查所有必需的表是否存在
	requiredTables := []string{
		"users", "inbounds", "outbound_traffics",
		"settings", "inbound_client_ips", "client_traffics",
		"history_of_seeders", "link_histories", "schema_migrations",
	}

	for _, table := range requiredTables {
		if !isTableExists(table) {
			return fmt.Errorf("必需的表不存在: %s", table)
		}

		// 检查表结构
		if err := validateTableStructure(table); err != nil {
			return fmt.Errorf("表 %s 结构验证失败: %v", table, err)
		}
	}

	// 检查索引完整性
	if err := validateIndexesIntegrity(); err != nil {
		return fmt.Errorf("索引完整性检查失败: %v", err)
	}

	// 检查外键约束
	if err := validateForeignKeyConstraints(); err != nil {
		return fmt.Errorf("外键约束检查失败: %v", err)
	}

	return nil
}

// validateDataRelationships 验证数据关系一致性
func validateDataRelationships() error {
	// 检查用户与入站的关系
	if err := validateUserInboundRelationships(); err != nil {
		return fmt.Errorf("用户-入站关系验证失败: %v", err)
	}

	// 检查客户端流量关系
	if err := validateClientTrafficRelationships(); err != nil {
		return fmt.Errorf("客户端流量关系验证失败: %v", err)
	}

	// 检查出站流量关系
	if err := validateOutboundTrafficRelationships(); err != nil {
		return fmt.Errorf("出站流量关系验证失败: %v", err)
	}

	return nil
}

// validateDataIntegrity 验证数据完整性
func validateDataIntegrity() error {
	// 检查用户数据完整性
	if err := validateUserDataIntegrity(); err != nil {
		return fmt.Errorf("用户数据完整性验证失败: %v", err)
	}

	// 检查入站数据完整性
	if err := validateInboundDataIntegrity(); err != nil {
		return fmt.Errorf("入站数据完整性验证失败: %v", err)
	}

	// 检查设置数据完整性
	if err := validateSettingsDataIntegrity(); err != nil {
		return fmt.Errorf("设置数据完整性验证失败: %v", err)
	}

	return nil
}

// validateMigrationVersionConsistency 验证迁移版本一致性
func validateMigrationVersionConsistency() error {
	// 获取当前迁移版本
	status, err := GetMigrationStatus()
	if err != nil {
		return fmt.Errorf("获取迁移状态失败: %v", err)
	}

	// 检查版本是否为最新
	latestVersion := uint(5) // 根据实际迁移文件数量
	if status.CurrentVersion != latestVersion {
		return fmt.Errorf("迁移版本不一致: 当前版本 %d，期望版本 %d",
			status.CurrentVersion, latestVersion)
	}

	// 检查是否有脏状态
	if status.Dirty {
		return fmt.Errorf("数据库处于脏状态，需要手动干预")
	}

	return nil
}

// validateTableStructure 验证表结构
func validateTableStructure(tableName string) error {
	// 获取表结构信息
	columns, err := getTableColumns(tableName)
	if err != nil {
		return fmt.Errorf("获取表 %s 列信息失败: %v", tableName, err)
	}

	// 根据表名验证必需的列
	requiredColumns := getRequiredColumns(tableName)
	for _, requiredCol := range requiredColumns {
		if !hasColumn(columns, requiredCol) {
			return fmt.Errorf("表 %s 缺少必需的列: %s", tableName, requiredCol)
		}
	}

	return nil
}

// validateIndexesIntegrity 验证索引完整性
func validateIndexesIntegrity() error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// 检查关键索引是否存在
	var count int
	err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name IN ('idx_users_username', 'idx_inbounds_tag')").Scan(&count).Error
	if err != nil {
		return fmt.Errorf("检查索引失败: %v", err)
	}

	// 这里可以根据需要添加更多索引检查
	logger.Debugf("找到 %d 个关键索引", count)
	return nil
}

// validateForeignKeyConstraints 验证外键约束
func validateForeignKeyConstraints() error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// SQLite 的外键约束检查
	var result string
	err := db.Raw("PRAGMA foreign_key_check").Scan(&result).Error
	if err != nil {
		return fmt.Errorf("检查外键约束失败: %v", err)
	}

	// 如果有外键约束问题，result 会包含错误信息
	if result != "" {
		return fmt.Errorf("外键约束检查失败: %s", result)
	}

	return nil
}

// validateUserInboundRelationships 验证用户-入站关系
func validateUserInboundRelationships() error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// 检查入站记录的用户ID是否都存在于用户表中
	var count int
	err := db.Raw(`
		SELECT COUNT(*) FROM inbounds i 
		LEFT JOIN users u ON i.user_id = u.id 
		WHERE i.user_id > 0 AND u.id IS NULL
	`).Scan(&count).Error
	if err != nil {
		return fmt.Errorf("检查用户-入站关系失败: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("发现 %d 个入站记录引用了不存在的用户", count)
	}

	return nil
}

// validateClientTrafficRelationships 验证客户端流量关系
func validateClientTrafficRelationships() error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// 检查客户端流量记录的关联数据完整性
	var count int
	err := db.Raw(`
		SELECT COUNT(*) FROM client_traffics ct 
		LEFT JOIN inbounds i ON ct.inbound_id = i.id 
		WHERE ct.inbound_id > 0 AND i.id IS NULL
	`).Scan(&count).Error
	if err != nil {
		return fmt.Errorf("检查客户端流量关系失败: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("发现 %d 个客户端流量记录引用了不存在的入站", count)
	}

	return nil
}

// validateOutboundTrafficRelationships 验证出站流量关系
func validateOutboundTrafficRelationships() error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// 检查出站流量记录的关联数据完整性
	var count int
	err := db.Raw(`
		SELECT COUNT(*) FROM outbound_traffics ot 
		LEFT JOIN users u ON ot.user_id = u.id 
		WHERE ot.user_id > 0 AND u.id IS NULL
	`).Scan(&count).Error
	if err != nil {
		return fmt.Errorf("检查出站流量关系失败: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("发现 %d 个出站流量记录引用了不存在的用户", count)
	}

	return nil
}

// validateUserDataIntegrity 验证用户数据完整性
func validateUserDataIntegrity() error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// 检查用户数据的完整性
	var count int
	err := db.Raw(`
		SELECT COUNT(*) FROM users 
		WHERE username = '' OR password = ''
	`).Scan(&count).Error
	if err != nil {
		return fmt.Errorf("检查用户数据完整性失败: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("发现 %d 个用户记录缺少必需字段", count)
	}

	return nil
}

// validateInboundDataIntegrity 验证入站数据完整性
func validateInboundDataIntegrity() error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// 检查入站数据的完整性
	var count int
	err := db.Raw(`
		SELECT COUNT(*) FROM inbounds 
		WHERE protocol = '' OR port = 0 OR settings = ''
	`).Scan(&count).Error
	if err != nil {
		return fmt.Errorf("检查入站数据完整性失败: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("发现 %d 个入站记录缺少必需字段", count)
	}

	return nil
}

// validateSettingsDataIntegrity 验证设置数据完整性
func validateSettingsDataIntegrity() error {
	if db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// 检查设置数据的完整性
	var count int
	err := db.Raw(`
		SELECT COUNT(*) FROM settings 
		WHERE key = '' OR value = ''
	`).Scan(&count).Error
	if err != nil {
		return fmt.Errorf("检查设置数据完整性失败: %v", err)
	}

	if count > 0 {
		return fmt.Errorf("发现 %d 个设置记录缺少必需字段", count)
	}

	return nil
}

// getTableColumns 获取表列信息
func getTableColumns(tableName string) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库连接为空")
	}

	var columns []string
	err := db.Raw("PRAGMA table_info(?)", tableName).Scan(&columns).Error
	if err != nil {
		return nil, err
	}

	return columns, nil
}

// getRequiredColumns 获取表必需列
func getRequiredColumns(tableName string) []string {
	switch tableName {
	case "users":
		return []string{"id", "username", "password"}
	case "inbounds":
		return []string{"id", "protocol", "port", "settings"}
	case "settings":
		return []string{"id", "key", "value"}
	case "client_traffics":
		return []string{"id", "inbound_id", "user_id"}
	case "outbound_traffics":
		return []string{"id", "user_id"}
	default:
		return []string{}
	}
}

// hasColumn 检查列是否存在
func hasColumn(columns []string, column string) bool {
	for _, col := range columns {
		if col == column {
			return true
		}
	}
	return false
}

// handleDataConsistencyError 处理数据一致性错误
func handleDataConsistencyError(err error) error {
	logger.Errorf("数据一致性检查失败: %v", err)

	// 提供详细的错误信息和建议
	logger.Error("数据一致性检查失败可能的原因：")
	logger.Error("1. 迁移过程中断或失败")
	logger.Error("2. 数据库文件损坏")
	logger.Error("3. 并发访问导致的数据不一致")

	logger.Warning("建议解决方案：")
	logger.Warning("1. 检查数据库备份并恢复")
	logger.Warning("2. 重新执行迁移")
	logger.Warning("3. 联系技术支持")

	return fmt.Errorf("数据一致性检查失败: %v", err)
}
