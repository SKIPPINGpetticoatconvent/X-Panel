package sys_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"x-ui/util/sys"
)

func TestBackupFile(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_file.txt")
	content := []byte("hello world")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Backup the file
	if err := sys.BackupFile(filePath); err != nil {
		t.Fatalf("BackupFile failed: %v", err)
	}

	// Check if backup exists
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	foundBackup := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "test_file.txt.bak.") {
			foundBackup = true
			break
		}
	}

	if !foundBackup {
		t.Error("Backup file not found")
	}
}

func TestAtomicWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "atomic_test.txt")
	content := []byte("atomic content")

	if err := sys.AtomicWriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	// Read check
	readContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("Atomic content = %s, want %s", string(readContent), string(content))
	}
}
