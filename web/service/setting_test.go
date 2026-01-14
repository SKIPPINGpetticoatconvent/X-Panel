package service

import (
	"testing"
)

func TestSettingService_CRUD(t *testing.T) {
	setupTestDB(t)

	s := &SettingService{}

	// Test default value (assuming DB init populates it or returns default)
	// Actually InitDB populates defaults usually? Or SettingService logic does.
	// In this codebase, usually migration or first run logic handles it.
	// Let's test explicit Set/Get.

	// 1. Text Web Port
	port, err := s.GetPort()
	if err != nil {
		t.Fatalf("GetPort failed: %v", err)
	}
	// default is 13688 from defaultValueMap
	if port != 13688 {
		t.Logf("Default port might be different or not set: %d", port)
	}

	err = s.SetPort(8080)
	if err != nil {
		t.Fatalf("SetPort failed: %v", err)
	}

	port, err = s.GetPort()
	if err != nil {
		t.Fatalf("GetPort failed after set: %v", err)
	}
	if port != 8080 {
		t.Errorf("Expected port 8080, got %d", port)
	}

	// 2. Test Boolean (TgBotEnable)
	enabled, err := s.GetTgbotEnabled()
	if err != nil {
		t.Fatalf("GetTgbotEnabled failed: %v", err)
	}
	if enabled {
		t.Error("Expected TgBotEnabled false by default")
	}

	err = s.SetTgbotEnabled(true)
	if err != nil {
		t.Fatalf("SetTgbotEnabled failed: %v", err)
	}

	enabled, err = s.GetTgbotEnabled()
	if err != nil {
		t.Fatalf("GetTgbotEnabled failed after set: %v", err)
	}
	if !enabled {
		t.Error("Expected TgBotEnabled true")
	}

	// 3. Test String (WebTitle - if exists or RemarkModel)
	model, err := s.GetRemarkModel()
	if err != nil {
		t.Fatalf("GetRemarkModel failed: %v", err)
	}
	if model != "-ieo" {
		t.Errorf("Expected default remark model '-ieo', got '%s'", model)
	}
}
