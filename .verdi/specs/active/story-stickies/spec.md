---
id: spec/story-stickies
kind: spec
title: "Story Stickies"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-R54-1
problem: { text: "scoping-canvas dc-1 makes stickies the stub authoring surface, but before this slice a story-shaped claim had no typed home on the wall: stubs could only be hand-edited into feature frontmatter, and an untyped note's yarn to an acceptance criterion carried no machine-readable coverage meaning", anchor: "#problem" }
outcome: { text: "a story proto-sticky parks handwritten in the stubs band of a feature wall, its yarn to acceptance criteria is the coverage claim, and graduation mints the declared stub — the yarn becoming the stub's acceptance_criteria list — typeset in place, with empty or illegal claims refused legibly", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "story proto-stickies are a typed annotation offered exactly where the server accepts them — feature-class walls only — and the picker refuses them elsewhere in plain language", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "graduating a story sticky whose yarn reaches acceptance criteria mints the declared stub in the feature's frontmatter with exactly those AC ids as its acceptance_criteria list, and typesets the card in place in the stubs band", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "a zero-yarn graduation is refused legibly and the sticky survives; a yarn pair with no scoping reading (story sticky to a non-AC endpoint) is refused by the type-directed picker", evidence: [behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/scoping-canvas#ac-2" }
decisions:
  - { id: dc-1, text: "retro-decomposition, disclosed: this story is minted after its behavior merged (ledger R4-I-87) — the round-5.4 annotation enum amendment (story/spike proto-stickies) and the wall implementation landed under the owner's orchestration contract before the store's story decomposition existed. This spec describes the built slice; it invents nothing new", anchor: "#dc-1" }
  - { id: dc-2, text: "the sticky is scratch tier first, contract second, exactly the sticky lifecycle the parent's dc-1 names: graduation is the only path from handwritten claim to frontmatter stub, and the attribution yarn stays untyped relates-threads (parent dc-5) — the endpoint pair carries the meaning", anchor: "#dc-2" }
constraints:
  - { id: co-1, text: "no network in any test: the wall paths are exercised by hermetic Playwright specs over provisioned fixture stores and Go unit tests over fixture projections, all under make verify", anchor: "#co-1" }
frozen: { at: 2026-07-28, commit: 42ccf1e2043781b80789fdf1a2eb8c99f90285e3, stub_matched: true }
---
# Story Stickies

## Problem

scoping-canvas dc-1 makes stickies the stub authoring surface, but before
this slice a story-shaped claim had no typed home on the wall: stubs could
only be hand-edited into feature frontmatter (the wall-receipts dogfood
hand-edited every one), and an untyped note's yarn to an acceptance
criterion carried no machine-readable coverage meaning — the wall could
not tell a coverage claim from a passing remark.

## Outcome

A story proto-sticky parks handwritten in the stubs band of a feature
wall. Its yarn to acceptance criteria is the coverage claim. Graduation
mints the declared stub — the yarn becoming the stub's
acceptance_criteria list — and typesets the card in place, the same
register ceremony the rest of the wall teaches. Empty or illegal claims
are refused legibly, and the sticky survives its own refusal.

## AC-1

Story proto-stickies are a typed annotation (the round-5.4 enum
amendment) offered exactly where the server accepts them — feature-class
walls only — and the picker refuses them elsewhere in plain language.
Evidence: behavioral (the annotation decode negative suite in
internal/artifact; e2e 31's "story and spike are offered exactly where
the server accepts them").

## AC-2

Graduating a story sticky whose yarn reaches acceptance criteria mints
the declared stub in the feature's frontmatter with exactly those AC ids
as its acceptance_criteria list, and typesets the card in place in the
stubs band. Evidence: behavioral (the workbench graduation actions;
e2e 32's "story sticky → coverage yarn → graduation typesets the stub in
place").

## AC-3

A zero-yarn graduation is refused legibly and the sticky survives; a
yarn pair with no scoping reading is refused by the type-directed picker
(parent dc-5: each sticky type has exactly one reading). Evidence:
behavioral (e2e 32's zero-yarn and illegal-pair refusal specs).

## DC-1

Retro-decomposition, disclosed: this story is minted after its behavior
merged (ledger R4-I-87). The round-5.4 amendments and the wall
implementation landed under the owner's orchestration contract before the
store's story decomposition existed. This spec describes the built slice;
it invents nothing new.

## DC-2

The sticky is scratch tier first, contract second — exactly the sticky
lifecycle the parent's dc-1 names. Graduation is the only path from
handwritten claim to frontmatter stub, and the attribution yarn stays
untyped relates-threads (parent dc-5): the endpoint pair carries the
meaning, so the closed edge vocabulary is untouched.

## CO-1

No network in any test: the wall paths are exercised by hermetic
Playwright specs over provisioned fixture stores and Go unit tests over
fixture projections, all under make verify.
