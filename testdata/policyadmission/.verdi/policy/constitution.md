---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: [local]
catalog:
  roles: [author]
  transitions: [accept]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: [make-verify]
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters: []
---
Fixture store adopting the constitution capability, used to prove the
`.verdi/policy/` top-level entry lints clean (SI-6's admission).
