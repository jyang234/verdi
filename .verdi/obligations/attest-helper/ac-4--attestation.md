---
id: obligation/attest-helper--ac-4--attestation
kind: obligation
title: "The operator affirms the round-trip is real: the real DecodeAttestation, at the fold's own derived path, exercised before authoring"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# The operator affirms the round-trip is real: the real DecodeAttestation, at the fold's own derived path, exercised before authoring

The attestation must affirm, after reading the merged diff: the round-trip
test reads back from the fold's own path-construction helper (not a
hand-typed path literal that could drift from the real convention) and calls
the real `internal/artifact.DecodeAttestation` — not a stand-in or a partial
frontmatter check — against a scaffold that still carries the unauthored
marker, proving the file is schema-valid before any claim exists. The
operator must confirm the verb's pre-write self-check genuinely refuses
(operational exit 2) rather than leaving a malformed attestation on disk,
and that the assertion is against the bytes actually written.
