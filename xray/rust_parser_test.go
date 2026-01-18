//go:build cgo

package xray

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTrafficStatRust(t *testing.T) {
	if !IsRustParserAvailable() {
		t.Skip("Rust parser not available")
	}

	tests := []struct {
		name           string
		input          string
		expectedType   int
		expectedTag    string
		expectedIsDown bool
	}{
		{
			name:           "Inbound Traffic Downlink",
			input:          "inbound>>>vmess-tcp>>>traffic>>>downlink",
			expectedType:   TrafficTypeInbound,
			expectedTag:    "vmess-tcp",
			expectedIsDown: true,
		},
		{
			name:           "Outbound Traffic Uplink",
			input:          "outbound>>>direct>>>traffic>>>uplink",
			expectedType:   TrafficTypeOutbound,
			expectedTag:    "direct",
			expectedIsDown: false,
		},
		{
			name:         "Invalid Format",
			input:        "invalid>>>format",
			expectedType: TrafficTypeNone,
		},
		{
			name:         "API Tag",
			input:        "inbound>>>api>>>traffic>>>downlink",
			expectedType: TrafficTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTrafficStatRust(tt.input)
			assert.Equal(t, tt.expectedType, result.TrafficType)
			if tt.expectedType != TrafficTypeNone {
				assert.Equal(t, tt.expectedTag, result.Tag)
				assert.Equal(t, tt.expectedIsDown, result.IsDownlink)
			}
		})
	}
}

func TestParseClientTrafficStatRust(t *testing.T) {
	if !IsRustParserAvailable() {
		t.Skip("Rust parser not available")
	}

	tests := []struct {
		name            string
		input           string
		expectedSuccess bool
		expectedEmail   string
		expectedIsDown  bool
	}{
		{
			name:            "Client Traffic Downlink",
			input:           "user>>>test@example.com>>>traffic>>>downlink",
			expectedSuccess: true,
			expectedEmail:   "test@example.com",
			expectedIsDown:  true,
		},
		{
			name:            "Client Traffic Uplink",
			input:           "user>>>user2>>>traffic>>>uplink",
			expectedSuccess: true,
			expectedEmail:   "user2",
			expectedIsDown:  false,
		},
		{
			name:            "Invalid Format",
			input:           "user>>>invalid",
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseClientTrafficStatRust(tt.input)
			assert.Equal(t, tt.expectedSuccess, result.Success)
			if result.Success {
				assert.Equal(t, tt.expectedEmail, result.Email)
				assert.Equal(t, tt.expectedIsDown, result.IsDownlink)
			}
		})
	}
}
