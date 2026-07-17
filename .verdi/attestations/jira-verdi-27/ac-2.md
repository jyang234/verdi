---
id: attestation/jira-verdi-27--ac-2
kind: attestation
title: "unauthored attestation scaffold: spec/home-status-glance ac-2"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/home-status-glance" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed the no-loss property at 6f34b86: the glance is additive above the Directory section, whose rendering byte-identity rests on the behavior-preserving href-helper extraction plus the pre-existing literal-href assertions (proof basis corrected under ADJ-47), and the e2e test 'every pre-existing directory section and link survives unchanged alongside the new glance' passed in CI on the merge. Nothing the directory rendered before the glance is absent or moved. The AC holds.
