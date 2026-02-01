package model

import (
	"testing"
)

func TestInbound_GenXrayInboundConfig(t *testing.T) {
	tests := []struct {
		name          string
		inbound       Inbound
		wantPort      int
		wantProtocol  string
		wantTag       string
		wantListenStr string // Listen 字段的字符串表示
	}{
		{
			name: "带 Listen IP",
			inbound: Inbound{
				Listen:         "127.0.0.1",
				Port:           443,
				Protocol:       VLESS,
				Settings:       `{"clients":[]}`,
				StreamSettings: `{"network":"tcp"}`,
				Tag:            "inbound-443",
				Sniffing:       `{"enabled":true}`,
			},
			wantPort:      443,
			wantProtocol:  "vless",
			wantTag:       "inbound-443",
			wantListenStr: `"127.0.0.1"`,
		},
		{
			name: "空 Listen",
			inbound: Inbound{
				Listen:         "",
				Port:           8080,
				Protocol:       VMESS,
				Settings:       `{"clients":[]}`,
				StreamSettings: `{"network":"ws"}`,
				Tag:            "inbound-8080",
				Sniffing:       `{}`,
			},
			wantPort:      8080,
			wantProtocol:  "vmess",
			wantTag:       "inbound-8080",
			wantListenStr: "",
		},
		{
			name: "Trojan 协议",
			inbound: Inbound{
				Listen:   "0.0.0.0",
				Port:     10086,
				Protocol: Trojan,
				Tag:      "trojan-in",
			},
			wantPort:      10086,
			wantProtocol:  "trojan",
			wantTag:       "trojan-in",
			wantListenStr: `"0.0.0.0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inbound.GenXrayInboundConfig()

			if got.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", got.Port, tt.wantPort)
			}
			if got.Protocol != tt.wantProtocol {
				t.Errorf("Protocol = %q, want %q", got.Protocol, tt.wantProtocol)
			}
			if got.Tag != tt.wantTag {
				t.Errorf("Tag = %q, want %q", got.Tag, tt.wantTag)
			}
			if string(got.Listen) != tt.wantListenStr {
				t.Errorf("Listen = %q, want %q", string(got.Listen), tt.wantListenStr)
			}
			if string(got.Settings) != tt.inbound.Settings {
				t.Errorf("Settings = %q, want %q", string(got.Settings), tt.inbound.Settings)
			}
			if string(got.StreamSettings) != tt.inbound.StreamSettings {
				t.Errorf("StreamSettings = %q, want %q", string(got.StreamSettings), tt.inbound.StreamSettings)
			}
			if string(got.Sniffing) != tt.inbound.Sniffing {
				t.Errorf("Sniffing = %q, want %q", string(got.Sniffing), tt.inbound.Sniffing)
			}
		})
	}
}
