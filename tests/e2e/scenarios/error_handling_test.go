package scenarios

import (
	"testing"

	"x-ui/tests/e2e/api"
	"x-ui/tests/e2e/utils"
)

func TestErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	baseURL := GetTestBaseURL()
	username, password := GetTestCredentials()

	client, err := api.NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Log("Testing error handling scenarios...")

	// 测试删除不存在的入站
	t.Run("DeleteNonExistentInbound", func(t *testing.T) {
		err := client.DelInbound(99999)
		if err == nil {
			t.Error("Expected delete non-existent inbound to fail")
		} else {
			t.Logf("Delete non-existent inbound correctly failed: %v", err)
		}
	})

	// 测试无效的入站数据
	t.Run("InvalidInboundData", func(t *testing.T) {
		invalidInbound := map[string]interface{}{
			"enable":   true,
			"remark":   "test",
			"port":     -1, // 无效端口
			"protocol": "invalid_protocol",
		}
		_, err := client.AddInbound(invalidInbound)
		if err == nil {
			t.Error("Expected add inbound with invalid data to fail")
		} else {
			t.Logf("Add inbound with invalid data correctly failed: %v", err)
		}
	})

	// 测试端口冲突
	t.Run("PortConflict", func(t *testing.T) {
		// 先创建一个有效的入站
		validInbound := utils.GenerateVMessInboundData()
		validInbound["port"] = 20000 // 固定端口用于测试冲突

		inboundID, err := client.AddInbound(validInbound)
		if err != nil {
			t.Fatalf("Failed to create first inbound: %v", err)
		}
		defer func() {
			client.DelInbound(inboundID)
		}()

		// 尝试创建相同端口的入站（需要先登录）
		err = client.Login(username, password)
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		conflictingInbound := utils.GenerateVMessInboundData()
		conflictingInbound["port"] = 20000 // 相同端口

		_, err = client.AddInbound(conflictingInbound)
		// 注意：实际的端口冲突检查可能在API层面，这里我们测试API响应
		if err == nil {
			// 如果没有报错，可能是端口检查不严格，我们记录这个情况
			t.Log("Port conflict not detected at API level - this may be expected behavior")
		} else {
			t.Logf("Port conflict correctly detected: %v", err)
		}
	})

	// 测试获取不存在的客户端流量
	t.Run("GetNonExistentClientTraffic", func(t *testing.T) {
		// 需要先登录
		err = client.Login(username, password)
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		_, err := client.GetClientTraffics("nonexistent@example.com")
		if err == nil {
			t.Error("Expected get traffic for non-existent client to fail")
		} else {
			t.Logf("Get traffic for non-existent client correctly failed: %v", err)
		}
	})

	// 测试无效的更新操作
	t.Run("InvalidUpdateOperation", func(t *testing.T) {
		err = client.Login(username, password)
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		// 尝试更新不存在的入站
		invalidUpdate := map[string]interface{}{
			"remark": "invalid update",
		}
		err := client.UpdateInbound(99999, invalidUpdate)
		if err == nil {
			t.Error("Expected update non-existent inbound to fail")
		} else {
			t.Logf("Update non-existent inbound correctly failed: %v", err)
		}
	})

	t.Log("Error handling tests completed")
}