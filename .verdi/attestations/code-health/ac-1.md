---
id: attestation/code-health--ac-1
kind: attestation
title: "AC-1 attested: no build output is tracked and the class is refused by the gate"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/code-health }
frozen: { at: 2026-07-14, commit: 49b779af64f9584f55cd3f0940e6c38fda544ed8 }
---
# AC-1 outcome attestation

Operator attests (round 6, 2026-07-14): the 21.8 MB e2eharness Mach-O is
gone from git and ignored, and internal/specalign's repo-hygiene check
walks git ls-files refusing any tracked Mach-O/ELF/PE by magic bytes —
witnessed red against the real binary before its removal, green since,
inside the gate that never shrinks. Proven by spec/fail-loud's archived
closure (jira:VERDI-QH-1, closure gate 3/3 on source: ci evidence).
