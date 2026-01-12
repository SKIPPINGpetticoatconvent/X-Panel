package scenarios

import (
	"testing"
	"time"

	"x-ui/tests/e2e/api"
	"x-ui/tests/e2e/utils"
)

func TestBackupRestore(t *testing.T) {
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

	// 创建测试数据
	t.Log("Creating test data for backup...")
	testInbound := utils.GenerateVMessInboundData()
	inboundID, err := client.AddInbound(testInbound)
	if err != nil {
		t.Fatalf("Failed to create test inbound: %v", err)
	}
	t.Logf("Test inbound created with ID: %d", inboundID)

	// 验证入站创建成功
	inbounds, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Failed to get inbounds: %v", err)
	}

	found := false
	for _, inbound := range inbounds {
		if id, ok := inbound["id"].(float64); ok && int(id) == inboundID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Test inbound not found in inbounds list")
	}

	// 执行数据库备份
	t.Log("Performing database backup...")
	backupData, err := client.DownloadBackup()
	if err != nil {
		t.Fatalf("Failed to download database backup: %v", err)
	}

	if len(backupData) == 0 {
		t.Fatalf("Backup data is empty")
	}
	t.Logf("Database backup successful, size: %d bytes", len(backupData))

	// 模拟数据丢失（删除测试入站）
	t.Log("Simulating data loss by deleting test inbound...")
	err = client.DelInbound(inboundID)
	if err != nil {
		t.Fatalf("Failed to delete test inbound: %v", err)
	}

	// 验证入站已删除
	inboundsAfterDelete, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Failed to get inbounds after delete: %v", err)
	}

	deleted := true
	for _, inbound := range inboundsAfterDelete {
		if id, ok := inbound["id"].(float64); ok && int(id) == inboundID {
			deleted = false
			break
		}
	}
	if !deleted {
		t.Fatalf("Test inbound was not deleted")
	}
	t.Log("Test inbound successfully deleted")

	// 执行数据库恢复
	t.Log("Performing database restore...")
	err = client.RestoreBackup(backupData)
	if err != nil {
		t.Fatalf("Database restore failed: %v", err)
	}
	t.Log("Database restore successful")

	// 等待服务重启
	t.Log("Waiting for service to restart after restore...")
	time.Sleep(3 * time.Second)

	// 重新登录（恢复后可能需要）
	client, err = api.NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to recreate client: %v", err)
	}
	err = client.Login(username, password)
	if err != nil {
		t.Fatalf("Login failed after restore: %v", err)
	}

	// 验证数据恢复
	t.Log("Verifying data restoration...")
	inboundsAfterRestore, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Failed to get inbounds after restore: %v", err)
	}

	restored := false
	for _, inbound := range inboundsAfterRestore {
		if id, ok := inbound["id"].(float64); ok && int(id) == inboundID {
			if remark, ok := inbound["remark"].(string); ok && remark == testInbound["remark"] {
				restored = true
				break
			}
		}
	}

	if !restored {
		t.Fatalf("Test inbound was not restored after database restore")
	}
	t.Log("Data restoration verified successfully")

	// 清理测试数据
	t.Log("Cleaning up test data...")
	err = client.DelInbound(inboundID)
	if err != nil {
		t.Logf("Warning: Failed to clean up test inbound: %v", err)
	} else {
		t.Log("Test data cleaned up successfully")
	}
}