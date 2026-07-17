---
id: attestation/jira-verdi-28--ac-2
kind: attestation
title: "unauthored attestation scaffold: spec/close-preflight ac-2"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed the exit and mutation discipline at b32afdb: 0 ready / 1 unmet / 2 operational is pinned by the exit-code matrix test with git-state snapshots proving nothing on disk changes in any mode, the publish-guard disclosure shares the real guard's own predicate (proven by the test driving both from identical setup), and the nine live groundwork preflights all exited 1 with clean trees. The AC holds.
