package json_util_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"x-ui/util/json_util"
)

func TestRawMessage_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		message json_util.RawMessage
		want    []byte
		wantErr bool
	}{
		{
			name:    "empty message",
			message: json_util.RawMessage{},
			want:    []byte("null"),
			wantErr: false,
		},
		{
			name:    "nil message",
			message: nil,
			want:    []byte("null"),
			wantErr: false,
		},
		{
			name:    "normal message",
			message: json_util.RawMessage(`{"foo":"bar"}`),
			want:    []byte(`{"foo":"bar"}`),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.message.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("RawMessage.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("RawMessage.MarshalJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRawMessage_UnmarshalJSON(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		m       *json_util.RawMessage
		args    args
		want    json_util.RawMessage
		wantErr bool
	}{
		{
			name: "normal unmarshal",
			m:    new(json_util.RawMessage),
			args: args{
				data: []byte(`{"foo":"bar"}`),
			},
			want:    json_util.RawMessage(`{"foo":"bar"}`),
			wantErr: false,
		},
		{
			name: "nil receiver",
			m:    nil,
			args: args{
				data: []byte(`{}`),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.UnmarshalJSON(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("RawMessage.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !bytes.Equal(*tt.m, tt.want) {
				t.Errorf("After UnmarshalJSON, m = %s, want %s", *tt.m, tt.want)
			}
		})
	}
}

func TestRawMessage_UsageInStruct(t *testing.T) {
	type TestStruct struct {
		Field json_util.RawMessage `json:"field"`
	}

	// Marshaling
	s := TestStruct{
		Field: json_util.RawMessage(`"hello"`),
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if string(data) != `{"field":"hello"}` {
		t.Errorf("Marshal result = %s, want %s", string(data), `{"field":"hello"}`)
	}

	// Unmarshaling
	var s2 TestStruct
	err = json.Unmarshal([]byte(`{"field":"world"}`), &s2)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if string(s2.Field) != `"world"` {
		t.Errorf("Unmarshal result = %s, want %s", string(s2.Field), `"world"`)
	}
}
