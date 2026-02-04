package database

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"time"

	"x-ui/config"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/crypto"
	"x-ui/util/json_util"
	"x-ui/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var db *gorm.DB

func GetDBProvider() *gorm.DB {
	return GetDB()
}

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
)

func initModels() error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&LinkHistory{}, // 把 LinkHistory 表也迁移
	}
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			logger.Errorf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

func initUser() error {
	empty, err := isTableEmpty("users")
	if err != nil {
		logger.Errorf("Error checking if users table is empty: %v", err)
		return err
	}
	if empty {
		hashedPassword, err := crypto.HashPasswordAsBcrypt(defaultPassword)
		if err != nil {
			logger.Errorf("Error hashing default password: %v", err)
			return err
		}

		user := &model.User{
			Username: defaultUsername,
			Password: hashedPassword,
		}
		return db.Create(user).Error
	}
	return nil
}

func runSeeders(isUsersEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		logger.Errorf("Error checking if users table is empty: %v", err)
		return err
	}

	if empty && isUsersEmpty {
		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return db.Create(hashSeeder).Error
	} else {
		var seedersHistory []string
		db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory)

		if !slices.Contains(seedersHistory, "TlsConfigMigration") {
			if err := migrateTlsInbounds(); err != nil {
				logger.Errorf("TlsConfigMigration seeder failed: %v", err)
				return err
			}
			db.Create(&model.HistoryOfSeeders{SeederName: "TlsConfigMigration"})
		}

		if !slices.Contains(seedersHistory, "XhttpFlowMigration") {
			if err := migrateXhttpFlow(); err != nil {
				logger.Errorf("XhttpFlowMigration seeder failed: %v", err)
				return err
			}
			db.Create(&model.HistoryOfSeeders{SeederName: "XhttpFlowMigration"})
		}

		if !slices.Contains(seedersHistory, "RealityTargetMigration") {
			if err := migrateRealityTarget(); err != nil {
				logger.Errorf("RealityTargetMigration seeder failed: %v", err)
				return err
			}
			db.Create(&model.HistoryOfSeeders{SeederName: "RealityTargetMigration"})
		}

		if !slices.Contains(seedersHistory, "UserPasswordHash") && !isUsersEmpty {
			var users []model.User
			db.Find(&users)

			for _, user := range users {
				hashedPassword, err := crypto.HashPasswordAsBcrypt(user.Password)
				if err != nil {
					logger.Errorf("Error hashing password for user '%s': %v", user.Username, err)
					return err
				}
				db.Model(&user).Update("password", hashedPassword)
			}

			hashSeeder := &model.HistoryOfSeeders{
				SeederName: "UserPasswordHash",
			}
			return db.Create(hashSeeder).Error
		}
	}

	return nil
}

func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

func InitDB(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, fs.ModePerm)
	if err != nil {
		return err
	}

	var gormLogger gormlogger.Interface

	if config.IsDebug() {
		gormLogger = gormlogger.Default
	} else {
		gormLogger = gormlogger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	db, err = gorm.Open(sqlite.Open(dbPath), c)
	if err != nil {
		return err
	}

	// 数据库连接池配置优化
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(25)                 // 最大打开连接数
	sqlDB.SetMaxIdleConns(5)                  // 最大空闲连接数
	sqlDB.SetConnMaxLifetime(5 * time.Minute) // 连接最大生命周期

	// 启用 SQLite WAL 模式优化
	db.Exec("PRAGMA journal_mode=WAL;")
	db.Exec("PRAGMA synchronous=NORMAL;")

	if err := initModels(); err != nil {
		return err
	}

	// 检查迁移状态
	if err := CheckMigrationStatus(); err != nil {
		return fmt.Errorf("迁移状态检查失败: %v", err)
	}

	// 执行数据库迁移（带自动备份和回滚）
	if err := RunMigrationsWithBackup(); err != nil {
		return handleMigrationError(err)
	}

	// 验证迁移成功
	if err := ValidateMigrationSuccess(); err != nil {
		return fmt.Errorf("迁移验证失败: %v", err)
	}

	// 严格数据一致性检查
	if err := StrictDataConsistencyCheck(); err != nil {
		return handleDataConsistencyError(err)
	}

	isUsersEmpty, err := isTableEmpty("users")
	if err != nil {
		return err
	}

	if err := initUser(); err != nil {
		return err
	}
	return runSeeders(isUsersEmpty)
}

func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func Checkpoint() error {
	// Update WAL
	err := db.Exec("PRAGMA wal_checkpoint;").Error
	if err != nil {
		return err
	}
	return nil
}

// WithTx 执行带事务的操作，自动处理 Commit/Rollback
// 如果 fn 返回 nil，事务将被提交；如果返回 error，事务将被回滚
func WithTx(fn func(tx *gorm.DB) error) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// WithTxResult 执行带事务的操作并返回结果，自动处理 Commit/Rollback
func WithTxResult[T any](fn func(tx *gorm.DB) (T, error)) (T, error) {
	var zero T
	tx := db.Begin()
	if tx.Error != nil {
		return zero, tx.Error
	}

	result, err := fn(tx)
	if err != nil {
		tx.Rollback()
		return zero, err
	}
	return result, tx.Commit().Error
}

// migrateTlsInbounds performs a one-time database migration for all inbound records,
// applying TLS configuration changes (remove allowInsecure, migrate verifyPeerCertInNames,
// migrate pinnedPeerCertSha256 separator) directly in the database.
func migrateTlsInbounds() error {
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}
	for _, inbound := range inbounds {
		if inbound.StreamSettings == "" {
			continue
		}
		raw := json_util.RawMessage(inbound.StreamSettings)
		if xray.MigrateTlsSettings(&raw) {
			if err := db.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("stream_settings", string(raw)).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateXhttpFlow performs a one-time database migration for all VLESS inbounds,
// adding "flow": "xtls-rprx-vision" to clients if they are using XHTTP and TLS/Reality.
func migrateXhttpFlow() error {
	var inbounds []model.Inbound
	// Only check VLESS protocol
	if err := db.Where("protocol = ?", "vless").Find(&inbounds).Error; err != nil {
		return err
	}

	for _, inbound := range inbounds {
		if inbound.Settings == "" || inbound.StreamSettings == "" {
			continue
		}
		settingsRaw := json_util.RawMessage(inbound.Settings)
		streamRaw := json_util.RawMessage(inbound.StreamSettings)

		if xray.MigrateXhttpFlowInSettings(&settingsRaw, streamRaw) {
			if err := db.Model(&model.Inbound{}).Where("id = ?", inbound.Id).
				Update("settings", string(settingsRaw)).Error; err != nil {
				return err
			}
			logger.Infof("Migrated XHTTP Flow for inbound %d (%s)", inbound.Id, inbound.Remark)
		}
	}
	return nil
}

// ValidateSQLiteDB opens the provided sqlite DB path with a throw-away connection
// and runs a PRAGMA integrity_check to ensure the file is structurally sound.
// It does not mutate global state or run migrations.
func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil { // file must exist
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	var res string
	if err := gdb.Raw("PRAGMA integrity_check;").Scan(&res).Error; err != nil {
		return err
	}
	if res != "ok" {
		return errors.New("sqlite integrity check failed: " + res)
	}
	return nil
}
