package policyauthority

// rulesStoreFiles returns a constitution store built around one policy
// (policy/rules) that carries one claim per operator category, so
// Resolve's narrow-only refinement-boundary tests can each add a single
// overlay file and observe exactly one failure or success mode. It is
// deliberately separate from minimalStoreFiles: the operator matrix needs
// its own subject and claim vocabulary the happy-path fixture has no
// reason to carry.
func rulesStoreFiles() map[string]string {
	return map[string]string{
		".verdi/policy/constitution.md": `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Rules-matrix fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: [local, production]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, close]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: []
  configuration: [go-version, env-key, coverage, region, region2]
  capability: []
  resource: [repo-tree]
  identity: [exemption-approval]
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
The rules-matrix fixture constitution.
`,
		".verdi/policy/policies/rules.md": `---
schema: verdi.policy/v1
id: policy/rules
kind: policy
title: "Refinement rules policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: allowed-region
    family: configuration
    operator: allowed-values
    subject: region
    values: ["us-east", "us-west", "eu-west"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: scoped-region
    family: configuration
    operator: allowed-values
    subject: region2
    values: ["us-east", "us-west"]
    scope: {phases: [], environments: [], paths: ["services/", "web/"], refs: []}
    overridable: true
  - id: env-required
    family: configuration
    operator: required-values
    subject: env-key
    values: ["x"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: env-forbidden
    family: configuration
    operator: forbidden-values
    subject: env-key
    values: ["z"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: coverage-min
    family: configuration
    operator: minimum
    subject: coverage
    values: []
    bound: 70
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: coverage-max
    family: configuration
    operator: maximum
    subject: coverage
    values: []
    bound: 90
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: exact-version
    family: configuration
    operator: equals
    subject: go-version
    values: ["1.25"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: not-legacy
    family: configuration
    operator: not-equals
    subject: go-version
    values: ["legacy"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: readable-paths
    family: resource
    operator: path-read
    subject: repo-tree
    values: ["docs/"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: writable-paths
    family: resource
    operator: path-write
    subject: repo-tree
    values: ["scripts/"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: same-owner
    family: identity
    operator: same-principal
    subject: exemption-approval
    values: ["author", "reviewer"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: diff-owner
    family: identity
    operator: different-principal
    subject: exemption-approval
    values: ["author", "policy-owner"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: locked-region
    family: configuration
    operator: allowed-values
    subject: region
    values: ["us-east", "us-west"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: false
instructions: []
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Rules for refinement-boundary testing.
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

// overlayFile builds a one-refinement overlay file targeting policy/rules,
// for tests that only need to vary the overlay's own scope and one
// refinement's operand.
func overlayFile(id, scopePaths, claim, operand string) string {
	return `---
schema: verdi.policy-overlay/v1
id: policy-overlay/` + id + `
kind: policy-overlay
title: "Rules overlay ` + id + `"
owners: [frontend-team]
refines: policy/rules
scope: {phases: [], environments: [], paths: [` + scopePaths + `], refs: []}
refinements:
  - claim: ` + claim + `
    ` + operand + `
template: {identity: "embedded:policy-overlay.md", digest: "sha256:c42fbc9f6c30311c940c91199d018ce99930466aad1e56108389f5d9a4be04e6"}
---
Overlay fixture for the refinement-boundary matrix.
`
}
