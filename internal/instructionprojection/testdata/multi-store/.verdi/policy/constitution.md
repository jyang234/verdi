---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Multi-adapter instruction projection fixture constitution"
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
  - id: claude-code
    version: "1"
    managed: [CLAUDE.md]
    discovery_filenames: [AGENTS.md, CLAUDE.md]
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
Fixture constitution for the realistic multi-adapter layout: two
adapters manage disjoint files, but claude-code's harness ALSO discovers
AGENTS.md, the file codex manages. A discovered instruction file is
satisfied when some adapter generates and digest-matches it (AC-1 asks
for "generated and digest-matched", not "managed by the adapter that
discovered it"), so this layout must generate and verify clean.
