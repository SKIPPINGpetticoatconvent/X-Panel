package database

import (
	"bytes"
	"database/sql"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"
	"time"

	"x-ui/config"
	"x-ui/database/model"
	"x-ui/util/crypto"
	"x-ui/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB


func initModels() error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&LinkHistory{},
	}
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

func runSeeders(isUsersEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
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

		if !slices.Contains(seedersHistory, "UserPasswordHash") && !isUsersEmpty {
			var users []model.User
			db.Find(&users)

			for _, user := range users {
				hashedPassword, err := crypto.HashPasswordAsBcrypt(user.Password)
				if err != nil {
					log.Printf("Error hashing password for user '%s': %v", user.Username, err)
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

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	
	// 【新增】: SQLite连接池配置
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	
	// 【增强】: 设置SQLite连接池参数
	sqlDB.SetMaxOpenConns(25)                    // 最大连接数
	sqlDB.SetMaxIdleConns(5)                     // 空闲连接数
	sqlDB.SetConnMaxLifetime(5 * time.Minute)    // 连接最大生存时间
	sqlDB.SetConnMaxIdleTime(2 * time.Minute)    // 连接最大空闲时间
	
	// 【新增】: SQLite性能优化配置
	sqlDB.Exec("PRAGMA journal_mode=WAL;")       // 启用WAL模式，提高并发性能
	sqlDB.Exec("PRAGMA synchronous=NORMAL;")     // 同步模式平衡性能和安全
	sqlDB.Exec("PRAGMA cache_size=10000;")       // 缓存大小（KB）
	sqlDB.Exec("PRAGMA temp_store=MEMORY;")      // 将临时表存储在内存中
	sqlDB.Exec("PRAGMA mmap_size=268435456;")    // 内存映射大小（256MB）
	sqlDB.Exec("PRAGMA foreign_keys=ON;")        // 启用外键约束
	sqlDB.Exec("PRAGMA busy_timeout=30000;")     // 忙等待超时（毫秒）
	
	db, err = gorm.Open(sqlite.Open(dbPath), c)
	if err != nil {
		return err
	}
	
	// 【新增】: 设置GORM连接池参数
	sqlDB2, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB2.SetMaxOpenConns(25)
	sqlDB2.SetMaxIdleConns(5)
	sqlDB2.SetConnMaxLifetime(5 * time.Minute)

	if err := initModels(); err != nil {
		return err
	}

	isUsersEmpty, err := isTableEmpty("users")
	if err != nil {
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

