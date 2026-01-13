package service

import (
	"testing"

	"x-ui/database"
	"x-ui/database/model"
)

// Helper to set setting directly in DB
func setSetting(t *testing.T, key, value string) {
	db := database.GetDB()
	setting := &model.Setting{Key: key, Value: value}
	// Upsert
	var count int64
	db.Model(&model.Setting{}).Where("key = ?", key).Count(&count)
	if count > 0 {
		db.Model(&model.Setting{}).Where("key = ?", key).Update("value", value)
	} else {
		db.Create(setting)
	}
}

func TestBusinessFlow(t *testing.T) {
	// 1. Setup Environment using helpers from inbound_test.go
	setupTestDB(t)
	setupMockProcess(t)

	// Setup Services
	userService := UserService{}
	err := userService.UpdateFirstUser("admin", "admin")
	if err != nil {
		t.Fatalf("UpdateFirstUser failed: %v", err)
	}

	// Setup InboundService with Mock API (reusing helper)
	inboundService := InboundService{}
	inboundService.SetXrayAPI(setupMockXrayAPI())

	// Setup Settings directly in DB (simulating initial environment)
	setSetting(t, "webPort", "0")
	setSetting(t, "username", "admin")
	setSetting(t, "password", "admin")
	setSetting(t, "webBasePath", "/")

	// 2. Verify Login Logic (User Service)
	// CheckUser returns *model.User on success
	user := userService.CheckUser("admin", "admin", "")
	if user == nil {
		t.Fatal("CheckUser failed for valid credentials")
	}

	// 3. Inbound Management Flow

	// Add Inbound
	inbound := &model.Inbound{
		UserId:         user.Id,
		Remark:         "Test Inbound",
		Enable:         true,
		Port:           12345,
		Protocol:       "vmess",
		Settings:       `{"clients": [{"id": "uuid", "alterId": 0}]}`,
		StreamSettings: `{"network": "tcp"}`,
		Tag:            "test_tag",
		Sniffing:       `{"enabled": true, "destOverride": ["http", "tls"]}`,
	}

	// AddInbound returns (inbound, needRestart, error)
	inbound, _, err = inboundService.AddInbound(inbound)
	if err != nil {
		t.Fatalf("AddInbound failed: %v", err)
	}
	if inbound.Id == 0 {
		t.Fatal("Inbound ID should not be 0")
	}

	// List Inbounds
	inbounds, err := inboundService.GetAllInbounds()
	if err != nil {
		t.Fatalf("GetAllInbounds failed: %v", err)
	}
	found := false
	for _, i := range inbounds {
		if i.Port == 12345 && i.Remark == "Test Inbound" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Inbound not found in list")
	}

	// Update Inbound
	inbound.Remark = "Updated Inbound"
	// UpdateInbound returns (inbound, needRestart, error)
	inbound, _, err = inboundService.UpdateInbound(inbound)
	if err != nil {
		t.Fatalf("UpdateInbound failed: %v", err)
	}

	// Verify Update via Service
	check, err := inboundService.GetInbound(inbound.Id)
	if err != nil {
		t.Fatalf("GetInbound failed: %v", err)
	}
	if check.Remark != "Updated Inbound" {
		t.Fatalf("Update failed, remark is %s", check.Remark)
	}

	// Verify Update via DB directly (Double Check)
	dbInbound := &model.Inbound{}
	database.GetDB().First(dbInbound, inbound.Id)
	if dbInbound.Remark != "Updated Inbound" {
		t.Fatalf("DB integrity check failed, remark is %s", dbInbound.Remark)
	}

	// Delete Inbound
	// DelInbound returns (needRestart, error)
	_, err = inboundService.DelInbound(inbound.Id)
	if err != nil {
		t.Fatalf("DelInbound failed: %v", err)
	}

	// Verify Delete
	check, err = inboundService.GetInbound(inbound.Id)
	// GetInbound should return error record not found or nil
	if err == nil {
		t.Fatal("GetInbound should return error after delete")
	}
}
