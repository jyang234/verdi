package artifact

import (
	"strings"
	"testing"
)

func TestDecodeExactJSONHappyAndFailClosed(t *testing.T) {
	type record struct {
		Name string `json:"name"`
	}
	var got record
	if err := DecodeExactJSON([]byte(`{"name":"valid"}`), &got); err != nil || got.Name != "valid" {
		t.Fatalf("DecodeExactJSON = %+v, %v", got, err)
	}
	for _, tt := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"duplicate", []byte(`{"name":"one","name":"two"}`), "duplicate"},
		{"invalid UTF-8", []byte{'{', '"', 'n', 'a', 'm', 'e', '"', ':', '"', 0xff, '"', '}'}, "UTF-8"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := DecodeExactJSON(tt.raw, &record{}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeExactJSON error = %v, want %q", err, tt.want)
			}
		})
	}
}
