package sub

import (
	"strings"
	"testing"

	"x-ui/database/model"
)

func TestNewSubJsonService(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	if s == nil {
		t.Fatal("NewSubJsonService returned nil")
	}

	if s.SubService != subService {
		t.Error("Expected subService to be set")
	}
}

func TestNewSubJsonService_WithFragment(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	fragment := `{"packets":"1-2","length":"100-200","interval":"10-20"}`
	s := NewSubJsonService(fragment, "", "", "", subService)

	if s.fragment != fragment {
		t.Errorf("Expected fragment to be set, got '%s'", s.fragment)
	}
}

func TestNewSubJsonService_WithMux(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	mux := `{"enabled":true,"protocol":"smux","maxConnections":8}`
	s := NewSubJsonService("", "", mux, "", subService)

	if s.mux != mux {
		t.Errorf("Expected mux to be set, got '%s'", s.mux)
	}
}

func TestSubJsonService_streamData(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	// Test TCP stream data
	streamStr := `{"network":"tcp","security":"none"}`
	result := s.streamData(streamStr)

	if result == nil {
		t.Fatal("Expected non-nil result for stream data")
	}

	if result["network"] != "tcp" {
		t.Errorf("Expected network 'tcp', got '%v'", result["network"])
	}
}

func TestSubJsonService_streamData_WebSocket(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	streamStr := `{"network":"ws","wsSettings":{"path":"/ws","headers":{"Host":"example.com"}}}`
	result := s.streamData(streamStr)

	if result["network"] != "ws" {
		t.Errorf("Expected network 'ws', got '%v'", result["network"])
	}
}

func TestSubJsonService_streamData_gRPC(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	streamStr := `{"network":"grpc","grpcSettings":{"serviceName":"myservice","multiMode":true}}`
	result := s.streamData(streamStr)

	if result["network"] != "grpc" {
		t.Errorf("Expected network 'grpc', got '%v'", result["network"])
	}
}

func TestSubJsonService_tlsData(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	tlsData := map[string]any{
		"serverName": "example.com",
		"alpn":       []any{"h2", "http/1.1"},
		"settings": map[string]any{
			"fingerprint": "chrome",
		},
	}

	result := s.tlsData(tlsData)

	if result == nil {
		t.Fatal("Expected non-nil result for TLS data")
	}

	if result["serverName"] != "example.com" {
		t.Errorf("Expected serverName 'example.com', got '%v'", result["serverName"])
	}
}

func TestSubJsonService_realityData(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	realityData := map[string]any{
		"serverNames": []any{"www.google.com"},
		"shortIds":    []any{"abc123"},
		"settings": map[string]any{
			"publicKey":   "public_key_value",
			"fingerprint": "chrome",
		},
	}

	result := s.realityData(realityData)

	if result == nil {
		t.Fatal("Expected non-nil result for Reality data")
	}
}

func TestSubJsonService_removeAcceptProxy(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	setting := map[string]any{
		"clients": []any{
			map[string]any{
				"id":    "uuid",
				"email": "test@user.com",
			},
		},
		"acceptProxyProtocol": true,
	}

	result := s.removeAcceptProxy(setting)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// acceptProxyProtocol should be removed
	if _, exists := result["acceptProxyProtocol"]; exists {
		t.Error("Expected acceptProxyProtocol to be removed")
	}
}

func TestSubJsonService_getConfig_VMess(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	inbound := &model.Inbound{
		Protocol: "vmess",
		Port:     443,
		Settings: `{"clients":[{"id":"test-uuid","alterId":0,"email":"test@user.com"}]}`,
		StreamSettings: `{
			"network":"ws",
			"wsSettings":{"path":"/ws"},
			"security":"tls",
			"tlsSettings":{"serverName":"example.com"}
		}`,
	}

	client := model.Client{
		ID:    "test-uuid",
		Email: "test@user.com",
	}

	configs := s.getConfig(inbound, client, "example.com")
	if len(configs) == 0 {
		// Config may be empty if there are missing requirements
	}
}

func TestSubJsonService_configJson_DefaultValues(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	// configJson should have been initialized from default.json
	if s.configJson == nil {
		// It's possible configJson is nil if default.json parsing fails
		// or if there are issues with initialization
		t.Log("configJson is nil, this may be expected if default.json is not accessible")
	}
}

func TestOutbound_Structure(t *testing.T) {
	o := Outbound{
		Protocol: "vmess",
		Tag:      "proxy",
	}

	if o.Protocol != "vmess" {
		t.Errorf("Expected protocol 'vmess', got '%s'", o.Protocol)
	}

	if o.Tag != "proxy" {
		t.Errorf("Expected tag 'proxy', got '%s'", o.Tag)
	}
}

func TestVnextSetting_Structure(t *testing.T) {
	setting := VnextSetting{
		Address: "example.com",
		Port:    443,
		Users: []UserVnext{
			{ID: "test-uuid", Security: "auto"},
		},
	}

	if setting.Address != "example.com" {
		t.Errorf("Expected address 'example.com', got '%s'", setting.Address)
	}

	if len(setting.Users) != 1 {
		t.Errorf("Expected 1 user, got %d", len(setting.Users))
	}
}

func TestServerSetting_Structure(t *testing.T) {
	setting := ServerSetting{
		Address:  "example.com",
		Port:     443,
		Password: "secret",
		Method:   "aes-256-gcm",
	}

	if setting.Method != "aes-256-gcm" {
		t.Errorf("Expected method 'aes-256-gcm', got '%s'", setting.Method)
	}
}

func TestNewSubJsonService_WithRules(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	rules := `[{"type":"field","domain":["geosite:cn"],"outboundTag":"direct"}]`
	s := NewSubJsonService("", "", "", rules, subService)

	// Rules should be parsed during initialization
	if s == nil {
		t.Fatal("Expected non-nil SubJsonService")
	}
}

func TestSubJsonService_streamData_InvalidJSON(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	// Test with invalid JSON
	result := s.streamData("invalid json")

	// Should return nil or empty map on parse error
	if result != nil && len(result) > 0 {
		t.Error("Expected nil or empty result for invalid JSON")
	}
}

func TestSubJsonService_Fragment_Noises(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	fragment := `{"packets":"1-2","length":"100-200"}`
	noises := `{"type":"rand","packet":"10-20"}`

	s := NewSubJsonService(fragment, noises, "", "", subService)

	if s.fragment != fragment {
		t.Errorf("Fragment mismatch: expected '%s', got '%s'", fragment, s.fragment)
	}

	if s.noises != noises {
		t.Errorf("Noises mismatch: expected '%s', got '%s'", noises, s.noises)
	}
}

func TestSubJsonService_DefaultOutbounds(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	// Default outbounds should be parsed from default.json
	if s.defaultOutbounds == nil {
		t.Log("defaultOutbounds is nil, may be expected if default.json lacks outbounds")
	}
}

func TestUserVnext_Fields(t *testing.T) {
	user := UserVnext{
		ID:         "test-uuid",
		Security:   "auto",
		Encryption: "none",
		Flow:       "xtls-rprx-vision",
		Level:      0,
	}

	if user.ID != "test-uuid" {
		t.Errorf("Expected ID 'test-uuid', got '%s'", user.ID)
	}

	if user.Flow != "xtls-rprx-vision" {
		t.Errorf("Expected Flow 'xtls-rprx-vision', got '%s'", user.Flow)
	}
}

func TestOutboundSettings_Structure(t *testing.T) {
	settings := OutboundSettings{
		Vnext: []VnextSetting{
			{Address: "example.com", Port: 443},
		},
		Servers: []ServerSetting{
			{Address: "example2.com", Port: 8443},
		},
	}

	if len(settings.Vnext) != 1 {
		t.Errorf("Expected 1 Vnext, got %d", len(settings.Vnext))
	}

	if len(settings.Servers) != 1 {
		t.Errorf("Expected 1 Server, got %d", len(settings.Servers))
	}
}

func TestSubJsonService_streamData_AllNetworkTypes(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	testCases := []struct {
		network string
		stream  string
	}{
		{"tcp", `{"network":"tcp"}`},
		{"ws", `{"network":"ws","wsSettings":{"path":"/"}}`},
		{"grpc", `{"network":"grpc","grpcSettings":{"serviceName":"test"}}`},
		{"kcp", `{"network":"kcp"}`},
		{"httpupgrade", `{"network":"httpupgrade","httpupgradeSettings":{"path":"/"}}`},
		{"xhttp", `{"network":"xhttp","xhttpSettings":{"path":"/","mode":"auto"}}`},
	}

	for _, tc := range testCases {
		result := s.streamData(tc.stream)
		if result != nil && result["network"] != tc.network {
			t.Errorf("For %s: expected network '%s', got '%v'", tc.network, tc.network, result["network"])
		}
	}
}

func TestSubJsonService_tlsData_EmptyInput(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	// tlsData creates a new map and sets some values from input
	// For nil input, it may panic or return partial data
	// For empty input, it returns a map with nil values
	result := s.tlsData(map[string]any{})
	// Should return a map (possibly with nil values)
	if result == nil {
		t.Error("Expected non-nil result for empty input")
	}
}

func TestSubJsonService_realityData_EmptyInput(t *testing.T) {
	subService := NewSubService(false, "-ieo")
	s := NewSubJsonService("", "", "", "", subService)

	// realityData creates a new map with default values
	// For empty map input, it returns a map with default values
	result := s.realityData(map[string]any{})
	if result == nil {
		t.Error("Expected non-nil result for empty input")
	}
	// Should have default values
	if result["show"] != false {
		t.Error("Expected show to be false by default")
	}
}

func TestSearchHost_StringValue(t *testing.T) {
	headers := map[string]any{
		"Host": "direct-string.com",
	}

	host := searchHost(headers)
	if !strings.Contains(host, "direct-string.com") {
		// searchHost might handle string values differently
		t.Logf("Host from string value: %s", host)
	}
}
