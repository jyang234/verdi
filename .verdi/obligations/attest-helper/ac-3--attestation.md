---
id: obligation/attest-helper--ac-3--attestation
kind: obligation
title: "The operator affirms VL-022 turns a real D6-18 mis-slug into a named, witness-carrying refusal, and stays silent where dc-4 says it must"
owners: [platform-team]
for_kind: attestation
links:
  - { type: verifies, ref: "spec/attest-helper" }
frozen: { at: 2026-07-16, commit: a42d24e2ed017bcf7fa839417755b98b90bb0f34 }
---
# The operator affirms VL-022 turns a real D6-18 mis-slug into a named, witness-carrying refusal, and stays silent where dc-4 says it must

The attestation must affirm, after reading the merged diff: VL-022 fires on
an attestation whose `verifies` target's story-ref slug disagrees with its
on-disk directory — the exact D6-18 misfiling that used to fold as a silent
`absent` — and names the offending value in the finding; and that the rule
stays silent (no finding) on every pre-existing attestation that carries no
`verifies` edge, so the grandfather corpus described in
`08-revision-notes.md` is out of scope by construction (dc-4), with no
enumerated baseline map to maintain. The operator must confirm the rule's
residual gap (a hand-moved attestation with no self-declared claim) is the
disclosed one, not a newly hidden failure.
