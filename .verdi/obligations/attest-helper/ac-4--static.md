---
id: obligation/attest-helper--ac-4--static
kind: obligation
title: "A unit test asserts the scaffold-rendering function's output always self-validates as kind: attestation while still unauthored"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# A unit test asserts the scaffold-rendering function's output always self-validates as kind: attestation while still unauthored

The static evidence must show a unit test (in
`internal/evidence/attestations_test.go`, the scaffold-rendering function's
own package) asserting that the function's output round-trips
`internal/artifact.DecodeAttestation` cleanly — strict-decodes and validates
as `kind: attestation` frontmatter — for a representative set of (story, ac)
inputs, while the unauthored marker is still present in the body (i.e.
before any claim is authored). The evidence must show the assertion holds on
the rendered bytes themselves, so a malformed scaffold shape is caught at
the rendering seam, not only after a write.
