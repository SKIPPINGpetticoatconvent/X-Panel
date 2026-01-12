package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

// E2EConfig holds all configuration values for E2E testing
type E2EConfig struct {
	// Database settings
	DBFolder string

	// Log settings
	LogFolder string
	LogLevel  string

	// Debug mode
	Debug bool

	// Binary paths
	BinFolder string
	SNIFolder string

	// Container mode
	RunInContainer bool

	// Security settings
	EnableFail2Ban bool

	// Web panel settings
	TestPort     int
	TestUsername string
	TestPassword string

	// Test environment
	GoEnv string
}

// DefaultE2EConfig returns the default configuration for E2E testing
func DefaultE2EConfig() *E2EConfig {
	return &E2EConfig{
		DBFolder:       "/tmp/x-panel-e2e/db",
		LogFolder:      "/tmp/x-panel-e2e/logs",
		LogLevel:       "debug",
		Debug:          true,
		BinFolder:      "bin",
		SNIFolder:      "bin/sni",
		RunInContainer: true,
		EnableFail2Ban: false,
		TestPort:       13688,
		TestUsername:   "e2e_admin",
		TestPassword:   "e2e_test_pass_123",
		GoEnv:          "test",
	}
}

// LoadE2EConfig loads configuration from .env.e2e file and environment variables
// Environment variables take precedence over .env.e2e file values
func LoadE2EConfig() (*E2EConfig, error) {
	// Try to load .env.e2e file from project root
	envFile := findEnvFile()
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			// Log warning but continue - env vars might be set directly
			fmt.Printf("Warning: Could not load %s: %v\n", envFile, err)
		}
	}

	config := DefaultE2EConfig()

	// Override with environment variables
	if val := os.Getenv("XUI_DB_FOLDER"); val != "" {
		config.DBFolder = val
	}
	if val := os.Getenv("XUI_LOG_FOLDER"); val != "" {
		config.LogFolder = val
	}
	if val := os.Getenv("XUI_LOG_LEVEL"); val != "" {
		config.LogLevel = val
	}
	if val := os.Getenv("XUI_DEBUG"); val != "" {
		config.Debug = val == "true"
	}
	if val := os.Getenv("XUI_BIN_FOLDER"); val != "" {
		config.BinFolder = val
	}
	if val := os.Getenv("XUI_SNI_FOLDER"); val != "" {
		config.SNIFolder = val
	}
	if val := os.Getenv("XPANEL_RUN_IN_CONTAINER"); val != "" {
		config.RunInContainer = val == "true"
	}
	if val := os.Getenv("XUI_ENABLE_FAIL2BAN"); val != "" {
		config.EnableFail2Ban = val == "true"
	}
	if val := os.Getenv("XUI_TEST_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.TestPort = port
		}
	}
	if val := os.Getenv("XUI_TEST_USERNAME"); val != "" {
		config.TestUsername = val
	}
	if val := os.Getenv("XUI_TEST_PASSWORD"); val != "" {
		config.TestPassword = val
	}
	if val := os.Getenv("GO_ENV"); val != "" {
		config.GoEnv = val
	}

	return config, nil
}

// findEnvFile searches for .env.e2e file in common locations
func findEnvFile() string {
	searchPaths := []string{
		".env.e2e",
		"../.env.e2e",
		"../../.env.e2e",
		"../../../.env.e2e",
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}
	return ""
}

// SetupTestEnvironment sets up the test environment by creating necessary directories
// and setting environment variables
func (c *E2EConfig) SetupTestEnvironment() error {
	// Create test directories
	dirs := []string{c.DBFolder, c.LogFolder}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Set environment variables for the application
	envVars := map[string]string{
		"GO_ENV":                  c.GoEnv,
		"XUI_DB_FOLDER":           c.DBFolder,
		"XUI_LOG_FOLDER":          c.LogFolder,
		"XUI_LOG_LEVEL":           c.LogLevel,
		"XUI_DEBUG":               strconv.FormatBool(c.Debug),
		"XUI_BIN_FOLDER":          c.BinFolder,
		"XUI_SNI_FOLDER":          c.SNIFolder,
		"XPANEL_RUN_IN_CONTAINER": strconv.FormatBool(c.RunInContainer),
		"XUI_ENABLE_FAIL2BAN":     strconv.FormatBool(c.EnableFail2Ban),
	}

	for key, value := range envVars {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}

	return nil
}

// CleanupTestEnvironment removes test directories and resets environment
func (c *E2EConfig) CleanupTestEnvironment() error {
	// Remove test directories
	dirs := []string{c.DBFolder, c.LogFolder}
	for _, dir := range dirs {
		// Only remove if it's under /tmp to prevent accidental deletion
		if filepath.HasPrefix(dir, "/tmp/") {
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("failed to remove directory %s: %w", dir, err)
			}
		}
	}
	return nil
}

// GetBaseURL returns the base URL for the test web panel
func (c *E2EConfig) GetBaseURL() string {
	return fmt.Sprintf("http://localhost:%d", c.TestPort)
}

// IsTestEnvironment returns true if running in test environment
func (c *E2EConfig) IsTestEnvironment() bool {
	return c.GoEnv == "test"
}