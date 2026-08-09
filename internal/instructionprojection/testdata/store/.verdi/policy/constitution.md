---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Instruction projection fixture constitution"
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
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md, docs/AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
Fixture constitution for internal/instructionprojection tests: one
adapter with two managed projection paths sharing one discovery
filename, matching the deterministic-content-per-adapter v1 rule.
