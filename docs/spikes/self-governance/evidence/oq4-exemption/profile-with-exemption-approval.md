---
schema: verdi.governance-profile/v1
id: starter-solo
class: solo
applicable_transitions: [accept, policy-disposition-approval, policy-exemption-approval]
identity_trust_sources:
  - {id: local, kind: local-operator}
role_mappings:
  - {role: author, trust_source: local, subjects: ["spike-operator@example.invalid"]}
  - {role: reviewer, trust_source: local, subjects: ["spike-operator@example.invalid"]}
  - {role: policy-owner, trust_source: local, subjects: ["spike-operator@example.invalid"]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [policy-disposition-approval], roles: [policy-owner], minimum: 1}
  - {transitions: [policy-exemption-approval], roles: [policy-owner], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
template: {identity: "embedded:governance-profile-solo.md", digest: "sha256:2aa3d820227468ac5ee42e19a853ed99800ec10e7090c601bc27f0438c352c5d"}
---
Starter solo profile: one authenticated principal fills every role, with
the collapsed separation of duties disclosed by the kernel. The bound
subject is this checkout's own configured Git identity — a bare
self-assertion the resolver reports as local-operator-asserted, never
independently verified. A policy disposition needs the policy owner's
approval; acceptance itself is the owner's merge to the default branch.
