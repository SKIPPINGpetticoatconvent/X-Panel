package service

import (
	"testing"
)

func TestIsValidGeofileName(t *testing.T) {
	s := &ServerService{}

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		// Valid cases
		{"Valid geoip", "geoip.dat", true},
		{"Valid geosite", "geosite.dat", true},
		{"Valid geoip regional", "geoip_RU.dat", true},
		{"Valid geosite regional", "geosite_CN.dat", true},

		// Invalid cases - Path Traversal
		{"Path traversal parent", "../geoip.dat", false},
		{"Path traversal root", "/etc/passwd", false},
		{"Path traversal mix", "foo/../bar.dat", false},

		// Invalid cases - Characters
		{"Invalid char space", "geoip .dat", false},
		{"Invalid char slash", "geoip/dat", false},
		{"Invalid char backslash", "geoip\\dat", false},
		{"Invalid char dollar", "geoip$.dat", false},

		// Invalid cases - Empty
		{"Empty string", "", false},

		// Invalid cases - Format
		{"No extension", "geoip", false},
		{"Wrong extension", "geoip.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.IsValidGeofileName(tt.filename); got != tt.want {
				t.Errorf("IsValidGeofileName(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}
