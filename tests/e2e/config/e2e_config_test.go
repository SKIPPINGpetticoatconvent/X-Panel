package config

import (
	"os"
	"testing"
)

func TestDefaultE2EConfig(t *testing.T) {
	config := DefaultE2EConfig()

	if config.DBFolder != "/tmp/x-panel-e2e/db" {
		t.Errorf("Expected DBFolder to be /tmp/x-panel-e2e/db, got %s", config.DBFolder)
	}
	if config.TestPort != 13688 {
		t.Errorf("Expected TestPort to be 13688, got %d", config.TestPort)
	}
	if config.GoEnv != "test" {
		t.Errorf("Expected GoEnv to be test, got %s", config.GoEnv)
	}
	if !config.Debug {
		t.Error("Expected Debug to be true")
	}
	if config.EnableFail2Ban {
		t.Error("Expected EnableFail2Ban to be false")
	}
}

func TestLoadE2EConfigFromEnv(t *testing.T) {
	// Set test environment variables
	os.Setenv("XUI_DB_FOLDER", "/custom/db/path")
	os.Setenv("XUI_TEST_PORT", "15000")
	os.Setenv("XUI_DEBUG", "false")
	defer func() {
		os.Unsetenv("XUI_DB_FOLDER")
		os.Unsetenv("XUI_TEST_PORT")
		os.Unsetenv("XUI_DEBUG")
	}()

	config, err := LoadE2EConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.DBFolder != "/custom/db/path" {
		t.Errorf("Expected DBFolder to be /custom/db/path, got %s", config.DBFolder)
	}
	if config.TestPort != 15000 {
		t.Errorf("Expected TestPort to be 15000, got %d", config.TestPort)
	}
	if config.Debug {
		t.Error("Expected Debug to be false")
	}
}

func TestSetupTestEnvironment(t *testing.T) {
	config := DefaultE2EConfig()
	// Use a unique temp directory for this test
	config.DBFolder = "/tmp/x-panel-e2e-test-setup/db"
	config.LogFolder = "/tmp/x-panel-e2e-test-setup/logs"

	err := config.SetupTestEnvironment()
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Verify directories were created
	if _, err := os.Stat(config.DBFolder); os.IsNotExist(err) {
		t.Error("DB folder was not created")
	}
	if _, err := os.Stat(config.LogFolder); os.IsNotExist(err) {
		t.Error("Log folder was not created")
	}

	// Verify environment variables were set
	if os.Getenv("GO_ENV") != "test" {
		t.Error("GO_ENV was not set correctly")
	}

	// Cleanup
	config.CleanupTestEnvironment()
}

func TestGetBaseURL(t *testing.T) {
	config := DefaultE2EConfig()
	expected := "http://localhost:13688"
	if config.GetBaseURL() != expected {
		t.Errorf("Expected base URL to be %s, got %s", expected, config.GetBaseURL())
	}
}

func TestIsTestEnvironment(t *testing.T) {
	config := DefaultE2EConfig()
	if !config.IsTestEnvironment() {
		t.Error("Expected IsTestEnvironment to return true for default config")
	}

	config.GoEnv = "production"
	if config.IsTestEnvironment() {
		t.Error("Expected IsTestEnvironment to return false for production")
	}
}