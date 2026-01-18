//go:build cgo

package xray

/*
#cgo CFLAGS: -I${SRCDIR}/../rust_parser/include
#cgo LDFLAGS: -L${SRCDIR}/../rust_parser/target/release -lxpanel_traffic_parser -Wl,-rpath,${SRCDIR}/../rust_parser/target/release
#include "traffic_parser.h"
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"unsafe"

	"x-ui/logger"
)

var (
	// rustParserAvailable 标记 Rust 解析器是否可用
	rustParserAvailable = true
	rustParserOnce      sync.Once
)

// TrafficTypeNone 未匹配
const TrafficTypeNone = 0

// TrafficTypeInbound Inbound 流量
const TrafficTypeInbound = 1

// TrafficTypeOutbound Outbound 流量
const TrafficTypeOutbound = 2

// TrafficTypeClient 用户流量
const TrafficTypeClient = 3

// TrafficParseResult 流量解析结果
type TrafficParseResult struct {
	TrafficType int    // TrafficTypeNone, TrafficTypeInbound, TrafficTypeOutbound
	Tag         string // 标签名称
	IsDownlink  bool   // 是否下行流量
}

// ClientTrafficParseResult 用户流量解析结果
type ClientTrafficParseResult struct {
	Success    bool   // 是否解析成功
	Email      string // 用户 email
	IsDownlink bool   // 是否下行流量
}

// initRustParser 初始化 Rust 解析器（测试可用性）
func initRustParser() {
	rustParserOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Warning("Rust traffic parser unavailable, falling back to Go regex")
				rustParserAvailable = false
			}
		}()
		// 测试调用
		testName := C.CString("test>>>name>>>traffic>>>downlink")
		defer C.free(unsafe.Pointer(testName))
		result := C.parse_traffic_stat(testName)
		C.free_traffic_result(result)
		logger.Info("Rust traffic parser initialized successfully")
	})
}

// IsRustParserAvailable 返回 Rust 解析器是否可用
func IsRustParserAvailable() bool {
	initRustParser()
	return rustParserAvailable
}

// ParseTrafficStatRust 使用 Rust 解析流量统计名称
// 返回: 流量类型、标签、是否下行
func ParseTrafficStatRust(name string) TrafficParseResult {
	if !rustParserAvailable {
		return TrafficParseResult{TrafficType: TrafficTypeNone}
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	result := C.parse_traffic_stat(cName)
	defer C.free_traffic_result(result)

	if result.traffic_type == C.TRAFFIC_TYPE_NONE {
		return TrafficParseResult{TrafficType: TrafficTypeNone}
	}

	var trafficType int
	switch result.traffic_type {
	case C.TRAFFIC_TYPE_INBOUND:
		trafficType = TrafficTypeInbound
	case C.TRAFFIC_TYPE_OUTBOUND:
		trafficType = TrafficTypeOutbound
	default:
		trafficType = TrafficTypeNone
	}

	tag := ""
	if result.tag != nil {
		tag = C.GoString(result.tag)
	}

	return TrafficParseResult{
		TrafficType: trafficType,
		Tag:         tag,
		IsDownlink:  result.is_downlink != 0,
	}
}

// ParseClientTrafficStatRust 使用 Rust 解析用户流量统计名称
// 返回: 是否成功、email、是否下行
func ParseClientTrafficStatRust(name string) ClientTrafficParseResult {
	if !rustParserAvailable {
		return ClientTrafficParseResult{Success: false}
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	result := C.parse_client_traffic_stat(cName)
	defer C.free_client_traffic_result(result)

	if result.success == 0 {
		return ClientTrafficParseResult{Success: false}
	}

	email := ""
	if result.email != nil {
		email = C.GoString(result.email)
	}

	return ClientTrafficParseResult{
		Success:    true,
		Email:      email,
		IsDownlink: result.is_downlink != 0,
	}
}
