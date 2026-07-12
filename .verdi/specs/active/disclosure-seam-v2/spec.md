---
id: spec/disclosure-seam-v2
kind: spec
title: "Disclosure Seam"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-R5-2
problem: { text: "spec/disclosure-seam's minimal scoping (rename disclosure text in place, introduce no new shared type or package) cannot satisfy its own ac-1: the three disclosure call sites hold structurally different data at their own point of rendering (a lint Finding's Rule+Path+Message, a gate condition's Name+Reason two-line block, and review_unavailable's bare sentence), so no string rename can make their phrasing textually identical — proven by a failing exerciser and filed as conflict/disclosure-seam-rename-insufficient", anchor: "#problem" }
outcome: { text: "the three disclosure call sites construct a shared Disclosure value and render it through one function, so their phrasing is identical by construction rather than by coincidentally-matching hand-authored strings", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the three call sites render through the one seam (behavioral)", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "equivalent states produce identical text (behavioral)", evidence: [behavioral], anchor: "#ac-2" }
links:
  - { type: implements, ref: "spec/disclosure-legibility#ac-1" }
  - { type: supersedes, ref: "spec/disclosure-seam" }
frozen: { at: 2026-07-11, commit: a66de5b6b656ebe9b123ed0e44aadf38a9ba762d, stub_matched: true }
---
# Disclosure Seam (v2)

## Problem

`spec/disclosure-seam` scoped its ac-1 to a rename-in-place attempt:
unify the three existing disclosure call sites' *text* without introducing
a new shared type or package. That attempt is preserved, frozen, on
`spec/disclosure-seam` (never edited — 03 §The amendment ladder rung 3),
and its own committed evidence — a failing behavioral test,
`TestDisclosureVocabulary_TextuallyIdentical` — proves the scoping
insufficient: the three call sites hold structurally different data at
their own point of rendering, so a shared leading vocabulary token is
achievable by rename, but textually identical phrasing is not.
`conflict/disclosure-seam-rename-insufficient` files this dispute formally,
witnessed by that same failing test and a corroborating self-hosted
`verdi align` judged finding.

## Outcome

The three disclosure call sites — `internal/lint`'s VL-017 notice,
`cmd/verdi` gate's disclosed-condition rendering, and
`internal/mcpserve`/`internal/workbench`'s `review_unavailable` field —
construct a shared `Disclosure` value (source, scope, text) and render it
through one function, so their phrasing is identical **by construction**,
not by three independently hand-aligned string literals that can silently
drift apart on the next edit. This is the spike's answer
(`docs/spikes/v1/disclosure-enumeration-spike.md`), specified precisely
enough that this story makes no further shape decisions: a new
`internal/disclosure` package owning the `Disclosure` struct and a
`Render(Disclosure) string` function, and the three existing call sites
migrated to construct a `Disclosure` at their existing decision point
(no producer's underlying judgment logic changes) and render through the
shared function.

## AC-1

The three call sites render through the one seam: `internal/lint`'s
VL-017 disclosure finding, `cmd/verdi` gate's disclosed
`gateCondition`s (merge and closure gate alike), and
`internal/mcpserve`/`internal/workbench`'s `review_unavailable` rendering
all construct an `internal/disclosure.Disclosure` value at their existing
decision point and produce their printed/returned text via
`internal/disclosure.Render`, never via an independently-authored format
string.

Evidence: behavioral — an exerciser confirms each call site's disclosure
text is produced by `Render`, not a local `fmt.Sprintf`.

## AC-2

Equivalent states produce identical text: given the same underlying
`Disclosure` value (same source, scope, and text), every surface renders
byte-identical output, and a reader who has learned to recognize one
disclosure recognizes all of them — the literal bar
`spec/disclosure-legibility#ac-1` sets, now actually satisfiable because
all three surfaces share one renderer.

Evidence: behavioral — `TestDisclosureVocabulary_TextuallyIdentical` (or
its successor) is rewritten to assert byte-identical output for an
equivalent `Disclosure`, and passes.
