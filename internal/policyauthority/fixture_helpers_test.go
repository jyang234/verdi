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
	const claimID = "spec/example#ac-example"
	targetDigest := "sha256:" + strings.Repeat("1", 64)
	claimDigest := "sha256:" + strings.Repeat("2", 64)
	authorityDigest := "sha256:" + strings.Repeat("3", 64)
	witness := policyartifact.SemanticWitness{
		TargetDigest: targetDigest,
		Claims: []policyartifact.SemanticClaimWitness{{
			ID:              claimID,
			Digest:          claimDigest,
			Category:        "acceptance-criterion",
			AuthorityDigest: authorityDigest,
			Scope:           semanticClaimScope(claimID),
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
    - id: %q
      digest: %q
      category: acceptance-criterion
      authority_digest: %q
      scope: {phases: [], environments: [], paths: [], refs: [%q]}
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
`, name, name, inputID, targetDigest, claimID, claimDigest, authorityDigest, claimID, strings.Repeat("4", 64), name)
}

func semanticClaimScope(id string) policyartifact.Scope {
	return policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{id}}
}

// renderWitnessYAML renders w as the exact column-zero `witness:` block a
// disposition frontmatter carries, stamping the input_id policyartifact
// itself recomputes on decode (the canonical digest of the witness with
// input_id cleared). Rendering the SAME Go value that produced the digest
// is what keeps a rich fixture's declared input_id from silently drifting
// away from its own content, exactly as dispositionFile does for the
// minimal witness.
//
// w.InputID must be empty, and claims and exemptions must already be
// sorted by id — the decoder refuses an unsorted witness rather than
// reordering it, so the fixture states the on-disk order it means.
func renderWitnessYAML(t *testing.T, w policyartifact.SemanticWitness) string {
	t.Helper()
	if w.InputID != "" {
		t.Fatalf("renderWitnessYAML: caller set InputID %q; the fixture computes it", w.InputID)
	}
	inputID, err := canonjson.Digest(w)
	if err != nil {
		t.Fatalf("renderWitnessYAML: computing input_id: %v", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "witness:\n  input_id: %q\n  target_digest: %q\n  claims:\n", inputID, w.TargetDigest)
	for _, c := range w.Claims {
		fmt.Fprintf(&b, "    - id: %q\n      digest: %q\n      category: %q\n      authority_digest: %q\n      scope: %s\n      values: %s\n",
			c.ID, c.Digest, c.Category, c.AuthorityDigest, scopeYAML(c.Scope), listYAML(c.Values))
		if c.Bound != nil {
			fmt.Fprintf(&b, "      bound: %d\n", *c.Bound)
		}
	}
	if len(w.Exemptions) == 0 {
		b.WriteString("  exemptions: []\n")
		return b.String()
	}
	b.WriteString("  exemptions:\n")
	for _, e := range w.Exemptions {
		fmt.Fprintf(&b, "    - id: %q\n      digest: %q\n", e.ID, e.Digest)
	}
	return b.String()
}

func scopeYAML(s policyartifact.Scope) string {
	return fmt.Sprintf("{phases: %s, environments: %s, paths: %s, refs: %s}",
		listYAML(s.Phases), listYAML(s.Environments), listYAML(s.Paths), listYAML(s.Refs))
}

func listYAML(vs []string) string {
	if len(vs) == 0 {
		return "[]"
	}
	quoted := make([]string, len(vs))
	for i, v := range vs {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// richWitness is the witness both maximal fixtures below share: two sorted
// claims (the first carrying a typed values set AND a bound, so the
// optional Bound pointer is exercised), and a nonempty sorted exemption
// witness set, so no deep-copy branch is left empty by construction.
func richWitness() policyartifact.SemanticWitness {
	bound := 3
	const acceptanceID = "spec/example#ac-bounded"
	const constraintID = "spec/example#co-second"
	return policyartifact.SemanticWitness{
		TargetDigest: "sha256:" + strings.Repeat("a", 64),
		Claims: []policyartifact.SemanticClaimWitness{
			{
				ID:              acceptanceID,
				Digest:          "sha256:" + strings.Repeat("b", 64),
				Category:        "acceptance-criterion",
				AuthorityDigest: "sha256:" + strings.Repeat("c", 64),
				Scope:           semanticClaimScope(acceptanceID),
				Values:          []string{"weekly"},
				Bound:           &bound,
			},
			{
				ID:              constraintID,
				Digest:          "sha256:" + strings.Repeat("d", 64),
				Category:        "constraint",
				AuthorityDigest: "sha256:" + strings.Repeat("c", 64),
				Scope:           semanticClaimScope(constraintID),
				Values:          []string{},
			},
		},
		Exemptions: []policyartifact.SemanticExemptionWitness{{
			ID:     "policy-exemption/legacy-service-go",
			Digest: "sha256:" + strings.Repeat("e", 64),
		}},
	}
}

// judgeResultDispositionFile returns a MAXIMAL judge-result disposition:
// every optional branch copyDisposition must deep-copy is populated —
// judgment provenance (primary and challenger), a bounded witness claim, a
// nonempty exemption witness set, compensating controls, expiry, review
// condition, and a template record. dispositionFile's minimal fixture
// leaves those branches nil/empty, where an accidental alias would survive
// unnoticed.
func judgeResultDispositionFile(t *testing.T, name string) string {
	t.Helper()
	return fmt.Sprintf(`---
schema: verdi.policy-disposition/v1
id: policy-disposition/%s
kind: policy-disposition
title: "Judge-result disposition %s"
owners: [platform-team, service-team]
scope: {phases: [], environments: [], paths: ["services/legacy/"], refs: []}
%sconclusion: no-conflict
origin: judge-result
judgment: {primary_digest: "sha256:%s", challenger_digest: "sha256:%s"}
compensating_controls:
  - "Re-run the semantic gate after every authority change."
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
expiry: "2026-12-31"
review_condition: "Revisit when the target capsule digest changes."
template: {identity: "embedded:policy-disposition.md", digest: "sha256:%s"}
---
Judge-result rationale for %s.
`, name, name, renderWitnessYAML(t, richWitness()),
		strings.Repeat("6", 64), strings.Repeat("7", 64), strings.Repeat("4", 64), name)
}

// humanFallbackDispositionFile returns a MAXIMAL human-fallback
// disposition: §8's fallback-only obligations (at least one compensating
// control plus a real expiry or review condition) alongside the same rich
// witness, so the human-fallback branch of the artifact grammar actually
// flows through Load and Resolve rather than never being loaded at all. A
// fallback never carries judge provenance, so judgment is absent by rule.
func humanFallbackDispositionFile(t *testing.T, name string) string {
	t.Helper()
	return fmt.Sprintf(`---
schema: verdi.policy-disposition/v1
id: policy-disposition/%s
kind: policy-disposition
title: "Human-fallback disposition %s"
owners: [platform-team]
scope: {phases: [build], environments: [production], paths: [], refs: []}
%sconclusion: conflict
origin: human-fallback
compensating_controls:
  - "Two-person review of every affected change."
  - "Weekly re-read of the disputed authority."
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
expiry: "2027-03-31"
review_condition: "Revisit once a judge transport is configured."
template: {identity: "embedded:policy-disposition.md", digest: "sha256:%s"}
---
Human-fallback rationale for %s.
`, name, name, renderWitnessYAML(t, richWitness()), strings.Repeat("4", 64), name)
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
