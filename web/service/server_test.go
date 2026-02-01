package service

import (
	"testing"
)

func TestProcessState_Constants(t *testing.T) {
	if Running != "running" {
		t.Errorf("Running = %q, want running", Running)
	}
	if Stop != "stop" {
		t.Errorf("Stop = %q, want stop", Stop)
	}
	if Error != "error" {
		t.Errorf("Error = %q, want error", Error)
	}
}

func TestHasSuffix(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		suffixes []string
		want     bool
	}{
		{
			name:     "匹配单个后缀",
			line:     "some log line [freedom]",
			suffixes: []string{"freedom"},
			want:     true,
		},
		{
			name:     "匹配多个后缀中的一个",
			line:     "some log line [blackhole]",
			suffixes: []string{"freedom", "blackhole"},
			want:     true,
		},
		{
			name:     "无匹配",
			line:     "some log line [proxy]",
			suffixes: []string{"freedom", "blackhole"},
			want:     false,
		},
		{
			name:     "空后缀列表",
			line:     "some log line [freedom]",
			suffixes: []string{},
			want:     false,
		},
		{
			name:     "空行",
			line:     "",
			suffixes: []string{"freedom"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSuffix(tt.line, tt.suffixes)
			if got != tt.want {
				t.Errorf("hasSuffix(%q, %v) = %v, want %v", tt.line, tt.suffixes, got, tt.want)
			}
		})
	}
}

func TestNormalizeCountryCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"United States", "US"},
		{"USA", "US"},
		{"United Kingdom", "GB"},
		{"UK", "GB"},
		{"Japan", "JP"},
		{"Germany", "DE"},
		{"France", "FR"},
		{"Canada", "CA"},
		{"Australia", "AU"},
		{"Singapore", "SG"},
		{"Hong Kong", "HK"},
		{"Taiwan", "TW"},
		{"us", "US"}, // 小写转大写
		{"jp", "JP"}, // 两字母代码直接返回
		{"Unknown Country", "Unknown"},
		{"  US  ", "US"}, // 空格处理
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeCountryCode(tt.input)
			if got != tt.want {
				t.Errorf("normalizeCountryCode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestServerService_IsValidGeofileName(t *testing.T) {
	s := &ServerService{}

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"合法名称", "geoip.dat", true},
		{"合法名称带下划线", "geoip_IR.dat", true},
		{"合法名称带连字符", "geosite-custom.dat", true},
		{"空字符串", "", false},
		{"路径遍历", "../etc/passwd", false},
		{"绝对路径", "/etc/passwd", false},
		{"反斜杠路径", "path\\file.dat", false},
		{"正斜杠路径", "path/file.dat", false},
		{"非 .dat 扩展名", "geoip.txt", false},
		{"无扩展名", "geoip", false},
		{"特殊字符", "geo ip.dat", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.IsValidGeofileName(tt.filename)
			if got != tt.want {
				t.Errorf("IsValidGeofileName(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestServerService_NormalizeDomain(t *testing.T) {
	s := &ServerService{}

	tests := []struct {
		input string
		want  string
	}{
		{"Example.COM", "example.com"},
		{"  test.org  ", "test.org"},
		{"UPPER.CASE.COM", "upper.case.com"},
		{"already.lower", "already.lower"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := s.normalizeDomain(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestServerService_RemoveDuplicatesFromSlice(t *testing.T) {
	s := &ServerService{}

	tests := []struct {
		name  string
		input []string
		want  int
	}{
		{"无重复", []string{"a.com", "b.com", "c.com"}, 3},
		{"有重复", []string{"a.com", "b.com", "a.com"}, 2},
		{"大小写重复", []string{"A.COM", "a.com"}, 1},
		{"空切片", []string{}, 0},
		{"单个元素", []string{"a.com"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.removeDuplicatesFromSlice(tt.input)
			if len(got) != tt.want {
				t.Errorf("removeDuplicatesFromSlice(%v) returned %d items, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestServerService_GetDefaultSNIDomains(t *testing.T) {
	s := &ServerService{}

	countries := []string{"US", "JP", "UK", "GB", "KR", "DE", "DEFAULT", "XX"}
	for _, c := range countries {
		domains := s.getDefaultSNIDomains(c)
		if len(domains) == 0 {
			t.Errorf("getDefaultSNIDomains(%q) returned empty list", c)
		}
		for _, d := range domains {
			if d == "" {
				t.Errorf("getDefaultSNIDomains(%q) contains empty domain", c)
			}
		}
	}
}

func TestServerService_GetNewUUID(t *testing.T) {
	s := &ServerService{}
	result, err := s.GetNewUUID()
	if err != nil {
		t.Fatalf("GetNewUUID() error: %v", err)
	}
	uuid, ok := result["uuid"]
	if !ok {
		t.Fatal("GetNewUUID() result missing 'uuid' key")
	}
	if len(uuid) != 36 {
		t.Errorf("UUID length = %d, want 36", len(uuid))
	}
}
