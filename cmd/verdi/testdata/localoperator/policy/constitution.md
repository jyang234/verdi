---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Local-operator fixture constitution"
owners: [platform-team]
selected_profile: local-operator
environments: [local, production]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, close, policy-disposition-approval]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: [make-verify]
  configuration: [go-version]
  capability: []
  resource: [repo-tree]
  identity: [exemption-approval]
  evidence: [verify-receipt]
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
The hermetic local-operator fixture constitution (2026-09-05 local-operator
disposition design, Task 2, ledger SI-178): selects the local-operator
profile and registers the governance catalog including the
policy-disposition-approval transition the design assigns the new trust
source kind.
