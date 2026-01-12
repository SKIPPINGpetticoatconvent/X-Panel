package service

import (
	"context"
	"os"
	"testing"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/xray"

	"github.com/xtls/xray-core/app/proxyman/command"
	"google.golang.org/grpc"
)

func setupTestDB(t *testing.T) {
	// Use a temporary file for the database
	f, err := os.CreateTemp("", "xui_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbPath := f.Name()
	f.Close()

	err = database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	// Clean up after test
	t.Cleanup(func() {
		database.CloseDB()
		os.Remove(dbPath)
	})
}

func setupMockXrayAPI() xray.API {
	// Mock HandlerServiceClient
	mockHandler := &MockHandlerServiceClient{
		AddInboundFunc: func(ctx context.Context, in *command.AddInboundRequest, opts ...grpc.CallOption) (*command.AddInboundResponse, error) {
			return &command.AddInboundResponse{}, nil
		},
		RemoveInboundFunc: func(ctx context.Context, in *command.RemoveInboundRequest, opts ...grpc.CallOption) (*command.RemoveInboundResponse, error) {
			return &command.RemoveInboundResponse{}, nil
		},
		AlterInboundFunc: func(ctx context.Context, in *command.AlterInboundRequest, opts ...grpc.CallOption) (*command.AlterInboundResponse, error) {
			return &command.AlterInboundResponse{}, nil
		},
	}

	var client command.HandlerServiceClient = mockHandler

	api := xray.XrayAPI{}
	api.HandlerServiceClient = &client

	// Wrap it!
	wrapper := &MockXrayWrapper{
		XrayAPI: &api,
	}

	return wrapper
}

// Mock the global 'p' variable in service package
func setupMockProcess(t *testing.T) {
	originalP := p
	t.Cleanup(func() { p = originalP })

	// Create a dummy process that returns 0 for API port to avoid network calls in Init(),
	// but is not nil to avoid panic.
	// Note: Init(0) will fail, preserving our manually set HandlerServiceClient.
	p = xray.NewProcess(&xray.Config{})
}

func TestInboundService_AddInbound(t *testing.T) {
	setupTestDB(t)
	setupMockProcess(t)

	s := &InboundService{}
	s.SetXrayAPI(setupMockXrayAPI())

	// Test Case 1: Add a valid inbound
	inbound := &model.Inbound{
		UserId:         1,
		Up:             0,
		Down:           0,
		Total:          0,
		Remark:         "Test Inbound",
		Enable:         true,
		ExpiryTime:     0,
		Listen:         "",
		Port:           10086,
		Protocol:       "vmess",
		Settings:       `{"clients": [{"id": "uuid", "alterId": 0}]}`,
		StreamSettings: `{"network": "tcp"}`,
		Tag:            "inbound-10086",
		Sniffing:       `{"enabled": true, "destOverride": ["http", "tls"]}`,
	}

	added, needRestart, err := s.AddInbound(inbound)
	if err != nil {
		t.Fatalf("AddInbound failed: %v", err)
	}
	if added.Id == 0 {
		t.Error("Expected Inbound ID to be set")
	}
	if needRestart {
		// needRestart depends on implementation, usually false if API call succeeds
		// API call succeeds because of mock.
	}

	// Verify DB persistence
	dbInbound, err := s.GetInbound(added.Id)
	if err != nil {
		t.Fatalf("Failed to retrieve added inbound: %v", err)
	}
	if dbInbound.Port != 10086 {
		t.Errorf("Expected port 10086, got %d", dbInbound.Port)
	}

	// Test Case 2: Duplicate Port
	_, _, err = s.AddInbound(inbound) // Same port
	if err == nil {
		t.Error("Expected error for duplicate port, got nil")
	}
}

func TestInboundService_DelInbound(t *testing.T) {
	setupTestDB(t)
	setupMockProcess(t)

	s := &InboundService{}
	s.SetXrayAPI(setupMockXrayAPI())

	// Add an inbound first
	inbound := &model.Inbound{
		Port:     10087,
		Protocol: "vless",
		Settings: `{"clients": []}`,
		Tag:      "inbound-10087",
		Enable:   true,
	}
	added, _, _ := s.AddInbound(inbound)

	// Delete it
	needRestart, err := s.DelInbound(added.Id)
	if err != nil {
		t.Fatalf("DelInbound failed: %v", err)
	}
	_ = needRestart

	// Verify deletion
	_, err = s.GetInbound(added.Id)
	if err == nil {
		t.Error("Expected error retrieving deleted inbound, got nil")
	}
}

func TestInboundService_UpdateInbound(t *testing.T) {
	setupTestDB(t)
	setupMockProcess(t)

	s := &InboundService{}
	s.SetXrayAPI(setupMockXrayAPI())

	inbound := &model.Inbound{
		Port:     10088,
		Protocol: "trojan",
		Settings: `{"clients": []}`,
		Tag:      "inbound-10088",
		Enable:   true,
		Remark:   "Original",
	}
	added, _, _ := s.AddInbound(inbound)

	// Update
	added.Remark = "Updated"
	updated, _, err := s.UpdateInbound(added)
	if err != nil {
		t.Fatalf("UpdateInbound failed: %v", err)
	}

	if updated.Remark != "Updated" {
		t.Errorf("Expected remark 'Updated', got '%s'", updated.Remark)
	}

	// Check DB
	dbInbound, _ := s.GetInbound(added.Id)
	if dbInbound.Remark != "Updated" {
		t.Errorf("DB remark not updated")
	}
}
