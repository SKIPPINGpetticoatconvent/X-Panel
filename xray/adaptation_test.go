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

func TestMigrate_InboundAllowInsecure(t *testing.T) {
	config := &Config{
		InboundConfigs: []InboundConfig{
			{
				StreamSettings: json_util.RawMessage(`{
					"network": "tcp",
					"security": "tls",
					"tlsSettings": {
						"serverName": "example.com",
						"allowInsecure": true
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
	if strings.Contains(out, "allowInsecure") {
		t.Errorf("failed to remove allowInsecure from inbound. Config: %s", out)
	}
	if !strings.Contains(out, "example.com") {
		t.Errorf("corrupted other fields. Config: %s", out)
	}
}

func TestMigrate_PinnedPeerCertSha256Separator(t *testing.T) {
	// Inbound: pinnedPeerCertSha256 is under tlsSettings.settings
	config := &Config{
		InboundConfigs: []InboundConfig{
			{
				StreamSettings: json_util.RawMessage(`{
					"network": "tcp",
					"security": "tls",
					"tlsSettings": {
						"serverName": "example.com",
						"settings": {
							"pinnedPeerCertSha256": "sha256hash1~sha256hash2~sha256hash3"
						}
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
	if strings.Contains(out, "~") {
		t.Errorf("failed to migrate pinnedPeerCertSha256 separator. Config: %s", out)
	}
	if !strings.Contains(out, "sha256hash1,sha256hash2,sha256hash3") {
		t.Errorf("pinnedPeerCertSha256 not migrated correctly. Config: %s", out)
	}
}

func TestMigrate_OutboundPinnedPeerCertSha256Separator(t *testing.T) {
	// Outbound: pinnedPeerCertSha256 is directly under tlsSettings
	rawJson := `[{
		"protocol": "vless",
		"streamSettings": {
			"network": "tcp",
			"security": "tls",
			"tlsSettings": {
				"serverName": "example.com",
				"pinnedPeerCertSha256": "hash1~hash2"
			}
		}
	}]`
	config := &Config{
		OutboundConfigs: json_util.RawMessage(rawJson),
	}

	err := config.AdaptToXrayCoreV25()
	if err != nil {
		t.Fatalf("AdaptToXrayCoreV25 failed: %v", err)
	}

	out := string(config.OutboundConfigs)
	if strings.Contains(out, "~") {
		t.Errorf("failed to migrate outbound pinnedPeerCertSha256 separator. Config: %s", out)
	}
	if !strings.Contains(out, "hash1,hash2") {
		t.Errorf("outbound pinnedPeerCertSha256 not migrated correctly. Config: %s", out)
	}
}

func TestMigrate_Combined(t *testing.T) {
	// Inbound with all legacy fields: verifyPeerCertInNames, allowInsecure, pinnedPeerCertSha256 with ~
	config := &Config{
		InboundConfigs: []InboundConfig{
			{
				StreamSettings: json_util.RawMessage(`{
					"network": "xhttp",
					"security": "tls",
					"tlsSettings": {
						"serverName": "example.com",
						"allowInsecure": false,
						"verifyPeerCertInNames": ["dns.google"],
						"settings": {
							"pinnedPeerCertSha256": "abc~def"
						}
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
	if strings.Contains(out, "allowInsecure") {
		t.Errorf("failed to remove allowInsecure. Config: %s", out)
	}
	if strings.Contains(out, "verifyPeerCertInNames") {
		t.Errorf("failed to remove verifyPeerCertInNames. Config: %s", out)
	}
	if !strings.Contains(out, `"verifyPeerCertByName":"dns.google"`) {
		t.Errorf("verifyPeerCertByName not set correctly. Config: %s", out)
	}
	if strings.Contains(out, "~") {
		t.Errorf("failed to migrate pinnedPeerCertSha256 separator. Config: %s", out)
	}
	if !strings.Contains(out, "abc,def") {
		t.Errorf("pinnedPeerCertSha256 not migrated correctly. Config: %s", out)
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
