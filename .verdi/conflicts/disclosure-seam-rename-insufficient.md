---
id: conflict/disclosure-seam-rename-insufficient
kind: conflict
title: "disclosure-seam's rename-in-place scoping cannot satisfy its own ac-1"
status: open
owners: [platform-team]
links:
  - { type: challenges, ref: spec/disclosure-seam }
---
# Conflict: disclosure-seam's rename-in-place scoping is insufficient

## What is disputed

`spec/disclosure-seam` scoped ac-1 ("the three existing disclosure call
sites ... emit textually identical phrasing for equivalent states") to a
minimal reading: unify the *text* the three call sites already produce, in
place, "without introducing a new shared type or package." That scoping is
disputed as wrong-for-this-story (03 §The amendment ladder rung 3: "the
story's own ACs or approach are invalidated, but the feature ACs it
implements still stand" — `spec/disclosure-legibility#ac-1` is untouched by
this dispute).

## Witness

Build-branch discovery on `feature/disclosure-seam`, commit `58cb8c8`
("build(disclosure-seam): rename-in-place attempt at ac-1 — insufficient,
evidence committed"):

1. Renamed `internal/lint`'s `Finding.String()`, `cmd/verdi` gate's
   `[NOTICE]` tag, and `cmd/verdi/gate_threads.go`'s
   `reviewUnavailableReason` to share a leading `"disclosed-unproven"`
   vocabulary token — the smallest change consistent with the story's own
   scoping.
2. Wrote `TestDisclosureVocabulary_TextuallyIdentical`
   (`cmd/verdi/disclosure_vocabulary_test.go`), ac-1's own declared
   behavioral exerciser, asserting the three renderers produce identical
   text for an equivalent disclosed-unproven fact.
3. The test **fails**, and is committed failing as the record: the three
   call sites hold structurally different data at their own point of
   rendering — `lint.Finding{Rule, Path, Message}` renders one combined
   line; `gateCondition{Name, Reason}` renders a two-line bracketed block;
   `review_unavailable` is a bare sentence with no equivalent fields at
   all. A shared leading token is achievable by rename; identical phrasing
   is not, because there is no shared data shape to render from — exactly
   `spec/disclosure-legibility#dc-1`'s own claim ("the rendered-state shape
   has to exist as a real seam other producers can call into before any
   one view can enumerate through it").
4. Independent corroboration: a self-hosted `verdi align` run against this
   same build (triggered incidentally by `go test ./...` — see
   round5-divergences.md D-11) produced an unprompted judged finding making
   the identical point: the shared token is "three duplicated string
   literals... a future edit to any one site would regress silently with
   all existing tests green."

## Resolution

Story supersession (03 §The amendment ladder rung 3): `spec/disclosure-seam-v2`
supersedes this spec, keeping the same `implements` target
(`spec/disclosure-legibility#ac-1`) and re-scoping ac-1's satisfying
approach to the spike's answer (`docs/spikes/v1/
disclosure-enumeration-spike.md`): a shared `internal/disclosure` package
carrying the `Disclosure` struct and a `Render` function, migrating all
three call sites to construct and render through it.
