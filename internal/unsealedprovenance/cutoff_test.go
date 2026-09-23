package unsealedprovenance

import (
	"strings"
	"testing"
)

func TestCutoffChangedCode(t *testing.T) {
	if CutoffChangedCode != "unsealed-provenance-cutoff-changed" {
		t.Fatalf("CutoffChangedCode = %q", CutoffChangedCode)
	}
}

func TestCutoffViolation(t *testing.T) {
	const other = "fedcba9876543210fedcba9876543210fedcba98"
	withCutoff := func(commit string) *Payload {
		p := fixturePayload()
		p.Cutoff = &Cutoff{Commit: commit}
		return p
	}
	withoutCutoff := func() *Payload {
		p := fixturePayload()
		p.Cutoff = nil
		return p
	}
	tests := []struct {
		name     string
		accepted *Payload
		proposed *Payload
		want     []string // nil means no violation
	}{
		{"kept", withCutoff(cutoff40), withCutoff(cutoff40), nil},
		{"kept while other fields change", withCutoff(cutoff40), func() *Payload {
			p := withCutoff(cutoff40)
			p.Cap = 3
			p.Permitted = false
			p.Inventory = []InventoryEntry{}
			return p
		}(), nil},
		{"added where none existed", withoutCutoff(), withCutoff(cutoff40), nil},
		{"added where no payload existed", nil, withCutoff(cutoff40), nil},
		{"neither records one", withoutCutoff(), withoutCutoff(), nil},
		{"no payload on either side", nil, nil, nil},
		{"removed from the payload", withCutoff(cutoff40), withoutCutoff(), []string{cutoff40, "records none"}},
		{"removed with the payload", withCutoff(cutoff40), nil, []string{cutoff40, "records none"}},
		{"replaced by another commit", withCutoff(cutoff40), withCutoff(other), []string{cutoff40, other}},
		{"replaced by a 64-character id", withCutoff(cutoff40), withCutoff(cutoff64), []string{cutoff40, cutoff64}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CutoffViolation(tc.accepted, tc.proposed)
			if tc.want == nil {
				if got != "" {
					t.Fatalf("CutoffViolation = %q, want no violation", got)
				}
				return
			}
			if got == "" {
				t.Fatal("CutoffViolation reported no violation")
			}
			for _, fragment := range tc.want {
				if !strings.Contains(got, fragment) {
					t.Fatalf("CutoffViolation = %q, want it to contain %q", got, fragment)
				}
			}
		})
	}
}
