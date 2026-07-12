---
id: attestation/disclosure-legibility--ac-1
kind: attestation
title: "outcome attestation: one consistent disclosure vocabulary"
owners: [platform-team]
schema: verdi.attestation/v1
frozen: { at: 2026-07-11, commit: 6a3465b }
---

I reviewed the three pre-existing disclosure surfaces (lint's VL-017
notice, the gate's disclosed conditions, the workbench/MCP
review-unavailable state) at main @ 6a3465b: all three now render through
`internal/disclosure` and read in one vocabulary — the same leading token,
the same what-is-unproven/why shape — verified by running each surface by
hand and by the migrated exercisers. A reader who learns one recognizes
all of them. ac-1's outcome is observably delivered.
