package scenarios

import (
	"testing"

	"x-ui/tests/e2e/api"
	"x-ui/tests/e2e/utils"
)

func TestInboundCRUD(t *testing.T) {
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

	// 获取初始入站数量
	initialInbounds, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Get inbounds failed: %v", err)
	}
	initialCount := len(initialInbounds)
	t.Logf("Initial inbounds count: %d", initialCount)

	// 测试添加入站
	t.Run("AddInbound", func(t *testing.T) {
		inboundData := utils.GenerateVMessInboundData()

		inboundID, err := client.AddInbound(inboundData)
		if err != nil {
			t.Fatalf("Add inbound failed: %v", err)
		}
		t.Logf("Inbound added successfully, ID: %d", inboundID)

		// 验证入站已添加
		inboundsAfterAdd, err := client.GetInbounds()
		if err != nil {
			t.Fatalf("Get inbounds after add failed: %v", err)
		}
		if len(inboundsAfterAdd) != initialCount+1 {
			t.Fatalf("Expected %d inbounds, got %d", initialCount+1, len(inboundsAfterAdd))
		}

		// 测试更新入站
		t.Run("UpdateInbound", func(t *testing.T) {
			inboundData["remark"] = utils.GenerateRandomRemark("updated-vmess")

			err := client.UpdateInbound(inboundID, inboundData)
			if err != nil {
				t.Fatalf("Update inbound failed: %v", err)
			}
			t.Log("Inbound updated successfully")
		})

		// 测试添加客户端
		t.Run("AddClient", func(t *testing.T) {
			clientData := utils.GenerateClientData(inboundID)

			clientID, err := client.AddInboundClient(clientData)
			if err != nil {
				t.Fatalf("Add inbound client failed: %v", err)
			}
			t.Logf("Client added successfully, ID: %d", clientID)

			// 测试获取客户端流量
			email := "test-000001@example.com" // 从 GenerateClientData 生成的
			traffics, err := client.GetClientTraffics(email)
			if err != nil {
				t.Fatalf("Get client traffics failed: %v", err)
			}
			t.Logf("Client traffics retrieved: %v", traffics)

			// 测试重置客户端流量
			err = client.ResetClientTraffic(inboundID, email)
			if err != nil {
				t.Fatalf("Reset client traffic failed: %v", err)
			}
			t.Log("Client traffic reset successfully")
		})

		// 清理：删除入站
		t.Run("DeleteInbound", func(t *testing.T) {
			err := client.DelInbound(inboundID)
			if err != nil {
				t.Fatalf("Delete inbound failed: %v", err)
			}
			t.Log("Inbound deleted successfully")

			// 验证入站已删除
			finalInbounds, err := client.GetInbounds()
			if err != nil {
				t.Fatalf("Get inbounds after delete failed: %v", err)
			}
			if len(finalInbounds) != initialCount {
				t.Fatalf("Expected %d inbounds after cleanup, got %d", initialCount, len(finalInbounds))
			}
		})
	})
}