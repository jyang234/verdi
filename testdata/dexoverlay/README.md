# dexoverlay — V1-P8's dex-only fixture overlay

Additive fixture content consumed ONLY by `internal/dex`'s tests and
`cmd/e2eharness`'s dex provisioning half — deliberately NOT part of
`testdata/corpus/` (whose walkers — lint's corpus-clean gate, corpus
decode goldens, align/audit tests — would all see any file added there;
mid-wave, V1-P9's audit work owns some of those, so this overlay stays
out of their input set).

Layout mirrors a store root:

- `.verdi/specs/active/borrower-update-mobile/deviation-report.md` — a
  LIVING alignment report for the v2 corpus's deviating story whose
  `accepted-deviation` finding id equals the story's own `ac-1`
  (`evidence.SpecStale` trigger (a), R4-I-18) — the fixture story that
  carries the `spec-stale` flag V1-P8's story-page badge renders.
- `.verdi/specs/archive/refi-rate-check-2024/` — a synthetic ROUND-FOUR
  archived quartet (spec, `layout.json` in the board slot, rollup,
  deviation report) so the dex by-story axis renders the round-four form
  (00 §Glossary "the quartet"; 03 §Alignment report round-four note);
  `testdata/corpus/`'s own `loan-refi-2023` quartet stays the
  grandfathered `board.json` form.
- `mr/accepted-pending-build-v2.spec.md` — the candidate v2 spec an OPEN
  supersession MR carries (served by `internal/forge/fake`'s
  `SeedFile`, never written into any store): its `supersession:` block
  amends `ac-2`, so `spec/borrower-update-mobile` (implements ac-1+ac-2)
  gets the `pending-supersession` flag while the stub-matched
  `spec/borrower-update-api` (ac-1 only) stays unflagged.

Commit SHAs cited by these files reuse `testdata/corpus/`'s layer-3
golden head (`93ddc5bb…`) — real in any repo that chains the corpus
layers, and never resolved by the dex render paths that read them.
