---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Experiment application fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: [local]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
Hermetic policy fixture for the experiment application core.
