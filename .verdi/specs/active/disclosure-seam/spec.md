---
id: spec/disclosure-seam
kind: spec
title: "Disclosure Seam"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-R5-2
problem: { text: "the three disclosure call sites verdi already ships — internal/lint's VL-017 notice, cmd/verdi gate's [NOTICE] rendering, and internal/mcpserve/internal/workbench's review_unavailable field — each invented their own wording for the same disclosed-unproven judgment, so an operator reading two of the three cannot tell by phrasing alone that they are looking at the same kind of claim", anchor: "#problem" }
outcome: { text: "the three existing disclosure call sites emit textually identical phrasing for equivalent disclosed-unproven states", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the three existing disclosure call sites (lint notice, gate [NOTICE], mcp/workbench review_unavailable) emit textually identical phrasing for equivalent states", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/disclosure-legibility#ac-1" }
frozen: { at: 2026-07-11, commit: 69d348f6b6dc237189619eccdd56791eca4997ee, stub_matched: true }
---
# Disclosure Seam

## Problem

verdi's own disclosed-unproven states are already real and already
rendered in three places: `internal/lint`'s VL-017 finding prints
`"notice: VL-017 <path>: <message>"`; `cmd/verdi`'s merge and closure
gates print `"[NOTICE] <name>\n       <reason>"` for a disclosed
`gateCondition`; and `internal/mcpserve`/`internal/workbench` surface a
configured-but-unreachable review forge as a free-text
`review_unavailable` string with no shared prefix at all. Each call site
independently decided how to say "this is disclosed, not proven" the
moment it needed to say it. Nothing today forces two of these three
surfaces to agree on wording, so they don't: three prefixes, three shapes,
one underlying judgment.

## Outcome

The three existing disclosure call sites emit textually identical
phrasing for equivalent disclosed-unproven states. This story scopes
itself to the minimal reading of `spec/disclosure-legibility`'s
`disclosure-seam` stub: unify the *text* the three call sites already
produce, in place, without introducing a new shared type or package. If
that reading proves sufficient, ac-1 is satisfied by a rename; if it
proves insufficient, the insufficiency is itself the story's most
important finding.

## AC-1

The three existing disclosure call sites — `internal/lint`'s VL-017
notice, `cmd/verdi` gate's `[NOTICE]` rendering, and
`internal/mcpserve`/`internal/workbench`'s `review_unavailable` field —
emit textually identical phrasing for equivalent disclosed-unproven
states: given the same underlying fact (e.g. "this input could not be
checked because X is absent/unreachable"), a reader looking at any of the
three outputs recognizes the same vocabulary.

Evidence: behavioral — an exerciser test asserts the three call sites'
rendered text agree on the shared vocabulary for an equivalent
disclosed-unproven case.
