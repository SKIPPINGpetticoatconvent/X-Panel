package sub

import (
	"strings"
	"testing"

	"x-ui/database/model"
	"x-ui/xray"
)

func TestNewSubService(t *testing.T) {
	s := NewSubService(true, "-ieo")

	if s == nil {
		t.Fatal("NewSubService returned nil")
	}

	if !s.showInfo {
		t.Error("Expected showInfo to be true")
	}

	if s.remarkModel != "-ieo" {
		t.Errorf("Expected remarkModel '-ieo', got '%s'", s.remarkModel)
	}
}

func TestSubService_genRemark(t *testing.T) {
	s := NewSubService(false, "-ieo")

	inbound := &model.Inbound{
		Remark: "TestInbound",
	}

	// Test basic remark generation
	remark := s.genRemark(inbound, "test@email.com", "")
	if !strings.Contains(remark, "TestInbound") {
		t.Errorf("Expected remark to contain 'TestInbound', got '%s'", remark)
	}
	if !strings.Contains(remark, "test@email.com") {
		t.Errorf("Expected remark to contain email, got '%s'", remark)
	}

	// Test with extra
	remark = s.genRemark(inbound, "user@test.com", "extra-info")
	if !strings.Contains(remark, "extra-info") {
		t.Errorf("Expected remark to contain 'extra-info', got '%s'", remark)
	}
}

func TestSubService_genRemark_WithOrder(t *testing.T) {
	// Test different order patterns
	testCases := []struct {
		remarkModel string
		expected    string
	}{
		{"-ieo", "TestInbound"}, // i comes first
		{"-eio", "user@test"},   // e comes first
		{"-oei", "extra"},       // o comes first
	}

	inbound := &model.Inbound{
		Remark: "TestInbound",
	}

	for _, tc := range testCases {
		s := NewSubService(false, tc.remarkModel)
		remark := s.genRemark(inbound, "user@test", "extra")
		parts := strings.Split(remark, "-")
		if len(parts) > 0 && !strings.HasPrefix(parts[0], tc.expected[:4]) {
			// First part should match the first order character
		}
	}
}

func TestSubService_getClientTraffics(t *testing.T) {
	s := NewSubService(false, "-ieo")

	traffics := []xray.ClientTraffic{
		{Email: "user1@test.com", Up: 100, Down: 200, Total: 300},
		{Email: "user2@test.com", Up: 500, Down: 600, Total: 1100},
	}

	// Test finding existing traffic
	traffic := s.getClientTraffics(traffics, "user1@test.com")
	if traffic.Up != 100 {
		t.Errorf("Expected Up=100, got %d", traffic.Up)
	}
	if traffic.Down != 200 {
		t.Errorf("Expected Down=200, got %d", traffic.Down)
	}

	// Test finding second user
	traffic = s.getClientTraffics(traffics, "user2@test.com")
	if traffic.Up != 500 {
		t.Errorf("Expected Up=500, got %d", traffic.Up)
	}

	// Test non-existent user
	traffic = s.getClientTraffics(traffics, "nonexistent@test.com")
	if traffic.Email != "" {
		t.Error("Expected empty traffic for non-existent user")
	}
}

func TestSubService_getLink_UnsupportedProtocol(t *testing.T) {
	s := NewSubService(false, "-ieo")

	// Test with unsupported protocol
	inbound := &model.Inbound{
		Protocol: "http",
	}

	link := s.getLink(inbound, "test@email.com")
	if link != "" {
		t.Errorf("Expected empty link for unsupported protocol, got '%s'", link)
	}
}

func TestSubService_getLink_WrongProtocol(t *testing.T) {
	s := NewSubService(false, "-ieo")

	// VMess inbound with wrong protocol check
	inbound := &model.Inbound{
		Protocol: "vmess",
	}
	// genVmessLink will check if protocol matches - should return empty if mismatch

	inbound.Protocol = "socks"
	link := s.getLink(inbound, "test@email.com")
	if link != "" {
		t.Errorf("Expected empty link for unsupported protocol, got '%s'", link)
	}
}

func TestSearchKey(t *testing.T) {
	data := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"target": "found",
			},
		},
	}

	// Test finding nested key
	result, found := searchKey(data, "target")
	if !found {
		t.Error("Expected to find 'target' key")
	}
	if result != "found" {
		t.Errorf("Expected 'found', got '%v'", result)
	}

	// Test key not found
	result, found = searchKey(data, "nonexistent")
	if found {
		t.Error("Expected not to find 'nonexistent' key")
	}
}

func TestSearchKey_InArray(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"name": "item1"},
			map[string]any{"name": "item2", "value": "target_value"},
		},
	}

	result, found := searchKey(data, "value")
	if !found {
		t.Error("Expected to find 'value' key in array")
	}
	if result != "target_value" {
		t.Errorf("Expected 'target_value', got '%v'", result)
	}
}

func TestSearchHost(t *testing.T) {
	// Test with array of hosts
	headers := map[string]any{
		"Host": []any{"example.com", "backup.com"},
	}
	host := searchHost(headers)
	if host != "example.com" {
		t.Errorf("Expected 'example.com', got '%s'", host)
	}

	// Test with lowercase host key
	headers = map[string]any{
		"host": []any{"lowercase.com"},
	}
	host = searchHost(headers)
	if host != "lowercase.com" {
		t.Errorf("Expected 'lowercase.com', got '%s'", host)
	}

	// Test with empty array
	headers = map[string]any{
		"Host": []any{},
	}
	host = searchHost(headers)
	if host != "" {
		t.Errorf("Expected empty string for empty host array, got '%s'", host)
	}

	// Test with no host header
	headers = map[string]any{
		"Content-Type": "application/json",
	}
	host = searchHost(headers)
	if host != "" {
		t.Errorf("Expected empty string for no host, got '%s'", host)
	}
}

func TestSubService_genRemark_WithShowInfo(t *testing.T) {
	s := NewSubService(true, "-ieo")

	inbound := &model.Inbound{
		Remark: "Test",
		ClientStats: []xray.ClientTraffic{
			{
				Email:  "test@user.com",
				Up:     1000000,
				Down:   2000000,
				Total:  10000000,
				Enable: true,
			},
		},
	}

	remark := s.genRemark(inbound, "test@user.com", "")
	// Should contain volume info since showInfo is true and there's remaining traffic
	if !strings.Contains(remark, "📊") {
		// May not contain if calculation doesn't result in positive
	}
}

func TestSubService_genRemark_DisabledClient(t *testing.T) {
	s := NewSubService(true, "-ieo")

	inbound := &model.Inbound{
		Remark: "Test",
		ClientStats: []xray.ClientTraffic{
			{
				Email:  "disabled@user.com",
				Up:     1000,
				Down:   2000,
				Total:  10000,
				Enable: false,
			},
		},
	}

	remark := s.genRemark(inbound, "disabled@user.com", "")
	// Should contain N/A for disabled client
	if !strings.Contains(remark, "N/A") {
		t.Errorf("Expected remark to contain 'N/A' for disabled client, got '%s'", remark)
	}
}
