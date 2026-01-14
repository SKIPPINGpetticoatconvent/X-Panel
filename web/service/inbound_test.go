package service

import (
	"testing"

	"x-ui/database"
	"x-ui/database/model"
)

func TestInboundService_GetInbounds(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	s := &InboundService{}

	// Setup data
	user1 := &model.User{Username: "user1", Password: "password"}
	user2 := &model.User{Username: "user2", Password: "password"}
	db.Create(user1)
	db.Create(user2)

	inbound1 := &model.Inbound{UserId: user1.Id, Tag: "inbound1", Protocol: model.VMESS, Port: 10001}
	inbound2 := &model.Inbound{UserId: user1.Id, Tag: "inbound2", Protocol: model.VLESS, Port: 10002}
	inbound3 := &model.Inbound{UserId: user2.Id, Tag: "inbound3", Protocol: model.VMESS, Port: 10003}

	db.Create(inbound1)
	db.Create(inbound2)
	db.Create(inbound3)

	// Test GetInbounds for user1
	inbounds, err := s.GetInbounds(user1.Id)
	if err != nil {
		t.Fatalf("GetInbounds failed: %v", err)
	}

	if len(inbounds) != 2 {
		t.Errorf("Expected 2 inbounds for user1, got %d", len(inbounds))
	}

	for _, i := range inbounds {
		if i.UserId != user1.Id {
			t.Errorf("Expected inbound for user1, got user %d", i.UserId)
		}
	}

	// Test GetInbounds for user2
	inbounds2, err := s.GetInbounds(user2.Id)
	if err != nil {
		t.Fatalf("GetInbounds failed: %v", err)
	}
	if len(inbounds2) != 1 {
		t.Errorf("Expected 1 inbound for user2, got %d", len(inbounds2))
	}
	if inbounds2[0].Tag != "inbound3" {
		t.Errorf("Expected inbound3 for user2, got %s", inbounds2[0].Tag)
	}
}
