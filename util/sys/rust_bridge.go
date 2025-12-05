//go:build linux && rust
// +build linux,rust

package sys

/*
#cgo LDFLAGS: -L../../rust/xpanel-sys/target/debug -lxpanel_sys -lm

#include <stdint.h>
#include <stdlib.h>

// 系统统计信息结构体
typedef struct {
    int32_t tcp_count;
    int32_t udp_count;
    uint64_t memory_used;
    uint64_t memory_total;
    float cpu_usage;
} SystemStats;

// 磁盘使用率统计结构体
typedef struct {
    uint64_t total;
    uint64_t used;
    uint64_t free;
} DiskStats;

// 连接信息结构体
typedef struct {
    uint8_t local_ip[16];
    uint16_t local_port;
    uint8_t remote_ip[16];
    uint16_t remote_port;
    uint8_t state;
    uint8_t protocol;
} ConnectionInfo;

// 连接列表结构体
typedef struct {
    ConnectionInfo* data;
    size_t len;
    size_t capacity;
} ConnectionList;

// Rust 函数声明
extern int32_t get_tcp_count();
extern int32_t get_udp_count();
extern uint64_t get_memory_used();
extern uint64_t get_memory_total();
extern float get_cpu_usage();
extern void get_system_stats(SystemStats* stats);
extern DiskStats get_disk_usage(char* path);
extern ConnectionList get_connections();
extern void free_connection_list(ConnectionList list);
*/
import "C"

import (
	"fmt"
	"net"
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

// DiskUsage Go 结构体，对应 Rust 的 DiskStats
type DiskUsage struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	Free  uint64 `json:"free"`
}

// Connection Go 结构体，对应 Rust 的 ConnectionInfo
type Connection struct {
	LocalIP    string `json:"local_ip"`
	LocalPort  uint16 `json:"local_port"`
	RemoteIP   string `json:"remote_ip"`
	RemotePort uint16 `json:"remote_port"`
	State      uint8  `json:"state"`
	Protocol   uint8  `json:"protocol"`
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

// IsRustAvailable 检查 Rust 动态库是否可用
func IsRustAvailable() bool {
	// 尝试调用一个简单的函数来测试库是否可用
	_, err := GetTCPCountRust()
	return err == nil
}

// GetMemoryUsagePercent 计算内存使用百分比（Rust 版本）
func GetMemoryUsagePercentRust() (float64, error) {
	stats, err := GetSystemStatsRust()
	if err != nil {
		return 0, err
	}

	if stats.MemoryTotal == 0 {
		return 0, fmt.Errorf("total memory is zero")
	}

	return float64(stats.MemoryUsed) / float64(stats.MemoryTotal) * 100.0, nil
}

// GetDiskUsageRust 获取磁盘使用率（Rust 版本）
func GetDiskUsageRust(path string) (*DiskUsage, error) {
	// 将 Go string 转换为 C char*
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// 调用 Rust 函数
	cStats := C.get_disk_usage(cPath)

	// 转换为 Go 结构体
	diskUsage := &DiskUsage{
		Total: uint64(cStats.total),
		Used:  uint64(cStats.used),
		Free:  uint64(cStats.free),
	}

	return diskUsage, nil
}

// GetConnectionsRust 获取网络连接信息（Rust 版本）
func GetConnectionsRust() ([]Connection, error) {
	// 调用 Rust 函数
	cList := C.get_connections()

	// 检查是否为空
	if cList.data == nil || cList.len == 0 {
		// 即使没有连接也要调用 free 来清理
		if cList.data != nil {
			C.free_connection_list(cList)
		}
		return []Connection{}, nil
	}

	// 转换为 Go slice
	var connections []Connection
	header := (*[1 << 30]C.ConnectionInfo)(unsafe.Pointer(cList.data))[:cList.len:cList.len]
	
	for i := 0; i < int(cList.len); i++ {
		conn := header[i]
		
		// 转换本地IP地址
		localIPBytes := make([]byte, 16)
		for j := 0; j < 16; j++ {
			localIPBytes[j] = byte(conn.local_ip[j])
		}
		localIP := convertIPBytes(localIPBytes)
		
		remoteIPBytes := make([]byte, 16)
		for j := 0; j < 16; j++ {
			remoteIPBytes[j] = byte(conn.remote_ip[j])
		}
		remoteIP := convertIPBytes(remoteIPBytes)
		
		connection := Connection{
			LocalIP:    localIP,
			LocalPort:  uint16(conn.local_port),
			RemoteIP:   remoteIP,
			RemotePort: uint16(conn.remote_port),
			State:      uint8(conn.state),
			Protocol:   uint8(conn.protocol),
		}
		
		connections = append(connections, connection)
	}

	// 释放 Rust 分配的内存
	C.free_connection_list(cList)

	return connections, nil
}

// convertIPBytes 将 [16]byte 转换为 IP 字符串
func convertIPBytes(ipBytes []byte) string {
	// 检查是否为 IPv4 映射格式 (::ffff:x.x.x.x)
	if len(ipBytes) >= 16 && 
		ipBytes[0] == 0 && ipBytes[1] == 0 && 
		ipBytes[2] == 0 && ipBytes[3] == 0 &&
		ipBytes[4] == 0 && ipBytes[5] == 0 && 
		ipBytes[6] == 0 && ipBytes[7] == 0 &&
		ipBytes[8] == 0 && ipBytes[9] == 0 && 
		ipBytes[10] == 0xff && ipBytes[11] == 0xff {
		// IPv4 映射格式，提取最后4个字节
		ipv4 := net.IPv4(ipBytes[12], ipBytes[13], ipBytes[14], ipBytes[15])
		return ipv4.String()
	}
	
	// 检查是否为 IPv6
	if ipBytes[0] != 0 || ipBytes[1] != 0 {
		// 标准 IPv6 地址
		var ip [16]byte
		copy(ip[:], ipBytes)
		ipv6 := net.IP(ip[:])
		return ipv6.String()
	}
	
	// 可能是 IPv4 或者全零地址
	if ipBytes[12] != 0 || ipBytes[13] != 0 || ipBytes[14] != 0 || ipBytes[15] != 0 {
		// 看起来像是 IPv4
		ipv4 := net.IPv4(ipBytes[12], ipBytes[13], ipBytes[14], ipBytes[15])
		return ipv4.String()
	}
	
	// 检查是否有 IPv6 内容
	for i := 0; i < 16; i++ {
		if ipBytes[i] != 0 {
			var ip [16]byte
			copy(ip[:], ipBytes)
			ipv6 := net.IP(ip[:])
			return ipv6.String()
		}
	}
	
	// 全零地址
	return "0.0.0.0"
}