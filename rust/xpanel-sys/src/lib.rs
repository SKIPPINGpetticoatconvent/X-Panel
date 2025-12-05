use std::fs;
use std::io;

/// 错误信息结构体
#[repr(C)]
pub struct SystemStats {
    pub tcp_count: i32,
    pub udp_count: i32,
    pub memory_used: u64,
    pub memory_total: u64,
    pub cpu_usage: f32,
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