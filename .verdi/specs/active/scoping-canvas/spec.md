---
id: spec/scoping-canvas
kind: spec
title: "Scoping Canvas"
owners: [platform-team]
class: feature
status: draft
problem: { text: "scoping is decided once, at the feature, but the wall gives scoping no surface: stubs are frontmatter-only — invisible on the board and not board-authorable — AC coverage is uncomputed on the wall, spike-to-question attribution has no home anywhere, and instantiating a story from a stub is manual copying", anchor: problem }
outcome: { text: "the feature wall is a scoping canvas: story and spike stickies graduate into stubs, yarn attributes coverage and question-resolution, every AC wears its computed coverage, and a stub instantiates its story on the paved road", anchor: outcome }
acceptance_criteria:
  - { id: ac-2, text: "a story sticky on a feature wall graduates into a declared stub, its yarn to acceptance criteria becoming the stub acceptance_criteria list", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "declared stubs render on the wall as first-class scoping cards with their coverage yarn projected", evidence: [behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "every feature acceptance criterion carries a computed coverage chip — covered by N stubs, or no stub — with no LLM in the computation", evidence: [behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "a spike sticky yarn to open questions records resolution attribution and survives graduation into the spec; one spike may answer many questions, and a question claimed by multiple spikes raises a soft smell — an observation, never a rule", evidence: [behavioral], anchor: "#ac-5" }
  - { id: ac-6, text: "instantiate-story-from-stub scaffolds the story spec pre-filled — title, story-ref prompt, implements edges to the stub acceptance criteria — bound to its stub by slug, with no new provenance record", evidence: [behavioral], anchor: "#ac-6" }
  - { id: ac-7, text: "the feature document stays downward-blind: only stubs enter it, and coverage is always computed, never declared", evidence: [static], anchor: "#ac-7" }
stubs:
  - { slug: story-stickies, acceptance_criteria: [ac-2] }
  - { slug: stub-cards, acceptance_criteria: [ac-3] }
  - { slug: coverage-chips, acceptance_criteria: [ac-4, ac-7] }
  - { slug: spike-resolution, acceptance_criteria: [ac-5] }
  - { slug: instantiate-from-stub, acceptance_criteria: [ac-6] }
constraints:
  - { id: co-1, text: "the stub schema spike-resolution attribution is a ratified 02 amendment that lands before any implementation consumes it", anchor: "#co-1" }
  - { id: co-2, text: "coverage and attribution compute from declared frontmatter only — no LLM, no position, no inference from proximity", anchor: "#co-2" }
decisions:
  - { id: dc-1, text: "story and spike stickies are the stub authoring surface — scratch tier first, graduation mints the frontmatter entry, exactly the sticky lifecycle", anchor: "#dc-1" }
  - { id: dc-2, text: "spike attribution graduates into the stub itself — a spike-flagged stub carrying the open-question ids it resolves — rather than a parallel record kind", anchor: "#dc-2" }
  - { id: dc-3, text: "the stub-to-story binding at instantiation is the ratified slug equality, not a new provenance link", anchor: "#dc-3" }
open_questions:
  - { id: oq-1, text: "the exact 02 stub-schema amendment: an optional resolves list on stubs, or a separate spike-stub shape?", anchor: "#oq-1" }
  - { id: oq-2, text: "do story and spike stickies need their own annotation types, or does graduation-time kind selection suffice?", anchor: "#oq-2" }
  - { id: oq-3, text: "which lane hosts scoping cards — a stubs zone beside references, or the scratch lane?", anchor: "#oq-3" }
---
# Scoping Canvas

## Problem

The two-level model decides scoping once, at the feature — and then gives
that act no surface. Stubs are frontmatter-only: invisible on the wall,
not board-authorable (the wall-receipts dogfood hand-edited every one).
Which ACs a stub covers is written by hand and computed nowhere the
author looks. A spike answering open questions has no attribution home at
all. And turning a stub into its story is manual copying — the paved road
the fast path depends on does not exist as a road.

## Outcome

The feature wall becomes the scoping canvas. Story and spike stickies are
the authoring surface: yarn from a story sticky to acceptance criteria is
the coverage claim, yarn from a spike sticky to open questions is the
resolution attribution, and graduation mints the frontmatter stub —
scratch first, contract second, exactly the sticky lifecycle. Every AC
wears its computed coverage; one spike answering many questions is
normal, many spikes on one question is a soft smell; a stub instantiates
its story pre-filled and slug-bound.

## ac-2

a story sticky on a feature wall graduates into a declared stub, its yarn to acceptance criteria becoming the stub acceptance_criteria list

## ac-3

declared stubs render on the wall as first-class scoping cards with their coverage yarn projected

## ac-4

every feature acceptance criterion carries a computed coverage chip — covered by N stubs, or no stub — with no LLM in the computation

## ac-5

a spike sticky yarn to open questions records resolution attribution and survives graduation into the spec; one spike may answer many questions, and a question claimed by multiple spikes raises a soft smell — an observation, never a rule

## ac-6

instantiate-story-from-stub scaffolds the story spec pre-filled — title, story-ref prompt, implements edges to the stub acceptance criteria — bound to its stub by slug, with no new provenance record

## ac-7

the feature document stays downward-blind: only stubs enter it, and coverage is always computed, never declared

## co-1

the stub schema spike-resolution attribution is a ratified 02 amendment that lands before any implementation consumes it

## co-2

coverage and attribution compute from declared frontmatter only — no LLM, no position, no inference from proximity

## dc-1

story and spike stickies are the stub authoring surface — scratch tier first, graduation mints the frontmatter entry, exactly the sticky lifecycle

## dc-2

spike attribution graduates into the stub itself — a spike-flagged stub carrying the open-question ids it resolves — rather than a parallel record kind

## dc-3

the stub-to-story binding at instantiation is the ratified slug equality, not a new provenance link

## oq-1

the exact 02 stub-schema amendment: an optional resolves list on stubs, or a separate spike-stub shape?

## oq-2

do story and spike stickies need their own annotation types, or does graduation-time kind selection suffice?

## oq-3

which lane hosts scoping cards — a stubs zone beside references, or the scratch lane?
