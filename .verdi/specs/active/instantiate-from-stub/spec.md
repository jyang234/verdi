---
id: spec/instantiate-from-stub
kind: spec
title: "Instantiate From Stub"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-R54-5
problem: { text: "turning a declared stub into its story was manual copying: the operator hand-invented the spec directory, frontmatter, and implements edges the stub's declaration already determined — the paved road the fast path depends on did not exist as a road", anchor: "#problem" }
outcome: { text: "a stub instantiates its story on the paved road: a pre-filled scaffold — title, story-ref prompt, implements edges to the stub's acceptance criteria — slug-bound to its stub with no new provenance record, spike stubs instantiating the spike variant, from either the board action or the CLI through one shared core", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "instantiating a plain stub scaffolds the story spec pre-filled — title from the slug, a story-ref prompt, implements edges to exactly the stub's acceptance_criteria — bound to its stub by slug equality with no new provenance record, on a design branch cut and committed by the action", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "a spike stub instantiates the spike variant: spike: true on the story and resolves edges to the open questions the stub claims, the same one-core path as the plain form", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "the board's stub-instantiate action and verdi design start --from-stub call the identical shared core with proven output equality, and refusals are plain: an already-existing branch, a sealed wall, or an unknown stub each name their reason", evidence: [behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/scoping-canvas#ac-6" }
decisions:
  - { id: dc-1, text: "retro-decomposition, disclosed: minted after its behavior merged (ledger R4-I-87) — and this story's own five siblings, itself included, were minted through exactly this paved road, the feature's tool closing its own loop", anchor: "#dc-1" }
  - { id: dc-2, text: "the stub-to-story binding is the ratified slug equality (parent dc-3), never a new provenance link — the scaffold writes exactly the edges the stub declares — implements edges to the parent's acceptance criteria for a plain stub, resolves edges to the parent's open questions for a spike stub (the ratified R4 resolves edge, carrying parent dc-2's graduated attribution into the instantiated spec) — and nothing beyond the stub's own declaration into the link graph", anchor: "#dc-2" }
constraints:
  - { id: co-1, text: "no network in any test: the shared core, both surfaces, the parity assertion, and every refusal are exercised by fixturegit-backed unit tests and hermetic Playwright specs, all under make verify", anchor: "#co-1" }
frozen: { at: 2026-07-28, commit: c37110a4c8813726ac38dbbd209f0e38fdd1a5ca, stub_matched: true }
---
# Instantiate From Stub

## Problem

Turning a declared stub into its story was manual copying: the operator
hand-invented the spec directory, frontmatter, and implements edges the
stub's declaration already determined — the paved road the fast path
depends on did not exist as a road.

## Outcome

A stub instantiates its story on the paved road: a pre-filled scaffold —
title, story-ref prompt, implements edges to the stub's acceptance
criteria — slug-bound to its stub with no new provenance record, spike
stubs instantiating the spike variant, from either the board action or
the CLI through one shared core.

## AC-1

Instantiating a plain stub scaffolds the story spec pre-filled — title
from the slug, a story-ref prompt, implements edges to exactly the
stub's acceptance_criteria — bound to its stub by slug equality with no
new provenance record, on a design branch cut and committed by the
action. Evidence: behavioral (the shared core's plain-form unit tests;
e2e 31's instantiate specs, including the consequence-first ordering and
the serving wall never moving).

## AC-2

A spike stub instantiates the spike variant: spike: true on the story
and resolves edges to the open questions the stub claims, the same
one-core path as the plain form. Evidence: behavioral (the shared core's
spike-form unit tests and the CLI's spike-form case).

## AC-3

The board's stub-instantiate action and verdi design start --from-stub
call the identical shared core with proven output equality, and refusals
are plain: an already-existing branch, a sealed wall, or an unknown stub
each name their reason. Evidence: behavioral (the CLI/board parity
assertion; the negative-path unit tests; e2e 31's second-instantiate
refusal).

## DC-1

Retro-decomposition, disclosed: minted after its behavior merged (ledger
R4-I-87) — and this story's own five siblings, itself included, were
minted through exactly this paved road, the feature's tool closing its
own loop.

## DC-2

The stub-to-story binding is the ratified slug equality (parent dc-3),
never a new provenance link. The scaffold writes exactly the edges the
stub declares — implements edges to the parent's acceptance criteria for
a plain stub, resolves edges to the parent's open questions for a spike
stub (the ratified R4 resolves edge, carrying parent dc-2's graduated
attribution into the instantiated spec) — and nothing beyond the stub's
own declaration into the link graph.

## CO-1

No network in any test: the shared core, both surfaces, the parity
assertion, and every refusal are exercised by fixturegit-backed unit
tests and hermetic Playwright specs, all under make verify.
