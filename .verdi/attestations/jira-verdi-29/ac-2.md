---
id: attestation/jira-verdi-29--ac-2
kind: attestation
title: "unauthored attestation scaffold: spec/attest-helper ac-2"
owners: ["platform-team"]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-17, commit: b32afdb39c1474e2c8b79f0af664fa28752d7824 }
---
I reviewed the refusal discipline at 291f66e: nonexistent (story, AC) pairs and already-existing attestations refuse as verdicts through an atomic create-only write, with operational failures separated per ADJ-51's exit-discipline fix. Live: the feature-class scope refusals cited dc-5 by id, and a deliberate re-scaffold spot-check refused without clobbering. The AC holds.
