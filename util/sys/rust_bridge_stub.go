//go:build !linux || !rust
// +build !linux,!rust

package sys

import (
	"errors"
	"fmt"
)

// SystemStats 系统统计信息结构体
type SystemStats struct {
	TCPCount    int32   `json:"tcp_count"`
	UDPCount    int32   `json:"udp_count"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryTotal uint64  `json:"memory_total"`
	CPUUsage    float32 `json:"cpu_usage"`
}

// DiskUsage 磁盘使用率统计结构体
type DiskUsage struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	Free  uint64 `json:"free"`
}

// Connection 连接信息结构体
type Connection struct {
	LocalIP    string `json:"local_ip"`
	LocalPort  uint16 `json:"local_port"`
	RemoteIP   string `json:"remote_ip"`
	RemotePort uint16 `json:"remote_port"`
	State      uint8  `json:"state"`
	Protocol   uint8  `json:"protocol"`
}

// GetTCPCountRust 获取 TCP 连接数（Rust 未启用版本）
// 当 Rust 不可用时，返回 0 和错误信息
func GetTCPCountRust() (int, error) {
	return 0, errors.New("Rust module not enabled: TCP connection count requires Rust library")
}

// GetUDPCountRust 获取 UDP 连接数（Rust 未启用版本）
// 当 Rust 不可用时，返回 0 和错误信息
func GetUDPCountRust() (int, error) {
	return 0, errors.New("Rust module not enabled: UDP connection count requires Rust library")
}

// GetMemoryUsedRust 获取已使用内存（Rust 未启用版本）
func GetMemoryUsedRust() (uint64, error) {
	return 0, errors.New("Rust module not enabled: memory usage requires Rust library")
}

// GetMemoryTotalRust 获取总内存（Rust 未启用版本）
func GetMemoryTotalRust() (uint64, error) {
	return 0, errors.New("Rust module not enabled: memory total requires Rust library")
}

// GetCPUUsageRust 获取 CPU 使用率（Rust 未启用版本）
func GetCPUUsageRust() (float32, error) {
	return 0.0, errors.New("Rust module not enabled: CPU usage requires Rust library")
}

// GetSystemStatsRust 获取完整系统统计信息（Rust 未启用版本）
func GetSystemStatsRust() (*SystemStats, error) {
	return nil, errors.New("Rust module not enabled: system stats requires Rust library")
}

// IsRustAvailable 检查 Rust 动态库是否可用
func IsRustAvailable() bool {
	return false
}

// GetMemoryUsagePercent 计算内存使用百分比（Rust 未启用版本）
func GetMemoryUsagePercent() (float64, error) {
	return 0.0, errors.New("Rust module not enabled: memory usage percent requires Rust library")
}

// GetDiskUsageRust 获取磁盘使用率（Rust 未启用版本）
func GetDiskUsageRust(path string) (*DiskUsage, error) {
	return nil, fmt.Errorf("Rust module not enabled: disk usage for path '%s' requires Rust library", path)
}

// GetConnectionsRust 获取网络连接信息（Rust 未启用版本）
func GetConnectionsRust() ([]Connection, error) {
	return nil, errors.New("Rust module not enabled: connection info requires Rust library")
}