package scenarios

import (
	"context"
	"os"
	"testing"

	"x-ui/tests/e2e/config"
	"x-ui/tests/e2e/infra"
)

var (
	testContainer *infra.XPanelContainer
	testClient    *infra.XPanelContainer // 可以添加客户端容器
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// 加载E2E配置
	e2eConfig, err := config.LoadE2EConfig()
	if err != nil {
		panic("Failed to load E2E config: " + err.Error())
	}

	// 设置测试环境
	if err := e2eConfig.SetupTestEnvironment(); err != nil {
		panic("Failed to setup test environment: " + err.Error())
	}

	// 创建X-Panel容器
	testContainer, err = infra.NewXPanelContainer(ctx)
	if err != nil {
		panic("Failed to create X-Panel container: " + err.Error())
	}

	// 等待容器就绪
	if err := testContainer.WaitForReady(ctx); err != nil {
		testContainer.Terminate(ctx)
		panic("Container failed to be ready: " + err.Error())
	}

	// 运行测试
	code := m.Run()

	// 清理资源
	if err := testContainer.Terminate(ctx); err != nil {
		panic("Failed to terminate container: " + err.Error())
	}

	// 清理测试环境
	if err := e2eConfig.CleanupTestEnvironment(); err != nil {
		panic("Failed to cleanup test environment: " + err.Error())
	}

	os.Exit(code)
}

// GetTestContainer 获取测试容器实例
func GetTestContainer() *infra.XPanelContainer {
	return testContainer
}

// GetTestBaseURL 获取测试基础URL
func GetTestBaseURL() string {
	if testContainer == nil {
		return ""
	}
	return testContainer.GetBaseURL()
}

// GetTestCredentials 获取测试凭据
func GetTestCredentials() (username, password string) {
	e2eConfig, _ := config.LoadE2EConfig()
	return e2eConfig.TestUsername, e2eConfig.TestPassword
}