package common

import (
	"errors"
	"strings"
	"testing"
)

func TestNewErrorf(t *testing.T) {
	err := NewErrorf("test %d", 123)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "test 123" {
		t.Errorf("expected 'test 123', got '%s'", err.Error())
	}
}

func TestNewError(t *testing.T) {
	err := NewError("hello", "world")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got '%s'", err.Error())
	}
}

func TestFormatTraffic(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0.00B"},
		{100, "100.00B"},
		{1023, "1023.00B"},
		{1024, "1.00KB"},
		{1024 * 1024, "1.00MB"},
		{1024 * 1024 * 1024, "1.00GB"},
		{1024 * 1024 * 1024 * 1024, "1.00TB"},
	}

	for _, tt := range tests {
		result := FormatTraffic(tt.input)
		if result != tt.expected {
			t.Errorf("FormatTraffic(%d): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

func TestCombine(t *testing.T) {
	t.Run("NoErrors", func(t *testing.T) {
		err := Combine(nil, nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("SingleError", func(t *testing.T) {
		e1 := errors.New("error 1")
		err := Combine(e1)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "error 1") {
			t.Errorf("expected error to contain 'error 1', got '%s'", err.Error())
		}
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		e1 := errors.New("error 1")
		e2 := errors.New("error 2")
		err := Combine(e1, nil, e2)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "error 1") || !strings.Contains(msg, "error 2") {
			t.Errorf("expected error to contain both errors, got '%s'", msg)
		}
	})
}

func TestRandomInt(t *testing.T) {
	for i := 0; i < 100; i++ {
		max := 10
		n := RandomInt(max)
		if n < 0 || n >= max {
			t.Errorf("RandomInt(%d) returned %d, expected [0, %d)", max, n, max)
		}
	}

	if RandomInt(0) != 0 {
		t.Error("RandomInt(0) should return 0")
	}
	if RandomInt(-1) != 0 {
		t.Error("RandomInt(-1) should return 0")
	}
}
