---
schema: verdi.governance-profile/v1
id: local-operator
class: solo
applicable_transitions: [policy-disposition-approval]
identity_trust_sources:
  - {id: local, kind: local-operator}
role_mappings:
  - {role: policy-owner, trust_source: local, subjects: [fixture@verdi.invalid]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [policy-disposition-approval], roles: [policy-owner], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
The solo local-operator profile (2026-09-05 local-operator disposition
design §2.1, ledger SI-183): one authenticated principal fills every role,
with the collapsed separation of duties disclosed by the kernel. Its
evidence is a bare self-assertion (the checkout's own configured Git
identity), never independently verified — the resolver mints the
local-operator-asserted witness for it, never trust-subject-verified, and
every report relying on it carries the local-operator-asserted disclosure.
The bound subject "fixture@verdi.invalid" is exactly the repo-local
identity internal/fixturegit.Build configures, so a fixture built from this
store's own committed content authenticates without any test-time git
config override.
