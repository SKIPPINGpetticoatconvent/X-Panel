package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// UpgradeE2ETestSuite 升级端到端测试套件
type UpgradeE2ETestSuite struct {
	ctx         context.Context
	container   testcontainers.Container
	projectRoot string
	tempDir     string
}

// SetupSuite 设置测试套件
func (s *UpgradeE2ETestSuite) SetupSuite(t *testing.T) {
	s.ctx = context.Background()

	// 解决 Testcontainers 在某些环境下对 XDG_RUNTIME_DIR 敏感导致的探测 Panic 问题
	os.Unsetenv("XDG_RUNTIME_DIR")

	// 自动识别 Docker Host
	if os.Getenv("DOCKER_HOST") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); err == nil {
			os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
		} else if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
			os.Setenv("DOCKER_HOST", "unix:///run/podman/podman.sock")
		}
	}

	// 获取项目根目录
	_, filename, _, _ := runtime.Caller(0)
	s.projectRoot = filepath.Join(filepath.Dir(filename), "../..")

	// 创建临时目录
	s.tempDir = t.TempDir()

	// 创建测试环境
	s.createUpgradeTestEnvironment(t)
}

// TearDownSuite 清理测试套件
func (s *UpgradeE2ETestSuite) TearDownSuite(t *testing.T) {
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(t, err, "无法清理测试容器")
	}
}

// createUpgradeTestEnvironment 创建升级测试环境
func (s *UpgradeE2ETestSuite) createUpgradeTestEnvironment(t *testing.T) {
	// 创建测试数据库目录
	dbDir := filepath.Join(s.tempDir, "db")
	err := os.MkdirAll(dbDir, 0o755)
	require.NoError(t, err)

	// 创建测试迁移目录
	migrationDir := filepath.Join(s.tempDir, "migrations")
	err = os.MkdirAll(migrationDir, 0o755)
	require.NoError(t, err)

	// 创建新版本迁移文件
	s.createNewVersionMigrations(t, migrationDir)

	// 创建旧版本数据库
	s.createOldVersionDatabase(t, dbDir)
}

// createNewVersionMigrations 创建新版本迁移文件
func (s *UpgradeE2ETestSuite) createNewVersionMigrations(t *testing.T, migrationDir string) {
	migrations := []struct {
		version int
		upSQL   string
		downSQL string
	}{
		{
			version: 1,
			upSQL:   "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);",
			downSQL: "DROP TABLE IF EXISTS users;",
		},
		{
			version: 2,
			upSQL:   "CREATE TABLE inbounds (id INTEGER PRIMARY KEY, port INTEGER, protocol TEXT, settings TEXT);",
			downSQL: "DROP TABLE IF EXISTS inbounds;",
		},
		{
			version: 3,
			upSQL:   "ALTER TABLE users ADD COLUMN status INTEGER DEFAULT 1;",
			downSQL: "CREATE TABLE users_temp (id INTEGER PRIMARY KEY, name TEXT, email TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP); INSERT INTO users_temp SELECT id, name, email, created_at FROM users; DROP TABLE users; ALTER TABLE users_temp RENAME TO users;",
		},
		{
			version: 4,
			upSQL:   "CREATE TABLE settings (id INTEGER PRIMARY KEY, key TEXT, value TEXT);",
			downSQL: "DROP TABLE IF EXISTS settings;",
		},
		{
			version: 5,
			upSQL:   "CREATE TABLE client_traffics (id INTEGER PRIMARY KEY, user_id INTEGER, upload INTEGER, download INTEGER);",
			downSQL: "DROP TABLE IF EXISTS client_traffics;",
		},
	}

	for _, mig := range migrations {
		upFile := filepath.Join(migrationDir, fmt.Sprintf("%03d_test.up.sql", mig.version))
		downFile := filepath.Join(migrationDir, fmt.Sprintf("%03d_test.down.sql", mig.version))

		err := os.WriteFile(upFile, []byte(mig.upSQL), 0o644)
		require.NoError(t, err)

		err = os.WriteFile(downFile, []byte(mig.downSQL), 0o644)
		require.NoError(t, err)
	}
}

// createOldVersionDatabase 创建旧版本数据库
func (s *UpgradeE2ETestSuite) createOldVersionDatabase(t *testing.T, dbDir string) {
	// 创建一个模拟的旧版本数据库
	oldDBPath := filepath.Join(dbDir, "x-ui.db")

	// 创建旧版本数据库结构（版本1）
	sqlContent := `
PRAGMA foreign_keys=OFF;
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT);
CREATE TABLE inbounds (id INTEGER PRIMARY KEY, port INTEGER, protocol TEXT);
INSERT INTO users VALUES (1, 'admin', 'admin@example.com');
INSERT INTO users VALUES (2, 'user1', 'user1@example.com');
INSERT INTO inbounds VALUES (1, 8080, 'vmess');
INSERT INTO inbounds VALUES (2, 8081, 'vless');
PRAGMA foreign_keys=ON;
`

	err := os.WriteFile(oldDBPath, []byte(sqlContent), 0o644)
	require.NoError(t, err)
}

// execCommand 在容器中执行命令
func (s *UpgradeE2ETestSuite) execCommand(cmd []string) (int, string, error) {
	if s.container == nil {
		return 1, "", fmt.Errorf("容器未初始化")
	}

	exitCode, reader, err := s.container.Exec(s.ctx, cmd)
	if err != nil {
		return exitCode, "", err
	}

	// 读取输出
	data := make([]byte, 1024)
	n, err := reader.Read(data)
	if err != nil && n == 0 {
		return exitCode, "", err
	}

	return exitCode, string(data[:n]), nil
}

// TestCompleteUpgradeE2E 完整升级端到端测试
func TestCompleteUpgradeE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode.")
	}

	suite := &UpgradeE2ETestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	t.Log("=== 完整升级端到端测试 ===")

	// 启动容器进行测试
	imageName := os.Getenv("E2E_IMAGE")
	fromDockerfile := testcontainers.FromDockerfile{}
	if imageName == "" {
		fromDockerfile = testcontainers.FromDockerfile{
			Context:    suite.projectRoot,
			Dockerfile: "tests/e2e/docker/Dockerfile.ubuntu22",
			KeepImage:  false,
		}
	}

	req := testcontainers.ContainerRequest{
		Image:          imageName,
		FromDockerfile: fromDockerfile,
		Privileged:     true,
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(suite.tempDir+"/db/x-ui.db", "/etc/x-ui/x-ui.db"),
			testcontainers.BindMount(suite.tempDir+"/migrations", "/home/x-ui/database/migrations"),
		),
		WaitingFor: wait.ForLog("x-ui started"),
	}

	container, err := testcontainers.GenericContainer(suite.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "无法启动容器")
	suite.container = container

	// 等待容器完全启动
	time.Sleep(5 * time.Second)

	// 1. 检查升级前的数据库状态
	t.Log("1. 检查升级前的数据库状态...")
	exitCode, output, err := suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", ".schema"})
	require.NoError(t, err)
	t.Logf("升级前数据库结构: %s", output)
	assert.Contains(t, output, "users", "应该包含 users 表")
	assert.Contains(t, output, "inbounds", "应该包含 inbounds 表")

	// 2. 检查升级前数据
	t.Log("2. 检查升级前数据...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", "SELECT COUNT(*) FROM users;"})
	require.NoError(t, err)
	t.Logf("升级前用户数量: %s", output)
	assert.Contains(t, output, "2", "应该有2个用户")

	// 3. 执行升级迁移
	t.Log("3. 执行升级迁移...")
	start := time.Now()
	exitCode, output, err = suite.execCommand([]string{"x-ui", "migrate", "up"})
	duration := time.Since(start)
	require.NoError(t, err)
	t.Logf("升级迁移输出: %s", output)
	assert.Equal(t, 0, exitCode, "升级迁移应该成功")
	t.Logf("升级耗时: %v", duration)

	// 4. 验证升级后的数据库结构
	t.Log("4. 验证升级后的数据库结构...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", ".schema"})
	require.NoError(t, err)
	t.Logf("升级后数据库结构: %s", output)
	assert.Contains(t, output, "users", "应该保留 users 表")
	assert.Contains(t, output, "inbounds", "应该保留 inbounds 表")
	assert.Contains(t, output, "settings", "应该新增 settings 表")
	assert.Contains(t, output, "client_traffics", "应该新增 client_traffics 表")

	// 5. 验证数据完整性
	t.Log("5. 验证数据完整性...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", "SELECT COUNT(*) FROM users;"})
	require.NoError(t, err)
	t.Logf("升级后用户数量: %s", output)
	assert.Contains(t, output, "2", "应该保留原有用户数据")

	// 6. 验证新字段
	t.Log("6. 验证新字段...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", "PRAGMA table_info(users);"})
	require.NoError(t, err)
	t.Logf("用户表结构: %s", output)
	assert.Contains(t, output, "status", "应该新增 status 字段")
	assert.Contains(t, output, "created_at", "应该新增 created_at 字段")

	// 7. 验证迁移版本
	t.Log("7. 验证迁移版本...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", "SELECT MAX(version) FROM schema_migrations;"})
	require.NoError(t, err)
	t.Logf("迁移版本: %s", output)
	assert.Contains(t, output, "5", "应该升级到版本5")

	// 8. 验证应用功能
	t.Log("8. 验证应用功能...")
	exitCode, output, err = suite.execCommand([]string{"x-ui", "status"})
	require.NoError(t, err)
	t.Logf("应用状态: %s", output)
	assert.Contains(t, output, "running", "应用应该正常运行")

	t.Log("✅ 完整升级端到端测试成功")
}

// TestUpgradeRollbackE2E 升级回滚端到端测试
func TestUpgradeRollbackE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode.")
	}

	suite := &UpgradeE2ETestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	t.Log("=== 升级回滚端到端测试 ===")

	// 启动容器进行测试
	imageName := os.Getenv("E2E_IMAGE")
	fromDockerfile := testcontainers.FromDockerfile{}
	if imageName == "" {
		fromDockerfile = testcontainers.FromDockerfile{
			Context:    suite.projectRoot,
			Dockerfile: "tests/e2e/docker/Dockerfile.ubuntu22",
			KeepImage:  false,
		}
	}

	req := testcontainers.ContainerRequest{
		Image:          imageName,
		FromDockerfile: fromDockerfile,
		Privileged:     true,
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(suite.tempDir+"/db/x-ui.db", "/etc/x-ui/x-ui.db"),
			testcontainers.BindMount(suite.tempDir+"/migrations", "/home/x-ui/database/migrations"),
		),
		WaitingFor: wait.ForLog("x-ui started"),
	}

	container, err := testcontainers.GenericContainer(suite.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "无法启动容器")
	suite.container = container

	// 等待容器完全启动
	time.Sleep(5 * time.Second)

	// 1. 先执行升级
	t.Log("1. 执行升级...")
	exitCode, output, err := suite.execCommand([]string{"x-ui", "migrate", "up"})
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode, "升级应该成功")
	t.Logf("升级输出: %s", output)

	// 2. 验证升级后的状态
	t.Log("2. 验证升级后的状态...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", ".tables"})
	require.NoError(t, err)
	t.Logf("升级后表列表: %s", output)
	assert.Contains(t, output, "settings", "应该包含新表")

	// 3. 执行回滚
	t.Log("3. 执行回滚...")
	exitCode, output, err = suite.execCommand([]string{"x-ui", "migrate", "down"})
	require.NoError(t, err)
	t.Logf("回滚输出: %s", output)

	// 4. 验证回滚后的状态
	t.Log("4. 验证回滚后的状态...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", ".tables"})
	require.NoError(t, err)
	t.Logf("回滚后表列表: %s", output)
	assert.Contains(t, output, "users", "应该保留原有表")
	assert.Contains(t, output, "inbounds", "应该保留原有表")
	assert.NotContains(t, output, "settings", "新表应该被删除")

	// 5. 验证数据完整性
	t.Log("5. 验证数据完整性...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", "SELECT COUNT(*) FROM users;"})
	require.NoError(t, err)
	t.Logf("回滚后用户数量: %s", output)
	assert.Contains(t, output, "2", "应该保留原有数据")

	t.Log("✅ 升级回滚端到端测试成功")
}

// TestUpgradeCompatibilityE2E 升级兼容性端到端测试
func TestUpgradeCompatibilityE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode.")
	}

	suite := &UpgradeE2ETestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	t.Log("=== 升级兼容性端到端测试 ===")

	// 启动容器进行测试
	imageName := os.Getenv("E2E_IMAGE")
	fromDockerfile := testcontainers.FromDockerfile{}
	if imageName == "" {
		fromDockerfile = testcontainers.FromDockerfile{
			Context:    suite.projectRoot,
			Dockerfile: "tests/e2e/docker/Dockerfile.ubuntu22",
			KeepImage:  false,
		}
	}

	req := testcontainers.ContainerRequest{
		Image:          imageName,
		FromDockerfile: fromDockerfile,
		Privileged:     true,
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(suite.tempDir+"/db/x-ui.db", "/etc/x-ui/x-ui.db"),
			testcontainers.BindMount(suite.tempDir+"/migrations", "/home/x-ui/database/migrations"),
		),
		WaitingFor: wait.ForLog("x-ui started"),
	}

	container, err := testcontainers.GenericContainer(suite.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "无法启动容器")
	suite.container = container

	// 等待容器完全启动
	time.Sleep(5 * time.Second)

	// 1. 测试多次升级
	t.Log("1. 测试多次升级...")
	for i := 0; i < 3; i++ {
		t.Logf("执行第 %d 次升级...", i+1)
		_, output, err := suite.execCommand([]string{"x-ui", "migrate", "up"})
		require.NoError(t, err)
		t.Logf("第 %d 次升级输出: %s", i+1, output)
		// 多次升级应该不会出错
	}

	// 2. 测试部分回滚
	t.Log("2. 测试部分回滚...")
	_, output, err := suite.execCommand([]string{"x-ui", "migrate", "down"})
	require.NoError(t, err)
	t.Logf("部分回滚输出: %s", output)

	// 3. 测试重新升级
	t.Log("3. 测试重新升级...")
	_, output, err = suite.execCommand([]string{"x-ui", "migrate", "up"})
	require.NoError(t, err)
	t.Logf("重新升级输出: %s", output)

	// 4. 验证最终状态
	t.Log("4. 验证最终状态...")
	_, output, err = suite.execCommand([]string{"x-ui", "migrate", "status"})
	require.NoError(t, err)
	t.Logf("最终状态: %s", output)

	// 5. 验证应用功能
	t.Log("5. 验证应用功能...")
	_, output, err = suite.execCommand([]string{"x-ui", "version"})
	require.NoError(t, err)
	t.Logf("应用版本: %s", output)

	t.Log("✅ 升级兼容性端到端测试成功")
}
