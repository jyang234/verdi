---
id: spec/disclosures-panel
kind: spec
title: "Disclosures Panel"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-R5-3
problem: { text: "the shared disclosure seam exists (spec/disclosure-seam-v2), so every migrated call site now speaks one vocabulary — but there is still no single place to stand: an operator who wants to know \"what is verdi currently not proving for this checkout, in total?\" must still run each verb and read each surface, because nothing enumerates the checkout's current disclosures in one view", anchor: "#problem" }
outcome: { text: "the workbench serves a disclosures view — and the dex ships its read-only edition through the same compute path — enumerating every current disclosure for the checkout through the internal/disclosure seam, so the operator's \"what is verdi not proving right now\" question has one answer surface", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the workbench serves a disclosures view enumerating every current disclosure for the checkout through the internal/disclosure seam, computed fresh per render, never persisted", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "each enumerated item carries the seam's fields — source, text, severity, stable id — in one consistent rendering", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "the dex ships the read-only edition of the same view, rendered through the same compute path — no separate logic path", evidence: [behavioral], anchor: "#ac-3" }
open_questions:
  - { id: oq-1, text: "ship list_disclosures (MCP) per the spike's recommendation — separate story?", anchor: "#oq-1" }
links:
  - { type: implements, ref: "spec/disclosure-legibility#ac-1" }
  - { type: implements, ref: "spec/disclosure-legibility#ac-2" }
frozen: { at: 2026-07-11, commit: db31d48d0828cc8a8d243faed7ec72e73d62657c, stub_matched: true }
---
# Disclosures Panel

## Problem

`spec/disclosure-seam-v2` delivered the vocabulary half of
`spec/disclosure-legibility`: every migrated disclosure call site now
constructs an `internal/disclosure.Disclosure` at its own decision point
and renders it through the one shared `Render` function, so equivalent
disclosed-unproven states read identically wherever they appear. What does
not exist yet is the *place* half. The feature's own problem statement
still holds at the checkout grain: an operator, a reviewer, or an auditor
who wants the total answer to "what is verdi currently not proving?" must
still tour every surface that might emit a disclosure — run `verdi lint`
and read its notices, start `verdi serve` and read the board chrome, know
which gate conditions can disclose — and mentally union the results. The
seam made the pieces consistent; nothing yet assembles them.

## Outcome

One view, two editions, one compute path. The workbench serves a
disclosures view that enumerates every current disclosure for the checkout
through the `internal/disclosure` seam — the checkout's live "what is
verdi not proving right now" surface — and the dex ships the read-only,
main-only edition of the same view, computed by the same code (05
§Lenses: "the dex ships their read-only, main-only editions, computed the
same way — no separate logic path", the same law the story-page ladder
badges already obey). The enumeration is computed fresh on every render
and never persisted: per the spike's answer
(`docs/spikes/v1/disclosure-enumeration-spike.md`) and the feature's dc-1,
a disclosure is a first-class rendered state reflecting the checkout's
current condition, not a historical log — you cannot honestly persist
"currently unproven".

## AC-1

The workbench serves a disclosures view enumerating every current
disclosure for the checkout through the `internal/disclosure` seam. The
enumeration is computed fresh on each render — it calls the same decision
points the producing surfaces already call (the lint engine's
disclosure-severity findings; the serving process's own disclosed context,
e.g. the review-feed-unavailable state) and collects their `Disclosure`
values — and is never persisted: no file, no cache, no log is written by
rendering the view (the spike's answer + the feature's dc-1). An empty
enumeration renders as an explicit positive claim ("no current
disclosures"), never as a blank page — silence is never a pass.

Evidence: behavioral — a Playwright exerciser confirms the served view
surfaces a disclosure the running checkout actually emits, and Go
exercisers confirm the compute path enumerates fresh per call and writes
nothing.

## AC-2

Each enumerated item carries the seam's fields — source, text, severity,
and the stable content-derived id — in one consistent rendering: every
item, regardless of which producer emitted it, is rendered by the same
shared markup with the same field structure, so recognizing one item
teaches you to recognize all of them (the feature's ac-1 vocabulary law,
applied to the view's own item shape).

Evidence: behavioral — exercisers assert an enumerated item's rendered
markup carries all four seam fields, identically structured across
producers and across both editions.

## AC-3

The dex ships the read-only edition of the same view, rendered through the
same compute path as the workbench edition — one shared enumeration and
one shared item rendering, consumed by both surfaces; never a dex-private
reimplementation (05 §Lenses' no-separate-logic-path law, exactly as the
story-page ladder badges already share their computation with the
workbench story lens). The dex edition is read-only and carries the dex's
own temporal honesty: a living-gated build stamp, since it reflects the
checkout state at build time rather than a live render.

Evidence: behavioral — a Playwright exerciser confirms the built dex site
serves the disclosures page with the same view structure and vocabulary,
read-only, and a Go exerciser confirms both editions render through the
one shared compute path over `internal/disclosure`.

## OQ-1

Ship `list_disclosures` (MCP) per the spike's recommendation — separate
story? The spike (`docs/spikes/v1/disclosure-enumeration-spike.md`)
answered the feature's oq-1 with "yes, build it" and specified the tool's
shape (`tool_list_disclosures.go`, input `{ scope?: ref }`, output
`{ disclosures: Disclosure[] }`, following the `get_board`/
`list_annotations` pattern). That recommendation is carried here honestly,
not scoped here: this story delivers the human-facing view the feature's
ac-2 commits to as its outcome floor; the machine-readable MCP surface is
additive read-surface work (a tenth tool against 05's current nine) with
its own ceremony weight, and belongs to its own story rather than riding
this one's acceptance.
