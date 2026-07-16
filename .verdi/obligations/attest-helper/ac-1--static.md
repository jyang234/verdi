---
id: obligation/attest-helper--ac-1--static
kind: obligation
title: "Static tests fix the scaffold-rendering function's frontmatter shape, its marker constant, and its owners/title derivation"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# Static tests fix the scaffold-rendering function's frontmatter shape, its marker constant, and its owners/title derivation

The static evidence must show `internal/evidence/attestations_test.go`,
table-driven, over the scaffold-rendering function AC-1 pins: it asserts the
rendered frontmatter carries exactly `id: attestation/<storySlug>--<acID>`,
`kind: attestation`, an identifier-shaped `title` (never claim-shaped
prose), `owners` copied verbatim from the resolved story spec, `schema:
verdi.attestation/v1`, a single bare `verifies` edge, and a `frozen` stamp;
and that the body is exactly the fixed unauthored marker followed by
instructional prose. Separate cases pin the marker constant
(`evidence.UnauthoredAttestationMarker`) and the three-way detection
function so the one literal the writer and every fold reader share is proven
in one place. The evidence must show owners/title are structure copied or
mechanically derived from identifiers already on hand — no case may show a
generated, defaulted, or `[unassigned]`-placeholder owner or a claim-shaped
title.
