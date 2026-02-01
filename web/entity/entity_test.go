package entity

import (
	"testing"
)

func TestAllSetting_CheckValid_ValidConfig(t *testing.T) {
	s := &AllSetting{
		WebListen:    "",
		WebPort:      8080,
		SubListen:    "",
		SubPort:      8443,
		WebBasePath:  "/panel/",
		SubPath:      "/sub/",
		SubJsonPath:  "/json/",
		TimeLocation: "Asia/Shanghai",
	}

	if err := s.CheckValid(); err != nil {
		t.Errorf("CheckValid() unexpected error: %v", err)
	}
}

func TestAllSetting_CheckValid_InvalidWebListen(t *testing.T) {
	s := &AllSetting{
		WebListen:    "not-an-ip",
		WebPort:      8080,
		SubPort:      8443,
		WebBasePath:  "/",
		SubPath:      "/sub/",
		SubJsonPath:  "/json/",
		TimeLocation: "UTC",
	}

	if err := s.CheckValid(); err == nil {
		t.Error("CheckValid() should return error for invalid WebListen IP")
	}
}

func TestAllSetting_CheckValid_InvalidSubListen(t *testing.T) {
	s := &AllSetting{
		WebListen:    "",
		WebPort:      8080,
		SubListen:    "invalid-ip",
		SubPort:      8443,
		WebBasePath:  "/",
		SubPath:      "/sub/",
		SubJsonPath:  "/json/",
		TimeLocation: "UTC",
	}

	if err := s.CheckValid(); err == nil {
		t.Error("CheckValid() should return error for invalid SubListen IP")
	}
}

func TestAllSetting_CheckValid_InvalidWebPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"零端口", 0},
		{"负端口", -1},
		{"超大端口", 70000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AllSetting{
				WebPort:      tt.port,
				SubPort:      8443,
				WebBasePath:  "/",
				SubPath:      "/sub/",
				SubJsonPath:  "/json/",
				TimeLocation: "UTC",
			}
			if err := s.CheckValid(); err == nil {
				t.Errorf("CheckValid() should return error for port %d", tt.port)
			}
		})
	}
}

func TestAllSetting_CheckValid_InvalidSubPort(t *testing.T) {
	s := &AllSetting{
		WebPort:      8080,
		SubPort:      0,
		WebBasePath:  "/",
		SubPath:      "/sub/",
		SubJsonPath:  "/json/",
		TimeLocation: "UTC",
	}

	if err := s.CheckValid(); err == nil {
		t.Error("CheckValid() should return error for invalid SubPort")
	}
}

func TestAllSetting_CheckValid_SamePortSameListen(t *testing.T) {
	s := &AllSetting{
		WebListen:    "0.0.0.0",
		WebPort:      8080,
		SubListen:    "0.0.0.0",
		SubPort:      8080,
		WebBasePath:  "/",
		SubPath:      "/sub/",
		SubJsonPath:  "/json/",
		TimeLocation: "UTC",
	}

	if err := s.CheckValid(); err == nil {
		t.Error("CheckValid() should return error when Sub and Web use same ip:port")
	}
}

func TestAllSetting_CheckValid_InvalidTimeLocation(t *testing.T) {
	s := &AllSetting{
		WebPort:      8080,
		SubPort:      8443,
		WebBasePath:  "/",
		SubPath:      "/sub/",
		SubJsonPath:  "/json/",
		TimeLocation: "Invalid/Location",
	}

	if err := s.CheckValid(); err == nil {
		t.Error("CheckValid() should return error for invalid TimeLocation")
	}
}

func TestAllSetting_CheckValid_PathNormalization(t *testing.T) {
	s := &AllSetting{
		WebPort:      8080,
		SubPort:      8443,
		WebBasePath:  "panel", // 缺少 / 前缀和后缀
		SubPath:      "sub",   // 缺少 / 前缀和后缀
		SubJsonPath:  "json",  // 缺少 / 前缀和后缀
		TimeLocation: "UTC",
	}

	if err := s.CheckValid(); err != nil {
		t.Fatalf("CheckValid() unexpected error: %v", err)
	}

	if s.WebBasePath != "/panel/" {
		t.Errorf("WebBasePath = %q, want /panel/", s.WebBasePath)
	}
	if s.SubPath != "/sub/" {
		t.Errorf("SubPath = %q, want /sub/", s.SubPath)
	}
	if s.SubJsonPath != "/json/" {
		t.Errorf("SubJsonPath = %q, want /json/", s.SubJsonPath)
	}
}

func TestAllSetting_CheckValid_ValidListenIP(t *testing.T) {
	s := &AllSetting{
		WebListen:    "127.0.0.1",
		WebPort:      8080,
		SubListen:    "192.168.1.1",
		SubPort:      8443,
		WebBasePath:  "/",
		SubPath:      "/sub/",
		SubJsonPath:  "/json/",
		TimeLocation: "UTC",
	}

	if err := s.CheckValid(); err != nil {
		t.Errorf("CheckValid() unexpected error: %v", err)
	}
}
