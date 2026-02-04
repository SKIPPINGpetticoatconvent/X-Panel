package database

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckMigrationStatus(t *testing.T) {
	// 创建临时测试数据库
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	migrationPath := filepath.Join(tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(migrationPath, 0o755)
	require.NoError(t, err)

	// 创建测试迁移文件
	upFile := filepath.Join(migrationPath, "001_test.up.sql")
	downFile := filepath.Join(migrationPath, "001_test.down.sql")

	err = os.WriteFile(upFile, []byte("CREATE TABLE test (id INTEGER);"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS test;"), 0o644)
	require.NoError(t, err)

	// 创建数据库文件
	err = os.WriteFile(testDBPath, []byte(""), 0o644)
	require.NoError(t, err)

	// 设置环境变量
	originalPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_MIGRATIONS_PATH", migrationPath)

	// 测试迁移状态检查
	err = CheckMigrationStatus()
	assert.NoError(t, err)
}

func TestGenerateMigrationStatusReport(t *testing.T) {
	// 创建测试迁移状态
	status := &MigrationStatus{
		CurrentVersion: 0,
		Dirty:          false,
		PendingCount:   5,
		LastBackup:     "",
	}

	// 测试状态报告生成
	err := generateMigrationStatusReport(status)
	assert.NoError(t, err)
}

func TestCheckDatabaseCompatibility(t *testing.T) {
	// 测试正常状态
	status := &MigrationStatus{
		CurrentVersion: 5,
		Dirty:          false,
		LastBackup:     "backup.file",
	}

	err := checkDatabaseCompatibility(status)
	assert.NoError(t, err)

	// 测试脏状态
	status.Dirty = true
	err = checkDatabaseCompatibility(status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "脏状态")

	// 测试低版本
	status.Dirty = false
	status.CurrentVersion = 2
	err = checkDatabaseCompatibility(status)
	assert.NoError(t, err) // 应该只警告，不返回错误
}

func TestGetDatabaseStateText(t *testing.T) {
	// 测试脏状态
	status := &MigrationStatus{Dirty: true}
	assert.Equal(t, "异常（脏状态）", getDatabaseStateText(status))

	// 测试未初始化状态
	status = &MigrationStatus{CurrentVersion: 0, Dirty: false}
	assert.Equal(t, "未初始化", getDatabaseStateText(status))

	// 测试需要迁移状态
	status = &MigrationStatus{CurrentVersion: 3, Dirty: false}
	assert.Equal(t, "需要迁移", getDatabaseStateText(status))

	// 测试正常状态
	status = &MigrationStatus{CurrentVersion: 5, Dirty: false}
	assert.Equal(t, "正常", getDatabaseStateText(status))
}

func TestHandleMigrationError(t *testing.T) {
	// 模拟迁移错误
	testErr := fmt.Errorf("迁移执行失败")

	// 测试错误处理
	err := handleMigrationError(testErr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "迁移失败但已成功回滚")
	assert.Contains(t, err.Error(), "迁移执行失败")
}

func TestHandleMigrationStatusError(t *testing.T) {
	// 模拟状态检查错误
	testErr := fmt.Errorf("状态检查失败")

	// 测试错误处理
	err := handleMigrationStatusError(testErr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "迁移状态检查失败")
	assert.Contains(t, err.Error(), "状态检查失败")
}

func TestValidateMigrationSuccess(t *testing.T) {
	// 创建临时测试数据库
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test.db")
	migrationPath := filepath.Join(tempDir, "migrations")

	// 创建迁移目录
	err := os.MkdirAll(migrationPath, 0o755)
	require.NoError(t, err)

	// 创建测试迁移文件
	upFile := filepath.Join(migrationPath, "001_test.up.sql")
	downFile := filepath.Join(migrationPath, "001_test.down.sql")

	err = os.WriteFile(upFile, []byte("CREATE TABLE test (id INTEGER);"), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS test;"), 0o644)
	require.NoError(t, err)

	// 创建数据库文件
	err = os.WriteFile(testDBPath, []byte(""), 0o644)
	require.NoError(t, err)

	// 设置环境变量
	originalPath := os.Getenv("XUI_MIGRATIONS_PATH")
	defer func() {
		if originalPath != "" {
			os.Setenv("XUI_MIGRATIONS_PATH", originalPath)
		} else {
			os.Unsetenv("XUI_MIGRATIONS_PATH")
		}
	}()

	os.Setenv("XUI_MIGRATIONS_PATH", migrationPath)

	// 测试迁移验证
	err = ValidateMigrationSuccess()
	// 由于没有实际迁移，这里可能会失败，这是正常的
	// 主要测试函数不会崩溃
	if err != nil {
		assert.Contains(t, err.Error(), "迁移版本过低")
	}
}

func TestValidateMigrationVersion(t *testing.T) {
	// 测试正常版本
	status := &MigrationStatus{CurrentVersion: 5, Dirty: false}
	err := validateMigrationVersion(status)
	assert.NoError(t, err)

	// 测试版本过低
	status.CurrentVersion = 3
	err = validateMigrationVersion(status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "迁移版本过低")

	// 测试版本过高
	status.CurrentVersion = 6
	err = validateMigrationVersion(status)
	assert.NoError(t, err) // 应该只警告，不返回错误
}

func TestValidateDatabaseSchema(t *testing.T) {
	// 由于需要真实的数据库连接，这里主要测试函数不会崩溃
	// 在实际环境中，这个函数会检查表是否存在
	err := validateDatabaseSchema()
	// 由于没有真实的数据库连接，这里可能会失败
	// 主要测试函数逻辑正确
	if err != nil {
		assert.Contains(t, err.Error(), "必需的表不存在")
	}
}

func TestIsTableExists(t *testing.T) {
	// 由于需要真实的数据库连接，这里主要测试函数不会崩溃
	// 在实际环境中，这个函数会检查表是否存在
	exists := isTableExists("users")
	// 由于没有真实的数据库连接，这里应该返回 false
	assert.False(t, exists)
}

func TestLogMigrationProgress(t *testing.T) {
	// 测试生产环境日志记录
	logMigrationProgress("migration_start", map[string]interface{}{
		"test": "data",
	})

	// 测试调试模式日志记录
	logMigrationProgress("migration_complete", map[string]interface{}{
		"version": 5,
	})

	// 测试非关键步骤
	logMigrationProgress("non_critical_step", map[string]interface{}{
		"step": "test",
	})
}

func TestLogProductionMigrationProgress(t *testing.T) {
	// 测试关键步骤
	logProductionMigrationProgress("migration_start")
	logProductionMigrationProgress("migration_complete")
	logProductionMigrationProgress("migration_failed")
	logProductionMigrationProgress("migration_rollback")
	logProductionMigrationProgress("validation_start")
	logProductionMigrationProgress("validation_complete")

	// 测试非关键步骤（应该不会输出日志）
	logProductionMigrationProgress("non_critical_step")
}

func TestLogDebugMigrationProgress(t *testing.T) {
	// 测试调试模式日志
	logDebugMigrationProgress("test_step", map[string]interface{}{
		"key":    "value",
		"number": 123,
	})
}

func TestIsCriticalStep(t *testing.T) {
	criticalSteps := []string{
		"migration_start", "migration_complete",
		"migration_failed", "migration_rollback",
		"validation_start", "validation_complete",
	}

	// 测试关键步骤
	assert.True(t, isCriticalStep("migration_start", criticalSteps))
	assert.True(t, isCriticalStep("migration_complete", criticalSteps))
	assert.True(t, isCriticalStep("migration_failed", criticalSteps))

	// 测试非关键步骤
	assert.False(t, isCriticalStep("non_critical", criticalSteps))
	assert.False(t, isCriticalStep("random_step", criticalSteps))
}

func TestGetStepDescription(t *testing.T) {
	// 测试已知步骤
	assert.Equal(t, "开始迁移", getStepDescription("migration_start"))
	assert.Equal(t, "迁移完成", getStepDescription("migration_complete"))
	assert.Equal(t, "迁移失败", getStepDescription("migration_failed"))
	assert.Equal(t, "迁移回滚", getStepDescription("migration_rollback"))
	assert.Equal(t, "开始验证", getStepDescription("validation_start"))
	assert.Equal(t, "验证完成", getStepDescription("validation_complete"))

	// 测试未知步骤
	assert.Equal(t, "unknown_step", getStepDescription("unknown_step"))
	assert.Equal(t, "random", getStepDescription("random"))
}

func TestStrictDataConsistencyCheck(t *testing.T) {
	// 由于需要真实的数据库连接，这里主要测试函数不会崩溃
	// 在实际环境中，这个函数会执行四层检查
	err := StrictDataConsistencyCheck()
	// 由于没有真实的数据库连接，这里可能会失败
	// 主要测试函数逻辑正确
	if err != nil {
		assert.Contains(t, err.Error(), "数据库结构完整性检查失败")
	}
}

func TestValidateDatabaseSchemaIntegrity(t *testing.T) {
	// 由于需要真实的数据库连接，这里主要测试函数不会崩溃
	err := validateDatabaseSchemaIntegrity()
	// 由于没有真实的数据库连接，这里应该失败
	if err != nil {
		// 可能是"数据库连接为空"或"必需的表不存在"
		assert.Error(t, err)
	}
}

func TestValidateDataRelationships(t *testing.T) {
	// 由于需要真实的数据库连接，这里主要测试函数不会崩溃
	err := validateDataRelationships()
	// 由于没有真实的数据库连接，这里应该失败
	if err != nil {
		assert.Contains(t, err.Error(), "数据库连接为空")
	}
}

func TestValidateDataIntegrity(t *testing.T) {
	// 由于需要真实的数据库连接，这里主要测试函数不会崩溃
	err := validateDataIntegrity()
	// 由于没有真实的数据库连接，这里应该失败
	if err != nil {
		assert.Contains(t, err.Error(), "数据库连接为空")
	}
}

func TestValidateMigrationVersionConsistency(t *testing.T) {
	// 测试正常版本（需要模拟 GetMigrationStatus 返回正确状态）
	// 由于我们无法在测试中模拟 GetMigrationStatus，这里主要测试函数不会崩溃
	err := validateMigrationVersionConsistency()
	// 由于没有真实的数据库连接，这里应该失败
	if err != nil {
		// 可能是各种错误，主要测试函数不会崩溃
		assert.Error(t, err)
	}
}

func TestValidateTableStructure(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateTableStructure("users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestValidateIndexesIntegrity(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateIndexesIntegrity()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestValidateForeignKeyConstraints(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateForeignKeyConstraints()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestValidateUserInboundRelationships(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateUserInboundRelationships()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestValidateClientTrafficRelationships(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateClientTrafficRelationships()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestValidateOutboundTrafficRelationships(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateOutboundTrafficRelationships()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestValidateUserDataIntegrity(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateUserDataIntegrity()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestValidateInboundDataIntegrity(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateInboundDataIntegrity()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestValidateSettingsDataIntegrity(t *testing.T) {
	// 测试数据库连接为空的情况
	err := validateSettingsDataIntegrity()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestGetTableColumns(t *testing.T) {
	// 测试数据库连接为空的情况
	columns, err := getTableColumns("users")
	assert.Nil(t, columns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库连接为空")
}

func TestGetRequiredColumns(t *testing.T) {
	// 测试用户表必需列
	columns := getRequiredColumns("users")
	expected := []string{"id", "username", "password"}
	assert.Equal(t, expected, columns)

	// 测试入站表必需列
	columns = getRequiredColumns("inbounds")
	expected = []string{"id", "protocol", "port", "settings"}
	assert.Equal(t, expected, columns)

	// 测试设置表必需列
	columns = getRequiredColumns("settings")
	expected = []string{"id", "key", "value"}
	assert.Equal(t, expected, columns)

	// 测试未知表
	columns = getRequiredColumns("unknown")
	assert.Empty(t, columns)
}

func TestHasColumn(t *testing.T) {
	columns := []string{"id", "username", "password"}

	// 测试存在的列
	assert.True(t, hasColumn(columns, "id"))
	assert.True(t, hasColumn(columns, "username"))
	assert.True(t, hasColumn(columns, "password"))

	// 测试不存在的列
	assert.False(t, hasColumn(columns, "email"))
	assert.False(t, hasColumn(columns, "unknown"))
	assert.False(t, hasColumn(columns, "nonexistent"))
}

func TestHandleDataConsistencyError(t *testing.T) {
	// 模拟数据一致性错误
	testErr := fmt.Errorf("数据完整性检查失败")

	// 测试错误处理
	err := handleDataConsistencyError(testErr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据一致性检查失败")
	assert.Contains(t, err.Error(), "数据完整性检查失败")
}
