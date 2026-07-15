---
id: attestation/escrow-autopay--ac-1
kind: attestation
title: "AC-1 outcome attested: borrower can update their application (fixture)"
owners: [product-lead]
links:
  - { type: verifies, ref: spec/escrow-autopay }
frozen: { at: 2026-07-13, commit: 791108c9fbc210e4ca2a23ba5625c9071883118b }
---
# AC-1 outcome attestation

**Outcome attestation fixture** (02 §Identity and references, 03 §Attestations
and waivers): reuses the attestation kind unchanged, compound name
`<feature-slug>--<ac-id>` = `escrow-autopay--ac-1`, path
`attestations/escrow-autopay/ac-1.md` — `<feature-slug>` is
`RefSlug` of the feature spec's own id (`spec/escrow-autopay`),
never tracker-derived (the feature carries only an optional `story:` epic
ref).

Product lead confirms ac-1's outcome — a borrower can update their
application — observed end-to-end in staging, satisfying the outcome
floor's minimum satisfying record.
