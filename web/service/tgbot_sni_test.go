package service

import (
	"reflect"
	"testing"
)

func TestGenerateRealityServerNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Standard Domain",
			input:    "google.com",
			expected: []string{"google.com", "www.google.com"},
		},
		{
			name:     "Domain with Port",
			input:    "google.com:443",
			expected: []string{"google.com", "www.google.com"},
		},
		{
			name:     "WWW Domain",
			input:    "www.walmart.com",
			expected: []string{"www.walmart.com", "walmart.com"},
		},
		{
			name:     "WWW Domain with Port",
			input:    "www.walmart.com:443",
			expected: []string{"www.walmart.com", "walmart.com"},
		},
		{
			name:     "Subdomain",
			input:    "api.walmart.com",
			expected: []string{"api.walmart.com", "www.api.walmart.com"},
		},
		{
			name:     "WWW Subdomain",
			input:    "www.api.walmart.com",
			expected: []string{"www.api.walmart.com", "api.walmart.com"},
		},
		{
			name:     "Domain with Different Port",
			input:    "example.com:8080",
			expected: []string{"example.com", "www.example.com"},
		},
		{
			name:     "WWW Domain with Different Port",
			input:    "www.example.com:8080",
			expected: []string{"www.example.com", "example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRealityServerNames(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GenerateRealityServerNames(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// 测试边缘情况
func TestGenerateRealityServerNames_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single Level WWW",
			input:    "www.com",
			expected: []string{"www.com", "com"},
		},
		{
			name:     "Multiple WWW prefixes should not occur",
			input:    "www.www.example.com",
			expected: []string{"www.www.example.com", "www.example.com"},
		},
		{
			name:     "Domain without TLD",
			input:    "localhost",
			expected: []string{"localhost", "www.localhost"},
		},
		{
			name:     "WWW Domain without TLD",
			input:    "www.localhost",
			expected: []string{"www.localhost", "localhost"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateRealityServerNames(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GenerateRealityServerNames(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}