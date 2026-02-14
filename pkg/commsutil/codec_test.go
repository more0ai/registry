package commsutil

import (
	"testing"
)

func TestEncodePayload(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    string
		wantErr bool
	}{
		{
			name:  "simple map",
			input: map[string]string{"key": "value"},
			want:  `{"key":"value"}`,
		},
		{
			name:  "struct",
			input: struct{ Name string }{Name: "test"},
			want:  `{"Name":"test"}`,
		},
		{
			name:  "int",
			input: 42,
			want:  "42",
		},
		{
			name:  "string",
			input: "hello",
			want:  `"hello"`,
		},
		{
			name:  "nil",
			input: nil,
			want:  "null",
		},
		{
			name:  "nested struct",
			input: map[string]interface{}{"outer": map[string]int{"inner": 1}},
			want:  `{"outer":{"inner":1}}`,
		},
		{
			name:  "slice",
			input: []int{1, 2, 3},
			want:  "[1,2,3]",
		},
		{
			name:    "channel is not serializable",
			input:   make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := EncodePayload(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatal("commsutil:codec_test - expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("commsutil:codec_test - unexpected error: %v", err)
			}

			got := string(data)
			if got != tt.want {
				t.Errorf("commsutil:codec_test - EncodePayload() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodePayload(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		target  interface{}
		check   func(t *testing.T, target interface{})
		wantErr bool
	}{
		{
			name:   "decode map",
			data:   `{"key":"value"}`,
			target: &map[string]string{},
			check: func(t *testing.T, target interface{}) {
				m := target.(*map[string]string)
				if (*m)["key"] != "value" {
					t.Errorf("commsutil:codec_test - expected key=value, got %s", (*m)["key"])
				}
			},
		},
		{
			name: "decode struct",
			data: `{"Name":"test","Age":30}`,
			target: &struct {
				Name string
				Age  int
			}{},
			check: func(t *testing.T, target interface{}) {
				s := target.(*struct {
					Name string
					Age  int
				})
				if s.Name != "test" {
					t.Errorf("commsutil:codec_test - expected Name=test, got %s", s.Name)
				}
				if s.Age != 30 {
					t.Errorf("commsutil:codec_test - expected Age=30, got %d", s.Age)
				}
			},
		},
		{
			name:    "invalid json",
			data:    `{invalid}`,
			target:  &map[string]string{},
			wantErr: true,
		},
		{
			name:    "empty data",
			data:    "",
			target:  &map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := DecodePayload([]byte(tt.data), tt.target)

			if tt.wantErr {
				if err == nil {
					t.Fatal("commsutil:codec_test - expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("commsutil:codec_test - unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, tt.target)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	type TestPayload struct {
		App        string   `json:"app"`
		Capability string   `json:"capability"`
		Major      int      `json:"major"`
		Tags       []string `json:"tags"`
	}

	original := TestPayload{
		App:        "more0",
		Capability: "doc.ingest",
		Major:      3,
		Tags:       []string{"ai", "document"},
	}

	data, err := EncodePayload(original)
	if err != nil {
		t.Fatalf("commsutil:codec_test - encode failed: %v", err)
	}

	var decoded TestPayload
	err = DecodePayload(data, &decoded)
	if err != nil {
		t.Fatalf("commsutil:codec_test - decode failed: %v", err)
	}

	if decoded.App != original.App {
		t.Errorf("commsutil:codec_test - App = %q, want %q", decoded.App, original.App)
	}
	if decoded.Capability != original.Capability {
		t.Errorf("commsutil:codec_test - Capability = %q, want %q", decoded.Capability, original.Capability)
	}
	if decoded.Major != original.Major {
		t.Errorf("commsutil:codec_test - Major = %d, want %d", decoded.Major, original.Major)
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Errorf("commsutil:codec_test - Tags length = %d, want %d", len(decoded.Tags), len(original.Tags))
	}
}
