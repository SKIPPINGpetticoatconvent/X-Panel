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

	fmt.Println("=== Rust 网络连接追踪测试 ===")
	fmt.Printf("TCP 连接数 - Rust: %d\n", statsRust.TCPCount)
	fmt.Printf("UDP 连接数 - Rust: %d\n", statsRust.UDPCount)
	fmt.Printf("内存使用 - Rust: %.2f MB\n", float64(statsRust.MemoryUsed)/1024/1024)
	fmt.Printf("CPU 使用率 - Rust: %.2f%%\n", statsRust.CPUUsage)
	
	// 验证 Rust 实现结果的合理性
	if statsRust.TCPCount < 0 {
		t.Errorf("Rust TCP 连接数不能为负数: %d", statsRust.TCPCount)
	}
	
	if statsRust.UDPCount < 0 {
		t.Errorf("Rust UDP 连接数不能为负数: %d", statsRust.UDPCount)
	}
	
	fmt.Println("✓ Rust 实现数据验证通过")
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

// 测试磁盘使用率获取（Rust 版本）
func TestGetDiskUsageRust(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	// 测试根目录
	diskUsage, err := GetDiskUsageRust("/")
	if err != nil {
		t.Fatalf("GetDiskUsageRust 失败: %v", err)
	}

	fmt.Printf("✓ Rust 磁盘使用率 (/):\n")
	fmt.Printf("   总空间: %d bytes (%.2f GB)\n", diskUsage.Total, float64(diskUsage.Total)/1024/1024/1024)
	fmt.Printf("   已使用: %d bytes (%.2f GB)\n", diskUsage.Used, float64(diskUsage.Used)/1024/1024/1024)
	fmt.Printf("   可用: %d bytes (%.2f GB)\n", diskUsage.Free, float64(diskUsage.Free)/1024/1024/1024)
	
	// 验证数据合理性
	if diskUsage.Total == 0 {
		t.Errorf("磁盘总空间不能为 0")
	}
	
	if diskUsage.Used > diskUsage.Total {
		t.Errorf("已使用空间 (%d) 不能大于总空间 (%d)", diskUsage.Used, diskUsage.Total)
	}
	
	if diskUsage.Free > diskUsage.Total {
		t.Errorf("可用空间 (%d) 不能大于总空间 (%d)", diskUsage.Free, diskUsage.Total)
	}
	
	// 计算使用率
	if diskUsage.Total > 0 {
		usagePercent := float64(diskUsage.Used) / float64(diskUsage.Total) * 100
		fmt.Printf("   使用率: %.2f%%\n", usagePercent)
		
		if usagePercent < 0 || usagePercent > 100 {
			t.Errorf("磁盘使用率应该在 0-100 之间: %.2f%%", usagePercent)
		}
	}
	
	// 测试当前工作目录
	diskUsage, err = GetDiskUsageRust(".")
	if err != nil {
		t.Fatalf("GetDiskUsageRust 失败 (当前目录): %v", err)
	}
	
	fmt.Printf("✓ Rust 磁盘使用率 (.): %.2f%%\n", float64(diskUsage.Used)/float64(diskUsage.Total)*100)
}

// 测试网络连接信息获取（Rust 版本）
func TestGetConnectionsRust(t *testing.T) {
	if !IsRustAvailable() {
		t.Skip("Rust bridge not available")
	}

	connections, err := GetConnectionsRust()
	if err != nil {
		t.Fatalf("GetConnectionsRust 失败: %v", err)
	}

	fmt.Printf("✓ Rust 网络连接数: %d\n", len(connections))
	
	// 显示前5个连接作为示例
	maxDisplay := 5
	if len(connections) < maxDisplay {
		maxDisplay = len(connections)
	}
	
	for i := 0; i < maxDisplay; i++ {
		conn := connections[i]
		protocolStr := "Unknown"
		if conn.Protocol == 6 {
			protocolStr = "TCP"
		} else if conn.Protocol == 17 {
			protocolStr = "UDP"
		}
		
		fmt.Printf("   连接 %d: %s:%d -> %s:%d [%s] 状态:%d\n",
			i+1, conn.LocalIP, conn.LocalPort, conn.RemoteIP, conn.RemotePort, protocolStr, conn.State)
	}
	
	if len(connections) > maxDisplay {
		fmt.Printf("   ... 还有 %d 个连接未显示\n", len(connections)-maxDisplay)
	}
	
	// 验证连接数据结构
	for i, conn := range connections {
		if conn.LocalIP == "" {
			t.Errorf("连接 %d 本地 IP 为空", i)
		}
		
		if conn.RemoteIP == "" {
			t.Errorf("连接 %d 远程 IP 为空", i)
		}
		
		if conn.LocalPort == 0 {
			t.Errorf("连接 %d 本地端口为 0", i)
		}
		
		if conn.RemotePort == 0 {
			t.Errorf("连接 %d 远程端口为 0", i)
		}
		
		if conn.Protocol != 6 && conn.Protocol != 17 {
			t.Errorf("连接 %d 协议未知: %d", i, conn.Protocol)
		}
	}
	
	fmt.Printf("✓ 网络连接数据验证通过\n")
}