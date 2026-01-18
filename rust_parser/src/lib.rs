//! X-Panel 流量统计解析器
//!
//! 高性能的 Rust 实现，用于解析 Xray Stats API 返回的统计名称。
//! 通过 FFI 暴露 C ABI 接口供 Go 调用。

use lazy_static::lazy_static;
use libc::{c_char, c_int, c_longlong};
use regex::Regex;
use std::ffi::{CStr, CString};
use std::ptr;

lazy_static! {
    /// 匹配 inbound/outbound 流量统计
    /// 格式: inbound>>>tag>>>traffic>>>downlink 或 outbound>>>tag>>>traffic>>>uplink
    static ref TRAFFIC_REGEX: Regex = Regex::new(
        r"^(inbound|outbound)>>>([^>]+)>>>traffic>>>(downlink|uplink)$"
    ).unwrap();

    /// 匹配用户流量统计
    /// 格式: user>>>email>>>traffic>>>downlink 或 user>>>email>>>traffic>>>uplink
    static ref CLIENT_TRAFFIC_REGEX: Regex = Regex::new(
        r"^user>>>([^>]+)>>>traffic>>>(downlink|uplink)$"
    ).unwrap();
}

/// 流量解析结果类型枚举
#[repr(C)]
pub enum TrafficType {
    /// 未匹配
    None = 0,
    /// Inbound 流量
    Inbound = 1,
    /// Outbound 流量
    Outbound = 2,
    /// 用户（客户端）流量
    Client = 3,
}

/// Inbound/Outbound 流量解析结果
#[repr(C)]
pub struct TrafficResult {
    /// 流量类型 (Inbound/Outbound/None)
    pub traffic_type: TrafficType,
    /// 标签名称 (需要调用 free_string 释放)
    pub tag: *mut c_char,
    /// 是否是下行流量
    pub is_downlink: c_int,
}

/// 用户流量解析结果
#[repr(C)]
pub struct ClientTrafficResult {
    /// 是否解析成功
    pub success: c_int,
    /// 用户 email (需要调用 free_string 释放)
    pub email: *mut c_char,
    /// 是否是下行流量
    pub is_downlink: c_int,
}

/// 批量解析结果中的单个流量条目
#[repr(C)]
pub struct TrafficEntry {
    /// 流量类型
    pub traffic_type: TrafficType,
    /// 标签或 email
    pub identifier: *mut c_char,
    /// 是否是下行流量
    pub is_downlink: c_int,
    /// 流量值
    pub value: c_longlong,
}

/// 批量解析结果
#[repr(C)]
pub struct BatchParseResult {
    /// Inbound/Outbound 流量条目数组
    pub traffic_entries: *mut TrafficEntry,
    /// Inbound/Outbound 流量条目数量
    pub traffic_count: c_int,
    /// 用户流量条目数组
    pub client_entries: *mut TrafficEntry,
    /// 用户流量条目数量
    pub client_count: c_int,
}

/// 解析单个流量统计名称 (inbound/outbound)
///
/// # Safety
/// - `name` 必须是有效的 C 字符串指针
/// - 调用者需要使用 `free_string` 释放返回的 `tag` 字段
#[no_mangle]
pub unsafe extern "C" fn parse_traffic_stat(name: *const c_char) -> TrafficResult {
    if name.is_null() {
        return TrafficResult {
            traffic_type: TrafficType::None,
            tag: ptr::null_mut(),
            is_downlink: 0,
        };
    }

    let name_str = match CStr::from_ptr(name).to_str() {
        Ok(s) => s,
        Err(_) => {
            return TrafficResult {
                traffic_type: TrafficType::None,
                tag: ptr::null_mut(),
                is_downlink: 0,
            }
        }
    };

    if let Some(caps) = TRAFFIC_REGEX.captures(name_str) {
        let direction = caps.get(1).map(|m| m.as_str()).unwrap_or("");
        let tag = caps.get(2).map(|m| m.as_str()).unwrap_or("");
        let link_type = caps.get(3).map(|m| m.as_str()).unwrap_or("");

        // 跳过 api 标签
        if tag == "api" {
            return TrafficResult {
                traffic_type: TrafficType::None,
                tag: ptr::null_mut(),
                is_downlink: 0,
            };
        }

        let traffic_type = if direction == "inbound" {
            TrafficType::Inbound
        } else {
            TrafficType::Outbound
        };

        let tag_cstring = CString::new(tag).unwrap_or_default();
        let is_downlink = if link_type == "downlink" { 1 } else { 0 };

        return TrafficResult {
            traffic_type,
            tag: tag_cstring.into_raw(),
            is_downlink,
        };
    }

    TrafficResult {
        traffic_type: TrafficType::None,
        tag: ptr::null_mut(),
        is_downlink: 0,
    }
}

/// 解析单个用户流量统计名称
///
/// # Safety
/// - `name` 必须是有效的 C 字符串指针
/// - 调用者需要使用 `free_string` 释放返回的 `email` 字段
#[no_mangle]
pub unsafe extern "C" fn parse_client_traffic_stat(name: *const c_char) -> ClientTrafficResult {
    if name.is_null() {
        return ClientTrafficResult {
            success: 0,
            email: ptr::null_mut(),
            is_downlink: 0,
        };
    }

    let name_str = match CStr::from_ptr(name).to_str() {
        Ok(s) => s,
        Err(_) => {
            return ClientTrafficResult {
                success: 0,
                email: ptr::null_mut(),
                is_downlink: 0,
            }
        }
    };

    if let Some(caps) = CLIENT_TRAFFIC_REGEX.captures(name_str) {
        let email = caps.get(1).map(|m| m.as_str()).unwrap_or("");
        let link_type = caps.get(2).map(|m| m.as_str()).unwrap_or("");

        let email_cstring = CString::new(email).unwrap_or_default();
        let is_downlink = if link_type == "downlink" { 1 } else { 0 };

        return ClientTrafficResult {
            success: 1,
            email: email_cstring.into_raw(),
            is_downlink,
        };
    }

    ClientTrafficResult {
        success: 0,
        email: ptr::null_mut(),
        is_downlink: 0,
    }
}

/// 释放由 Rust 分配的 C 字符串
///
/// # Safety
/// - `s` 必须是由本库函数返回的指针，或者是 null
#[no_mangle]
pub unsafe extern "C" fn free_string(s: *mut c_char) {
    if !s.is_null() {
        drop(CString::from_raw(s));
    }
}

/// 释放 TrafficResult 中的内存
///
/// # Safety
/// - `result` 必须是由 `parse_traffic_stat` 返回的结构体
#[no_mangle]
pub unsafe extern "C" fn free_traffic_result(result: TrafficResult) {
    free_string(result.tag);
}

/// 释放 ClientTrafficResult 中的内存
///
/// # Safety
/// - `result` 必须是由 `parse_client_traffic_stat` 返回的结构体
#[no_mangle]
pub unsafe extern "C" fn free_client_traffic_result(result: ClientTrafficResult) {
    free_string(result.email);
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::ffi::CString;

    #[test]
    fn test_parse_inbound_traffic() {
        let name = CString::new("inbound>>>vmess-tcp>>>traffic>>>downlink").unwrap();
        unsafe {
            let result = parse_traffic_stat(name.as_ptr());
            assert!(matches!(result.traffic_type, TrafficType::Inbound));
            assert_eq!(result.is_downlink, 1);

            let tag = CStr::from_ptr(result.tag).to_str().unwrap();
            assert_eq!(tag, "vmess-tcp");

            free_traffic_result(result);
        }
    }

    #[test]
    fn test_parse_outbound_traffic() {
        let name = CString::new("outbound>>>direct>>>traffic>>>uplink").unwrap();
        unsafe {
            let result = parse_traffic_stat(name.as_ptr());
            assert!(matches!(result.traffic_type, TrafficType::Outbound));
            assert_eq!(result.is_downlink, 0);

            let tag = CStr::from_ptr(result.tag).to_str().unwrap();
            assert_eq!(tag, "direct");

            free_traffic_result(result);
        }
    }

    #[test]
    fn test_skip_api_tag() {
        let name = CString::new("inbound>>>api>>>traffic>>>downlink").unwrap();
        unsafe {
            let result = parse_traffic_stat(name.as_ptr());
            assert!(matches!(result.traffic_type, TrafficType::None));
            assert!(result.tag.is_null());
        }
    }

    #[test]
    fn test_parse_client_traffic() {
        let name = CString::new("user>>>user@example.com>>>traffic>>>downlink").unwrap();
        unsafe {
            let result = parse_client_traffic_stat(name.as_ptr());
            assert_eq!(result.success, 1);
            assert_eq!(result.is_downlink, 1);

            let email = CStr::from_ptr(result.email).to_str().unwrap();
            assert_eq!(email, "user@example.com");

            free_client_traffic_result(result);
        }
    }

    #[test]
    fn test_invalid_format() {
        let name = CString::new("invalid>>>format").unwrap();
        unsafe {
            let result = parse_traffic_stat(name.as_ptr());
            assert!(matches!(result.traffic_type, TrafficType::None));

            let client_result = parse_client_traffic_stat(name.as_ptr());
            assert_eq!(client_result.success, 0);
        }
    }
}
