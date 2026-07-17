---
id: attestation/closure-ergonomics--ac-4
kind: attestation
title: "AC-4 attested: verdi sync works in a plain local checkout, deriving the forge repo from git origin and honoring the fold's own ancestor rule"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/closure-ergonomics }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
# AC-4 outcome attestation

Operator attests (closure-ergonomics, 2026-07-17): I reviewed the sync outcome: verdi sync ran in plain local checkouts with no CI environment variables, deriving the repository from the git origin and accepting the bundle under the fold's own ancestor rule — live in both family groundworks (the authentication token a private repository demands of every environment is orthogonal and recorded as ADJ-58). D6-14 and D6-32 are closed. The AC holds.
