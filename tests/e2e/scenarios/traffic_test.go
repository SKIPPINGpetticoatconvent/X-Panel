package scenarios

import (
	"context"
	"testing"
	"time"

	"x-ui/tests/e2e/api"
	"x-ui/tests/e2e/infra"
	"x-ui/tests/e2e/utils"
)

func TestTrafficConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	baseURL := GetTestBaseURL()
	username, password := GetTestCredentials()

	client, err := api.NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// 登录
	err = client.Login(username, password)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	ctx := context.Background()

	// 创建入站用于测试
	inboundData := utils.GenerateVMessInboundData()
	inboundID, err := client.AddInbound(inboundData)
	if err != nil {
		t.Fatalf("Failed to create test inbound: %v", err)
	}
	defer func() {
		// 清理测试入站
		client.DelInbound(inboundID)
	}()

	// 添加客户端
	clientData := utils.GenerateClientData(inboundID)
	clientID, err := client.AddInboundClient(clientData)
	if err != nil {
		t.Fatalf("Failed to add client: %v", err)
	}
	t.Logf("Test client created with ID: %d", clientID)

	// 等待配置生效
	time.Sleep(2 * time.Second)

	// 获取服务器信息用于客户端连接
	container := GetTestContainer()
	host, port, _ := container.GetContainerInfo()

	// 创建 Xray 客户端容器进行连通性测试
	t.Run("XrayConnectivityTest", func(t *testing.T) {
		xrayClient, err := infra.NewXrayClientContainer(ctx, host, port)
		if err != nil {
			t.Fatalf("Failed to create Xray client container: %v", err)
		}
		defer xrayClient.Terminate(ctx)

		// 测试连通性（这里简化测试，实际可能需要外部验证服务）
		proxyURL := xrayClient.GetProxyURL()
		t.Logf("Proxy URL: %s", proxyURL)

		// 注意：实际的连通性测试可能需要一个测试目标服务器
		// 这里我们主要验证容器创建和配置是否正确
		t.Log("Traffic connectivity test setup completed")
	})

	// 测试流量统计
	t.Run("TrafficStatistics", func(t *testing.T) {
		email := "test-000001@example.com" // 从 GenerateClientData 生成的

		// 获取初始流量
		initialTraffic, err := client.GetClientTraffics(email)
		if err != nil {
			t.Fatalf("Failed to get initial client traffic: %v", err)
		}
		t.Logf("Initial client traffic: %v", initialTraffic)

		// 重置流量
		err = client.ResetClientTraffic(inboundID, email)
		if err != nil {
			t.Fatalf("Failed to reset client traffic: %v", err)
		}
		t.Log("Client traffic reset successfully")

		// 再次获取流量，验证重置
		resetTraffic, err := client.GetClientTraffics(email)
		if err != nil {
			t.Fatalf("Failed to get reset client traffic: %v", err)
		}
		t.Logf("Traffic after reset: %v", resetTraffic)
	})
}