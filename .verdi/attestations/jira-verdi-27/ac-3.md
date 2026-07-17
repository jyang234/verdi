---
id: attestation/jira-verdi-27--ac-3
kind: attestation
title: "outcome attestation: every bucket renders its heading, count, and empty-state notice even with zero specs"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed the empty-bucket behavior at 6f34b86: all three buckets render structurally on every successful render — heading, zero count, explicit empty-state notice — proven through the real pipeline over a real zero-spec store (the ADJ-40 register: git-init store with a bare origin and set-head, served by the production handler), by e2e test 3 and the harness integration tests. The index-failure carve-out is co-2's own clause, adjudicated ADJ-40 and recorded on the story's deviation report. The AC holds on every render the contract governs.
