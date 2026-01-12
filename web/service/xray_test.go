package service

import (
	"testing"

	"x-ui/xray"
)

func Test_GetXrayErr_WhenPIsNil_ReturnsNil(t *testing.T) {
	originalP := p
	defer func() { p = originalP }()

	p = nil

	s := &XrayService{}
	err := s.GetXrayErr()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
}

func Test_GetXrayErr_WhenGetErrReturnsNil_ReturnsNil(t *testing.T) {
	originalP := p
	defer func() { p = originalP }()

	config := &xray.Config{}
	process := xray.NewProcess(config)
	// exitErr 默认 nil

	p = process

	s := &XrayService{}
	err := s.GetXrayErr()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
}

// 注意：由于私有字段无法设置，我们跳过设置 exitErr 的测试
// 在生产代码中，这些路径会被覆盖，当 Process 启动失败时 exitErr 被设置