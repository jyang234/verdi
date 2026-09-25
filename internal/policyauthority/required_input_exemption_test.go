package policyauthority

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// requiredInputExemptionPath is where requiredInputStoreFiles stores its
// required-input exemption (the one exemption home, DC-24).
const requiredInputExemptionPath = ".verdi/policy/exemptions/unsealed-vatc-machine-projections.md"

// requiredInputExemptionFile returns a required-input-family exemption
// (unsealed-provenance exemption design §4; SI-241, SI-255): a
// `required_input` witness, NO `witnesses` key, and a mandatory expiry.
// escalation, when non-empty, is spliced in verbatim as the witness's
// optional escalation block; extraApprovals is appended to the approval
// sequence.
func requiredInputExemptionFile(escalation, extraApprovals string) string {
	return `---
schema: verdi.policy-exemption/v1
id: policy-exemption/unsealed-vatc-machine-projections
kind: policy-exemption
title: "Unsealed provenance for the vatc-machine-projections implementation"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
required_input:
  phase: review
  inputs: [builder-receipt, evidence-bundle, result-diff]
  story: spec/vatc-machine-projections
  accepted_spec_digest: "sha256:` + strings.Repeat("a", 64) + `"
  implementation:
    commit: "` + strings.Repeat("1", 40) + `"
    tree: "` + strings.Repeat("2", 40) + `"
  inventory_entry: vatc-machine-projections
  landed_snapshot: "` + strings.Repeat("3", 40) + `"
` + escalation + `compensating_controls:
  - "Every acceptance criterion folds to evidenced from CI-produced records."
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
` + extraApprovals + `expiry: "2026-12-31"
template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}
---
The implementation was built before a sealed review path existed.
`
}

// requiredInputStoreFiles is minimalStoreFiles plus one required-input
// exemption beside the existing claim-family one.
func requiredInputStoreFiles(exemption string) map[string]string {
	files := minimalStoreFiles()
	files[requiredInputExemptionPath] = exemption
	return files
}

// TestLoad_RequiredInputExemption proves a store carrying a required-input
// exemption loads through BOTH entry points (Load's filesystem adapter and
// LoadFromSource) with no required-input handling in this package:
// crossValidate's claim-witness loop simply sees no claim witnesses, and
// the store's claim-family exemption is untouched beside it.
func TestLoad_RequiredInputExemption(t *testing.T) {
	escalationRole := policyartifact.EscalationRole
	escalation := `  escalation:
    use_history_digest: "sha256:` + strings.Repeat("b", 64) + `"
    corrective_action: "Adopt the sealed review path before any further unsealed close."
    sealed_execution_unavailable_reason: "The sealed review capsule was not yet adopted."
`
	escalationApproval := "  - role: " + escalationRole + "\n    principal: principal/github-org/Ym9i\n"

	tests := []struct {
		name  string
		files func() map[string]string
	}{
		{"without escalation", func() map[string]string {
			return requiredInputStoreFiles(requiredInputExemptionFile("", ""))
		}},
		{"with escalation and its registered approval role", func() map[string]string {
			files := requiredInputStoreFiles(requiredInputExemptionFile(escalation, escalationApproval))
			files[".verdi/policy/constitution.md"] = strings.Replace(files[".verdi/policy/constitution.md"],
				"roles: [author, reviewer, policy-owner]", "roles: [author, reviewer, policy-owner, "+escalationRole+"]", 1)
			return files
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := tt.files()
			root := t.TempDir()
			writeTree(t, root, files)
			fromDisk, err := Load(root)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			source := make(fstest.MapFS, len(files))
			for name, data := range files {
				source[name] = &fstest.MapFile{Data: []byte(data)}
			}
			fromSource, err := LoadFromSource(source)
			if err != nil {
				t.Fatalf("LoadFromSource() error = %v", err)
			}
			var digests []string
			for _, label := range []string{"Load", "LoadFromSource"} {
				s := map[string]*Store{"Load": fromDisk, "LoadFromSource": fromSource}[label]
				e, ok := s.Exemptions["policy-exemption/unsealed-vatc-machine-projections"]
				if !ok {
					t.Fatalf("%s: required-input exemption missing from Store.Exemptions: %v", label, s.Exemptions)
				}
				if e.RequiredInput == nil || e.RequiredInput.Story != "spec/vatc-machine-projections" {
					t.Fatalf("%s: RequiredInput = %+v", label, e.RequiredInput)
				}
				if e.Witnesses == nil || len(e.Witnesses) != 0 {
					t.Fatalf("%s: Witnesses = %#v, want the empty non-nil slice", label, e.Witnesses)
				}
				d, err := e.Digest()
				if err != nil {
					t.Fatalf("%s: Digest: %v", label, err)
				}
				digests = append(digests, d)
				claim, ok := s.Exemptions["policy-exemption/legacy-service-go"]
				if !ok {
					t.Fatalf("%s: claim-family exemption missing beside the required-input one", label)
				}
				if claim.RequiredInput != nil || len(claim.Witnesses) != 1 {
					t.Fatalf("%s: claim-family exemption changed: witnesses %v, required_input %+v", label, claim.Witnesses, claim.RequiredInput)
				}
			}
			if digests[0] != digests[1] {
				t.Fatalf("Load and LoadFromSource digests differ: %s vs %s", digests[0], digests[1])
			}
		})
	}
}

// TestLoad_RequiredInputExemptionRefusals proves the store refuses, naming
// the offending file, a required-input exemption the artifact decoder
// refuses (mixed families, no expiry), and — through the unchanged
// approval-role cross-validation — an escalation approval row whose role
// the constitution catalog does not register.
func TestLoad_RequiredInputExemptionRefusals(t *testing.T) {
	valid := requiredInputExemptionFile("", "")
	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"mixed families", strings.Replace(valid, "required_input:\n", "witnesses: []\nrequired_input:\n", 1), "both witness families"},
		{"no expiry", strings.Replace(valid, "expiry: \"2026-12-31\"\n", "", 1), "required-input exemption must carry an expiry"},
		{"unregistered escalation role", requiredInputExemptionFile("", "  - role: "+policyartifact.EscalationRole+"\n    principal: principal/github-org/Ym9i\n"), policyartifact.EscalationRole},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := requiredInputStoreFiles(tt.doc)
			source := make(fstest.MapFS, len(files))
			for name, data := range files {
				source[name] = &fstest.MapFile{Data: []byte(data)}
			}
			_, err := LoadFromSource(source)
			if err == nil {
				t.Fatalf("LoadFromSource() = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) || !strings.Contains(err.Error(), "unsealed-vatc-machine-projections") {
				t.Fatalf("LoadFromSource() error = %v, want it to name the exemption and contain %q", err, tt.wantSub)
			}
		})
	}
}
