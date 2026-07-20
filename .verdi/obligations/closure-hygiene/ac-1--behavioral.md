---
id: obligation/closure-hygiene--ac-1--behavioral
kind: obligation
title: "Three fixturegit repositories — pattern-a RED, pattern-b RED, and GREEN — prove each witness line and the exit-code split"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/closure-hygiene" }
frozen: { at: 2026-07-20, commit: f8c298d3ad712ead9c108d707a10c49547a440ce }
---
# Three fixturegit repositories — pattern-a RED, pattern-b RED, and GREEN — prove each witness line and the exit-code split

The behavioral evidence must drive three separate fixturegit repositories.

RED pattern-a: a fixture whose default branch carries an active-zone spec
`status: accepted-pending-build` (for example `widget`), plus a local
`close/widget` branch — unmerged — whose own tip commit moves
`.verdi/specs/active/widget/spec.md` to
`.verdi/specs/archive/widget/spec.md` (mirroring the real
`close/showcase-corpus-renovation` shape named in the spec's own problem
statement). The test asserts `verdi audit`'s `== Closure hygiene audit ==`
section prints a witness line naming the spec, the `close/widget` branch,
and its tip SHA, and that the process exits 1.

RED pattern-b: a fixture with a `class: feature` `status:
accepted-pending-build` spec declaring `stubs[]` whose every slug has a
matching `.verdi/specs/archive/<slug>/spec.md` at `status: closed`. The
test asserts the section prints a witness line naming the feature and its
fully-realized stub set, and that this fixture alone — no pattern-a
instance present — exits 0, not 1 (dc-3).

GREEN: every active-zone spec's status consistent with git reality,
including one `status: superseded` spec left in place, unarchived. The
test asserts neither witness line appears, the section reports clean, and
the run exits 0.
