# OVERLAY-NOTES — parked content from testdata/dexoverlay/README.md

Temporary parking file (Task 1.2 of the public-rollout plan): this is the
former `testdata/dexoverlay/README.md`, preserved verbatim-in-substance
after `testdata/dexoverlay/` was folded into this tree and deleted. Its
content belongs in `examples/showcase/README.md` per Task 1.7 — do not
treat this file as a permanent fixture; it is scaffolding until that task
absorbs it and deletes this file.

Original text follows, with path references updated for the fold (paths
that used to be relative to `testdata/dexoverlay/` are now relative to
this directory, `examples/showcase/`; `testdata/corpus/` renamed to
`examples/showcase/` per Task 1.1):

---

Originally: "dexoverlay — V1-P8's dex-only fixture overlay". Additive
fixture content consumed by `internal/dex`'s tests and
`cmd/e2eharness`'s dex provisioning half. It used to live outside
`examples/showcase/` (then `testdata/corpus/`) so that walkers — lint's
corpus-clean gate, corpus decode goldens, align/audit tests — would not
see it as part of the base corpus. As of Task 1.2 it is folded directly
into `examples/showcase/`, added to `layers.txt` as layer 4 (pinning
layer 3's head, mirroring the existing layer-2/layer-3 discipline), and
is now part of the same fixturegit-built history as the rest of the
committed zone.

Layout (paths now relative to `examples/showcase/`):

- `.verdi/specs/active/borrower-update-mobile/deviation-report.md` — a
  LIVING alignment report for the deviating story whose
  `accepted-deviation` finding id equals the story's own `ac-1`
  (`evidence.SpecStale` trigger (a), R4-I-18) — the fixture story that
  carries the `spec-stale` flag the story-page badge renders.
- `.verdi/specs/archive/refi-rate-check-2024/` — a synthetic ROUND-FOUR
  archived quartet (spec, `layout.json` in the board slot, rollup,
  deviation report) so the dex by-story axis renders the round-four form
  (00 §Glossary "the quartet"; 03 §Alignment report round-four note);
  `loan-refi-2023` stays the grandfathered `board.json` form.
- `mr/accepted-pending-build-v2.spec.md` — the candidate v2 spec an OPEN
  supersession MR carries (served by `internal/forge/fake`'s
  `SeedFile`, never written into any store): its `supersession:` block
  amends `ac-2`, so `spec/borrower-update-mobile` (implements ac-1+ac-2)
  gets the `pending-supersession` flag while the stub-matched
  `spec/borrower-update-api` (ac-1 only) stays unflagged. This file
  stays outside `.verdi/` (a forge-seed, never committed into any real
  store) at `examples/showcase/mr/`.
- `.verdi/specs/active/{escrow-notify,escrow-notify-v2,rate-lock,rate-lock-v2}/spec.md`
  — the supersession-chain surface fixtures (feature-rung `rate-lock`
  pair, story-rung `escrow-notify` pair) that prove the board/dex render
  the terminal `superseded` state at both rungs (dc-4). Not documented
  in the original overlay README's file list, but present in
  `testdata/dexoverlay/.verdi/specs/active/` alongside it; preserved
  here for completeness.

Commit SHAs cited by these files reuse layer 3's golden head — real in
any repo that chains the corpus layers, and (per the original note)
never resolved by the dex render paths that read them.
