---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Starter constitution"
owners: [spike-operator]
selected_profile: starter-solo
environments: [local]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, policy-disposition-approval, policy-exemption-approval]
  evidence_sources: []
  escalation_metrics: []
subjects:
  action: []
  configuration: [golangci-lint-linter-set]
  capability: []
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
template: {identity: "embedded:policy-constitution.md", digest: "sha256:58f1ff20d7680c1d9cc0a5fe03e5c428978bbafe3e6ac78c54b471df134dc6eb"}
---
Starter constitution written by `verdi policy adopt --starter`. It selects
the starter-solo governance profile and registers the minimal
catalog that profile needs: three roles, the accept and
policy-disposition-approval transitions, one local environment, no
constraint subjects, and no harness adapters. Add subjects, environments,
and adapters through the project's own review process; acceptance of this
file is the owner's merge to the default branch.
