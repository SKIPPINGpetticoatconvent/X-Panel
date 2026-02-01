package locale

import (
	"testing"
)

func TestCreateTemplateData(t *testing.T) {
	tests := []struct {
		name      string
		params    []string
		separator []string
		wantKeys  []string
		wantVals  []string
	}{
		{
			name:     "默认分隔符",
			params:   []string{"name==John", "age==30"},
			wantKeys: []string{"name", "age"},
			wantVals: []string{"John", "30"},
		},
		{
			name:      "自定义分隔符",
			params:    []string{"key:value", "foo:bar"},
			separator: []string{":"},
			wantKeys:  []string{"key", "foo"},
			wantVals:  []string{"value", "bar"},
		},
		{
			name:     "空参数",
			params:   []string{},
			wantKeys: []string{},
		},
		{
			name:     "无分隔符的参数",
			params:   []string{"nosep"},
			wantKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]any
			if len(tt.separator) > 0 {
				result = createTemplateData(tt.params, tt.separator...)
			} else {
				result = createTemplateData(tt.params)
			}

			for i, key := range tt.wantKeys {
				val, ok := result[key]
				if !ok {
					t.Errorf("key %q not found in result", key)
					continue
				}
				if val != tt.wantVals[i] {
					t.Errorf("result[%q] = %q, want %q", key, val, tt.wantVals[i])
				}
			}
		})
	}
}

func TestI18nType_Constants(t *testing.T) {
	if Bot != "bot" {
		t.Errorf("Bot = %q, want bot", Bot)
	}
	if Web != "web" {
		t.Errorf("Web = %q, want web", Web)
	}
}
