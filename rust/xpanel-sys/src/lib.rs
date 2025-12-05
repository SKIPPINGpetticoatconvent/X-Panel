use std::fs;
use std::io;
use std::os::raw::c_char;
use std::ptr;
use std::ffi::CStr;
use std::process::Command;

/// 错误信息结构体
#[repr(C)]
pub struct SystemStats {
    pub tcp_count: i32,
    pub udp_count: i32,
    pub memory_used: u64,
    pub memory_total: u64,
    pub cpu_usage: f32,
}

/// 磁盘使用率统计结构体
#[repr(C)]
pub struct DiskStats {
    pub total: u64,
    pub used: u64,
    pub free: u64,
}

/// 网络连接信息结构体
#[repr(C)]
pub struct ConnectionInfo {
    pub local_ip: [u8; 16],
    pub local_port: u16,
    pub remote_ip: [u8; 16],
    pub remote_port: u16,
    pub state: u8,
    pub protocol: u8,
}

/// 连接列表结构体
#[repr(C)]
pub struct ConnectionList {
    pub data: *mut ConnectionInfo,
    pub len: usize,
    pub capacity: usize,
}

/// 计算 TCP 连接数
fn count_tcp_connections() -> io::Result<i32> {
    let mut count = 0;
    
    // 尝试读取 IPv4 TCP 连接
    if let Ok(content) = fs::read_to_string("/proc/net/tcp") {
        for line in content.lines() {
            // 跳过标题行
            if !line.starts_with("sl") {
                count += 1;
            }
        }
    }
    
    // 尝试读取 IPv6 TCP 连接
    if let Ok(content) = fs::read_to_string("/proc/net/tcp6") {
        for line in content.lines() {
            // 跳过标题行
            if !line.starts_with("sl") {
                count += 1;
            }
        }
    }
    
    Ok(count)
}

/// 计算 UDP 连接数
fn count_udp_connections() -> io::Result<i32> {
    let mut count = 0;
    
    // 尝试读取 IPv4 UDP 连接
    if let Ok(content) = fs::read_to_string("/proc/net/udp") {
        for line in content.lines() {
            // 跳过标题行
            if !line.starts_with("sl") {
                count += 1;
            }
        }
    }
    
    // 尝试读取 IPv6 UDP 连接
    if let Ok(content) = fs::read_to_string("/proc/net/udp6") {
        for line in content.lines() {
            // 跳过标题行
            if !line.starts_with("sl") {
                count += 1;
            }
        }
    }
    
    Ok(count)
}

/// 获取系统内存使用情况
fn get_memory_stats() -> (u64, u64) {
    let mut used = 0u64;
    let mut total = 0u64;
    
    // 读取 /proc/meminfo
    if let Ok(content) = fs::read_to_string("/proc/meminfo") {
        for line in content.lines() {
            if line.starts_with("MemTotal:") {
                // 格式：MemTotal:       8192000 kB
                if let Some(kb_str) = line.split_whitespace().nth(1) {
                    if let Ok(kb) = kb_str.parse::<u64>() {
                        total = kb * 1024; // 转换为字节
                    }
                }
            } else if line.starts_with("MemAvailable:") {
                // 格式：MemAvailable:    4096000 kB
                if let Some(kb_str) = line.split_whitespace().nth(1) {
                    if let Ok(kb) = kb_str.parse::<u64>() {
                        let available = kb * 1024; // 转换为字节
                        if total > 0 {
                            used = total - available;
                        }
                    }
                }
            } else if line.starts_with("MemFree:") {
                // 如果没有 MemAvailable，使用 MemFree
                if total > 0 && used == 0 {
                    if let Some(kb_str) = line.split_whitespace().nth(1) {
                        if let Ok(kb) = kb_str.parse::<u64>() {
                            let free = kb * 1024;
                            used = total - free;
                        }
                    }
                }
            }
        }
    }
    
    (used, total)
}

/// 获取 CPU 使用率
fn calculate_cpu_usage() -> f32 {
    // 读取 /proc/stat 获取 CPU 使用率
    if let Ok(content) = fs::read_to_string("/proc/stat") {
        for line in content.lines() {
            if line.starts_with("cpu ") {
                let parts: Vec<&str> = line.split_whitespace().collect();
                if parts.len() >= 5 {
                    // cpu  user nice system idle iowait irq softirq steal guest guest_nice
                    if let (Ok(user), Ok(nice), Ok(system), Ok(idle)) = (
                        parts[1].parse::<u64>(),
                        parts[2].parse::<u64>(),
                        parts[3].parse::<u64>(),
                        parts[4].parse::<u64>(),
                    ) {
                        let total = user + nice + system + idle;
                        if total > 0 {
                            let used = user + nice + system;
                            return (used as f32 / total as f32) * 100.0;
                        }
                    }
                }
                break;
            }
        }
    }
    
    0.0
}

/// 导出给 C 调用的函数：获取 TCP 连接数
#[no_mangle]
pub extern "C" fn get_tcp_count() -> i32 {
    match count_tcp_connections() {
        Ok(count) => count,
        Err(_) => -1, // 错误码
    }
}

/// 导出给 C 调用的函数：获取 UDP 连接数
#[no_mangle]
pub extern "C" fn get_udp_count() -> i32 {
    match count_udp_connections() {
        Ok(count) => count,
        Err(_) => -1, // 错误码
    }
}

/// 导出给 C 调用的函数：获取内存使用字节数
#[no_mangle]
pub extern "C" fn get_memory_used() -> u64 {
    let (used, _) = get_memory_stats();
    used
}

/// 导出给 C 调用的函数：获取内存总字节数
#[no_mangle]
pub extern "C" fn get_memory_total() -> u64 {
    let (_, total) = get_memory_stats();
    total
}

/// 导出给 C 调用的函数：获取 CPU 使用率（百分比）
#[no_mangle]
pub extern "C" fn get_cpu_usage() -> f32 {
    calculate_cpu_usage()
}

/// 导出给 C 调用的函数：获取完整系统统计信息
#[no_mangle]
pub extern "C" fn get_system_stats(stats: *mut SystemStats) {
    if stats.is_null() {
        return;
    }
    
    unsafe {
        let stats_ref = &mut *stats;
        
        stats_ref.tcp_count = get_tcp_count();
        stats_ref.udp_count = get_udp_count();
        stats_ref.memory_used = get_memory_used();
        stats_ref.memory_total = get_memory_total();
        stats_ref.cpu_usage = get_cpu_usage();
    }
}

/// 解析 IP 地址字符串为字节数组
fn parse_ip_address(ip_str: &str) -> [u8; 16] {
    let mut result = [0u8; 16];
    
    if ip_str.contains(':') {
        // IPv6 地址
        if let Ok(ip) = ip_str.parse::<std::net::Ipv6Addr>() {
            let octets = ip.octets();
            result.copy_from_slice(&octets);
        }
    } else {
        // IPv4 地址，转换为 IPv6 映射格式 ::ffff:x.x.x.x
        if let Ok(ip) = ip_str.parse::<std::net::Ipv4Addr>() {
            let octets = ip.octets();
            // IPv4 映射到 IPv6 的格式
            result[10] = 0xff;
            result[11] = 0xff;
            result[12] = octets[0];
            result[13] = octets[1];
            result[14] = octets[2];
            result[15] = octets[3];
        }
    }
    
    result
}

/// 解析端口号
fn parse_port(port_str: &str) -> u16 {
    u16::from_str_radix(port_str, 16).unwrap_or(0)
}

/// 获取磁盘使用率
#[no_mangle]
pub extern "C" fn get_disk_usage(path: *const c_char) -> DiskStats {
    let mut stats = DiskStats {
        total: 0,
        used: 0,
        free: 0,
    };
    
    if path.is_null() {
        return stats;
    }
    
    // 安全地获取路径字符串
    let path_str = match unsafe { CStr::from_ptr(path).to_str() } {
        Ok(s) => s,
        Err(_) => return stats,
    };
    
    // 使用 df 命令获取磁盘使用情况
    match Command::new("df")
        .arg("-k")
        .arg(path_str)
        .output()
    {
        Ok(output) => {
            if let Ok(df_output) = String::from_utf8(output.stdout) {
                let lines: Vec<&str> = df_output.lines().collect();
                if lines.len() >= 2 {
                    let fields: Vec<&str> = lines[1].split_whitespace().collect();
                    if fields.len() >= 4 {
                        if let (Ok(total_kb), Ok(used_kb), Ok(free_kb)) = (
                            fields[1].parse::<u64>(),
                            fields[2].parse::<u64>(),
                            fields[3].parse::<u64>(),
                        ) {
                            stats.total = total_kb * 1024; // 转换为字节
                            stats.used = used_kb * 1024;
                            stats.free = free_kb * 1024;
                        }
                    }
                }
            }
        }
        Err(_) => {},
    }
    
    stats
}

/// 解析单个 /proc/net 文件并返回连接信息
fn parse_net_file(file_path: &str, protocol: u8) -> Vec<ConnectionInfo> {
    let mut connections = Vec::new();
    
    if let Ok(content) = fs::read_to_string(file_path) {
        for (line_num, line) in content.lines().enumerate() {
            if line_num == 0 {
                continue; // 跳过标题行
            }
            
            let parts: Vec<&str> = line.split_whitespace().collect();
            if parts.len() < 4 {
                continue;
            }
            
            // 解析本地地址和端口
            let local_addr = parts[1];
            let remote_addr = parts[2];
            
            if let Some((local_ip, local_port_str)) = local_addr.rsplit_once(':') {
                if let Some((remote_ip, remote_port_str)) = remote_addr.rsplit_once(':') {
                    let local_port = parse_port(local_port_str);
                    let remote_port = parse_port(remote_port_str);
                    let state = if parts.len() > 3 {
                        u8::from_str_radix(parts[3], 16).unwrap_or(0)
                    } else {
                        0
                    };
                    
                    let mut conn_info = ConnectionInfo {
                        local_ip: parse_ip_address(local_ip),
                        local_port,
                        remote_ip: parse_ip_address(remote_ip),
                        remote_port,
                        state,
                        protocol,
                    };
                    
                    // IPv4 地址转换为 IPv6 映射格式
                    if !local_ip.contains(':') {
                        conn_info.local_ip = parse_ip_address(local_ip);
                    }
                    if !remote_ip.contains(':') {
                        conn_info.remote_ip = parse_ip_address(remote_ip);
                    }
                    
                    connections.push(conn_info);
                }
            }
        }
    }
    
    connections
}

/// 获取所有网络连接信息
#[no_mangle]
pub extern "C" fn get_connections() -> ConnectionList {
    let mut all_connections = Vec::new();
    
    // 解析 TCP 连接
    all_connections.extend(parse_net_file("/proc/net/tcp", 6)); // TCP protocol number: 6
    all_connections.extend(parse_net_file("/proc/net/tcp6", 6));
    
    // 解析 UDP 连接
    all_connections.extend(parse_net_file("/proc/net/udp", 17)); // UDP protocol number: 17
    all_connections.extend(parse_net_file("/proc/net/udp6", 17));
    
    let len = all_connections.len();
    if len == 0 {
        return ConnectionList {
            data: ptr::null_mut(),
            len: 0,
            capacity: 0,
        };
    }
    
    // 分配内存并复制数据
    let mut connection_list = ConnectionList {
        data: ptr::null_mut(),
        len,
        capacity: len,
    };
    
    // 使用 Box::into_raw 分配内存
    let mut boxed_slice = all_connections.into_boxed_slice();
    connection_list.data = boxed_slice.as_mut_ptr();
    std::mem::forget(boxed_slice);
    
    connection_list
}

/// 释放连接列表占用的内存
#[no_mangle]
pub extern "C" fn free_connection_list(list: ConnectionList) {
    if !list.data.is_null() && list.len > 0 {
        // 重新构造 Box 并自动释放
        let _ = unsafe { Box::from_raw(std::slice::from_raw_parts_mut(list.data, list.len)) };
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tcp_count() {
        let count = get_tcp_count();
        assert!(count >= 0);
    }

    #[test]
    fn test_udp_count() {
        let count = get_udp_count();
        assert!(count >= 0);
    }

    #[test]
    fn test_memory_stats() {
        let used = get_memory_used();
        let total = get_memory_total();
        assert!(used <= total);
        assert!(total > 0);
    }

    #[test]
    fn test_cpu_usage() {
        let usage = get_cpu_usage();
        assert!(usage >= 0.0 && usage <= 100.0);
    }
}