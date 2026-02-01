package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetVersion(t *testing.T) {
	v := GetVersion()
	if v == "" {
		t.Error("GetVersion() should not return empty string")
	}
}

func TestGetName(t *testing.T) {
	n := GetName()
	if n == "" {
		t.Error("GetName() should not return empty string")
	}
}

func TestLogLevelConstants(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{Debug, "debug"},
		{Info, "info"},
		{Notice, "notice"},
		{Warning, "warning"},
		{Error, "error"},
	}
	for _, tt := range tests {
		if string(tt.level) != tt.want {
			t.Errorf("LogLevel %q != %q", tt.level, tt.want)
		}
	}
}

func TestIsDebug(t *testing.T) {
	// 保存并恢复环境变量
	orig := os.Getenv("XUI_DEBUG")
	defer func() { os.Setenv("XUI_DEBUG", orig) }()

	os.Setenv("XUI_DEBUG", "true")
	if !IsDebug() {
		t.Error("IsDebug() should return true when XUI_DEBUG=true")
	}

	os.Setenv("XUI_DEBUG", "false")
	if IsDebug() {
		t.Error("IsDebug() should return false when XUI_DEBUG=false")
	}

	os.Unsetenv("XUI_DEBUG")
	if IsDebug() {
		t.Error("IsDebug() should return false when XUI_DEBUG is not set")
	}
}

func TestGetLogLevel(t *testing.T) {
	origDebug := os.Getenv("XUI_DEBUG")
	origLevel := os.Getenv("XUI_LOG_LEVEL")
	defer func() {
		os.Setenv("XUI_DEBUG", origDebug)
		os.Setenv("XUI_LOG_LEVEL", origLevel)
	}()

	// debug 模式优先
	os.Setenv("XUI_DEBUG", "true")
	os.Unsetenv("XUI_LOG_LEVEL")
	if GetLogLevel() != Debug {
		t.Errorf("GetLogLevel() = %q, want %q when debug mode", GetLogLevel(), Debug)
	}

	// 非 debug 模式，使用环境变量
	os.Setenv("XUI_DEBUG", "false")
	os.Setenv("XUI_LOG_LEVEL", "warning")
	if GetLogLevel() != Warning {
		t.Errorf("GetLogLevel() = %q, want %q", GetLogLevel(), Warning)
	}

	// 非 debug，无环境变量，默认 info
	os.Unsetenv("XUI_LOG_LEVEL")
	if GetLogLevel() != Info {
		t.Errorf("GetLogLevel() = %q, want %q (default)", GetLogLevel(), Info)
	}
}

func TestGetBinFolderPath(t *testing.T) {
	orig := os.Getenv("XUI_BIN_FOLDER")
	defer func() { os.Setenv("XUI_BIN_FOLDER", orig) }()

	os.Setenv("XUI_BIN_FOLDER", "/custom/bin")
	if got := GetBinFolderPath(); got != "/custom/bin" {
		t.Errorf("GetBinFolderPath() = %q, want /custom/bin", got)
	}

	os.Unsetenv("XUI_BIN_FOLDER")
	if got := GetBinFolderPath(); got != "bin" {
		t.Errorf("GetBinFolderPath() = %q, want bin (default)", got)
	}
}

func TestGetDBFolderPath(t *testing.T) {
	orig := os.Getenv("XUI_DB_FOLDER")
	defer func() { os.Setenv("XUI_DB_FOLDER", orig) }()

	os.Setenv("XUI_DB_FOLDER", "/tmp/test-db")
	if got := GetDBFolderPath(); got != "/tmp/test-db" {
		t.Errorf("GetDBFolderPath() = %q, want /tmp/test-db", got)
	}
}

func TestGetDBPath(t *testing.T) {
	orig := os.Getenv("XUI_DB_FOLDER")
	defer func() { os.Setenv("XUI_DB_FOLDER", orig) }()

	os.Setenv("XUI_DB_FOLDER", "/tmp/test-db")
	got := GetDBPath()
	name := GetName()
	want := "/tmp/test-db/" + name + ".db"
	if got != want {
		t.Errorf("GetDBPath() = %q, want %q", got, want)
	}
}

func TestGetLogFolder(t *testing.T) {
	orig := os.Getenv("XUI_LOG_FOLDER")
	defer func() { os.Setenv("XUI_LOG_FOLDER", orig) }()

	os.Setenv("XUI_LOG_FOLDER", "/tmp/test-log")
	if got := GetLogFolder(); got != "/tmp/test-log" {
		t.Errorf("GetLogFolder() = %q, want /tmp/test-log", got)
	}
}

func TestGetSNIFolderPath(t *testing.T) {
	orig := os.Getenv("XUI_SNI_FOLDER")
	defer func() { os.Setenv("XUI_SNI_FOLDER", orig) }()

	os.Setenv("XUI_SNI_FOLDER", "/tmp/test-sni")
	if got := GetSNIFolderPath(); got != "/tmp/test-sni" {
		t.Errorf("GetSNIFolderPath() = %q, want /tmp/test-sni", got)
	}
}

func TestCopyFile(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	// 写入源文件
	content := []byte("hello world")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// 复制
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	// 验证
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copied content = %q, want %q", got, content)
	}
}

func TestCopyFile_SrcNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	err := copyFile(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dst"))
	if err == nil {
		t.Error("copyFile() should return error for nonexistent source")
	}
}
