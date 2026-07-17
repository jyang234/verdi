---
id: attestation/jira-verdi-30--ac-3
kind: attestation
title: "outcome attestation: a verb-recorded disposition survives align --freeze byte-for-byte"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/disposition-verb" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed spec/disposition-verb's freeze survival at 63b804e (PR #115): a verb-recorded disposition survives align --freeze byte-for-byte via the FreezeInPlace path, proven against the drifting-judge harness, with digest and integrity independently re-verified after every write. The verb's own six-sweep loop ended with it recording the disposition on its own report, and PR #117's loop ran entirely verb-recorded. The AC holds.
