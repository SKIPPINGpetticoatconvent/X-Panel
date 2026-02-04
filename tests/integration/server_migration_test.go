package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-ui/config"
	"x-ui/database"
)

// TestServerMigrationIntegration 测试服务器数据库迁移集成
func TestServerMigrationIntegration(t *testing.T) {
	// 获取服务器数据库文件的绝对路径
	dbPath := "/home/ub/X-Panel/database/test_server.db"
	absPath, err := filepath.Abs(dbPath)
	require.NoError(t, err)

	t.Logf("数据库文件路径: %s", absPath)

	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skipf("服务器数据库文件不存在: %s", absPath)
		return
	}

	// 设置环境变量使用服务器数据库
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

	os.Setenv("XUI_DB_PATH", absPath)
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")

	// 刷新配置
	config.RefreshEnvConfig()

	t.Log("=== 使用服务器真实数据库测试 golang-migrate ===")

	// 1. 创建迁移管理器测试
	t.Log("1. 创建迁移管理器测试...")
	manager, err := database.NewMigrationManager(absPath)
	if err != nil {
		t.Logf("创建迁移管理器失败: %v", err)
		t.FailNow()
	}
	defer manager.Close()

	t.Log("迁移管理器创建成功")

	// 获取当前状态
	err = manager.Status()
	if err != nil {
		t.Logf("获取状态失败: %v", err)
	} else {
		t.Log("状态获取成功")
	}

	// 2. 检查迁移状态
	t.Log("2. 检查迁移状态...")
	err = database.CheckMigrationStatus()
	assert.NoError(t, err, "迁移状态检查应该成功")

	// 3. 执行数据库迁移
	t.Log("3. 执行数据库迁移...")
	err = database.RunMigrationsWithBackup()
	assert.NoError(t, err, "数据库迁移应该成功")

	// 4. 验证迁移成功
	t.Log("4. 验证迁移成功...")
	err = database.ValidateMigrationSuccess()
	assert.NoError(t, err, "迁移验证应该成功")

	// 5. 获取迁移状态报告
	t.Log("5. 获取迁移状态报告...")
	status, err := database.GetMigrationStatus()
	assert.NoError(t, err, "获取迁移状态应该成功")
	assert.NotNil(t, status, "迁移状态不应该为空")

	t.Logf("迁移版本: %d", status.CurrentVersion)
	t.Logf("数据库状态: %s", func() string {
		if status.Dirty {
			return "脏状态"
		}
		return "正常"
	}())

	t.Log("=== 服务器数据库测试完成 ===")
}

// TestUpgradeCompatibilityIntegration 测试升级兼容性集成
func TestUpgradeCompatibilityIntegration(t *testing.T) {
	// 获取服务器数据库文件的绝对路径
	dbPath := "/home/ub/X-Panel/database/test_server.db"
	absPath, err := filepath.Abs(dbPath)
	require.NoError(t, err)

	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skipf("服务器数据库文件不存在: %s", absPath)
		return
	}

	// 创建临时副本进行测试
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test_upgrade.db")

	// 复制服务器数据库到临时位置
	data, err := os.ReadFile(absPath)
	require.NoError(t, err)
	err = os.WriteFile(testDBPath, data, 0o644)
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
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")

	// 刷新配置
	config.RefreshEnvConfig()

	t.Log("=== 升级兼容性测试 ===")
	t.Logf("测试数据库: %s", testDBPath)

	// 执行升级流程
	err = database.CheckMigrationStatus()
	assert.NoError(t, err, "迁移状态检查应该成功")

	err = database.RunMigrationsWithBackup()
	assert.NoError(t, err, "数据库迁移应该成功")

	err = database.ValidateMigrationSuccess()
	assert.NoError(t, err, "迁移验证应该成功")

	// 验证升级结果
	status, err := database.GetMigrationStatus()
	assert.NoError(t, err, "获取迁移状态应该成功")
	assert.NotNil(t, status, "迁移状态不应该为空")

	t.Logf("升级后版本: %d", status.CurrentVersion)
	assert.Greater(t, status.CurrentVersion, uint(0), "版本应该大于0")
	assert.False(t, status.Dirty, "数据库不应该处于脏状态")

	t.Log("✅ 升级兼容性测试成功")
}

// TestInitDBCompatibilityIntegration 测试 InitDB 兼容性集成
func TestInitDBCompatibilityIntegration(t *testing.T) {
	// 创建临时测试数据库
	tempDir := t.TempDir()
	testDBPath := filepath.Join(tempDir, "test_initdb.db")

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
	os.Setenv("XUI_MIGRATIONS_PATH", "/home/ub/X-Panel/database/migrations")

	// 刷新配置
	config.RefreshEnvConfig()

	t.Log("=== InitDB 兼容性测试 ===")
	t.Logf("测试数据库: %s", testDBPath)

	// 模拟 InitDB 流程（不包含用户初始化部分）
	err := database.CheckMigrationStatus()
	assert.NoError(t, err, "迁移状态检查应该成功")

	err = database.RunMigrationsWithBackup()
	assert.NoError(t, err, "数据库迁移应该成功")

	err = database.ValidateMigrationSuccess()
	assert.NoError(t, err, "迁移验证应该成功")

	// 验证基础数据一致性
	status, err := database.GetMigrationStatus()
	assert.NoError(t, err, "获取迁移状态应该成功")
	assert.NotNil(t, status, "迁移状态不应该为空")

	t.Logf("InitDB 流程完成 - 版本: %d, 状态: 正常", status.CurrentVersion)
	assert.Greater(t, status.CurrentVersion, uint(0), "版本应该大于0")
	assert.False(t, status.Dirty, "数据库不应该处于脏状态")

	t.Log("✅ InitDB 兼容性测试成功")
}
