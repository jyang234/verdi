---
id: obligation/guide-claims-gate--ac-4--static
kind: obligation
title: "the gate is wired into make verify; its doc comment discloses inventory-only scope (row-to-witness, not yet guide-to-row completeness)"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/guide-claims-gate" }
frozen: { at: 2026-07-20, commit: 1b0976c1039e0aa95e2be207dad8256b6d3b509e }
---
# the gate is wired into make verify; its doc comment discloses inventory-only scope (row-to-witness, not yet guide-to-row completeness)

The static evidence must show `internal/specalign`'s gate registration
wiring `guideclaims_test.go`'s check into the package `make verify`
already runs (visible as an added step in the `Makefile`/`spec-align`
target only if a new top-level target is actually needed — the story's
own outcome text says it runs under existing spec-align otherwise), so
the check is exercised on every full gate run rather than requiring a
separate, rememberable invocation. It must also show the gate's own doc
comment (mirroring `vocabprose_test.go`'s own disclosed-scope comment
convention) stating explicitly, in prose a reader encounters without
having to infer it: this gate proves row-to-witness binding only; it
does NOT prove guide-to-row completeness (that every claim the guide's
own prose makes has a corresponding manifest row at all), since that
needs the guide itself in-repo to compare against — a later-phase, hard
requirement (Task 18's set-equality check) this story does not claim to
satisfy. The doc-comment text itself, reviewable in the diff, is the
static artifact. Green in CI's test step, as part of `make verify`.
