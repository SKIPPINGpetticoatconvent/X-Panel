package service

import (
	"testing"

	"x-ui/database"
	"x-ui/database/model"
)

func TestInboundService_GetAllInbounds(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	s := &InboundService{}

	// 空数据库
	inbounds, err := s.GetAllInbounds()
	if err != nil {
		t.Fatalf("GetAllInbounds on empty DB failed: %v", err)
	}
	if len(inbounds) != 0 {
		t.Errorf("Expected 0 inbounds on empty DB, got %d", len(inbounds))
	}

	// 创建用户和入站
	user := &model.User{Username: "testuser", Password: "pass"}
	db.Create(user)

	inbound1 := &model.Inbound{UserId: user.Id, Tag: "in-1", Protocol: model.VMESS, Port: 20001}
	inbound2 := &model.Inbound{UserId: user.Id, Tag: "in-2", Protocol: model.VLESS, Port: 20002}
	inbound3 := &model.Inbound{UserId: user.Id, Tag: "in-3", Protocol: model.Trojan, Port: 20003}
	db.Create(inbound1)
	db.Create(inbound2)
	db.Create(inbound3)

	inbounds, err = s.GetAllInbounds()
	if err != nil {
		t.Fatalf("GetAllInbounds failed: %v", err)
	}
	if len(inbounds) != 3 {
		t.Errorf("Expected 3 inbounds, got %d", len(inbounds))
	}
}

func TestInboundService_AddInbound_PortConflict(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	s := &InboundService{}

	user := &model.User{Username: "testuser2", Password: "pass"}
	db.Create(user)

	// 先创建一个入站
	first := &model.Inbound{
		UserId:   user.Id,
		Tag:      "first",
		Protocol: model.VMESS,
		Port:     30001,
		Settings: `{"clients":[]}`,
	}
	_, _, err := s.AddInbound(first)
	if err != nil {
		t.Fatalf("AddInbound first failed: %v", err)
	}

	// 相同端口应该冲突
	duplicate := &model.Inbound{
		UserId:   user.Id,
		Tag:      "duplicate",
		Protocol: model.VMESS,
		Port:     30001,
		Settings: `{"clients":[]}`,
	}
	_, _, err = s.AddInbound(duplicate)
	if err == nil {
		t.Error("Expected port conflict error, got nil")
	}
}

func TestInboundService_contains(t *testing.T) {
	s := &InboundService{}

	tests := []struct {
		name  string
		slice []string
		str   string
		want  bool
	}{
		{"精确匹配", []string{"a", "b", "c"}, "b", true},
		{"不区分大小写", []string{"Hello", "World"}, "hello", true},
		{"不区分大小写反向", []string{"hello"}, "HELLO", true},
		{"不存在", []string{"a", "b"}, "c", false},
		{"空切片", []string{}, "a", false},
		{"空字符串匹配", []string{"", "a"}, "", true},
		{"空字符串不匹配", []string{"a", "b"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.contains(tt.slice, tt.str); got != tt.want {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.str, got, tt.want)
			}
		})
	}
}
