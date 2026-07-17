---
id: attestation/jira-verdi-28--ac-3
kind: attestation
title: "outcome attestation: preflight's disclosure and a real close agree on the same refusal, story and feature alike"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed the agreement property at b32afdb: for each defect class, fixture tests pair the preflight's disclosure with a real verdi close refusing on the byte-identical store for exactly the same reason, at both story and feature scope, and a ready fixture preflights 0 then actually closes. ADJ-56 made the agreement structural at the detail layer as dc-2 made it at the verdict layer. The AC holds.
