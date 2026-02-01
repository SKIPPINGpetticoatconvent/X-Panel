package xray

import (
	"testing"

	"x-ui/util/json_util"
)

func TestInboundConfig_Equals(t *testing.T) {
	tests := []struct {
		name string
		a    InboundConfig
		b    InboundConfig
		want bool
	}{
		{
			name: "完全相同",
			a: InboundConfig{
				Listen:         json_util.RawMessage(`"0.0.0.0"`),
				Port:           443,
				Protocol:       "vless",
				Settings:       json_util.RawMessage(`{"clients":[]}`),
				StreamSettings: json_util.RawMessage(`{"network":"tcp"}`),
				Tag:            "inbound-443",
				Sniffing:       json_util.RawMessage(`{"enabled":true}`),
			},
			b: InboundConfig{
				Listen:         json_util.RawMessage(`"0.0.0.0"`),
				Port:           443,
				Protocol:       "vless",
				Settings:       json_util.RawMessage(`{"clients":[]}`),
				StreamSettings: json_util.RawMessage(`{"network":"tcp"}`),
				Tag:            "inbound-443",
				Sniffing:       json_util.RawMessage(`{"enabled":true}`),
			},
			want: true,
		},
		{
			name: "端口不同",
			a:    InboundConfig{Port: 443, Protocol: "vless", Tag: "t1"},
			b:    InboundConfig{Port: 8443, Protocol: "vless", Tag: "t1"},
			want: false,
		},
		{
			name: "协议不同",
			a:    InboundConfig{Port: 443, Protocol: "vless", Tag: "t1"},
			b:    InboundConfig{Port: 443, Protocol: "vmess", Tag: "t1"},
			want: false,
		},
		{
			name: "Tag 不同",
			a:    InboundConfig{Port: 443, Protocol: "vless", Tag: "t1"},
			b:    InboundConfig{Port: 443, Protocol: "vless", Tag: "t2"},
			want: false,
		},
		{
			name: "Listen 不同",
			a:    InboundConfig{Listen: json_util.RawMessage(`"0.0.0.0"`), Port: 443},
			b:    InboundConfig{Listen: json_util.RawMessage(`"127.0.0.1"`), Port: 443},
			want: false,
		},
		{
			name: "Settings 不同",
			a:    InboundConfig{Settings: json_util.RawMessage(`{"a":1}`)},
			b:    InboundConfig{Settings: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "StreamSettings 不同",
			a:    InboundConfig{StreamSettings: json_util.RawMessage(`{"network":"tcp"}`)},
			b:    InboundConfig{StreamSettings: json_util.RawMessage(`{"network":"ws"}`)},
			want: false,
		},
		{
			name: "Sniffing 不同",
			a:    InboundConfig{Sniffing: json_util.RawMessage(`{"enabled":true}`)},
			b:    InboundConfig{Sniffing: json_util.RawMessage(`{"enabled":false}`)},
			want: false,
		},
		{
			name: "两个空配置",
			a:    InboundConfig{},
			b:    InboundConfig{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(&tt.b); got != tt.want {
				t.Errorf("InboundConfig.Equals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_Equals(t *testing.T) {
	tests := []struct {
		name string
		a    Config
		b    Config
		want bool
	}{
		{
			name: "两个空配置",
			a:    Config{},
			b:    Config{},
			want: true,
		},
		{
			name: "相同配置",
			a: Config{
				LogConfig: json_util.RawMessage(`{"loglevel":"warning"}`),
				DNSConfig: json_util.RawMessage(`{"servers":["8.8.8.8"]}`),
				InboundConfigs: []InboundConfig{
					{Port: 443, Protocol: "vless", Tag: "in-1"},
				},
			},
			b: Config{
				LogConfig: json_util.RawMessage(`{"loglevel":"warning"}`),
				DNSConfig: json_util.RawMessage(`{"servers":["8.8.8.8"]}`),
				InboundConfigs: []InboundConfig{
					{Port: 443, Protocol: "vless", Tag: "in-1"},
				},
			},
			want: true,
		},
		{
			name: "LogConfig 不同",
			a:    Config{LogConfig: json_util.RawMessage(`{"loglevel":"warning"}`)},
			b:    Config{LogConfig: json_util.RawMessage(`{"loglevel":"info"}`)},
			want: false,
		},
		{
			name: "RouterConfig 不同",
			a:    Config{RouterConfig: json_util.RawMessage(`{"rules":[]}`)},
			b:    Config{RouterConfig: json_util.RawMessage(`{"rules":[1]}`)},
			want: false,
		},
		{
			name: "DNSConfig 不同",
			a:    Config{DNSConfig: json_util.RawMessage(`{"a":1}`)},
			b:    Config{DNSConfig: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "OutboundConfigs 不同",
			a:    Config{OutboundConfigs: json_util.RawMessage(`[{"protocol":"freedom"}]`)},
			b:    Config{OutboundConfigs: json_util.RawMessage(`[{"protocol":"blackhole"}]`)},
			want: false,
		},
		{
			name: "Transport 不同",
			a:    Config{Transport: json_util.RawMessage(`{"a":1}`)},
			b:    Config{Transport: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "Policy 不同",
			a:    Config{Policy: json_util.RawMessage(`{"a":1}`)},
			b:    Config{Policy: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "API 不同",
			a:    Config{API: json_util.RawMessage(`{"a":1}`)},
			b:    Config{API: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "Stats 不同",
			a:    Config{Stats: json_util.RawMessage(`{"a":1}`)},
			b:    Config{Stats: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "Reverse 不同",
			a:    Config{Reverse: json_util.RawMessage(`{"a":1}`)},
			b:    Config{Reverse: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "FakeDNS 不同",
			a:    Config{FakeDNS: json_util.RawMessage(`{"a":1}`)},
			b:    Config{FakeDNS: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "Metrics 不同",
			a:    Config{Metrics: json_util.RawMessage(`{"a":1}`)},
			b:    Config{Metrics: json_util.RawMessage(`{"a":2}`)},
			want: false,
		},
		{
			name: "Inbound 数量不同",
			a: Config{
				InboundConfigs: []InboundConfig{{Port: 443}},
			},
			b: Config{
				InboundConfigs: []InboundConfig{{Port: 443}, {Port: 8443}},
			},
			want: false,
		},
		{
			name: "Inbound 内容不同",
			a: Config{
				InboundConfigs: []InboundConfig{{Port: 443, Protocol: "vless"}},
			},
			b: Config{
				InboundConfigs: []InboundConfig{{Port: 443, Protocol: "vmess"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(&tt.b); got != tt.want {
				t.Errorf("Config.Equals() = %v, want %v", got, tt.want)
			}
		})
	}
}
