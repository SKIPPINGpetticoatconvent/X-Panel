package service

import (
	"testing"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/util/crypto"
)

// Helper function to clean up users table for test isolation
func cleanupUsers(t *testing.T) {
	db := database.GetDB()
	db.Where("1 = 1").Delete(&model.User{})
}

func TestUserService_GetFirstUser(t *testing.T) {
	setupTestDB(t)
	cleanupUsers(t)

	s := &UserService{}

	// Initially no user
	_, err := s.GetFirstUser()
	if err == nil {
		t.Error("Expected error when no user exists")
	}

	// Create a user
	db := database.GetDB()
	hashedPw, _ := crypto.HashPasswordAsBcrypt("testpass")
	db.Create(&model.User{Username: "testuser", Password: hashedPw})

	// Now should succeed
	user, err := s.GetFirstUser()
	if err != nil {
		t.Fatalf("GetFirstUser failed: %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", user.Username)
	}
}

func TestUserService_CheckUser(t *testing.T) {
	setupTestDB(t)
	cleanupUsers(t)

	// Use real SettingService - it works with DB settings
	s := &UserService{
		settingService: SettingService{},
	}

	// Create a user
	db := database.GetDB()
	hashedPw, _ := crypto.HashPasswordAsBcrypt("correctpass")
	db.Create(&model.User{Username: "admin", Password: hashedPw})

	// Test correct credentials (2FA disabled by default)
	user := s.CheckUser("admin", "correctpass", "")
	if user == nil {
		t.Error("Expected user to be returned with correct credentials")
	}

	// Test wrong password
	user = s.CheckUser("admin", "wrongpass", "")
	if user != nil {
		t.Error("Expected nil with wrong password")
	}

	// Test non-existent user
	user = s.CheckUser("nonexistent", "anypass", "")
	if user != nil {
		t.Error("Expected nil for non-existent user")
	}
}

func TestUserService_UpdateFirstUser(t *testing.T) {
	setupTestDB(t)
	cleanupUsers(t)

	s := &UserService{}

	// Test empty username
	err := s.UpdateFirstUser("", "password")
	if err == nil {
		t.Error("Expected error for empty username")
	}

	// Test empty password
	err = s.UpdateFirstUser("user", "")
	if err == nil {
		t.Error("Expected error for empty password")
	}

	// Create first user
	err = s.UpdateFirstUser("admin", "adminpass")
	if err != nil {
		t.Fatalf("UpdateFirstUser failed to create user: %v", err)
	}

	// Verify user was created
	user, err := s.GetFirstUser()
	if err != nil {
		t.Fatalf("Failed to get first user: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", user.Username)
	}

	// Update existing user
	err = s.UpdateFirstUser("newadmin", "newpass")
	if err != nil {
		t.Fatalf("UpdateFirstUser failed to update user: %v", err)
	}

	// Verify update
	user, _ = s.GetFirstUser()
	if user.Username != "newadmin" {
		t.Errorf("Expected username 'newadmin', got '%s'", user.Username)
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	setupTestDB(t)
	cleanupUsers(t)

	// Use real SettingService
	s := &UserService{
		settingService: SettingService{},
	}

	// Create a user first
	db := database.GetDB()
	hashedPw, _ := crypto.HashPasswordAsBcrypt("oldpass")
	user := &model.User{Username: "olduser", Password: hashedPw}
	db.Create(user)

	// Update the user
	err := s.UpdateUser(user.Id, "newuser", "newpass")
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	// Verify update in DB
	var updatedUser model.User
	db.First(&updatedUser, user.Id)
	if updatedUser.Username != "newuser" {
		t.Errorf("Expected username 'newuser', got '%s'", updatedUser.Username)
	}

	// Verify password was hashed
	if !crypto.CheckPasswordHash(updatedUser.Password, "newpass") {
		t.Error("Password was not correctly hashed")
	}
}

func TestUserService_CheckUser_WrongPassword(t *testing.T) {
	setupTestDB(t)
	cleanupUsers(t)

	s := &UserService{
		settingService: SettingService{},
	}

	// Create a user
	db := database.GetDB()
	hashedPw, _ := crypto.HashPasswordAsBcrypt("secret123")
	db.Create(&model.User{Username: "testuser", Password: hashedPw})

	// Test wrong password
	result := s.CheckUser("testuser", "wrongpassword", "")
	if result != nil {
		t.Error("Expected nil for wrong password")
	}

	// Test correct password
	result = s.CheckUser("testuser", "secret123", "")
	if result == nil {
		t.Error("Expected user for correct password")
	}
}

func TestUserService_UpdateFirstUser_CreateNew(t *testing.T) {
	setupTestDB(t)
	cleanupUsers(t)

	s := &UserService{}

	// When no user exists, should create one
	err := s.UpdateFirstUser("firstuser", "firstpass")
	if err != nil {
		t.Fatalf("UpdateFirstUser should create user when none exists: %v", err)
	}

	// Verify
	user, err := s.GetFirstUser()
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if user.Username != "firstuser" {
		t.Errorf("Expected 'firstuser', got '%s'", user.Username)
	}
}

func TestUserService_GetFirstUser_Multiple(t *testing.T) {
	setupTestDB(t)
	cleanupUsers(t)

	s := &UserService{}
	db := database.GetDB()

	// Create multiple users
	hashedPw, _ := crypto.HashPasswordAsBcrypt("pass")
	db.Create(&model.User{Username: "user1", Password: hashedPw})
	db.Create(&model.User{Username: "user2", Password: hashedPw})

	// GetFirstUser should return the first one (by ID)
	user, err := s.GetFirstUser()
	if err != nil {
		t.Fatalf("GetFirstUser failed: %v", err)
	}
	if user.Username != "user1" {
		t.Errorf("Expected first user 'user1', got '%s'", user.Username)
	}
}
