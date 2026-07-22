---
id: attestation/creation-surfaces--ac-4
kind: attestation
title: "AC-4 attested: obligations are born at accept — the X-9 gap closed by construction, proven by dogfood"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/creation-surfaces }
frozen: { at: 2026-07-22, commit: 7c35d887ad9ce0ae355de82d6c3af90bf2fd73ec }
---
# AC-4 outcome attestation

Stand-in operator attests (Phase 2, 2026-07-22): accept can no longer
complete while a story's declared (ac, kind) pair is missing its
obligation. The freeze-moment backstop scaffolds exactly the missing
pairs to disk BEFORE the in-ritual lint gate, stamps each preFlipHead
(identical to the spec's own flip stamp), stages them into the accept
commit itself so the pairing can never be replayed away, and unlinks
only what it newly created on any refusal — a pristine tree, never
orphaned stubs. `verdi obligation author` gives the design branch a
pre-freeze surface through the one shared renderer seam (moved to
`internal/evidence`, byte-identical output proven by the board's own
tests unmodified), refusing outright on any obligation a merge to main
has frozen.

The proof is dogfood, not assertion: the two stories that accepted
AFTER this one merged — spec/cli-creation and spec/verb-surfaces — each
had ALL of their declared obligations auto-scaffolded into their own
accept commits, zero hand-authored. This story was the last in the
build that ever hand-authored an obligation, which is exactly the
outcome it promised. The backstop's coverage predicate is deliberately
STRICTER than VL-020 (it catches at accept-time a misfile VL-011 reds
at lint); the frozen "same predicate as VL-020" wording is queued for
ratification. The OUTCOME — a story is born with its obligations — holds.
