---
schema: verdi.governance-profile/v1
id: {{safe .ProfileID}}
class: solo
applicable_transitions: [accept, policy-disposition-approval]
identity_trust_sources:
  - {id: local, kind: local-operator}
role_mappings:
  - {role: author, trust_source: local, subjects: [{{printf "%q" .Subject}}]}
  - {role: reviewer, trust_source: local, subjects: [{{printf "%q" .Subject}}]}
  - {role: policy-owner, trust_source: local, subjects: [{{printf "%q" .Subject}}]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [policy-disposition-approval], roles: [policy-owner], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Starter solo profile: one authenticated principal fills every role, with
the collapsed separation of duties disclosed by the kernel. The bound
subject is this checkout's own configured Git identity — a bare
self-assertion the resolver reports as local-operator-asserted, never
independently verified. A policy disposition needs the policy owner's
approval; acceptance itself is the owner's merge to the default branch.
