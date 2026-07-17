---
id: attestation/jira-verdi-28--ac-1
kind: attestation
title: "unauthored attestation scaffold: spec/close-preflight ac-1"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed spec/close-preflight's build at b32afdb (PR #117, ADJ-56 remediation): the preflight reports every condition a real close would refuse on, for stories and features alike, through the identical gate functions close itself calls, with per-kind detail projected from the fold's own evaluation (the ADJ-56 fold-API refactor — no second derivation anywhere), distinguishing absent from scaffolded-but-unauthored attestations with exact paths. Beyond the defect-class test suite, I verified this live: the nine Phase-4 groundwork preflights across both families matched their stores' actual state line-for-line, including the evidence-kind asymmetries and the feature floor's OR rendering. The AC holds.
