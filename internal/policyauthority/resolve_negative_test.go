package policyauthority

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// loadAndResolve writes files to a fresh temp dir and runs Load then
// Resolve, returning the first error either stage produces (or nil if
// both succeed). Tests in this file assert on that single combined
// outcome because several of DC-3's boundary checks are legitimately
// split between Load's structural cross-validation and Resolve's
// narrow-only semantic rules (see store.go's checkOperandKind doc).
func loadAndResolve(t *testing.T, files map[string]string) (*EffectivePolicy, error) {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	s, err := Load(root)
	if err != nil {
		return nil, err
	}
	return Resolve(s)
}

func withRulesOverlay(overlay string) map[string]string {
	files := rulesStoreFiles()
	files[".verdi/policy/overlays/o.md"] = overlay
	return files
}

func TestResolve_NarrowOnlyRefinement_Matrix(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "allowed-values valid subset narrows",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "allowed-region", `values: ["us-east"]`)),
		},
		{
			name:      "allowed-values widening rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "allowed-region", `values: ["us-east", "ap-south"]`)),
			wantErr:   true,
			errSubstr: "not a subset",
		},
		{
			name: "allowed-values empty intersection across two overlays",
			files: func() map[string]string {
				files := rulesStoreFiles()
				files[".verdi/policy/overlays/o1.md"] = overlayFile("o1", `"web/"`, "allowed-region", `values: ["us-east"]`)
				files[".verdi/policy/overlays/o2.md"] = overlayFile("o2", `"web/"`, "allowed-region", `values: ["us-west"]`)
				return files
			}(),
			wantErr:   true,
			errSubstr: "empty intersection",
		},
		{
			name:  "required-values valid superset unions",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "env-required", `values: ["x", "y"]`)),
		},
		{
			name:      "required-values dropping a base member rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "env-required", `values: ["y"]`)),
			wantErr:   true,
			errSubstr: "drops a base value",
		},
		{
			name:  "forbidden-values valid superset unions",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "env-forbidden", `values: ["z", "y"]`)),
		},
		{
			name:      "forbidden-values dropping a base member rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "env-forbidden", `values: ["y"]`)),
			wantErr:   true,
			errSubstr: "drops a base value",
		},
		{
			name:  "minimum raising the bound accepted",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "coverage-min", `bound: 80`)),
		},
		{
			name:      "minimum with a lower bound rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "coverage-min", `bound: 50`)),
			wantErr:   true,
			errSubstr: "must be >= the base bound",
		},
		{
			name:  "maximum lowering the bound accepted",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "coverage-max", `bound: 85`)),
		},
		{
			name:      "maximum with a higher bound rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "coverage-max", `bound: 95`)),
			wantErr:   true,
			errSubstr: "must be <= the base bound",
		},
		{
			name:      "equals is not refinable",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "exact-version", `values: ["1.24"]`)),
			wantErr:   true,
			errSubstr: "not refinable",
		},
		{
			name:      "not-equals is not refinable",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "not-legacy", `values: ["ancient"]`)),
			wantErr:   true,
			errSubstr: "not refinable",
		},
		{
			name:      "same-principal accepts no refinement operand",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "same-owner", `values: ["x"]`)),
			wantErr:   true,
			errSubstr: "accepts no refinement operand",
		},
		{
			name:      "different-principal accepts no refinement operand",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "diff-owner", `values: ["x"]`)),
			wantErr:   true,
			errSubstr: "accepts no refinement operand",
		},
		{
			name:      "path-read is not refinable",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "readable-paths", `values: ["docs/sub/"]`)),
			wantErr:   true,
			errSubstr: "not refinable",
		},
		{
			name:      "path-write is not refinable",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "writable-paths", `values: ["scripts/sub/"]`)),
			wantErr:   true,
			errSubstr: "not refinable",
		},
		{
			name:      "refinement of a non-overridable claim rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "locked-region", `values: ["us-east"]`)),
			wantErr:   true,
			errSubstr: "not overridable",
		},
		{
			name:  "overlay scope subset of nonempty claim scope accepted",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "scoped-region", `values: ["us-east"]`)),
		},
		{
			name:      "overlay universal scope against nonempty claim scope rejected",
			files:     withRulesOverlay(overlayFile("o", ``, "scoped-region", `values: ["us-east"]`)),
			wantErr:   true,
			errSubstr: "universal",
		},
		{
			name:      "overlay scope spelling mismatch is not a provable subset",
			files:     withRulesOverlay(overlayFile("o", `"Web/"`, "scoped-region", `values: ["us-east"]`)),
			wantErr:   true,
			errSubstr: "not a provable subset",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadAndResolve(t, tc.files)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// loadMinimal returns a freshly loaded Store over the minimal fixture,
// for the composition probes below that hand-mutate its exported maps.
func loadMinimal(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())
	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	return s
}

// TestResolve_RejectsSmuggledOverlay proves Resolve re-proves the
// store's composition rather than trusting Load's: an overlay decoded
// independently and inserted straight into the exported Overlays map
// cannot refine a claim the governing policy declares non-overridable
// (DC-3), even though it never passed through Load's cross-validation.
func TestResolve_RejectsSmuggledOverlay(t *testing.T) {
	s := loadMinimal(t)
	o, err := policyartifact.DecodeOverlay([]byte(`---
schema: verdi.policy-overlay/v1
id: policy-overlay/smuggled
kind: policy-overlay
title: "Smuggled overlay"
owners: [frontend-team]
refines: policy/go-toolchain
scope: {phases: [build], environments: [], paths: [], refs: []}
refinements:
  - claim: verify-required
    values: ["clean-exit", "extra-receipt"]
template: {identity: "embedded:policy-overlay.md", digest: "sha256:c42fbc9f6c30311c940c91199d018ce99930466aad1e56108389f5d9a4be04e6"}
---
An overlay inserted directly into a loaded store's exported map.
`))
	if err != nil {
		t.Fatalf("DecodeOverlay() error: %v", err)
	}
	s.Overlays[o.ID] = o

	_, err = Resolve(s)
	if err == nil {
		t.Fatal("Resolve() succeeded on a store carrying a smuggled overlay, want error")
	}
	if !strings.Contains(err.Error(), "not overridable") {
		t.Fatalf("error = %v, want not-overridable text", err)
	}
}

// TestResolve_RejectsDeletedWitnessedPolicy proves deleting a policy an
// exemption witnesses cannot leave Resolve emitting an effective policy
// whose recorded exemption points at nothing (CO-1: a dangling witness is
// never a silent pass).
func TestResolve_RejectsDeletedWitnessedPolicy(t *testing.T) {
	s := loadMinimal(t)
	delete(s.Policies, "policy/go-toolchain")
	delete(s.Overlays, "policy-overlay/frontend-go-version")

	_, err := Resolve(s)
	if err == nil {
		t.Fatal("Resolve() succeeded with a witnessed policy deleted, want error")
	}
	if !strings.Contains(err.Error(), "policy-exemption/legacy-service-go") || !strings.Contains(err.Error(), "is not loaded") {
		t.Fatalf("error = %v, want the dangling witness named", err)
	}
}

// TestResolve_RejectsSwappedPolicy proves swapping a differently-decoded
// policy into the exported map cannot silently stale every exemption
// witness bound to the original claim digests (DC-8's exact witnesses).
func TestResolve_RejectsSwappedPolicy(t *testing.T) {
	s := loadMinimal(t)
	p, err := policyartifact.DecodePolicy([]byte(`---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.23", "1.24", "1.25"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
instructions: []
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
A widened policy swapped into a loaded store's exported map.
`))
	if err != nil {
		t.Fatalf("DecodePolicy() error: %v", err)
	}
	s.Policies[p.ID] = p

	if _, err := Resolve(s); err == nil {
		t.Fatal("Resolve() succeeded with a swapped policy, want a stale-witness error")
	} else if !strings.Contains(err.Error(), "stale witness") {
		t.Fatalf("error = %v, want stale-witness text", err)
	}
}

// TestResolve_RejectsDispositionKeyMismatch is
// TestResolve_RejectsSmuggledOverlay's own case for a disposition inserted
// under a key that does not equal its own id: checkMapKeyIdentity must
// catch a smuggled or duplicated disposition exactly like every other
// artifact family, proving the mutation-after-load / duplicate-id refusal
// this task's custody path is required to prove.
func TestResolve_RejectsDispositionKeyMismatch(t *testing.T) {
	s := loadMinimal(t)
	d, err := policyartifact.DecodeDisposition([]byte(dispositionFile(t, "review-no-conflict")))
	if err != nil {
		t.Fatalf("DecodeDisposition() error: %v", err)
	}
	s.Dispositions = map[string]*policyartifact.Disposition{"policy-disposition/different-key": d}

	_, err = Resolve(s)
	if err == nil {
		t.Fatal("Resolve() succeeded on a store carrying a disposition filed under a mismatched key, want error")
	}
	if !strings.Contains(err.Error(), "must equal the identity") {
		t.Fatalf("error = %v, want the key-identity mismatch text", err)
	}
}

// TestRefineClaim_RejectsNonOverridableClaim proves the narrowing
// routine itself refuses a non-overridable claim, independent of the
// Load-time and Resolve-time cross-validation that also reject it: DC-3's
// authority rule holds at every layer that could reach a narrowing.
func TestRefineClaim_RejectsNonOverridableClaim(t *testing.T) {
	universal := policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
	c := policyartifact.Claim{
		ID:          "locked",
		Family:      policyartifact.FamilyConfiguration,
		Operator:    policyartifact.OpAllowedValues,
		Subject:     "region",
		Values:      []string{"us-east", "us-west"},
		Scope:       universal,
		Overridable: false,
	}
	o := &policyartifact.Overlay{
		ID:          "policy-overlay/o",
		Refines:     "policy/p",
		Scope:       universal,
		Refinements: []policyartifact.Refinement{{Claim: "locked", Values: []string{"us-east"}}},
	}
	_, err := refineClaim("policy/p", c, []*policyartifact.Overlay{o})
	if err == nil {
		t.Fatal("refineClaim() succeeded on a non-overridable claim, want error")
	}
	if !strings.Contains(err.Error(), "not overridable") {
		t.Fatalf("error = %v, want not-overridable text", err)
	}
}
