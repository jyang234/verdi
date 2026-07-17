---
id: obligation/vocabulary-surfaces--ac-2--behavioral
kind: obligation
title: "Go render tests over boardspecrender/dex/wallbadge label functions, plus a Playwright spec driving a served board over a vocab-rename fixture, prove the board and dex render the model's state/verb/class display names"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/vocabulary-surfaces" }
frozen: { at: 2026-07-17, commit: 6fb386f1c7d53f9318519b7710144c9adcb4e33d }
---
# Go render tests over boardspecrender/dex/wallbadge label functions, plus a Playwright spec driving a served board over a vocab-rename fixture, prove the board and dex render the model's state/verb/class display names

The behavioral evidence must show Go tests over each rendering
surface's own label-producing functions — `internal/workbench/
boardspecrender.go`'s column headers and card chips,
`internal/dex/lens.go`'s lens data, and `internal/wallbadge/ladder.go`'s
ladder badges — each exercised with a resolved `*model.Model` carrying
vocabulary renames and proving the label each surface emits is the
model's `DisplayState`/`DisplayVerb`/class-display resolution rather
than the bare id, with one case per surface so a regression on any
single surface fails independently of the other two. It must also show
one Playwright spec, `e2e/vocabulary.spec.ts` (following `verdi/e2e/`'s
existing fixture-store convention), that drives a served board over a
vocab-rename fixture store and asserts the rendered page's column chip
reads the renamed label — "Ready to build," never
"accepted-pending-build" — proving the rename reaches a real browser's
DOM, not merely a Go-level render function returning the right string.
Green in CI's test step.
