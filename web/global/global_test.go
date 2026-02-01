package global

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- HashStorage 测试 ---

func TestNewHashStorage(t *testing.T) {
	hs := NewHashStorage(time.Minute)
	if hs == nil {
		t.Fatal("NewHashStorage should not return nil")
	}
	if hs.Expiration != time.Minute {
		t.Errorf("Expiration = %v, want %v", hs.Expiration, time.Minute)
	}
	if len(hs.Data) != 0 {
		t.Errorf("Data should be empty, got %d entries", len(hs.Data))
	}
}

func TestHashStorage_SaveAndGetValue(t *testing.T) {
	hs := NewHashStorage(time.Hour)

	query := "test-query-string"
	hash := hs.SaveHash(query)

	// 验证 hash 是正确的 SHA256
	expected := sha256.Sum256([]byte(query))
	expectedStr := hex.EncodeToString(expected[:])
	if hash != expectedStr {
		t.Errorf("SaveHash() = %q, want %q", hash, expectedStr)
	}

	// 验证可以获取值
	value, exists := hs.GetValue(hash)
	if !exists {
		t.Error("GetValue() should return true for saved hash")
	}
	if value != query {
		t.Errorf("GetValue() = %q, want %q", value, query)
	}
}

func TestHashStorage_GetValue_NotFound(t *testing.T) {
	hs := NewHashStorage(time.Hour)
	_, exists := hs.GetValue("nonexistent")
	if exists {
		t.Error("GetValue() should return false for nonexistent hash")
	}
}

func TestHashStorage_IsHash(t *testing.T) {
	hs := NewHashStorage(time.Hour)

	tests := []struct {
		input string
		want  bool
	}{
		{strings.Repeat("a", 64), true},
		{strings.Repeat("0", 64), true},
		{"abc123def456abc123def456abc123def456abc123def456abc123def456abcd", true},
		{"not-a-hash", false},
		{strings.Repeat("g", 64), false}, // g 不是合法的 hex 字符
		{"", false},
		{strings.Repeat("a", 63), false},
		{strings.Repeat("a", 65), false},
	}

	for _, tt := range tests {
		got := hs.IsHash(tt.input)
		if got != tt.want {
			t.Errorf("IsHash(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestHashStorage_RemoveExpiredHashes(t *testing.T) {
	hs := NewHashStorage(50 * time.Millisecond)

	hs.SaveHash("old-entry")
	time.Sleep(100 * time.Millisecond)
	hs.SaveHash("new-entry")

	hs.RemoveExpiredHashes()

	if len(hs.Data) != 1 {
		t.Errorf("Expected 1 entry after cleanup, got %d", len(hs.Data))
	}
}

func TestHashStorage_Reset(t *testing.T) {
	hs := NewHashStorage(time.Hour)
	hs.SaveHash("entry1")
	hs.SaveHash("entry2")

	hs.Reset()

	if len(hs.Data) != 0 {
		t.Errorf("Data should be empty after Reset, got %d entries", len(hs.Data))
	}
}

func TestHashStorage_ConcurrentAccess(t *testing.T) {
	hs := NewHashStorage(time.Hour)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			hash := hs.SaveHash("query-" + string(rune(i)))
			hs.GetValue(hash)
			hs.IsHash(hash)
		}(i)
	}
	wg.Wait()
}

// --- WebServer/SubServer 全局变量测试 ---

type mockWebServer struct{}

func (m *mockWebServer) GetCron() any { return nil }
func (m *mockWebServer) GetCtx() any  { return nil }

type mockSubServer struct{}

func (m *mockSubServer) GetCtx() any { return nil }

func TestSetGetWebServer(t *testing.T) {
	// 保存并恢复
	orig := webServer
	defer func() { webServer = orig }()

	SetWebServer(nil)
	if got := GetWebServer(); got != nil {
		t.Error("GetWebServer() should return nil after SetWebServer(nil)")
	}
}

func TestSetGetSubServer(t *testing.T) {
	orig := subServer
	defer func() { subServer = orig }()

	SetSubServer(nil)
	if got := GetSubServer(); got != nil {
		t.Error("GetSubServer() should return nil after SetSubServer(nil)")
	}
}
