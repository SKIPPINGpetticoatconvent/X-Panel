/**
 * X-Panel Traffic Parser - C Header File
 *
 * 高性能 Rust 流量统计解析器的 C 接口定义。
 * 用于 Go CGO 集成。
 */

#ifndef XPANEL_TRAFFIC_PARSER_H
#define XPANEL_TRAFFIC_PARSER_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/**
 * 流量类型枚举
 */
typedef enum {
    TRAFFIC_TYPE_NONE = 0,
    TRAFFIC_TYPE_INBOUND = 1,
    TRAFFIC_TYPE_OUTBOUND = 2,
    TRAFFIC_TYPE_CLIENT = 3
} TrafficType;

/**
 * Inbound/Outbound 流量解析结果
 */
typedef struct {
    TrafficType traffic_type;
    char* tag;          /* 需要调用 free_string 释放 */
    int is_downlink;
} TrafficResult;

/**
 * 用户流量解析结果
 */
typedef struct {
    int success;
    char* email;        /* 需要调用 free_string 释放 */
    int is_downlink;
} ClientTrafficResult;

/**
 * 解析单个流量统计名称 (inbound/outbound)
 *
 * @param name 统计名称 C 字符串
 * @return TrafficResult 结构体，调用者需释放 tag 字段
 */
TrafficResult parse_traffic_stat(const char* name);

/**
 * 解析单个用户流量统计名称
 *
 * @param name 统计名称 C 字符串
 * @return ClientTrafficResult 结构体，调用者需释放 email 字段
 */
ClientTrafficResult parse_client_traffic_stat(const char* name);

/**
 * 释放由 Rust 分配的 C 字符串
 *
 * @param s 要释放的字符串指针
 */
void free_string(char* s);

/**
 * 释放 TrafficResult 中的内存
 *
 * @param result 要释放的结构体
 */
void free_traffic_result(TrafficResult result);

/**
 * 释放 ClientTrafficResult 中的内存
 *
 * @param result 要释放的结构体
 */
void free_client_traffic_result(ClientTrafficResult result);

#ifdef __cplusplus
}
#endif

#endif /* XPANEL_TRAFFIC_PARSER_H */
