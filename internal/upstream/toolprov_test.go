package upstream

import (
	"strings"
	"testing"
)

func TestDecodeToolProvenance_Happy(t *testing.T) {
	p, err := DecodeToolProvenance([]byte(`{"tool":"v0.0.0-20260707202836-cd38b1a56bb7"}` + "\n"))
	if err != nil {
		t.Fatalf("DecodeToolProvenance: %v", err)
	}
	if p.Tool != "v0.0.0-20260707202836-cd38b1a56bb7" {
		t.Errorf("Tool = %q, want the recorded pseudo-version", p.Tool)
	}
}

func TestDecodeToolProvenance_Negative(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		wantSub string
	}{
		{"unknown field", `{"tool":"v0.0.0-20260707202836-cd38b1a56bb7","extra":1}`, "decoding toolchain.json"},
		{"trailing data", `{"tool":"v0.0.0-20260707202836-cd38b1a56bb7"}{}`, "decoding toolchain.json"},
		{"not JSON", `nope`, "decoding toolchain.json"},
		{"empty tool", `{"tool":""}`, "empty tool field"},
		{"missing tool", `{}`, "empty tool field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeToolProvenance([]byte(tc.data))
			if err == nil {
				t.Fatalf("DecodeToolProvenance(%q): want error, got nil", tc.data)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want substring %q", err, tc.wantSub)
			}
		})
	}
}
