package policyauthority

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// dispositionFile returns valid verdi.policy-disposition/v1 file content
// for name: one judge-result disposition witnessing a single placeholder
// claim, with a real witness input_id computed the same way
// policyartifact's own (unexported) witnessInputID does — clear input_id,
// then canonjson.Digest the witness — so callers never hand-type a digest
// that could silently drift from the real grammar (mirroring
// fixtures_test.go's goVersionClaimDigest discipline, computed rather than
// invented).
func dispositionFile(t *testing.T, name string) string {
	t.Helper()
	universal := policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
	targetDigest := "sha256:" + strings.Repeat("1", 64)
	claimDigest := "sha256:" + strings.Repeat("2", 64)
	authorityDigest := "sha256:" + strings.Repeat("3", 64)
	witness := policyartifact.SemanticWitness{
		TargetDigest: targetDigest,
		Claims: []policyartifact.SemanticClaimWitness{{
			ID:              "ac-example",
			Digest:          claimDigest,
			Category:        "acceptance-criterion",
			AuthorityDigest: authorityDigest,
			Scope:           universal,
			Values:          []string{},
		}},
		Exemptions: []policyartifact.SemanticExemptionWitness{},
	}
	inputID, err := canonjson.Digest(witness)
	if err != nil {
		t.Fatalf("dispositionFile(%s): computing test witness input_id: %v", name, err)
	}
	return fmt.Sprintf(`---
schema: verdi.policy-disposition/v1
id: policy-disposition/%s
kind: policy-disposition
title: "Test disposition %s"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
witness:
  input_id: %q
  target_digest: %q
  claims:
    - id: ac-example
      digest: %q
      category: acceptance-criterion
      authority_digest: %q
      scope: {phases: [], environments: [], paths: [], refs: []}
      values: []
  exemptions: []
conclusion: no-conflict
origin: judge-result
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
template: {identity: "embedded:policy-disposition.md", digest: "sha256:%s"}
---
Test rationale for %s.
`, name, name, inputID, targetDigest, claimDigest, authorityDigest, strings.Repeat("4", 64), name)
}

// minimalStoreFiles returns a small but complete constitution-store file
// set: one policy (an overridable allowed-values claim and a
// non-overridable required-values claim), one overlay narrowing the
// overridable claim, one exemption witnessing the required claim... no,
// witnessing the go-version claim's base value, and one solo profile.
// Shared by Load and Resolve tests that need a valid baseline to mutate.
func minimalStoreFiles() map[string]string {
	return map[string]string{
		".verdi/policy/constitution.md": `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: [local, production]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, close]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: [make-verify]
  configuration: [go-version]
  capability: []
  resource: [repo-tree]
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
The fixture constitution.
`,
		".verdi/policy/policies/go-toolchain.md": `---
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
    values: ["1.25", "1.24"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
instructions:
  - "Run make verify before claiming completion."
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Pin the toolchain and the verification gate.
`,
		".verdi/policy/overlays/frontend-go-version.md": `---
schema: verdi.policy-overlay/v1
id: policy-overlay/frontend-go-version
kind: policy-overlay
title: "Frontend Go version overlay"
owners: [frontend-team]
refines: policy/go-toolchain
scope: {phases: [], environments: [], paths: ["web/"], refs: []}
refinements:
  - claim: go-version
    values: ["1.25"]
template: {identity: "embedded:policy-overlay.md", digest: "sha256:c42fbc9f6c30311c940c91199d018ce99930466aad1e56108389f5d9a4be04e6"}
---
The frontend narrows the toolchain choice.
`,
		".verdi/policy/exemptions/legacy-service-go.md": `---
schema: verdi.policy-exemption/v1
id: policy-exemption/legacy-service-go
kind: policy-exemption
title: "Legacy service stays on Go 1.23"
owners: [service-team]
scope: {phases: [], environments: [], paths: ["services/legacy/"], refs: []}
witnesses:
  - policy: policy/go-toolchain
    claim: go-version
    claim_digest: "` + goVersionClaimDigest + `"
compensating_controls:
  - "Weekly CVE review of the pinned toolchain."
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
expiry: "2026-12-31"
template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}
---
The legacy service departs from the toolchain policy under review.
`,
		".verdi/policy/profiles/solo-default.md": `---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: github-org, kind: forge}
role_mappings:
  - {role: author, trust_source: github-org, subjects: [alice]}
  - {role: policy-owner, trust_source: github-org, subjects: [alice]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
The solo operator profile.
`,
	}
}

// goVersionClaimDigest is the exact ClaimDigest of minimalStoreFiles'
// go-toolchain policy's go-version claim (values ["1.25","1.24"],
// normalized sorted to ["1.24","1.25"]), pinned so the exemption fixture's
// witness stays a REAL exact witness rather than a synthetic placeholder
// (mirroring internal/policyartifact/fixtures_test.go's own discipline).
// Computed once by TestFixtureExemptionWitnessDigest below; kept in sync
// by that test failing loudly if the policy fixture ever changes.
const goVersionClaimDigest = "sha256:939dc350ca2599363d9b5b89ecf681061f35081ed39025e785696d8f92c23261"
