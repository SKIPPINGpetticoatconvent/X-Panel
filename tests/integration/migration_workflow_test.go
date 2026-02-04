package integration

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-ui/database"
)

// TestDatabaseMigrationBasic 基础数据库迁移集成测试
func TestDatabaseMigrationBasic(t *testing.T) {
	// 创建临时测试数据库
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	migrationDir := filepath.Join(tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(migrationDir, 0o755)
	require.NoError(t, err)

	// 创建测试迁移文件
	upFile := filepath.Join(migrationDir, "001_test.up.sql")
	downFile := filepath.Join(migrationDir, "001_test.down.sql")

	err = os.WriteFile(upFile, []byte("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS users;"), 0o644)
	require.NoError(t, err)

	// 设置环境变量
	originalPath := os.Getenv("XUI_DB_PATH")
	originalMigrationPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_DB_PATH")
		}
		if originalMigrationPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalMigrationPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_DB_PATH", testDBPath)
	os.Setenv("XUI_MIGRATIONS_PATH", migrationDir)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行迁移
	err = manager.Up()
	assert.NoError(t, err)

	// 验证表是否创建
	db, err := sql.Open("sqlite3", testDBPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestDatabaseMigrationStatus 测试数据库迁移状态
func TestDatabaseMigrationStatus(t *testing.T) {
	// 创建临时测试数据库
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	migrationDir := filepath.Join(tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(migrationDir, 0o755)
	require.NoError(t, err)

	// 创建测试迁移文件
	upFile := filepath.Join(migrationDir, "001_test.up.sql")
	downFile := filepath.Join(migrationDir, "001_test.down.sql")

	err = os.WriteFile(upFile, []byte("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS users;"), 0o644)
	require.NoError(t, err)

	// 设置环境变量
	originalPath := os.Getenv("XUI_DB_PATH")
	originalMigrationPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_DB_PATH")
		}
		if originalMigrationPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalMigrationPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_DB_PATH", testDBPath)
	os.Setenv("XUI_MIGRATIONS_PATH", migrationDir)

	// 检查初始状态
	err = database.CheckMigrationStatus()
	assert.NoError(t, err)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行迁移
	err = manager.Up()
	require.NoError(t, err)

	// 检查迁移后状态
	err = database.CheckMigrationStatus()
	assert.NoError(t, err)

	// 获取详细状态
	status, err := database.GetMigrationStatus()
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, uint(1), status.CurrentVersion)
	assert.False(t, status.Dirty)
}

// TestDatabaseMigrationValidation 测试数据库迁移验证
func TestDatabaseMigrationValidation(t *testing.T) {
	// 创建临时测试数据库
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	migrationDir := filepath.Join(tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(migrationDir, 0o755)
	require.NoError(t, err)

	// 创建测试迁移文件
	upFile := filepath.Join(migrationDir, "001_test.up.sql")
	downFile := filepath.Join(migrationDir, "001_test.down.sql")

	err = os.WriteFile(upFile, []byte("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS users;"), 0o644)
	require.NoError(t, err)

	// 设置环境变量
	originalPath := os.Getenv("XUI_DB_PATH")
	originalMigrationPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_DB_PATH")
		}
		if originalMigrationPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalMigrationPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_DB_PATH", testDBPath)
	os.Setenv("XUI_MIGRATIONS_PATH", migrationDir)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行迁移
	err = manager.Up()
	require.NoError(t, err)

	// 验证迁移成功
	err = database.ValidateMigrationSuccess()
	assert.NoError(t, err)
}

// TestDatabaseMigrationErrorHandling 测试数据库迁移错误处理
func TestDatabaseMigrationErrorHandling(t *testing.T) {
	// 测试无效数据库路径
	invalidManager, err := database.NewMigrationManager("/invalid/path/test.db")
	assert.Error(t, err)
	assert.Nil(t, invalidManager)

	// 测试无效迁移路径
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")

	originalMigrationPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalMigrationPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalMigrationPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_MIGRATIONS_PATH", "/invalid/path")
	invalidManager, err = database.NewMigrationManager(testDBPath)
	assert.Error(t, err)
	assert.Nil(t, invalidManager)
}

// TestDatabaseMigrationRollback 测试数据库迁移回滚
func TestDatabaseMigrationRollback(t *testing.T) {
	// 创建临时测试数据库
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	migrationDir := filepath.Join(tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(migrationDir, 0o755)
	require.NoError(t, err)

	// 创建测试迁移文件
	upFile := filepath.Join(migrationDir, "001_test.up.sql")
	downFile := filepath.Join(migrationDir, "001_test.down.sql")

	err = os.WriteFile(upFile, []byte("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS users;"), 0o644)
	require.NoError(t, err)

	// 设置环境变量
	originalPath := os.Getenv("XUI_DB_PATH")
	originalMigrationPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_DB_PATH")
		}
		if originalMigrationPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalMigrationPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_DB_PATH", testDBPath)
	os.Setenv("XUI_MIGRATIONS_PATH", migrationDir)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行迁移
	err = manager.Up()
	require.NoError(t, err)

	// 验证表存在
	db, err := sql.Open("sqlite3", testDBPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// 执行回滚
	err = manager.Down()
	assert.NoError(t, err)

	// 验证表已删除
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}
