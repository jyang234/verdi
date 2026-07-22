---
id: attestation/creation-surfaces--ac-5
kind: attestation
title: "AC-5 attested: verdi waive ships per guide 8.4 and the verb vocabulary category is born enforced"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/creation-surfaces }
frozen: { at: 2026-07-22, commit: 7c35d887ad9ce0ae355de82d6c3af90bf2fd73ec }
---
# AC-5 outcome attestation

Stand-in operator attests (Phase 2, 2026-07-22): `verdi waive` lands per
the guide's own 8.4 — a waiver record over the existing `waivers/` kind,
`--expires`, a reaffirmation flow, and audit counting wired into the
audit surface (a `waivers_stale_threshold` with its own stale section).

The load-bearing half is the vocabulary: `TestVocabProseWitness` was
extended from class/state words to VERB words, and running it caught 16
bare verb-word hits across 13 production sites — each then routed
through `DisplayVerb` (2 genuinely verb-speaking surfaces, including the
accept backstop's own obligation-body disclosure, proven to render a
renamed verb) or marked `// vocab:identity` with a stated reason (11
sites: CLI usage grammar, non-vocabulary homographs, UI fragments). A
mutation-witness test proves the guard bites — RED on a deliberately
bare verb word, GREEN once routed. The category is enforced by
construction, not merely possible. This story's build-mode judge round
returned zero findings on its first pass. One disclosed deviation:
`--reaffirm` re-confirms through the waiver record rather than minting a
`reaffirmations/` file (that kind's schema needs an Old!=New AC-text
pair a waive reaffirmation lacks); the judge reviewed and accepted it.
The OUTCOME — the waive verb and an enforced verb vocabulary — holds.
