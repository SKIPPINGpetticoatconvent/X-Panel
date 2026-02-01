package xray

import (
	"testing"
)

func TestLogWriter_Write_ReturnsCorrectLength(t *testing.T) {
	lw := NewLogWriter()
	msg := []byte("2024/01/15 12:00:00.000000 [Info] some message")

	n, err := lw.Write(msg)
	if err != nil {
		t.Errorf("Write() error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write() n = %d, want %d", n, len(msg))
	}
}

func TestLogWriter_Write_EmptyMessage(t *testing.T) {
	lw := NewLogWriter()
	n, err := lw.Write([]byte(""))
	if err != nil {
		t.Errorf("Write() error: %v", err)
	}
	if n != 0 {
		t.Errorf("Write() n = %d, want 0", n)
	}
}

func TestLogWriter_Write_CrashDetection(t *testing.T) {
	lw := NewLogWriter()

	crashMessages := []string{
		"panic: runtime error",
		"FATAL ERROR: something went wrong",
		"stack trace follows",
		"Exception caught in handler",
	}

	for _, msg := range crashMessages {
		n, err := lw.Write([]byte(msg))
		if err != nil {
			t.Errorf("Write(%q) error: %v", msg, err)
		}
		if n != len(msg) {
			t.Errorf("Write(%q) n = %d, want %d", msg, n, len(msg))
		}
	}
}

func TestLogWriter_Write_FormattedLogLine(t *testing.T) {
	lw := NewLogWriter()

	tests := []struct {
		name string
		msg  string
	}{
		{"Info级别", "2024/01/15 12:00:00.000000 [Info] Server started"},
		{"Warning级别", "2024/01/15 12:00:00.000000 [Warning] High memory usage"},
		{"Error级别", "2024/01/15 12:00:00.000000 [Error] Connection failed"},
		{"Debug级别", "2024/01/15 12:00:00.000000 [Debug] Processing request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := lw.Write([]byte(tt.msg))
			if err != nil {
				t.Errorf("Write() error: %v", err)
			}
			if n != len(tt.msg) {
				t.Errorf("Write() n = %d, want %d", n, len(tt.msg))
			}
		})
	}
}

func TestLogWriter_Write_TLSHandshakeSkipped(t *testing.T) {
	lw := NewLogWriter()

	// TLS handshake error 应该被处理（不 panic）
	msg := "tls handshake error from 192.168.1.1"
	n, err := lw.Write([]byte(msg))
	if err != nil {
		t.Errorf("Write() error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write() n = %d, want %d", n, len(msg))
	}
}

func TestLogWriter_Write_MultipleLines(t *testing.T) {
	lw := NewLogWriter()

	msg := "2024/01/15 12:00:00.000000 [Info] line1\n2024/01/15 12:00:01.000000 [Info] line2"
	n, err := lw.Write([]byte(msg))
	if err != nil {
		t.Errorf("Write() error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write() n = %d, want %d", n, len(msg))
	}
}

func TestNewLogWriter(t *testing.T) {
	lw := NewLogWriter()
	if lw == nil {
		t.Fatal("NewLogWriter() should not return nil")
	}
	if lw.lastLine != "" {
		t.Errorf("lastLine should be empty, got %q", lw.lastLine)
	}
}
