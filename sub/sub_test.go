package sub

import (
	"encoding/base64"
	"strings"
	"testing"

	"x-ui/database/model"

	"github.com/stretchr/testify/assert"
)

// Mock implementations
type mockInboundService struct {
	InboundProvider
	clients []model.Client
}

func (m *mockInboundService) GetClients(inbound *model.Inbound) ([]model.Client, error) {
	return m.clients, nil
}

type mockSettingService struct {
	SettingProvider
}

func (m *mockSettingService) GetDatepicker() (string, error) { return "gregorian", nil }
func (m *mockSettingService) GetSubDomain() (string, error)  { return "", nil }

func TestGenVmessLink(t *testing.T) {
	inboundSvc := &mockInboundService{
		clients: []model.Client{
			{ID: "client-uuid-123", Email: "test@example.com", Enable: true},
		},
	}
	settingSvc := &mockSettingService{}
	s := NewSubService(false, "-ieo", inboundSvc, settingSvc)
	s.address = "1.2.3.4"

	// Case 1: TCP
	inbound := &model.Inbound{
		Protocol:       model.VMESS,
		Port:           12345,
		Remark:         "TestInbound",
		StreamSettings: `{"network": "tcp", "security": "none"}`,
	}
	link := s.genVmessLink(inbound, "test@example.com")
	assert.True(t, strings.HasPrefix(link, "vmess://"))

	decoded, _ := base64.StdEncoding.DecodeString(link[8:])
	assert.Contains(t, string(decoded), "1.2.3.4")
	assert.Contains(t, string(decoded), "client-uuid-123")
	assert.Contains(t, string(decoded), "12345")
	assert.Contains(t, string(decoded), "TestInbound")

	// Case 2: WS
	inboundWS := &model.Inbound{
		Protocol:       model.VMESS,
		Port:           443,
		Remark:         "WS-Inbound",
		StreamSettings: `{"network": "ws", "security": "tls", "wsSettings": {"path": "/vpath"}}`,
	}
	linkWS := s.genVmessLink(inboundWS, "test@example.com")
	decodedWS, _ := base64.StdEncoding.DecodeString(linkWS[8:])
	assert.Contains(t, string(decodedWS), "/vpath")
	assert.Contains(t, string(decodedWS), "ws")
	assert.Contains(t, string(decodedWS), "tls")
}

func TestGenVlessLink(t *testing.T) {
	inboundSvc := &mockInboundService{
		clients: []model.Client{
			{ID: "vless-uuid-456", Email: "vuser@example.com", Enable: true, Flow: "xtls-rprx-vision"},
		},
	}
	settingSvc := &mockSettingService{}
	s := NewSubService(false, "-ieo", inboundSvc, settingSvc)
	s.address = "domain.com"

	inbound := &model.Inbound{
		Protocol:       model.VLESS,
		Port:           443,
		Remark:         "VLESS-Reality",
		Settings:       `{"encryption": "none"}`,
		StreamSettings: `{"network": "tcp", "security": "reality", "realitySettings": {"settings": {"publicKey": "pbk-xyz", "fingerprint": "chrome"}, "serverNames": ["domain.com"], "shortIds": ["sid123"]}}`,
	}
	link := s.genVlessLink(inbound, "vuser@example.com")
	assert.True(t, strings.HasPrefix(link, "vless://"))
	assert.Contains(t, link, "domain.com:443")
	assert.Contains(t, link, "security=reality")
	assert.Contains(t, link, "pbk=pbk-xyz")
	assert.Contains(t, link, "fp=chrome")
	assert.Contains(t, link, "sid=sid123")
	assert.Contains(t, link, "flow=xtls-rprx-vision")
}

func TestGenTrojanLink(t *testing.T) {
	inboundSvc := &mockInboundService{
		clients: []model.Client{
			{Password: "trojan-pass", Email: "tuser@example.com", Enable: true},
		},
	}
	settingSvc := &mockSettingService{}
	s := NewSubService(false, "-ieo", inboundSvc, settingSvc)
	s.address = "trojan.domain"

	inbound := &model.Inbound{
		Protocol:       model.Trojan,
		Port:           443,
		Remark:         "Trojan-Test",
		StreamSettings: `{"network": "ws", "security": "tls", "wsSettings": {"path": "/trojan-ws"}}`,
	}
	link := s.genTrojanLink(inbound, "tuser@example.com")
	assert.True(t, strings.HasPrefix(link, "trojan://"))
	assert.Contains(t, link, "trojan-pass@trojan.domain:443")
	assert.Contains(t, link, "path=%2Ftrojan-ws")
	assert.Contains(t, link, "security=tls")
}

func TestGenShadowsocksLink(t *testing.T) {
	inboundSvc := &mockInboundService{
		clients: []model.Client{
			{Password: "ss-pass", Email: "suser@example.com", Enable: true},
		},
	}
	settingSvc := &mockSettingService{}
	s := NewSubService(false, "-ieo", inboundSvc, settingSvc)
	s.address = "ss.domain"

	inbound := &model.Inbound{
		Protocol:       model.Shadowsocks,
		Port:           10000,
		Remark:         "SS-Test",
		Settings:       `{"method": "aes-256-gcm", "password": "master-pass"}`,
		StreamSettings: `{"network": "tcp", "security": "none"}`,
	}
	link := s.genShadowsocksLink(inbound, "suser@example.com")
	assert.True(t, strings.HasPrefix(link, "ss://"))

	expectedEnc := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:ss-pass"))
	assert.Contains(t, link, expectedEnc+"@ss.domain:10000")
}
