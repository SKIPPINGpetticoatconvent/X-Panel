package service

import (
	"testing"
	"x-ui/database"
	"x-ui/database/model"
)

func TestSettingService_CRUD(t *testing.T) {
	setupTestDB(t)
	s := &SettingService{}

	// Test Port
	port, err := s.GetPort()
	if err != nil && !database.IsNotFound(err) {
		// Default might be missing or default val
	}

	err = s.SetPort(12345)
	if err != nil {
		t.Fatalf("SetPort failed: %v", err)
	}

	port, err = s.GetPort()
	if err != nil {
		t.Fatalf("GetPort failed: %v", err)
	}
	if port != 12345 {
		t.Errorf("Expected port 12345, got %d", port)
	}

	// Test TgBot Enable
	enabled, err := s.GetTgbotEnabled()
	// Default is false or error if not set (but defaultValueMap handles defaults?)
	// setting.go has defaultValueMap.

	err = s.SetTgbotEnabled(true)
	if err != nil {
		t.Errorf("SetTgbotEnabled failed: %v", err)
	}

	enabled, err = s.GetTgbotEnabled()
	if err != nil {
		t.Fatalf("GetTgbotEnabled failed: %v", err)
	}
	if !enabled {
		t.Error("Expected tgbot enabled = true")
	}
}

func TestSettingService_ResetSettings(t *testing.T) {
	setupTestDB(t)
	s := &SettingService{}

	s.SetPort(54321)
	s.SetTgbotEnabled(true)

	err := s.ResetSettings()
	if err != nil {
		t.Fatalf("ResetSettings failed: %v", err)
	}

	// Verify reset (should revert to defaults or empty)
	// GetPort might return default from map if DB entry gone.
	// Default port in map is "13688".

	port, _ := s.GetPort()
	if port != 13688 {
		t.Errorf("Expected default port 13688 after reset, got %d", port)
	}

	// Verify DB is empty of settings?
	// ResetSettings implementation deletes all FROM settings.
	var count int64
	database.GetDB().Model(&model.Setting{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 settings in DB, got %d", count)
	}
}
