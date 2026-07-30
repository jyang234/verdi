---
id: spec/stub-cards
kind: spec
title: "Stub Cards"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-R54-2
problem: { text: "declared stubs were frontmatter-only — invisible on the wall — so the one place scoping is decided rendered no trace of the decomposition it had decided, and the coverage claims stubs carry had no projection a reader could follow", anchor: "#problem" }
outcome: { text: "declared stubs render as first-class scoping cards in a kind-locked stubs zone with their coverage yarn projected, positionable like object cards (the round-5.5 ratification), and sealed against every mutation the spec register forbids", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "every declared stub of a feature wall renders as a scoping card in the dedicated kind-locked stubs zone between open questions and references, its label class-aware (story vs spike)", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "the card's coverage yarn is projected — covers threads to the acceptance criteria a plain stub declares, resolves threads to the open questions a spike stub claims — with yarn-key rows for both planning readings, and the projections carry no graduate, delete, or retype affordance in any mode", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "stub cards take stored positions under the round-5.5 stub:<slug> layout keys — draggable with reload-deterministic persistence — while the trash refuses a declared stub in plain language and a read-only wall refuses the drag with the sealed record's own words", evidence: [behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/scoping-canvas#ac-3" }
decisions:
  - { id: dc-1, text: "retro-decomposition, disclosed: minted after its behavior merged (ledger R4-I-87); this spec describes the built slice and invents nothing new", anchor: "#dc-1" }
  - { id: dc-2, text: "stored stub positions are the round-5.5 ratification (08-revision-notes: stub:<slug> layout keys, VL-018 extended), which resolved the parent dc-6's computed-only deferral on the owner's demand signal — this story's ac-3 binds to the ratified state, not the deferred one", anchor: "#dc-2" }
constraints:
  - { id: co-1, text: "no network in any test: rendering, yarn projection, positions, and refusals are exercised by Go unit tests over fixture projections and hermetic Playwright specs over provisioned stores, all under make verify", anchor: "#co-1" }
frozen: { at: 2026-07-28, commit: 4b57eed8be251564fde4b081ed911a0af9d522e9, stub_matched: true }
---
# Stub Cards

## Problem

Declared stubs were frontmatter-only — invisible on the wall — so the one
place scoping is decided rendered no trace of the decomposition it had
decided, and the coverage claims stubs carry had no projection a reader
could follow: which ACs a stub covers was written by hand and computed
nowhere the author looks.

## Outcome

Declared stubs render as first-class scoping cards in a kind-locked stubs
zone with their coverage yarn projected, positionable like object cards
(the round-5.5 ratification), and sealed against every mutation the spec
register forbids — a stub card can never wear the scratch lane's
handwritten voice (the parent dc-6's register law).

## AC-1

Every declared stub of a feature wall renders as a scoping card in the
dedicated kind-locked stubs zone between open questions and references,
its label class-aware (story vs spike). Evidence: behavioral (the
workbench scoping-canvas render suite's stub-card and zone-label tests;
e2e 30's committed-corpus-fixture spec).

## AC-2

The card's coverage yarn is projected — covers threads to the acceptance
criteria a plain stub declares, resolves threads to the open questions a
spike stub claims — with yarn-key rows for both planning readings, and
the projections carry no graduate, delete, or retype affordance in any
mode: scoping edges are projections of frontmatter, never editable
objects. Evidence: behavioral (the scoping-yarn, yarn-key, and
no-affordance render tests; e2e 32's covers/resolves yarn specs).

## AC-3

Stub cards take stored positions under the round-5.5 stub:<slug> layout
keys — draggable, threads re-anchoring live, the drop persisting
reload-deterministically — while the trash refuses a declared stub in
plain language and a read-only wall refuses the drag with the sealed
record's own words. Evidence: behavioral (the stub-view-position render
test; e2e 30's drag, trash-refusal, and read-only-refusal specs).

## DC-1

Retro-decomposition, disclosed: minted after its behavior merged (ledger
R4-I-87); this spec describes the built slice and invents nothing new.

## DC-2

Stored stub positions are the round-5.5 ratification (08-revision-notes:
stub:<slug> layout keys, VL-018 extended), which resolved the parent
dc-6's computed-only deferral on the owner's demand signal ("why are the
stubs immovable?"). This story's ac-3 binds to the ratified state, not
the deferred one.

## CO-1

No network in any test: rendering, yarn projection, positions, and
refusals are exercised by Go unit tests over fixture projections and
hermetic Playwright specs over provisioned stores, all under make verify.
