---
id: attestation/jira-verdi-29--ac-3
kind: attestation
title: "outcome attestation: VL-022 catches story misfiling as a named violation, never a silent absent"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed VL-022 at 291f66e as rescoped under ADJ-51 to the D6-18 story-misfiling class: an attestation whose path or slug does not resolve to the story AC its verifies edge claims is a named, witness-carrying lint violation, never a silent absent — proven by the table suite including the misfiled fixture, while the store's eleven legitimate feature-outcome attestations lint clean with no assertion loosened. The residual (feature-target protection) is on the ledger with its future story. The AC holds within its adjudicated scope.
