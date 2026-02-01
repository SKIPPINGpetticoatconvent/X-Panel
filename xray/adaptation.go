package xray

import (
	"encoding/json"

	"x-ui/util/json_util"
)

// AdaptToXrayCoreV25 removes deprecated fields like allowInsecure from the config
// to ensure compatibility with Xray-core v25+.
func (c *Config) AdaptToXrayCoreV25() error {
	// Sanitize Outbounds
	if len(c.OutboundConfigs) > 0 {
		var outbounds []map[string]interface{}
		// If unmarshal fails, we can't sanitize, so we just return (or error).
		// For backward compatibility, ignoring error might be safer, but let's return nil to proceed.
		if err := json.Unmarshal(c.OutboundConfigs, &outbounds); err == nil {
			modified := false
			for _, outbound := range outbounds {
				if stream, ok := outbound["streamSettings"].(map[string]interface{}); ok {
					if tls, ok := stream["tlsSettings"].(map[string]interface{}); ok {
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
	return nil
}
