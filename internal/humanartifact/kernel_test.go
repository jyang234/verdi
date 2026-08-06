package humanartifact

import (
	"sort"
	"testing"
)

// TestKernelFields_KnownKinds pins the exact kernel field-name formula
// for both artifact worlds this package bridges (AC-1/DC-4): the shared
// spec-store Base fields (id, kind, title, owners, schema, links,
// frozen, provenance) plus status for the kinds whose own decoder
// carries one, and the constitution kinds' full L1 frontmatter key set.
func TestKernelFields_KnownKinds(t *testing.T) {
	tests := []struct {
		kind string
		want []string
	}{
		{"feature", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "status"}},
		{"story", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "status"}},
		{"adr", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "status"}},
		{"attestation", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance"}},
		{"waiver", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "status"}},
		{"reaffirmation", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance"}},
		{"obligation", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance"}},
		{"policy", []string{"schema", "id", "kind", "title", "owners", "template", "scope", "claims", "instructions", "payloads"}},
		{"policy-overlay", []string{"schema", "id", "kind", "title", "owners", "template", "refines", "scope", "refinements"}},
		{"policy-exemption", []string{"schema", "id", "kind", "title", "owners", "template", "scope", "witnesses", "compensating_controls", "approvals", "expiry", "review_condition"}},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got, ok := KernelFields(tt.kind)
			if !ok {
				t.Fatalf("KernelFields(%q) ok = false, want true", tt.kind)
			}
			gotSorted := append([]string{}, got...)
			wantSorted := append([]string{}, tt.want...)
			sort.Strings(gotSorted)
			sort.Strings(wantSorted)
			if len(gotSorted) != len(wantSorted) {
				t.Fatalf("KernelFields(%q) = %v, want %v", tt.kind, got, tt.want)
			}
			for i := range gotSorted {
				if gotSorted[i] != wantSorted[i] {
					t.Fatalf("KernelFields(%q) = %v, want %v", tt.kind, got, tt.want)
				}
			}
		})
	}
}

// TestKernelFields_Unknown proves an unrecognized kind fails closed
// (ok=false), never a silently-empty kernel field set.
func TestKernelFields_Unknown(t *testing.T) {
	if _, ok := KernelFields("no-such-kind"); ok {
		t.Fatal("KernelFields(unknown) ok = true, want false")
	}
}

// TestKernelFields_ReturnsCopy proves the returned slice is a fresh copy,
// not the package's own internal table — a caller mutating its result
// must never corrupt every future caller's answer.
func TestKernelFields_ReturnsCopy(t *testing.T) {
	got, ok := KernelFields("policy")
	if !ok {
		t.Fatal("KernelFields(policy) ok = false")
	}
	if len(got) == 0 {
		t.Fatal("KernelFields(policy) returned no fields")
	}
	got[0] = "mutated-should-not-stick"
	got2, _ := KernelFields("policy")
	for _, f := range got2 {
		if f == "mutated-should-not-stick" {
			t.Fatal("KernelFields must return a fresh copy, not the internal slice")
		}
	}
}
