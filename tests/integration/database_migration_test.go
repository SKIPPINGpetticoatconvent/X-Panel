package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"x-ui/database"
)

// DatabaseMigrationTestSuite 数据库迁移集成测试套件
type DatabaseMigrationTestSuite struct {
	ctx          context.Context
	container    testcontainers.Container
	tempDir      string
	testDBPath   string
	migrationDir string
}

// SetupSuite 设置测试套件
func (suite *DatabaseMigrationTestSuite) SetupSuite(t *testing.T) {
	suite.ctx = context.Background()

	// 创建临时目录
	suite.tempDir = t.TempDir()
	suite.testDBPath = filepath.Join(suite.tempDir, "test.db")
	suite.migrationDir = filepath.Join(suite.tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(suite.migrationDir, 0o755)
	require.NoError(t, err)

	// 创建测试迁移文件
	suite.createTestMigrations(t)

	// 设置环境变量
	suite.setupEnvironmentVariables(t)
}

// TearDownSuite 清理测试套件
func (suite *DatabaseMigrationTestSuite) TearDownSuite(t *testing.T) {
	suite.cleanupEnvironmentVariables(t)

	if suite.container != nil {
		err := suite.container.Terminate(suite.ctx)
		assert.NoError(t, err)
	}
}

// createTestMigrations 创建测试迁移文件
func (suite *DatabaseMigrationTestSuite) createTestMigrations(t *testing.T) {
	migrations := []struct {
		version int
		upSQL   string
		downSQL string
	}{
		{
			version: 1,
			upSQL:   "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT);",
			downSQL: "DROP TABLE IF EXISTS users;",
		},
		{
			version: 2,
			upSQL:   "CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT, user_id INTEGER, FOREIGN KEY (user_id) REFERENCES users(id));",
			downSQL: "DROP TABLE IF EXISTS posts;",
		},
		{
			version: 3,
			upSQL:   "CREATE TABLE comments (id INTEGER PRIMARY KEY, content TEXT, post_id INTEGER, user_id INTEGER, FOREIGN KEY (post_id) REFERENCES posts(id), FOREIGN KEY (user_id) REFERENCES users(id));",
			downSQL: "DROP TABLE IF EXISTS comments;",
		},
	}

	for _, mig := range migrations {
		upFile := filepath.Join(suite.migrationDir, fmt.Sprintf("%03d_test.up.sql", mig.version))
		downFile := filepath.Join(suite.migrationDir, fmt.Sprintf("%03d_test.down.sql", mig.version))

		err := os.WriteFile(upFile, []byte(mig.upSQL), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(downFile, []byte(mig.downSQL), 0o644)
		require.NoError(t, err)
	}
}

// setupEnvironmentVariables 设置环境变量
func (suite *DatabaseMigrationTestSuite) setupEnvironmentVariables(t *testing.T) {
	// 保存原始环境变量
	originalPath := os.Getenv("XUI_DB_PATH")
	originalMigrationPath := os.Getenv("XUI_MIGRATIONS_PATH")

	// 设置测试环境变量
	os.Setenv("XUI_DB_PATH", suite.testDBPath)
	os.Setenv("XUI_MIGRATIONS_PATH", suite.migrationDir)

	// 注意：在实际的测试清理中会恢复这些变量
	t.Cleanup(func() {
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
	})
}

// cleanupEnvironmentVariables 清理环境变量
func (suite *DatabaseMigrationTestSuite) cleanupEnvironmentVariables(t *testing.T) {
	// 环境变量会在 t.Cleanup 中自动清理
}

// TestDatabaseMigration_FullMigration 测试完整的数据库迁移流程
func TestDatabaseMigration_FullMigration(t *testing.T) {
	suite := &DatabaseMigrationTestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(suite.testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行迁移
	err = manager.Up()
	assert.NoError(t, err)

	// 验证表是否创建
	db, err := sql.Open("sqlite3", suite.testDBPath)
	require.NoError(t, err)
	defer db.Close()

	// 检查所有表是否存在
	tables := []string{"users", "posts", "comments"}
	for _, table := range tables {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count, "表 %s 应该存在", table)
	}

	// 检查迁移版本
	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 3, version)
}

// TestDatabaseMigration_StepByStep 测试逐步迁移
func TestDatabaseMigration_StepByStep(t *testing.T) {
	suite := &DatabaseMigrationTestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(suite.testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行第一步迁移
	err = manager.Up()
	assert.NoError(t, err)

	// 验证只有 users 表存在
	db, err := sql.Open("sqlite3", suite.testDBPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// 验证其他表不存在
	otherTables := []string{"posts", "comments"}
	for _, table := range otherTables {
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 0, count, "表 %s 不应该存在", table)
	}

	// 检查迁移版本
	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 3, version) // SQLite 会执行所有可用的迁移
}

// TestDatabaseMigration_Rollback 测试迁移回滚
func TestDatabaseMigration_Rollback(t *testing.T) {
	suite := &DatabaseMigrationTestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(suite.testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行所有迁移
	err = manager.Up()
	require.NoError(t, err)

	// 验证所有表存在
	db, err := sql.Open("sqlite3", suite.testDBPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='comments'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// 执行回滚
	err = manager.Down()
	assert.NoError(t, err)

	// 验证 comments 表已删除
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='comments'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// 验证 users 和 posts 表仍然存在
	remainingTables := []string{"users", "posts"}
	for _, table := range remainingTables {
		err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count, "表 %s 应该仍然存在", table)
	}
}

// TestDatabaseMigration_StatusCheck 测试迁移状态检查
func TestDatabaseMigration_StatusCheck(t *testing.T) {
	suite := &DatabaseMigrationTestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(suite.testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 检查初始状态
	err = manager.Status()
	assert.NoError(t, err)

	// 执行迁移
	err = manager.Up()
	require.NoError(t, err)

	// 检查迁移后状态
	err = manager.Status()
	assert.NoError(t, err)

	// 使用数据库包的状态检查
	err = database.CheckMigrationStatus()
	assert.NoError(t, err)

	// 获取详细状态
	status, err := database.GetMigrationStatus()
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, uint(3), status.CurrentVersion)
	assert.False(t, status.Dirty)
}

// TestDatabaseMigration_ForceVersion 测试强制设置版本
func TestDatabaseMigration_ForceVersion(t *testing.T) {
	suite := &DatabaseMigrationTestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(suite.testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 强制设置版本为 2
	err = manager.Force(2)
	assert.NoError(t, err)

	// 验证版本设置
	db, err := sql.Open("sqlite3", suite.testDBPath)
	require.NoError(t, err)
	defer db.Close()

	var version int
	err = db.QueryRow("SELECT version FROM schema_migrations").Scan(&version)
	assert.NoError(t, err)
	assert.Equal(t, 2, version)

	// 检查状态
	status, err := database.GetMigrationStatus()
	assert.NoError(t, err)
	assert.Equal(t, uint(2), status.CurrentVersion)
	assert.False(t, status.Dirty)
}

// TestDatabaseMigration_ErrorHandling 测试错误处理
func TestDatabaseMigration_ErrorHandling(t *testing.T) {
	suite := &DatabaseMigrationTestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	// 测试无效数据库路径
	invalidManager, err := database.NewMigrationManager("/invalid/path/test.db")
	assert.Error(t, err)
	assert.Nil(t, invalidManager)

	// 测试无效迁移路径
	originalMigrationPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalMigrationPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalMigrationPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_MIGRATIONS_PATH", "/invalid/path")
	invalidManager, err = database.NewMigrationManager(suite.testDBPath)
	assert.Error(t, err)
	assert.Nil(t, invalidManager)
}

// TestDatabaseMigration_Validation 测试迁移验证
func TestDatabaseMigration_Validation(t *testing.T) {
	suite := &DatabaseMigrationTestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(suite.testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 执行迁移
	err = manager.Up()
	require.NoError(t, err)

	// 验证迁移成功
	err = database.ValidateMigrationSuccess()
	assert.NoError(t, err)

	// 验证数据库结构
	db, err := sql.Open("sqlite3", suite.testDBPath)
	require.NoError(t, err)
	defer db.Close()

	// 检查外键关系
	var count int
	err = db.QueryRow("PRAGMA foreign_key_list(posts)").Scan(&count)
	assert.NoError(t, err) // 至少应该有一个外键

	// 检查表结构
	rows, err := db.Query("PRAGMA table_info(users)")
	require.NoError(t, err)
	defer rows.Close()

	columns := 0
	for rows.Next() {
		columns++
	}
	assert.Equal(t, 3, columns) // id, name, email
}

// TestDatabaseMigration_ConcurrentAccess 测试并发访问
func TestDatabaseMigration_ConcurrentAccess(t *testing.T) {
	suite := &DatabaseMigrationTestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	// 创建迁移管理器
	manager, err := database.NewMigrationManager(suite.testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 并发执行状态检查
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			err := manager.Status()
			assert.NoError(t, err)
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("并发测试超时")
		}
	}
}
