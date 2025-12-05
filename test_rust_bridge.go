//go:build linux && rust
// +build linux,rust

package main

/*
#cgo LDFLAGS: -L./rust/xpanel-sys/target/release -lxpanel_sys -lm

#include <stdint.h>
#include <stdlib.h>

// Rust 函数声明
extern int32_t get_tcp_count();
extern int32_t get_udp_count();
extern uint64_t get_memory_used();
extern uint64_t get_memory_total();
extern float get_cpu_usage();

// 系统统计信息结构体，需要与 Rust 端保持一致
typedef struct {
    int32_t tcp_count;
    int32_t udp_count;
    uint64_t memory_used;
    uint64_t memory_total;
    float cpu_usage;
} SystemStats;

extern void get_system_stats(SystemStats* stats);
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// SystemStats Go 结构体，对应 Rust 的 SystemStats
type SystemStats struct {
	TCPCount    int32   `json:"tcp_count"`
	UDPCount    int32   `json:"udp_count"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryTotal uint64  `json:"memory_total"`
	CPUUsage    float32 `json:"cpu_usage"`
}

// GetTCPCount 获取 TCP 连接数（Rust 版本）
func GetTCPCountRust() (int, error) {
	count := C.get_tcp_count()
	if count < 0 {
		return 0, fmt.Errorf("failed to get TCP count from Rust")
	}
	return int(count), nil
}

// GetUDPCount 获取 UDP 连接数（Rust 版本）
func GetUDPCountRust() (int, error) {
	count := C.get_udp_count()
	if count < 0 {
		return 0, fmt.Errorf("failed to get UDP count from Rust")
	}
	return int(count), nil
}

// GetMemoryUsed 获取已使用内存（Rust 版本）
func GetMemoryUsedRust() (uint64, error) {
	used := C.get_memory_used()
	return uint64(used), nil
}

// GetMemoryTotal 获取总内存（Rust 版本）
func GetMemoryTotalRust() (uint64, error) {
	total := C.get_memory_total()
	return uint64(total), nil
}

// GetCPUUsage 获取 CPU 使用率（Rust 版本）
func GetCPUUsageRust() (float32, error) {
	usage := C.get_cpu_usage()
	return float32(usage), nil
}

// GetSystemStats 获取完整系统统计信息（Rust 版本）
func GetSystemStatsRust() (*SystemStats, error) {
	// 分配 C 结构体内存
	cStats := (*C.SystemStats)(C.malloc(C.sizeof_SystemStats))
	defer C.free(unsafe.Pointer(cStats))

	// 调用 Rust 函数
	C.get_system_stats(cStats)

	// 转换为 Go 结构体
	stats := &SystemStats{
		TCPCount:    int32(cStats.tcp_count),
		UDPCount:    int32(cStats.udp_count),
		MemoryUsed:  uint64(cStats.memory_used),
		MemoryTotal: uint64(cStats.memory_total),
		CPUUsage:    float32(cStats.cpu_usage),
	}

	// 检查是否有错误
	if stats.TCPCount < 0 || stats.UDPCount < 0 {
		return stats, fmt.Errorf("error getting system stats from Rust")
	}

	return stats, nil
}

func main() {
	fmt.Println("=== Rust 动态库测试程序 ===")

	// 测试基本函数
	fmt.Println("\n1. 测试基本函数:")
	
	tcpCount, err := GetTCPCountRust()
	if err != nil {
		fmt.Printf("❌ 获取 TCP 连接数失败: %v\n", err)
		return
	}
	fmt.Printf("✓ TCP 连接数: %d\n", tcpCount)

	udpCount, err := GetUDPCountRust()
	if err != nil {
		fmt.Printf("❌ 获取 UDP 连接数失败: %v\n", err)
		return
	}
	fmt.Printf("✓ UDP 连接数: %d\n", udpCount)

	memoryUsed, err := GetMemoryUsedRust()
	if err != nil {
		fmt.Printf("❌ 获取内存使用失败: %v\n", err)
		return
	}
	fmt.Printf("✓ 内存使用: %d bytes (%.2f MB)\n", memoryUsed, float64(memoryUsed)/1024/1024)

	memoryTotal, err := GetMemoryTotalRust()
	if err != nil {
		fmt.Printf("❌ 获取内存总计失败: %v\n", err)
		return
	}
	fmt.Printf("✓ 内存总计: %d bytes (%.2f GB)\n", memoryTotal, float64(memoryTotal)/1024/1024/1024)

	cpuUsage, err := GetCPUUsageRust()
	if err != nil {
		fmt.Printf("❌ 获取 CPU 使用率失败: %v\n", err)
		return
	}
	fmt.Printf("✓ CPU 使用率: %.2f%%\n", cpuUsage)

	// 测试完整系统统计信息
	fmt.Println("\n2. 测试完整系统统计信息:")
	stats, err := GetSystemStatsRust()
	if err != nil {
		fmt.Printf("❌ 获取系统统计信息失败: %v\n", err)
		return
	}

	fmt.Printf("✓ 获取到系统统计信息:\n")
	fmt.Printf("  TCP 连接数: %d\n", stats.TCPCount)
	fmt.Printf("  UDP 连接数: %d\n", stats.UDPCount)
	fmt.Printf("  内存使用: %d bytes (%.2f MB)\n", stats.MemoryUsed, float64(stats.MemoryUsed)/1024/1024)
	fmt.Printf("  内存总计: %d bytes (%.2f GB)\n", stats.MemoryTotal, float64(stats.MemoryTotal)/1024/1024/1024)

	if stats.MemoryTotal > 0 {
		memPercent := float64(stats.MemoryUsed) / float64(stats.MemoryTotal) * 100
		fmt.Printf("  内存使用率: %.2f%%\n", memPercent)
	}

	fmt.Printf("  CPU 使用率: %.2f%%\n", stats.CPUUsage)

	fmt.Println("\n✓ 所有测试通过！Rust 动态库工作正常。")
}