package artifact

import (
	"strings"
	"testing"
)

func TestDecodeClosedJSONHappyAndFailClosed(t *testing.T) {
	type nested struct {
		Value string `json:"value"`
	}
	type record struct {
		Name  string   `json:"name"`
		Items []nested `json:"items"`
	}

	var got record
	if err := DecodeClosedJSON([]byte(`{"name":"valid","items":[{"value":"a"}]}`), &got); err != nil {
		t.Fatalf("DecodeClosedJSON = %v", err)
	}
	if got.Name != "valid" || len(got.Items) != 1 || got.Items[0].Value != "a" {
		t.Fatalf("DecodeClosedJSON decoded %+v", got)
	}
	// An OMITTED key stays valid — only an explicit null is refused.
	if err := DecodeClosedJSON([]byte(`{"name":"valid"}`), &record{}); err != nil {
		t.Fatalf("omitted key must remain valid: %v", err)
	}
	// So does an explicitly empty array.
	if err := DecodeClosedJSON([]byte(`{"items":[]}`), &record{}); err != nil {
		t.Fatalf("empty array must remain valid: %v", err)
	}

	for _, tt := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"top-level null", []byte(`null`), "null"},
		{"field null", []byte(`{"name":null}`), "null"},
		{"nested null", []byte(`{"items":[{"value":null}]}`), "null"},
		{"array element null", []byte(`{"items":[null]}`), "null"},
		{"duplicate key still refused", []byte(`{"name":"one","name":"two"}`), "duplicate"},
		{"unknown field still refused", []byte(`{"bogus":1}`), "unknown"},
		{"trailing data still refused", []byte(`{"name":"a"}{}`), "trailing"},
		{"malformed", []byte(`{`), "EOF"},
		{"non-string key", []byte(`{1:2}`), "scanning"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := DecodeClosedJSON(tt.raw, &record{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeClosedJSON error = %v, want one containing %q", err, tt.want)
			}
		})
	}
}

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
