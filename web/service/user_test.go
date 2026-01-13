package service

import (
	"path/filepath"
	"testing"

	"x-ui/database"
	"x-ui/util/crypto"
)

// setupTestDB initializes an in-memory SQLite database for testing
func setupTestDB(t *testing.T) {
	// Use a temporary file or in-memory DB.
	// InitDB requires a path.
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init test db: %v", err)
	}

	// AutoMigrate is done in InitDB, so tables should exist.
}

func cleanupTestDB() {
	// Assuming CloseDB exists or we just let it close on process exit (sqlite)
	// database.CloseDB() - it's not exported or doesn't allow cleanly switching usually in singletons
	// But for sequential tests in one package it might be okay.
	// For now, we will just rely on t.TempDir removing files,
	// but the gorm connection might stay open.
}

func TestUpdateFirstUser(t *testing.T) {
	setupTestDB(t)

	s := &UserService{}

	// 1. Update with valid data (Creates first user)
	err := s.UpdateFirstUser("admin", "admin")
	if err != nil {
		t.Errorf("UpdateFirstUser failed: %v", err)
	}

	// 2. Verify existence
	user, err := s.GetFirstUser()
	if err != nil {
		t.Errorf("GetFirstUser failed: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("Expected username admin, got %s", user.Username)
	}
	if !crypto.CheckPasswordHash(user.Password, "admin") {
		t.Errorf("Password mismatch")
	}

	// 3. Update again
	err = s.UpdateFirstUser("newadmin", "newpass")
	if err != nil {
		t.Errorf("UpdateFirstUser update failed: %v", err)
	}

	user, err = s.GetFirstUser()
	if err != nil {
		t.Errorf("GetFirstUser failed after update: %v", err)
	}
	if user.Username != "newadmin" {
		t.Errorf("Expected username newadmin, got %s", user.Username)
	}
}

func TestUpdateFirstUser_Validation(t *testing.T) {
	// Note: Since DB is a package global singleton in this project,
	// parallel tests or stateful tests reusing generic setup might conflict.
	// For now, we reuse the same DB or assume sequential execution if setupTestDB re-opens.
	// InitDB re-opening same path is fine.

	setupTestDB(t)
	s := &UserService{}

	err := s.UpdateFirstUser("", "pass")
	if err == nil {
		t.Error("Expected error for empty username")
	}

	err = s.UpdateFirstUser("user", "")
	if err == nil {
		t.Error("Expected error for empty password")
	}
}
