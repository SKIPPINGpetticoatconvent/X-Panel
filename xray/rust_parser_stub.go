//go:build !cgo

package xray

import (
	"x-ui/logger"
)

// IsRustParserAvailable Returns false when CGO is disabled
func IsRustParserAvailable() bool {
	logger.Debug("Rust parser disabled (CGO_ENABLED=0)")
	return false
}

// ParseTrafficStatRust Stub implementation
func ParseTrafficStatRust(name string) TrafficParseResult {
	return TrafficParseResult{TrafficType: TrafficTypeNone}
}

// ParseClientTrafficStatRust Stub implementation
func ParseClientTrafficStatRust(name string) ClientTrafficParseResult {
	return ClientTrafficParseResult{Success: false}
}
