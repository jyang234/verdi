package artifact

import "testing"

const validBindingsYAML = `schema: verdi.bindings/v1
spec: spec/stale-decline
bindings:
  - { producer: audit-before-publish, kind: static, acs: [ac-1, ac-2] }
  - { producer: refund-flow, kind: behavioral, acs: [ac-3] }
`

func TestDecodeBindings_Happy(t *testing.T) {
	got, err := DecodeBindings([]byte(validBindingsYAML))
	if err != nil {
		t.Fatalf("DecodeBindings: %v", err)
	}
	if got.Spec != "spec/stale-decline" {
		t.Fatalf("Spec = %q, want %q", got.Spec, "spec/stale-decline")
	}
	if len(got.Bindings) != 2 {
		t.Fatalf("got %d bindings, want 2", len(got.Bindings))
	}
	if got.Bindings[0].Producer != "audit-before-publish" || got.Bindings[0].Kind != EvidenceStatic {
		t.Fatalf("bindings[0] = %+v, unexpected", got.Bindings[0])
	}
	if len(got.Bindings[0].ACs) != 2 || got.Bindings[0].ACs[0] != "ac-1" || got.Bindings[0].ACs[1] != "ac-2" {
		t.Fatalf("bindings[0].ACs = %v, want [ac-1 ac-2]", got.Bindings[0].ACs)
	}
}

func TestDecodeBindings_Negative(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"unknown top-level field", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\nextra: true\n"},
		{"wrong schema", "schema: verdi.bindings/v0\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\n"},
		{"spec not a ref", "schema: verdi.bindings/v1\nspec: not a ref\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\n"},
		{"spec wrong kind", "schema: verdi.bindings/v1\nspec: adr/0001-outbox-events\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\n"},
		{"no bindings", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: []\n"},
		{"empty producer", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: \"\", kind: static, acs: [ac-1]}]\n"},
		{"unknown evidence kind", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: bogus, acs: [ac-1]}]\n"},
		{"empty acs", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: []}]\n"},
		{"malformed ac id", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [not-an-ac]}]\n"},
		{"duplicate producer", "schema: verdi.bindings/v1\nspec: spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [ac-1]}, {producer: a, kind: behavioral, acs: [ac-2]}]\n"},
		{"dialect anchor", "schema: verdi.bindings/v1\nspec: &s spec/stale-decline\nbindings: [{producer: a, kind: static, acs: [ac-1]}]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeBindings([]byte(tc.data)); err == nil {
				t.Fatalf("DecodeBindings(%s): want error, got nil", tc.name)
			}
		})
	}
}
