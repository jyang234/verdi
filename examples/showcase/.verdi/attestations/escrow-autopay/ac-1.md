---
id: attestation/escrow-autopay--ac-1
kind: attestation
title: "AC-1 outcome attested: autopay mandate created against the escrow account (fixture)"
owners: [product-lead]
links:
  - { type: verifies, ref: spec/escrow-autopay }
frozen: { at: 2026-07-13, commit: faf8d8c412c9df35b5a445146a5fe0e8309caa71 }
---
# AC-1 outcome attestation

**Outcome attestation fixture** (02 §Identity and references, 03 §Attestations
and waivers): reuses the attestation kind unchanged, compound name
`<feature-slug>--<ac-id>` = `escrow-autopay--ac-1`, path
`attestations/escrow-autopay/ac-1.md` — `<feature-slug>` is
`RefSlug` of the feature spec's own id (`spec/escrow-autopay`),
never tracker-derived (the feature carries only an optional `story:` epic
ref).

Product lead confirms ac-1's outcome — an autopay mandate is created
against a submitted application's escrow account, tied to the payment
method already on file — observed end-to-end in a staging enrollment
walkthrough, ahead of either stub being realized: the outcome floor is
satisfied even though the fold below still reads no-signal, since no
implementing story yet exists to carry it past the "closed or eligible"
bar (03 §The feature fold) — an attestation alone was never meant to be
sufficient on its own.
