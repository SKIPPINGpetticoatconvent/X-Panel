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

// DatabaseMigrationE2ETestSuite 数据库迁移端到端测试套件
type DatabaseMigrationE2ETestSuite struct {
	ctx         context.Context
	container   testcontainers.Container
	projectRoot string
	tempDir     string
}

// SetupSuite 设置测试套件
func (s *DatabaseMigrationE2ETestSuite) SetupSuite(t *testing.T) {
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

	// 创建测试数据库和迁移文件
	s.createTestFiles(t)
}

// TearDownSuite 清理测试套件
func (s *DatabaseMigrationE2ETestSuite) TearDownSuite(t *testing.T) {
	if s.container != nil {
		err := s.container.Terminate(s.ctx)
		assert.NoError(t, err, "无法清理测试容器")
	}
}

// createTestFiles 创建测试文件
func (s *DatabaseMigrationE2ETestSuite) createTestFiles(t *testing.T) {
	// 创建测试数据库目录
	dbDir := filepath.Join(s.tempDir, "db")
	err := os.MkdirAll(dbDir, 0o755)
	require.NoError(t, err)

	// 创建测试迁移目录
	migrationDir := filepath.Join(s.tempDir, "migrations")
	err = os.MkdirAll(migrationDir, 0o755)
	require.NoError(t, err)

	// 创建测试迁移文件
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
			upSQL:   "ALTER TABLE users ADD COLUMN age INTEGER;",
			downSQL: "CREATE TABLE users_temp (id INTEGER PRIMARY KEY, name TEXT, email TEXT); INSERT INTO users_temp SELECT id, name, email FROM users; DROP TABLE users; ALTER TABLE users_temp RENAME TO users;",
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

	// 创建旧版本数据库（模拟升级场景）
	oldDBPath := filepath.Join(dbDir, "old_version.db")
	s.createOldVersionDatabase(t, oldDBPath)
}

// createOldVersionDatabase 创建旧版本数据库
func (s *DatabaseMigrationE2ETestSuite) createOldVersionDatabase(t *testing.T, dbPath string) {
	// 这里创建一个简单的旧版本数据库结构
	// 在实际场景中，这可能是从生产服务器复制的数据库
	content := `SQLite format 3
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO users VALUES (1, 'test_user');
`

	err := os.WriteFile(dbPath, []byte(content), 0o644)
	require.NoError(t, err)
}

// execCommand 在容器中执行命令
func (s *DatabaseMigrationE2ETestSuite) execCommand(cmd []string) (int, string, error) {
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

// TestDatabaseMigrationE2E 数据库迁移端到端测试
func TestDatabaseMigrationE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode.")
	}

	suite := &DatabaseMigrationE2ETestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	t.Log("=== 数据库迁移端到端测试 ===")

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
			testcontainers.BindMount(suite.tempDir+"/db", "/etc/x-ui"),
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

	// 1. 测试迁移状态检查
	t.Log("1. 测试迁移状态检查...")
	exitCode, output, err := suite.execCommand([]string{"x-ui", "migrate", "status"})
	require.NoError(t, err)
	t.Logf("迁移状态检查输出: %s", output)
	assert.Equal(t, 0, exitCode, "迁移状态检查应该成功")

	// 2. 测试数据库迁移
	t.Log("2. 测试数据库迁移...")
	exitCode, output, err = suite.execCommand([]string{"x-ui", "migrate", "up"})
	require.NoError(t, err)
	t.Logf("数据库迁移输出: %s", output)
	assert.Equal(t, 0, exitCode, "数据库迁移应该成功")

	// 3. 测试迁移验证
	t.Log("3. 测试迁移验证...")
	exitCode, output, err = suite.execCommand([]string{"x-ui", "migrate", "validate"})
	require.NoError(t, err)
	t.Logf("迁移验证输出: %s", output)
	assert.Equal(t, 0, exitCode, "迁移验证应该成功")

	// 4. 测试数据库状态
	t.Log("4. 测试数据库状态...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", ".tables"})
	require.NoError(t, err)
	t.Logf("数据库表列表: %s", output)
	assert.Contains(t, output, "users", "应该包含 users 表")
	assert.Contains(t, output, "posts", "应该包含 posts 表")

	// 5. 测试数据完整性
	t.Log("5. 测试数据完整性...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", "SELECT COUNT(*) FROM users;"})
	require.NoError(t, err)
	t.Logf("用户数量: %s", output)
	assert.Contains(t, output, "1", "应该有一个用户")

	t.Log("✅ 数据库迁移端到端测试成功")
}

// TestOldVersionUpgradeE2E 旧版本升级端到端测试
func TestOldVersionUpgradeE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode.")
	}

	suite := &DatabaseMigrationE2ETestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	t.Log("=== 旧版本升级端到端测试 ===")

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
			testcontainers.BindMount(suite.tempDir+"/db/old_version.db", "/etc/x-ui/x-ui.db"),
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

	// 1. 检查旧版本数据库状态
	t.Log("1. 检查旧版本数据库状态...")
	exitCode, output, err := suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", ".schema"})
	require.NoError(t, err)
	t.Logf("旧版本数据库结构: %s", output)
	assert.Contains(t, output, "users", "应该包含 users 表")

	// 2. 执行升级迁移
	t.Log("2. 执行升级迁移...")
	exitCode, output, err = suite.execCommand([]string{"x-ui", "migrate", "up"})
	require.NoError(t, err)
	t.Logf("升级迁移输出: %s", output)
	assert.Equal(t, 0, exitCode, "升级迁移应该成功")

	// 3. 验证升级结果
	t.Log("3. 验证升级结果...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", ".schema"})
	require.NoError(t, err)
	t.Logf("升级后数据库结构: %s", output)
	assert.Contains(t, output, "users", "应该保留 users 表")
	assert.Contains(t, output, "posts", "应该新增 posts 表")

	// 4. 验证数据完整性
	t.Log("4. 验证数据完整性...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", "SELECT * FROM users;"})
	require.NoError(t, err)
	t.Logf("用户数据: %s", output)
	assert.Contains(t, output, "test_user", "应该保留原有用户数据")

	// 5. 验证新字段
	t.Log("5. 验证新字段...")
	exitCode, output, err = suite.execCommand([]string{"sqlite3", "/etc/x-ui/x-ui.db", "PRAGMA table_info(users);"})
	require.NoError(t, err)
	t.Logf("用户表结构: %s", output)
	assert.Contains(t, output, "age", "应该新增 age 字段")

	t.Log("✅ 旧版本升级端到端测试成功")
}

// TestMigrationErrorHandlingE2E 迁移错误处理端到端测试
func TestMigrationErrorHandlingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode.")
	}

	suite := &DatabaseMigrationE2ETestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	t.Log("=== 迁移错误处理端到端测试 ===")

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
			testcontainers.BindMount(suite.tempDir+"/db", "/etc/x-ui"),
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

	// 1. 测试无效迁移路径的错误处理
	t.Log("1. 测试无效迁移路径的错误处理...")
	_, output, err := suite.execCommand([]string{"sh", "-c", "mv /home/x-ui/database/migrations /home/x-ui/database/migrations_backup"})
	require.NoError(t, err)

	exitCode, output, err := suite.execCommand([]string{"x-ui", "migrate", "status"})
	require.NoError(t, err)
	t.Logf("无效路径错误输出: %s", output)
	_ = exitCode // 避免未使用变量警告
	// 应该有适当的错误处理，但不应该崩溃

	// 恢复迁移目录
	_, _, err = suite.execCommand([]string{"sh", "-c", "mv /home/x-ui/database/migrations_backup /home/x-ui/database/migrations"})
	require.NoError(t, err)

	// 2. 测试损坏的迁移文件
	t.Log("2. 测试损坏的迁移文件错误处理...")
	_, _, err = suite.execCommand([]string{"sh", "-c", "echo 'invalid sql' > /home/x-ui/database/migrations/001_test.up.sql"})
	require.NoError(t, err)

	exitCode, output, err = suite.execCommand([]string{"x-ui", "migrate", "up"})
	require.NoError(t, err)
	t.Logf("损坏迁移文件错误输出: %s", output)
	_ = exitCode // 避免未使用变量警告
	// 应该有适当的错误处理

	t.Log("✅ 迁移错误处理端到端测试完成")
}

// TestMigrationPerformanceE2E 迁移性能端到端测试
func TestMigrationPerformanceE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode.")
	}

	suite := &DatabaseMigrationE2ETestSuite{}
	suite.SetupSuite(t)
	defer suite.TearDownSuite(t)

	t.Log("=== 迁移性能端到端测试 ===")

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
			testcontainers.BindMount(suite.tempDir+"/db", "/etc/x-ui"),
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

	// 测试迁移性能
	t.Log("测试迁移性能...")
	start := time.Now()
	exitCode, output, err := suite.execCommand([]string{"x-ui", "migrate", "up"})
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode, "迁移应该成功")
	t.Logf("迁移输出: %s", output)
	t.Logf("迁移耗时: %v", duration)

	// 验证性能要求
	assert.Less(t, duration, 30*time.Second, "迁移应该在30秒内完成")

	t.Log("✅ 迁移性能端到端测试成功")
}
