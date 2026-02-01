package xray

import (
	"strings"
	"testing"

	"x-ui/util/json_util"
)

func TestConfig_AdaptToXrayCoreV25(t *testing.T) {
	// 1. Prepare a config with allowInsecure (legacy format)
	rawJson := `[
		{
			"protocol": "vless",
			"streamSettings": {
				"network": "ws",
				"security": "tls",
				"tlsSettings": {
					"allowInsecure": true,
					"serverName": "example.com"
				}
			}
		}
	]`
	config := &Config{
		OutboundConfigs: json_util.RawMessage(rawJson),
	}

	// 2. Run adaptation
	err := config.AdaptToXrayCoreV25()
	if err != nil {
		t.Fatalf("AdaptToXrayCoreV25 failed: %v", err)
	}

	// 3. Verify allowInsecure is gone
	outLog := string(config.OutboundConfigs)
	if strings.Contains(outLog, "allowInsecure") {
		t.Errorf("AdaptToXrayCoreV25 failed to remove allowInsecure. Config: %s", outLog)
	}

	if !strings.Contains(outLog, "example.com") {
		t.Errorf("AdaptToXrayCoreV25 corrupted other fields.")
	}
}

func TestConfig_AdaptVerifyPeerCertInNames(t *testing.T) {
	// Inbound with legacy verifyPeerCertInNames (array format)
	config := &Config{
		InboundConfigs: []InboundConfig{
			{
				StreamSettings: json_util.RawMessage(`{
					"network": "xhttp",
					"security": "tls",
					"tlsSettings": {
						"serverName": "example.com",
						"verifyPeerCertInNames": ["dns.google", "cloudflare-dns.com"]
					}
				}`),
			},
		},
	}

	err := config.AdaptToXrayCoreV25()
	if err != nil {
		t.Fatalf("AdaptToXrayCoreV25 failed: %v", err)
	}

	out := string(config.InboundConfigs[0].StreamSettings)
	if strings.Contains(out, "verifyPeerCertInNames") {
		t.Errorf("failed to remove verifyPeerCertInNames. Config: %s", out)
	}
	if !strings.Contains(out, `"verifyPeerCertByName":"dns.google,cloudflare-dns.com"`) {
		t.Errorf("verifyPeerCertByName not set correctly. Config: %s", out)
	}
	if !strings.Contains(out, "example.com") {
		t.Errorf("corrupted other fields. Config: %s", out)
	}
}
