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
