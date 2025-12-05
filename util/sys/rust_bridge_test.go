//go:build linux && rust
// +build linux,rust

package sys

import (
	"fmt"
	"testing"
	"time"
)

func TestRustBridgeAvailable(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	fmt.Println("✓ Rust 动态库可用")
}

func TestGetTCPCountRust(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	count, err := GetTCPCountRust()
	if err != nil {
		t.Fatalf("GetTCPCountRust 失败: %v", err)
	}

	fmt.Printf("✓ Rust TCP 连接数: %d\n", count)
	
	// 验证结果合理性
	if count < 0 {
		t.Errorf("TCP 连接数不能为负数: %d", count)
	}
}

func TestGetUDPCountRust(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	count, err := GetUDPCountRust()
	if err != nil {
		t.Fatalf("GetUDPCountRust 失败: %v", err)
	}

	fmt.Printf("✓ Rust UDP 连接数: %d\n", count)
	
	// 验证结果合理性
	if count < 0 {
		t.Errorf("UDP 连接数不能为负数: %d", count)
	}
}

func TestGetMemoryStatsRust(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	used, err := GetMemoryUsedRust()
	if err != nil {
		t.Fatalf("GetMemoryUsedRust 失败: %v", err)
	}

	total, err := GetMemoryTotalRust()
	if err != nil {
		t.Fatalf("GetMemoryTotalRust 失败: %v", err)
	}

	fmt.Printf("✓ Rust 内存使用: %d bytes (%.2f MB)\n", used, float64(used)/1024/1024)
	fmt.Printf("✓ Rust 内存总计: %d bytes (%.2f GB)\n", total, float64(total)/1024/1024/1024)
	
	// 验证结果合理性
	if used > total && total > 0 {
		t.Errorf("使用内存 (%d) 不能大于总内存 (%d)", used, total)
	}
	
	if total == 0 {
		t.Errorf("总内存不能为 0")
	}
}

func TestGetCPUUsageRust(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	usage, err := GetCPUUsageRust()
	if err != nil {
		t.Fatalf("GetCPUUsageRust 失败: %v", err)
	}

	fmt.Printf("✓ Rust CPU 使用率: %.2f%%\n", usage)
	
	// 验证结果合理性
	if usage < 0 || usage > 100 {
		t.Errorf("CPU 使用率应该在 0-100 之间: %.2f%%", usage)
	}
}

func TestGetSystemStatsRust(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	stats, err := GetSystemStatsRust()
	if err != nil {
		t.Fatalf("GetSystemStatsRust 失败: %v", err)
	}

	fmt.Println("✓ Rust 完整系统统计信息:")
	fmt.Printf("   TCP 连接数: %d\n", stats.TCPCount)
	fmt.Printf("   UDP 连接数: %d\n", stats.UDPCount)
	fmt.Printf("   内存使用: %d bytes (%.2f MB)\n", stats.MemoryUsed, float64(stats.MemoryUsed)/1024/1024)
	fmt.Printf("   内存总计: %d bytes (%.2f GB)\n", stats.MemoryTotal, float64(stats.MemoryTotal)/1024/1024/1024)
	fmt.Printf("   CPU 使用率: %.2f%%\n", stats.CPUUsage)
	
	// 计算内存使用百分比
	if stats.MemoryTotal > 0 {
		memPercent := float64(stats.MemoryUsed) / float64(stats.MemoryTotal) * 100
		fmt.Printf("   内存使用率: %.2f%%\n", memPercent)
	}
	
	// 验证数据结构合理性
	if stats.TCPCount < 0 {
		t.Errorf("TCP 连接数不能为负数: %d", stats.TCPCount)
	}
	
	if stats.UDPCount < 0 {
		t.Errorf("UDP 连接数不能为负数: %d", stats.UDPCount)
	}
	
	if stats.CPUUsage < 0 || stats.CPUUsage > 100 {
		t.Errorf("CPU 使用率应该在 0-100 之间: %.2f%%", stats.CPUUsage)
	}
}

func TestGetMemoryUsagePercentRust(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	percent, err := GetMemoryUsagePercentRust()
	if err != nil {
		t.Fatalf("GetMemoryUsagePercentRust 失败: %v", err)
	}

	fmt.Printf("✓ Rust 内存使用率: %.2f%%\n", percent)
	
	// 验证结果合理性
	if percent < 0 || percent > 100 {
		t.Errorf("内存使用率应该在 0-100 之间: %.2f%%", percent)
	}
}

// 对比测试：Rust vs Go 原生实现
func TestCompareRustAndGo(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	// 获取 Rust 版本结果
	statsRust, err := GetSystemStatsRust()
	if err != nil {
		t.Fatalf("获取 Rust 统计信息失败: %v", err)
	}

	// 获取 Go 原生版本结果（如果可用）
	tcpGo, _ := GetTCPCount()
	udpGo, _ := GetUDPCount()

	fmt.Println("=== Rust vs Go 原生实现对比 ===")
	fmt.Printf("TCP 连接数 - Rust: %d, Go: %d\n", statsRust.TCPCount, tcpGo)
	fmt.Printf("UDP 连接数 - Rust: %d, Go: %d\n", statsRust.UDPCount, udpGo)
	fmt.Printf("内存使用 - Rust: %.2f MB\n", float64(statsRust.MemoryUsed)/1024/1024)
	fmt.Printf("CPU 使用率 - Rust: %.2f%%\n", statsRust.CPUUsage)
	
	// 简单验证差异是否合理（允许一定误差）
	tcpDiff := abs(int(statsRust.TCPCount) - tcpGo)
	udpDiff := abs(int(statsRust.UDPCount) - udpGo)
	
	fmt.Printf("TCP 差异: %d (%.1f%%)\n", tcpDiff, float64(tcpDiff)/float64(max(tcpGo, 1))*100)
	fmt.Printf("UDP 差异: %d (%.1f%%)\n", udpDiff, float64(udpDiff)/float64(max(udpGo, 1))*100)
}

// 性能基准测试
func BenchmarkRustSystemStats(b *testing.B) {
	if !IsRustAvailable() {
		b.Skip("Rust bridge not available")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetSystemStatsRust()
		if err != nil {
			b.Fatalf("基准测试失败: %v", err)
		}
	}
}

// 辅助函数
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// 交互式测试函数
func RunInteractiveRustTest() {
	fmt.Println("=== Rust 动态库交互式测试 ===")
	
	if !IsRustAvailable() {
		fmt.Println("❌ Rust 动态库不可用")
		return
	}
	
	fmt.Println("✓ Rust 动态库可用")
	
	// 获取统计信息
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
	
	// 连续监控一段时间
	fmt.Println("\n连续监控 5 秒...")
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		usage, _ := GetCPUUsageRust()
		fmt.Printf("  第 %d 秒 - CPU: %.2f%%\n", i+1, usage)
	}
	
	fmt.Println("✓ 测试完成")
}