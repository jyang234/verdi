---
id: obligation/attest-helper--ac-2--attestation
kind: obligation
title: "The operator affirms each refusal is a genuine verdict (exit 1), never a partial write, a silent degrade, or a check-then-write race"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# The operator affirms each refusal is a genuine verdict (exit 1), never a partial write, a silent degrade, or a check-then-write race

The attestation must affirm, after reading the merged diff: all four AC-2
refusal cases exit 1 (a verdict, not exit 0 and not the exit 2 an
operational failure gets), no bytes are written to the working tree on any
refusal, and the already-exists guard is the atomic `O_EXCL` create rather
than a check-then-write sequence that a concurrently-appearing file could
slip through (dc-2's "never overwrite a human record" made mechanically
race-safe). The operator must confirm that grouping "story-ref does not
resolve" under the verdict (exit 1) — rather than mirroring `matrix`'s
exit-2 posture for the same resolution seam — is the disclosed dc-5
divergence, read off co-2's own text, not an accident.
