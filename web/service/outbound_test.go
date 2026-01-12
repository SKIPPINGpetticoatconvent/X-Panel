package service

import (
	"testing"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/xray"
)

func TestOutboundService_AddTraffic(t *testing.T) {
	setupTestDB(t)

	s := &OutboundService{}

	// Test with empty traffics
	err, needRestart := s.AddTraffic(nil, nil)
	if err != nil {
		t.Fatalf("AddTraffic with nil failed: %v", err)
	}
	if needRestart {
		t.Error("Expected needRestart to be false")
	}

	// Test with outbound traffic
	traffics := []*xray.Traffic{
		{
			IsInbound:  false,
			IsOutbound: true,
			Tag:        "direct",
			Up:         1000,
			Down:       2000,
		},
	}

	err, _ = s.AddTraffic(traffics, nil)
	if err != nil {
		t.Fatalf("AddTraffic failed: %v", err)
	}

	// Verify traffic was recorded
	outbounds, err := s.GetOutboundsTraffic()
	if err != nil {
		t.Fatalf("GetOutboundsTraffic failed: %v", err)
	}

	found := false
	for _, ob := range outbounds {
		if ob.Tag == "direct" {
			found = true
			if ob.Up != 1000 {
				t.Errorf("Expected Up=1000, got %d", ob.Up)
			}
			if ob.Down != 2000 {
				t.Errorf("Expected Down=2000, got %d", ob.Down)
			}
			if ob.Total != 3000 {
				t.Errorf("Expected Total=3000, got %d", ob.Total)
			}
		}
	}
	if !found {
		t.Error("Expected to find 'direct' outbound traffic")
	}
}

func TestOutboundService_AddTraffic_Accumulates(t *testing.T) {
	setupTestDB(t)

	s := &OutboundService{}

	// Add initial traffic
	traffics := []*xray.Traffic{
		{IsOutbound: true, Tag: "proxy", Up: 100, Down: 200},
	}
	s.AddTraffic(traffics, nil)

	// Add more traffic to same tag
	traffics = []*xray.Traffic{
		{IsOutbound: true, Tag: "proxy", Up: 50, Down: 100},
	}
	s.AddTraffic(traffics, nil)

	// Verify accumulation
	outbounds, _ := s.GetOutboundsTraffic()
	for _, ob := range outbounds {
		if ob.Tag == "proxy" {
			if ob.Up != 150 {
				t.Errorf("Expected accumulated Up=150, got %d", ob.Up)
			}
			if ob.Down != 300 {
				t.Errorf("Expected accumulated Down=300, got %d", ob.Down)
			}
		}
	}
}

func TestOutboundService_GetOutboundsTraffic(t *testing.T) {
	setupTestDB(t)

	s := &OutboundService{}

	// Test empty result
	outbounds, err := s.GetOutboundsTraffic()
	if err != nil {
		t.Fatalf("GetOutboundsTraffic failed: %v", err)
	}
	if len(outbounds) != 0 {
		t.Errorf("Expected empty result, got %d items", len(outbounds))
	}

	// Add some data directly
	db := database.GetDB()
	db.Create(&model.OutboundTraffics{Tag: "test1", Up: 100, Down: 200, Total: 300})
	db.Create(&model.OutboundTraffics{Tag: "test2", Up: 500, Down: 600, Total: 1100})

	// Verify retrieval
	outbounds, err = s.GetOutboundsTraffic()
	if err != nil {
		t.Fatalf("GetOutboundsTraffic failed: %v", err)
	}
	if len(outbounds) != 2 {
		t.Errorf("Expected 2 items, got %d", len(outbounds))
	}
}

func TestOutboundService_ResetOutboundTraffic(t *testing.T) {
	setupTestDB(t)

	s := &OutboundService{}

	// Create some traffic data
	db := database.GetDB()
	db.Create(&model.OutboundTraffics{Tag: "tag1", Up: 100, Down: 200, Total: 300})
	db.Create(&model.OutboundTraffics{Tag: "tag2", Up: 500, Down: 600, Total: 1100})

	// Reset specific tag
	err := s.ResetOutboundTraffic("tag1")
	if err != nil {
		t.Fatalf("ResetOutboundTraffic failed: %v", err)
	}

	// Verify only tag1 was reset
	outbounds, _ := s.GetOutboundsTraffic()
	for _, ob := range outbounds {
		if ob.Tag == "tag1" {
			if ob.Up != 0 || ob.Down != 0 || ob.Total != 0 {
				t.Error("Expected tag1 traffic to be reset to 0")
			}
		}
		if ob.Tag == "tag2" {
			if ob.Up == 0 {
				t.Error("Expected tag2 traffic to remain unchanged")
			}
		}
	}
}

func TestOutboundService_ResetOutboundTraffic_AllTags(t *testing.T) {
	setupTestDB(t)

	s := &OutboundService{}

	// Create some traffic data
	db := database.GetDB()
	db.Create(&model.OutboundTraffics{Tag: "tag1", Up: 100, Down: 200, Total: 300})
	db.Create(&model.OutboundTraffics{Tag: "tag2", Up: 500, Down: 600, Total: 1100})

	// Reset all tags
	err := s.ResetOutboundTraffic("-alltags-")
	if err != nil {
		t.Fatalf("ResetOutboundTraffic(-alltags-) failed: %v", err)
	}

	// Verify all were reset
	outbounds, _ := s.GetOutboundsTraffic()
	for _, ob := range outbounds {
		if ob.Up != 0 || ob.Down != 0 || ob.Total != 0 {
			t.Errorf("Expected all traffic to be reset, but %s still has data", ob.Tag)
		}
	}
}

func TestOutboundService_AddTraffic_IgnoresInbound(t *testing.T) {
	setupTestDB(t)

	s := &OutboundService{}

	// Add inbound traffic (should be ignored by OutboundService)
	traffics := []*xray.Traffic{
		{IsInbound: true, IsOutbound: false, Tag: "inbound-tag", Up: 1000, Down: 2000},
	}

	err, _ := s.AddTraffic(traffics, nil)
	if err != nil {
		t.Fatalf("AddTraffic failed: %v", err)
	}

	// Verify nothing was recorded
	outbounds, _ := s.GetOutboundsTraffic()
	if len(outbounds) != 0 {
		t.Error("Expected no outbound traffic to be recorded for inbound data")
	}
}
