package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationManager_NewMigrationManager 测试迁移管理器创建
func TestMigrationManager_NewMigrationManager(t *testing.T) {
	// 测试正常创建
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")

	manager, err := NewMigrationManager(testDBPath)
	require.NoError(t, err)
	require.NotNil(t, manager)
	defer manager.Close()

	// 验证管理器属性
	assert.Equal(t, testDBPath, manager.dbPath)
	assert.NotNil(t, manager.migrate)
	assert.NotEmpty(t, manager.migrationPath)
}

// TestMigrationManager_NewMigrationManager_WithEnvPath 测试使用环境变量路径创建
func TestMigrationManager_NewMigrationManager_WithEnvPath(t *testing.T) {
	// 设置环境变量
	tempDir := t.TempDir()
	customMigrationPath := filepath.Join(tempDir, "custom_migrations")

	originalPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_MIGRATIONS_PATH", customMigrationPath)

	testDBPath := filepath.Join(tempDir, "test.db")
	manager, err := NewMigrationManager(testDBPath)
	require.NoError(t, err)
	require.NotNil(t, manager)
	defer manager.Close()

	// 验证使用了自定义路径
	assert.Equal(t, customMigrationPath, manager.migrationPath)
}

// TestMigrationManager_NewMigrationManager_InvalidPath 测试无效路径
func TestMigrationManager_NewMigrationManager_InvalidPath(t *testing.T) {
	// 测试无效数据库路径
	manager, err := NewMigrationManager("/invalid/path/test.db")
	assert.Error(t, err)
	assert.Nil(t, manager)

	// 测试不存在的迁移目录
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	invalidMigrationPath := filepath.Join(tempDir, "nonexistent")

	originalPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_MIGRATIONS_PATH", invalidMigrationPath)

	manager, err = NewMigrationManager(testDBPath)
	assert.Error(t, err)
	assert.Nil(t, manager)
}

// TestMigrationManager_Up_Mock 测试 Up 方法的 mock 版本
func TestMigrationManager_Up_Mock(t *testing.T) {
	// 创建 mock 数据库
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// 设置 mock 期望
	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SELECT version, dirty FROM schema_migrations").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO schema_migrations").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// 注意：这里我们无法直接 mock golang-migrate，因为它使用自己的数据库连接
	// 但我们可以测试错误处理逻辑
	t.Run("Up with invalid manager", func(t *testing.T) {
		manager := &MigrationManager{
			migrate: nil, // 故意设置为 nil 来测试错误处理
		}

		err = manager.Up()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "迁移执行失败")
	})
}

// TestMigrationManager_Down_Mock 测试 Down 方法的 mock 版本
func TestMigrationManager_Down_Mock(t *testing.T) {
	t.Run("Down with invalid manager", func(t *testing.T) {
		manager := &MigrationManager{
			migrate: nil, // 故意设置为 nil 来测试错误处理
		}

		err := manager.Down()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "回滚迁移失败")
	})
}

// TestMigrationManager_Status_Mock 测试 Status 方法的 mock 版本
func TestMigrationManager_Status_Mock(t *testing.T) {
	t.Run("Status with invalid manager", func(t *testing.T) {
		manager := &MigrationManager{
			migrate: nil, // 故意设置为 nil 来测试错误处理
		}

		err := manager.Status()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "获取迁移状态失败")
	})
}

// TestMigrationManager_Force_Mock 测试 Force 方法的 mock 版本
func TestMigrationManager_Force_Mock(t *testing.T) {
	t.Run("Force with invalid manager", func(t *testing.T) {
		manager := &MigrationManager{
			migrate: nil, // 故意设置为 nil 来测试错误处理
		}

		err := manager.Force(1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "强制设置迁移版本失败")
	})
}

// TestMigrationManager_Close 测试 Close 方法
func TestMigrationManager_Close(t *testing.T) {
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")

	manager, err := NewMigrationManager(testDBPath)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// 测试正常关闭
	err = manager.Close()
	assert.NoError(t, err)

	// 测试重复关闭（应该不报错）
	err = manager.Close()
	assert.NoError(t, err)
}

// TestMigrationManager_Integration 集成测试
func TestMigrationManager_Integration(t *testing.T) {
	// 创建临时测试数据库
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	migrationPath := filepath.Join(tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(migrationPath, 0o755)
	require.NoError(t, err)

	// 创建多个测试迁移文件
	migrations := []struct {
		version int
		upSQL   string
		downSQL string
	}{
		{
			version: 1,
			upSQL:   "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);",
			downSQL: "DROP TABLE IF EXISTS users;",
		},
		{
			version: 2,
			upSQL:   "CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT, user_id INTEGER);",
			downSQL: "DROP TABLE IF EXISTS posts;",
		},
	}

	for _, mig := range migrations {
		upFile := filepath.Join(migrationPath, fmt.Sprintf("%03d_test.up.sql", mig.version))
		downFile := filepath.Join(migrationPath, fmt.Sprintf("%03d_test.down.sql", mig.version))

		err = os.WriteFile(upFile, []byte(mig.upSQL), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(downFile, []byte(mig.downSQL), 0o644)
		require.NoError(t, err)
	}

	// 创建迁移管理器
	manager, err := NewMigrationManager(testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行所有迁移
	err = manager.Up()
	assert.NoError(t, err)

	// 验证表是否创建
	db, err := sql.Open("sqlite3", testDBPath)
	require.NoError(t, err)
	defer db.Close()

	// 检查 users 表
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// 检查 posts 表
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='posts'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// 检查迁移版本
	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 2, version)

	// 回滚一个迁移
	err = manager.Down()
	assert.NoError(t, err)

	// 验证 posts 表已删除
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='posts'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// 验证 users 表仍然存在
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// 检查迁移版本
	err = db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 1, version)
}
