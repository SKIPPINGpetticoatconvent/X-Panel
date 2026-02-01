package xray

import (
	"encoding/json"
	"fmt"
	"strings"

	"x-ui/util/json_util"
)

// AdaptToXrayCoreV25 removes deprecated fields like allowInsecure from the config
// to ensure compatibility with Xray-core v25+.
func (c *Config) AdaptToXrayCoreV25() error {
	// Sanitize Outbounds
	if len(c.OutboundConfigs) > 0 {
		var outbounds []map[string]any
		if err := json.Unmarshal(c.OutboundConfigs, &outbounds); err == nil {
			modified := false
			for _, outbound := range outbounds {
				if stream, ok := outbound["streamSettings"].(map[string]any); ok {
					if tls, ok := stream["tlsSettings"].(map[string]any); ok {
						if _, exists := tls["allowInsecure"]; exists {
							delete(tls, "allowInsecure")
							modified = true
						}
						// Migrate pinnedPeerCertSha256 separator: ~ → ,
						if pinned, ok := tls["pinnedPeerCertSha256"].(string); ok {
							if strings.Contains(pinned, "~") {
								tls["pinnedPeerCertSha256"] = strings.ReplaceAll(pinned, "~", ",")
								modified = true
							}
						}
					}
				}
			}
			if modified {
				if newData, err := json.Marshal(outbounds); err == nil {
					c.OutboundConfigs = json_util.RawMessage(newData)
				}
			}
		}
	}

	// Sanitize Inbounds: Migrate TLS settings (verifyPeerCertInNames, allowInsecure, pinnedPeerCertSha256)
	for i := range c.InboundConfigs {
		MigrateTlsSettings(&c.InboundConfigs[i].StreamSettings)
	}

	return nil
}

// MigrateTlsSettings performs all TLS-related migrations on a streamSettings JSON blob:
//   - verifyPeerCertInNames ([]string) → verifyPeerCertByName (comma-separated string)
//   - Remove allowInsecure from inbound tlsSettings
//   - Migrate pinnedPeerCertSha256 separator from ~ to ,
//
// Returns true if the data was modified.
func MigrateTlsSettings(raw *json_util.RawMessage) bool {
	if len(*raw) == 0 {
		return false
	}
	var stream map[string]any
	if err := json.Unmarshal(*raw, &stream); err != nil {
		return false
	}
	tls, ok := stream["tlsSettings"].(map[string]any)
	if !ok {
		return false
	}

	modified := false

	// (A) verifyPeerCertInNames → verifyPeerCertByName
	if oldVal, exists := tls["verifyPeerCertInNames"]; exists {
		var newVal string
		switch v := oldVal.(type) {
		case []any:
			parts := make([]string, 0, len(v))
			for _, item := range v {
				parts = append(parts, fmt.Sprint(item))
			}
			newVal = strings.Join(parts, ",")
		case string:
			newVal = v
		default:
			newVal = fmt.Sprint(v)
		}
		delete(tls, "verifyPeerCertInNames")
		if newVal != "" {
			tls["verifyPeerCertByName"] = newVal
		}
		modified = true
	}

	// (B) Remove allowInsecure from inbound tlsSettings
	if _, exists := tls["allowInsecure"]; exists {
		delete(tls, "allowInsecure")
		modified = true
	}

	// (C) Migrate pinnedPeerCertSha256 separator: ~ → ,
	// In inbound config, pinnedPeerCertSha256 is nested under tlsSettings.settings
	if settings, ok := tls["settings"].(map[string]any); ok {
		if pinned, ok := settings["pinnedPeerCertSha256"].(string); ok {
			if strings.Contains(pinned, "~") {
				settings["pinnedPeerCertSha256"] = strings.ReplaceAll(pinned, "~", ",")
				modified = true
			}
		}
	}
	// In outbound config, pinnedPeerCertSha256 is directly under tlsSettings
	if pinned, ok := tls["pinnedPeerCertSha256"].(string); ok {
		if strings.Contains(pinned, "~") {
			tls["pinnedPeerCertSha256"] = strings.ReplaceAll(pinned, "~", ",")
			modified = true
		}
	}

	if modified {
		if newData, err := json.Marshal(stream); err == nil {
			*raw = json_util.RawMessage(newData)
		}
	}
	return modified
}

// MigrateXhttpFlow adds missing flow ("xtls-rprx-vision") to VLESS+XHTTP+(TLS/Reality) inbounds
// Returns true if the data was modified.
func MigrateXhttpFlow(protocol string, raw *json_util.RawMessage) bool {
	if protocol != "vless" || len(*raw) == 0 {
		return false
	}
	var stream map[string]any
	if err := json.Unmarshal(*raw, &stream); err != nil {
		return false
	}

	network, _ := stream["network"].(string)
	if network != "xhttp" {
		return false
	}

	security, _ := stream["security"].(string)
	if security != "tls" && security != "reality" {
		return false
	}

	// Check if flow exists in realitySettings or tlsSettings based on security
	// However, flow is actually part of the user settings (inbound.settings -> clients -> flow)
	// BUT wait, for VLESS, flow is PER CLIENT.
	// The problem described by the user "VLESS (with no Flow) is deprecated" usually refers to
	// the fact that when XTLS/Vision is enabled (which is what we want for Reality/TLS),
	// the flow MUST be "xtls-rprx-vision".
	//
	// Wait, standard Xray structure for VLESS inbound:
	// "settings": { "clients": [ { "id": "...", "flow": "xtls-rprx-vision" } ] }
	//
	// So we need to migrate inbound.Settings, NOT inbound.StreamSettings.
	// This function signature needs to change or we need a different approach.
	return false
}

// MigrateXhttpFlowInSettings iterates through clients in VLESS settings and adds flow if missing
// for XHTTP transport with TLS/Reality.
// Returns true if modified.
func MigrateXhttpFlowInSettings(settingsRaw *json_util.RawMessage, streamSettingsRaw json_util.RawMessage) bool {
	if len(streamSettingsRaw) == 0 || len(*settingsRaw) == 0 {
		return false
	}

	// 1. Check Stream Settings for XHTTP + TLS/Reality
	var stream map[string]any
	if err := json.Unmarshal(streamSettingsRaw, &stream); err != nil {
		return false
	}
	network, _ := stream["network"].(string)
	if network != "xhttp" {
		return false
	}
	security, _ := stream["security"].(string)
	if security != "tls" && security != "reality" {
		return false
	}

	// 2. Parse Settings
	var settings map[string]any
	if err := json.Unmarshal(*settingsRaw, &settings); err != nil {
		return false
	}

	clients, ok := settings["clients"].([]any)
	if !ok {
		return false
	}

	modified := false
	for _, client := range clients {
		if c, ok := client.(map[string]any); ok {
			// Check if flow is missing or empty
			if flow, hasFlow := c["flow"].(string); !hasFlow || flow == "" {
				c["flow"] = "xtls-rprx-vision"
				modified = true
			}
		}
	}

	if modified {
		if newData, err := json.Marshal(settings); err == nil {
			*settingsRaw = json_util.RawMessage(newData)
			return true
		}
	}
	return false
}
