package humanartifact

import (
	"sort"
	"strings"
	"testing"
)

// TestKernelFields_KnownKinds pins the exact kernel field-name formula
// for both artifact worlds this package bridges (AC-1/DC-4): the shared
// spec-store Base fields (id, kind, title, owners, schema, links,
// frozen, provenance) plus status where the kind's own decoder carries
// one, plus every other identity/governance frontmatter key that kind's
// own decoder recognizes (class/custom for feature/story, decided for
// adr, reason/expiry for waiver, object/hash for reaffirmation, for_kind
// for obligation) — excluding each kind's own body-prose content fields
// (problem, outcome, acceptance_criteria, constraints, decisions) — and
// the constitution kinds' full L1 frontmatter key set.
func TestKernelFields_KnownKinds(t *testing.T) {
	tests := []struct {
		kind string
		want []string
	}{
		{"feature", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "status", "class", "custom"}},
		{"story", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "status", "class", "custom"}},
		{"adr", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "status", "decided"}},
		{"attestation", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance"}},
		{"waiver", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "status", "reason", "expiry"}},
		{"reaffirmation", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "object", "hash"}},
		{"obligation", []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance", "for_kind", "quality"}},
		{"policy", []string{"schema", "id", "kind", "title", "owners", "template", "scope", "claims", "instructions", "payloads"}},
		{"policy-overlay", []string{"schema", "id", "kind", "title", "owners", "template", "refines", "scope", "refinements"}},
		{"policy-exemption", []string{"schema", "id", "kind", "title", "owners", "template", "scope", "witnesses", "required_input", "compensating_controls", "approvals", "expiry", "review_condition"}},
		{"policy-disposition", []string{"schema", "id", "kind", "title", "owners", "template", "scope", "witness", "conclusion", "origin", "judgment", "compensating_controls", "approvals", "expiry", "review_condition"}},
		{"policy-constitution", []string{"schema", "id", "kind", "title", "owners", "template", "selected_profile", "environments", "catalog", "subjects", "adapters"}},
		{"governance-profile", []string{"schema", "id", "class", "applicable_transitions", "identity_trust_sources", "role_mappings", "ownership_sources", "signature_requirements", "required_approvers", "distinctness_rules", "evidence_source_restrictions", "escalation_thresholds", "template"}},
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

// TestContract_PolicyExemptionRequiredInputIsKernel proves the policy-
// exemption row's required_input entry does its one job: a model-declared
// extension can never shadow or synthesize the required-input witness
// (AC-1), in any case spelling.
func TestContract_PolicyExemptionRequiredInputIsKernel(t *testing.T) {
	for _, name := range []string{"required_input", "Required_Input"} {
		t.Run(name, func(t *testing.T) {
			err := Contract{Kind: "policy-exemption", Extensions: []ExtensionField{{Name: name, Type: ExtensionString}}}.Validate()
			if err == nil {
				t.Fatalf("Contract.Validate() accepted an extension %q shadowing the required-input witness", name)
			}
			if !strings.Contains(err.Error(), "shadows kernel field") {
				t.Fatalf("Contract.Validate() error = %v, want a kernel-shadow refusal", err)
			}
		})
	}
}
