---
id: spec/coverage-chips
kind: spec
title: "Coverage Chips"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-R54-3
problem: { text: "which acceptance criteria the declared stubs cover was knowable only by reading frontmatter by hand: the wall showed no computed answer, and nothing structural prevented coverage from being asserted as prose rather than derived from declarations", anchor: "#problem" }
outcome: { text: "every feature acceptance criterion wears a computed coverage chip — covered by N stubs, or no stub — derived mechanically from declared frontmatter alone, with the feature document staying downward-blind: only stubs enter it, and coverage is always computed at render, never declared or persisted", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "every acceptance criterion row of a feature wall wears a computed coverage chip reading exactly 'no stub', 'covered by 1 stub', or 'covered by N stubs', derived from the declared stubs' acceptance_criteria lists", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "the computation is mechanical and closed over declarations: no LLM, no position input, no proximity inference — the chip is a pure function of the feature frontmatter the render loads", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "the feature document stays downward-blind: only stubs enter it, and no coverage count, chip state, or derived summary is ever written into the spec or layout — coverage exists only in the rendered projection", evidence: [static], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/scoping-canvas#ac-4" }
  - { type: implements, ref: "spec/scoping-canvas#ac-7" }
decisions:
  - { id: dc-1, text: "retro-decomposition, disclosed: minted after its behavior merged (ledger R4-I-87); this spec describes the built slice and invents nothing new", anchor: "#dc-1" }
constraints:
  - { id: co-1, text: "coverage computes from declared frontmatter only (parent co-2) — the chip renderer's inputs are the loaded feature spec's acceptance criteria and stubs, nothing else", anchor: "#co-1" }
  - { id: co-2, text: "no network in any test: chips are exercised by Go unit tests over fixture projections and hermetic Playwright specs, all under make verify", anchor: "#co-2" }
frozen: { at: 2026-07-28, commit: ef6760fcdbc2ccc9a32bb4871508c200dc08e768, stub_matched: true }
---
# Coverage Chips

## Problem

Which acceptance criteria the declared stubs cover was knowable only by
reading frontmatter by hand: the wall showed no computed answer, and
nothing structural prevented coverage from being asserted as prose rather
than derived from declarations — exactly the silently-diverging claim the
parent's ac-7 exists to forbid.

## Outcome

Every feature acceptance criterion wears a computed coverage chip —
covered by N stubs, or no stub — derived mechanically from declared
frontmatter alone. The feature document stays downward-blind: only stubs
enter it, and coverage is always computed at render, never declared or
persisted.

## AC-1

Every acceptance criterion row of a feature wall wears a computed
coverage chip reading exactly "no stub", "covered by 1 stub", or
"covered by N stubs", derived from the declared stubs'
acceptance_criteria lists. Evidence: behavioral (the scoping-canvas
render suite's coverage-chip test; the board e2e suite exercising the
committed corpus fixture's chips).

## AC-2

The computation is mechanical and closed over declarations: no LLM, no
position input, no proximity inference — the chip is a pure function of
the feature frontmatter the render loads (parent co-2's exact bar).
Evidence: static (the renderer compiles, vets, and lint-store clean with
its inputs visibly limited to the loaded spec's declarations) and
behavioral (the same chip tests proving counts change only when
declarations change).

## AC-3

The feature document stays downward-blind: only stubs enter it, and no
coverage count, chip state, or derived summary is ever written into the
spec or layout — coverage exists only in the rendered projection.
Evidence: static (no write path in the chip renderer; the board write
API's strict-decoded request schema admits no coverage field).

## DC-1

Retro-decomposition, disclosed: minted after its behavior merged (ledger
R4-I-87); this spec describes the built slice and invents nothing new.

## CO-1

Coverage computes from declared frontmatter only (parent co-2): the chip
renderer's inputs are the loaded feature spec's acceptance criteria and
stubs, nothing else.

## CO-2

No network in any test: chips are exercised by Go unit tests over
fixture projections and hermetic Playwright specs, all under make verify.
