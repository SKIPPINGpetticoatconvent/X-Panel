package service

import (
	"testing"
)

func TestGenerateRealityServerNames(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		wantLen  int
		wantFirst string
		wantSecond string
	}{
		{
			name:       "带 www 前缀和端口",
			host:       "www.walmart.com:443",
			wantLen:    2,
			wantFirst:  "www.walmart.com",
			wantSecond: "walmart.com",
		},
		{
			name:       "带 www 前缀无端口",
			host:       "www.google.com",
			wantLen:    2,
			wantFirst:  "www.google.com",
			wantSecond: "google.com",
		},
		{
			name:       "不带 www 前缀",
			host:       "google.com",
			wantLen:    2,
			wantFirst:  "google.com",
			wantSecond: "www.google.com",
		},
		{
			name:       "不带 www 前缀带端口",
			host:       "example.com:8443",
			wantLen:    2,
			wantFirst:  "example.com",
			wantSecond: "www.example.com",
		},
		{
			name:       "子域名",
			host:       "api.walmart.com",
			wantLen:    2,
			wantFirst:  "api.walmart.com",
			wantSecond: "www.api.walmart.com",
		},
		{
			name:       "仅 www.",
			host:       "www.",
			wantLen:    1,
			wantFirst:  "www.",
			wantSecond: "",
		},
		{
			name:       "空字符串",
			host:       "",
			wantLen:    2,
			wantFirst:  "",
			wantSecond: "www.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateRealityServerNames(tt.host)
			if len(got) != tt.wantLen {
				t.Errorf("GenerateRealityServerNames(%q) returned %d items, want %d", tt.host, len(got), tt.wantLen)
				return
			}
			if got[0] != tt.wantFirst {
				t.Errorf("GenerateRealityServerNames(%q)[0] = %q, want %q", tt.host, got[0], tt.wantFirst)
			}
			if tt.wantLen > 1 && got[1] != tt.wantSecond {
				t.Errorf("GenerateRealityServerNames(%q)[1] = %q, want %q", tt.host, got[1], tt.wantSecond)
			}
		})
	}
}
