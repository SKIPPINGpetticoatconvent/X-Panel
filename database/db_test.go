package database

import (
	"bytes"
	"testing"
)

func TestIsSQLiteDB(t *testing.T) {
	// Valid header
	validHeader := []byte("SQLite format 3\x00")
	reader := bytes.NewReader(validHeader)
	isSqlite, err := IsSQLiteDB(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !isSqlite {
		t.Error("Expected true for valid SQLite header")
	}

	// Invalid header
	invalidHeader := []byte("NotSQLiteFileButLongEnoughToRead")
	reader = bytes.NewReader(invalidHeader)
	isSqlite, err = IsSQLiteDB(reader)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if isSqlite {
		t.Error("Expected false for invalid SQLite header")
	}
}
