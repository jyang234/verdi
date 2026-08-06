package policyauthority

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
