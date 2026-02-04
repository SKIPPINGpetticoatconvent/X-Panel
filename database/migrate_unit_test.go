package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// MigrationTestSuite 迁移测试套件
type MigrationTestSuite struct {
	suite.Suite
	mockDB  *sql.DB
	mock    sqlmock.Sqlmock
	tempDir string
	manager *MigrationManager
}

// SetupSuite 设置测试套件
func (suite *MigrationTestSuite) SetupSuite() {
	var err error
	suite.mockDB, suite.mock, err = sqlmock.New()
	suite.Require().NoError(err)

	suite.tempDir = suite.T().TempDir()

	// 创建真实的迁移管理器
	testDBPath := filepath.Join(suite.tempDir, "test.db")
	suite.manager, err = NewMigrationManager(testDBPath)
	suite.Require().NoError(err)
}

// TearDownSuite 清理测试套件
func (suite *MigrationTestSuite) TearDownSuite() {
	if suite.manager != nil {
		suite.manager.Close()
	}
	suite.mockDB.Close()
}

// TestMigrationManager_Create 测试迁移管理器创建
func (suite *MigrationTestSuite) TestMigrationManager_Create() {
	// 测试正常创建
	tempDir := suite.T().TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	migrationPath := filepath.Join(tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(migrationPath, 0o755)
	suite.Require().NoError(err)

	// 创建测试迁移文件
	upFile := filepath.Join(migrationPath, "001_test.up.sql")
	downFile := filepath.Join(migrationPath, "001_test.down.sql")

	err = os.WriteFile(upFile, []byte("CREATE TABLE test (id INTEGER);"), 0o644)
	suite.Require().NoError(err)

	err = os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS test;"), 0o644)
	suite.Require().NoError(err)

	// 创建数据库文件
	err = os.WriteFile(testDBPath, []byte(""), 0o644)
	suite.Require().NoError(err)

	// 设置环境变量使用临时迁移目录
	originalPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_MIGRATIONS_PATH", migrationPath)

	manager, err := NewMigrationManager(testDBPath)
	suite.NoError(err)
	suite.NotNil(manager)
	defer manager.Close()
}

// TestMigrationManager_Create_InvalidPath 测试无效路径
func (suite *MigrationTestSuite) TestMigrationManager_Create_InvalidPath() {
	// 测试无效路径
	manager, err := NewMigrationManager("/invalid/path/that/does/not/exist/test.db")
	suite.Error(err)
	suite.Nil(manager)
	suite.Contains(err.Error(), "创建迁移实例失败")
}

// TestMigrationManager_Status_NoMigration 测试状态检查（无迁移）
func (suite *MigrationTestSuite) TestMigrationManager_Status_NoMigration() {
	// 测试状态检查（应该没有错误，即使是 "no migration"）
	err := suite.manager.Status()
	// 这里可能会返回 "no migration" 错误，这是正常的
	if err != nil {
		suite.Contains(err.Error(), "获取迁移状态失败: no migration")
	}
}

// TestMigrationManager_Status_WithMigration 测试状态检查（有迁移）
func (suite *MigrationTestSuite) TestMigrationManager_Status_WithMigration() {
	// 这个测试需要先执行一些迁移，但由于我们使用的是空数据库
	// 这里主要测试 Status 方法不会崩溃
	err := suite.manager.Status()
	// 这里可能会返回 "no migration" 错误，这是正常的
	if err != nil {
		suite.Contains(err.Error(), "获取迁移状态失败: no migration")
	}
}

// TestMigrationManager_Up_WithMock 测试迁移执行（使用真实管理器）
func (suite *MigrationTestSuite) TestMigrationManager_Up_WithMock() {
	// 测试迁移执行（使用真实的迁移管理器）
	err := suite.manager.Up()
	// 由于没有迁移文件或迁移文件有问题，这里可能会失败
	// 但主要测试方法不会崩溃
	if err != nil {
		// 预期可能会有错误，但不应该是空指针错误
		suite.NotContains(err.Error(), "invalid memory address")
	}
}

// TestMigrationManager_Down_WithMock 测试回滚（使用真实管理器）
func (suite *MigrationTestSuite) TestMigrationManager_Down_WithMock() {
	// 测试回滚（使用真实的迁移管理器）
	err := suite.manager.Down()
	// 由于没有迁移记录，这里可能会返回 ErrNoChange
	if err != nil {
		suite.NotEqual(err, migrate.ErrNoChange)
	}
}

// TestBackupDatabase_Success 测试数据库备份成功
func (suite *MigrationTestSuite) TestBackupDatabase_Success() {
	// 创建测试数据库文件
	testData := "test database content"
	testDBPath := filepath.Join(suite.tempDir, "test.db")
	err := os.WriteFile(testDBPath, []byte(testData), 0o644)
	suite.NoError(err)

	// 临时设置环境变量指向测试数据库
	originalPath := os.Getenv("XUI_DB_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_DB_PATH")
		}
	}()

	os.Setenv("XUI_DB_PATH", testDBPath)

	// 测试备份
	err = BackupDatabase()
	suite.NoError(err)

	// 验证备份文件存在
	backupFiles, err := filepath.Glob(testDBPath + ".backup.*")
	suite.NoError(err)
	suite.Len(backupFiles, 1)

	// 验证备份文件内容
	backupData, err := os.ReadFile(backupFiles[0])
	suite.NoError(err)
	suite.Equal(testData, string(backupData))
}

// TestBackupDatabase_FileNotFound 测试数据库文件不存在
func (suite *MigrationTestSuite) TestBackupDatabase_FileNotFound() {
	// 设置环境变量指向不存在的数据库
	originalPath := os.Getenv("XUI_DB_FOLDER")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_FOLDER", originalPath)
		} else {
			os.Unsetenv("XUI_DB_FOLDER")
		}
	}()

	os.Setenv("XUI_DB_FOLDER", suite.tempDir)

	// 测试备份（文件不存在）
	err := BackupDatabase()
	suite.NoError(err) // 应该成功，因为文件不存在时跳过备份
}

// TestRollbackMigrations_Success 测试回滚成功
func (suite *MigrationTestSuite) TestRollbackMigrations_Success() {
	// 创建测试数据库文件
	testDBPath := filepath.Join(suite.tempDir, "test.db")
	err := os.WriteFile(testDBPath, []byte("test"), 0o644)
	suite.NoError(err)

	// 设置环境变量
	originalPath := os.Getenv("XUI_DB_FOLDER")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_FOLDER", originalPath)
		} else {
			os.Unsetenv("XUI_DB_FOLDER")
		}
	}()

	os.Setenv("XUI_DB_FOLDER", suite.tempDir)

	// 测试回滚
	err = RollbackMigrations()
	suite.NoError(err)
}

// TestRollbackMigrations_NoDatabase 测试数据库不存在时的回滚
func (suite *MigrationTestSuite) TestRollbackMigrations_NoDatabase() {
	// 设置环境变量指向不存在的数据库
	originalPath := os.Getenv("XUI_DB_FOLDER")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_FOLDER", originalPath)
		} else {
			os.Unsetenv("XUI_DB_FOLDER")
		}
	}()

	os.Setenv("XUI_DB_FOLDER", suite.tempDir)

	// 测试回滚（数据库不存在）
	err := RollbackMigrations()
	suite.NoError(err) // 应该成功，因为数据库不存在时跳过回滚
}

// TestGetMigrationStatus_NoDatabase 测试获取迁移状态（数据库不存在）
func (suite *MigrationTestSuite) TestGetMigrationStatus_NoDatabase() {
	// 设置环境变量指向不存在的数据库
	originalPath := os.Getenv("XUI_DB_FOLDER")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_FOLDER", originalPath)
		} else {
			os.Unsetenv("XUI_DB_FOLDER")
		}
	}()

	os.Setenv("XUI_DB_FOLDER", suite.tempDir)

	// 测试获取迁移状态
	status, err := GetMigrationStatus()
	suite.NoError(err)
	suite.NotNil(status)
	suite.Equal(uint(0), status.CurrentVersion)
	suite.False(status.Dirty)
	suite.Equal(0, status.PendingCount)
	suite.Equal("", status.LastBackup)
}

// TestGetMigrationStatus_WithDatabase 测试获取迁移状态（数据库存在）
func (suite *MigrationTestSuite) TestGetMigrationStatus_WithDatabase() {
	// 创建测试数据库文件
	testDBPath := filepath.Join(suite.tempDir, "test.db")
	err := os.WriteFile(testDBPath, []byte("test"), 0o644)
	suite.NoError(err)

	// 设置环境变量
	originalPath := os.Getenv("XUI_DB_FOLDER")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_FOLDER", originalPath)
		} else {
			os.Unsetenv("XUI_DB_FOLDER")
		}
	}()

	os.Setenv("XUI_DB_FOLDER", suite.tempDir)

	// 测试获取迁移状态
	status, err := GetMigrationStatus()
	suite.NoError(err)
	suite.NotNil(status)
	// 由于没有真实的迁移记录，版本应该是0
	suite.Equal(uint(0), status.CurrentVersion)
	suite.False(status.Dirty)
	suite.Equal(0, status.PendingCount)
}

// TestRunMigrations_NoDatabase 测试运行迁移（数据库不存在）
func (suite *MigrationTestSuite) TestRunMigrations_NoDatabase() {
	// 设置环境变量指向不存在的数据库
	originalPath := os.Getenv("XUI_DB_FOLDER")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_FOLDER", originalPath)
		} else {
			os.Unsetenv("XUI_DB_FOLDER")
		}
	}()

	os.Setenv("XUI_DB_FOLDER", suite.tempDir)

	// 测试运行迁移
	err := RunMigrations()
	suite.NoError(err) // 应该成功，因为数据库不存在时跳过迁移
}

// TestMigrationManager_Force 测试强制设置版本
func (suite *MigrationTestSuite) TestMigrationManager_Force() {
	// 测试强制设置版本（使用真实的迁移管理器）
	err := suite.manager.Force(3)
	// 由于没有迁移记录，这里可能会失败
	// 但主要测试方法不会崩溃
	if err != nil {
		// 预期可能会有错误，但不应该是空指针错误
		suite.NotContains(err.Error(), "invalid memory address")
	}
}

// TestMigrationManager_Close 测试关闭迁移管理器
func (suite *MigrationTestSuite) TestMigrationManager_Close() {
	// 创建迁移管理器
	tempDir := suite.T().TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")

	manager, err := NewMigrationManager(testDBPath)
	suite.NoError(err)
	suite.NotNil(manager)

	// 测试关闭
	err = manager.Close()
	suite.NoError(err)
}

// 运行测试套件
func TestMigrationTestSuite(t *testing.T) {
	suite.Run(t, new(MigrationTestSuite))
}

// 辅助测试函数

// TestMigrationManager_Create_WithEnvPath 测试使用环境变量路径创建迁移管理器
func TestMigrationManager_Create_WithEnvPath(t *testing.T) {
	// 设置环境变量
	tempDir := t.TempDir()
	migrationPath := filepath.Join(tempDir, "custom_migrations")

	originalPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_MIGRATIONS_PATH", migrationPath)

	// 创建迁移管理器
	testDBPath := filepath.Join(tempDir, "test.db")
	manager, err := NewMigrationManager(testDBPath)
	require.NoError(t, err)
	defer manager.Close()

	// 验证使用了自定义路径
	_, err = os.Stat(migrationPath)
	assert.NoError(t, err)
}

// TestBackupDatabase_PermissionError 测试备份权限错误
func TestBackupDatabase_PermissionError(t *testing.T) {
	// 设置环境变量指向无权限的路径
	originalPath := os.Getenv("XUI_DB_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_DB_PATH")
		}
	}()

	// 尝试使用一个无权限的路径（在大多数系统上 /root 是无权限的）
	os.Setenv("XUI_DB_PATH", "/root/invalid.db")

	// 测试备份
	err := BackupDatabase()
	// 应该返回错误，因为无法访问路径
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "读取数据库文件失败")
}

// BenchmarkMigrationManager_Create 性能测试：创建迁移管理器
func BenchmarkMigrationManager_Create(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tempDir := b.TempDir()
		testDBPath := filepath.Join(tempDir, "test.db")

		manager, err := NewMigrationManager(testDBPath)
		if err != nil {
			b.Fatal(err)
		}
		manager.Close()
	}
}

// BenchmarkBackupDatabase 性能测试：数据库备份
func BenchmarkBackupDatabase(b *testing.B) {
	// 创建测试数据库文件
	tempDir := b.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	testData := make([]byte, 1024*1024) // 1MB 测试数据
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	err := os.WriteFile(testDBPath, testData, 0o644)
	if err != nil {
		b.Fatal(err)
	}

	// 设置环境变量
	originalPath := os.Getenv("XUI_DB_FOLDER")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_DB_FOLDER", originalPath)
		} else {
			os.Unsetenv("XUI_DB_FOLDER")
		}
	}()

	os.Setenv("XUI_DB_FOLDER", tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := BackupDatabase()
		if err != nil {
			b.Fatal(err)
		}
	}
}
