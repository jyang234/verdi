---
id: attestation/jira-verdi-29--ac-4
kind: attestation
title: "outcome attestation: the scaffold self-validates before writing, never leaving a malformed file"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed the round-trip property at 291f66e: the scaffold self-validates its bytes through strict decode before writing and refuses rather than leaving a malformed file — proven by the self-validation tests and live by eleven scaffolds all passing the store lint immediately. The AC holds.
