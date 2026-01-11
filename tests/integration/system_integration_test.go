package integration

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSystemIntegration 测试系统集成，包括容器部署、服务通信和日志收集
func TestSystemIntegration(t *testing.T) {
	// 跳过集成测试，除非明确启用
	if os.Getenv("RUN_SYSTEM_INTEGRATION_TESTS") != "true" {
		t.Skip("跳过系统集成测试，使用 RUN_SYSTEM_INTEGRATION_TESTS=true 启用")
	}

	// 获取项目根目录
	projectRoot := getProjectRoot(t)

	// Docker Compose 文件路径
	composeFile := filepath.Join(projectRoot, "docker-compose.yml")

	// 测试配置
	testConfig := &SystemTestConfig{
		ComposeFile:    composeFile,
		ProjectRoot:    projectRoot,
		WebPort:        GetTestConfig("TEST_WEB_PORT"),
		SubPort:        GetTestConfig("TEST_SUB_PORT"),
		HealthCheckURL: fmt.Sprintf("http://localhost:%s", GetTestConfig("TEST_WEB_PORT")),
		LogFilePath:    filepath.Join(projectRoot, "test-logs", "x-ui.log"),
		StartupTimeout: 120 * time.Second,
		TestTimeout:    300 * time.Second,
	}

	// 运行集成测试
	runSystemIntegrationTest(t, testConfig)
}

// SystemTestConfig 系统测试配置
type SystemTestConfig struct {
	ComposeFile    string
	ProjectRoot    string
	WebPort        string
	SubPort        string
	HealthCheckURL string
	LogFilePath    string
	StartupTimeout time.Duration
	TestTimeout    time.Duration
}

// runSystemIntegrationTest 执行系统集成测试
func runSystemIntegrationTest(t *testing.T, config *SystemTestConfig) {
	// 创建日志目录
	logDir := filepath.Dir(config.LogFilePath)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("创建日志目录失败: %v", err)
	}

	// 创建容器管理器
	containerManager := NewContainerManager(config.ComposeFile)

	// 启动容器
	t.Log("启动 Docker 容器...")
	if err := containerManager.StartContainers(); err != nil {
		t.Fatalf("启动容器失败: %v", err)
	}
	defer func() {
		t.Log("停止 Docker 容器...")
		containerManager.StopContainers()
	}()

	// 等待服务启动
	t.Log("等待服务启动...")
	healthCheck := func() bool { return isServiceHealthy(config.HealthCheckURL) }
	if err := containerManager.WaitForContainers(config.StartupTimeout, healthCheck); err != nil {
		t.Fatalf("服务启动失败: %v", err)
	}

	// 测试服务通信
	t.Log("测试服务通信...")
	testServiceCommunication(t, config)

	// 测试日志收集
	t.Log("测试日志收集...")
	testLogCollection(t, config)
}

// isServiceHealthy 检查服务健康状态
func isServiceHealthy(url string) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url + "/login")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// testServiceCommunication 测试服务通信
func testServiceCommunication(t *testing.T, config *SystemTestConfig) {
	serviceTester := NewServiceTester()

	// 测试 Web API
	if err := serviceTester.TestWebAPI(t, config.HealthCheckURL); err != nil {
		t.Errorf("Web API 测试失败: %v", err)
	}

	// 测试 Sub 服务
	subURL := fmt.Sprintf("http://localhost:%s", config.SubPort)
	if err := serviceTester.TestSubService(t, subURL); err != nil {
		t.Errorf("Sub 服务测试失败: %v", err)
	}

	// 测试服务间通信
	if err := serviceTester.TestInterServiceCommunication(t, config.HealthCheckURL, subURL); err != nil {
		t.Errorf("服务间通信测试失败: %v", err)
	}
}

// testLogCollection 测试日志收集
func testLogCollection(t *testing.T, config *SystemTestConfig) {
	logTester := NewLogTester(config.LogFilePath)

	if err := logTester.TestLogCollection(t); err != nil {
		t.Errorf("日志收集测试失败: %v", err)
	}

	if err := logTester.TestLogRotation(t); err != nil {
		t.Errorf("日志轮转测试失败: %v", err)
	}
}

// getProjectRoot 获取项目根目录
func getProjectRoot(t *testing.T) string {
	// 从当前文件位置向上查找 go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前目录失败: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("未找到项目根目录 (go.mod)")
		}
		dir = parent
	}
}
