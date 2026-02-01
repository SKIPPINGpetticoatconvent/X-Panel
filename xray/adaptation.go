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

	// Sanitize Inbounds: Migrate verifyPeerCertInNames -> verifyPeerCertByName
	for i := range c.InboundConfigs {
		migrateTlsSettings(&c.InboundConfigs[i].StreamSettings)
	}

	return nil
}

// migrateTlsSettings renames verifyPeerCertInNames to verifyPeerCertByName
// and converts the value from []string to a comma-separated string.
// Returns true if the data was modified.
func migrateTlsSettings(raw *json_util.RawMessage) bool {
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

	oldVal, exists := tls["verifyPeerCertInNames"]
	if !exists {
		return false
	}

	// Convert []any to comma-separated string
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

	if newData, err := json.Marshal(stream); err == nil {
		*raw = json_util.RawMessage(newData)
		return true
	}
	return false
}
