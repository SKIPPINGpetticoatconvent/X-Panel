package xray

import (
	"testing"
)

func TestTraffic_Fields(t *testing.T) {
	traffic := Traffic{
		IsInbound:  true,
		IsOutbound: false,
		Tag:        "inbound-443",
		Up:         1024,
		Down:       2048,
	}

	if !traffic.IsInbound {
		t.Error("IsInbound should be true")
	}
	if traffic.IsOutbound {
		t.Error("IsOutbound should be false")
	}
	if traffic.Tag != "inbound-443" {
		t.Errorf("Tag = %q, want inbound-443", traffic.Tag)
	}
	if traffic.Up != 1024 {
		t.Errorf("Up = %d, want 1024", traffic.Up)
	}
	if traffic.Down != 2048 {
		t.Errorf("Down = %d, want 2048", traffic.Down)
	}
}

func TestClientTraffic_Fields(t *testing.T) {
	ct := ClientTraffic{
		Id:         1,
		InboundId:  10,
		Enable:     true,
		Email:      "test@example.com",
		Up:         100,
		Down:       200,
		AllTime:    300,
		ExpiryTime: 1700000000,
		Total:      1000,
		Reset:      0,
		LastOnline: 1699999999,
	}

	if ct.Email != "test@example.com" {
		t.Errorf("Email = %q, want test@example.com", ct.Email)
	}
	if !ct.Enable {
		t.Error("Enable should be true")
	}
	if ct.Total != 1000 {
		t.Errorf("Total = %d, want 1000", ct.Total)
	}
}
