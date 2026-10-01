---
id: spec/wall-canvas-v2
kind: spec
title: "Wall canvas"
owners: [platform-team]
class: story
story: jira:VERDI-WR-2
problem: { text: "On today's wall, threading is a drag nobody discovers, the write affordances hide in a rail of eight panels, and nothing shows what a card can do or what it is threaded to. Declaring a new object takes a dialog.", anchor: problem }
outcome: { text: "Clicking a card selects it and puts everything that can be done to it in a contextual toolbar; its threads light up and everything else recedes. Threading is a drag from a card's pin to a picker of the legal edge types, a new object is declared in place, and the keyboard and a minimap reach every card.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Every card on the wall (object, stub, reference, and sticky) renders at the design's footprint, unrotated, and keeps its receipts: obligation rows, evidence slots, attestation chips, and coverage chips with their existing texts. A stub card carries its slug inside the card as its first line, and a card that a Focus next concern names carries the readiness mark. Yarn draws in two layers: every thread under the cards, and the selection's threads above them.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "Clicking a card selects it: the card and every card threaded to it are emphasized, everything else recedes, and a status pill names the card and its threads, or says it has none. Clicking the wall or the same card clears the selection, and clicking a thread's chip selects the thread.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "In authoring mode the contextual toolbar offers exactly the actions legal for the selection. With nothing selected: the sticky, card, and pin-artifact actions through the existing dialogs, and the yarn key. With a card selected: edit, the thread hint, graduate for a sticky, read in document for an object card or stub, the thread count, and delete. With a thread selected: retype, graduate for a relates thread, and delete. A stub card's toolbar offers no graduate, delete, or retype action. In review and read-only modes the toolbar shows only the yarn key and the read actions.", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "Dragging a pin from one spec object to another opens a type picker offering only the edge types legal for that source and target, each with its consequence; a gate-bearing type asks for confirmation; and choosing writes the edge through the existing typed-edge path and selects it. The picker opens only for a drag between two spec objects: a stub card's pin anchors its coverage yarn but starts no thread, and a sticky's attribution yarn and graduation drops keep their existing paths.", evidence: [behavioral], anchor: ac-4 }
  - { id: ac-5, text: "A slot at the foot of each object column declares a new object in place with the next id the server gives, as one typed operation (Enter declares, Escape cancels), and the existing add-object dialog stays for keyboard-only use. Enter or a double click edits the selected card in place (Enter applies, Escape cancels).", evidence: [behavioral], anchor: ac-5 }
  - { id: ac-6, text: "The keyboard reaches every card and action: the arrows move the selection within and across columns and reveal the card, Enter edits, Escape closes the innermost open thing, and Delete removes the selected card or thread through the existing confirmation. The trash and Delete both refuse a declared stub in plain language. A minimap shows every card and the viewport and moves the viewport when dragged.", evidence: [behavioral], anchor: ac-6 }
constraints:
  - { id: co-1, text: "New behavior ships in new JavaScript assets of at most 64 KiB each, and boardspec.js does not grow (parent co-1).", anchor: co-1 }
  - { id: co-2, text: "Local edits stay optimistic until the server's projection returns, and the projection is not swapped in while a drag, the picker, a card edit, or an add slot is open (the existing applyProjection contract).", anchor: co-2 }
decisions:
  - { id: dc-1, text: "Cards keep their coverage chips' existing texts (parent ac-2). The design's readiness mark (a dot, and an unresolved or no stub chip, on a card a Focus next concern names) is added beside them instead of replacing the uncovered coverage chip as the handoff drew it.", anchor: dc-1 }
  - { id: dc-2, text: "The design's type picker lists six edge types. This story offers only the types legal for the source and target kinds, with each type's consequence label and the confirmation on gate-bearing types (parent co-7; 05 element taxonomy); the six rows are the styling.", anchor: dc-2 }
  - { id: dc-3, text: "Parent dc-13's four closed-story readings bind here: the parked sticky's handwriting, the trash and Delete refusing a declared stub, the stub pin as an anchor but not a handle, and the sticky's attribution yarn and graduation drops.", anchor: dc-3 }
  - { id: dc-4, text: "boardspecrender.go is split by region with wall-strip-and-drawer: this story owns the canvas region, and e2e/tests/helpers.ts changes only in this story (plan step 2, shared files).", anchor: dc-4 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-2" }
  - { type: supersedes, ref: "spec/wall-canvas" }
---
# Wall canvas

## Problem

On today's wall, threading is a drag nobody discovers, the write affordances hide in a rail of eight panels, and nothing
shows what a card can do or what it is threaded to. Declaring a new object takes a dialog.

## Outcome

Clicking a card selects it and puts everything that can be done to it in a contextual toolbar; its threads light up and
everything else recedes. Threading is a drag from a card's pin to a picker of the legal edge types, a new object is
declared in place, and the keyboard and a minimap reach every card.

## What changed from v1

v2 revises spec/wall-canvas through rung 3 of the amendment ladder (conflict/wall-canvas-playwright-producers). v1
declared its 6 Playwright-backed behavioral obligations as unresolved design debt (SI-288), because no producer grammar
existed for a Playwright test, and GLG v3 keeps design debt from crossing verdi build start. The Playwright producer now
exists (SI-292 to SI-294), so each of those obligations names its test as an elaborated producer, titled `wall-canvas ›
<obligation title>` in the file v1 named. The criteria, constraints, and decisions are v1's, unchanged.

## ac-1

Every card on the wall (object, stub, reference, and sticky) renders at the design's footprint, unrotated, and keeps its
receipts: obligation rows, evidence slots, attestation chips, and coverage chips with their existing texts. A stub card
carries its slug inside the card as its first line, and a card that a Focus next concern names carries the readiness
mark. Yarn draws in two layers: every thread under the cards, and the selection's threads above them.

## ac-2

Clicking a card selects it: the card and every card threaded to it are emphasized, everything else recedes, and a status
pill names the card and its threads, or says it has none. Clicking the wall or the same card clears the selection, and
clicking a thread's chip selects the thread.

## ac-3

In authoring mode the contextual toolbar offers exactly the actions legal for the selection. With nothing selected: the
sticky, card, and pin-artifact actions through the existing dialogs, and the yarn key. With a card selected: edit, the
thread hint, graduate for a sticky, read in document for an object card or stub, the thread count, and delete. With a
thread selected: retype, graduate for a relates thread, and delete. A stub card's toolbar offers no graduate, delete, or
retype action. In review and read-only modes the toolbar shows only the yarn key and the read actions.

## ac-4

Dragging a pin from one spec object to another opens a type picker offering only the edge types legal for that source
and target, each with its consequence; a gate-bearing type asks for confirmation; and choosing writes the edge through
the existing typed-edge path and selects it. The picker opens only for a drag between two spec objects: a stub card's
pin anchors its coverage yarn but starts no thread, and a sticky's attribution yarn and graduation drops keep their
existing paths.

## ac-5

A slot at the foot of each object column declares a new object in place with the next id the server gives, as one typed
operation (Enter declares, Escape cancels), and the existing add-object dialog stays for keyboard-only use. Enter or a
double click edits the selected card in place (Enter applies, Escape cancels).

## ac-6

The keyboard reaches every card and action: the arrows move the selection within and across columns and reveal the card,
Enter edits, Escape closes the innermost open thing, and Delete removes the selected card or thread through the existing
confirmation. The trash and Delete both refuse a declared stub in plain language. A minimap shows every card and the
viewport and moves the viewport when dragged.

## co-1

New behavior ships in new JavaScript assets of at most 64 KiB each, and boardspec.js does not grow (parent co-1).

## co-2

Local edits stay optimistic until the server's projection returns, and the projection is not swapped in while a drag,
the picker, a card edit, or an add slot is open (the existing applyProjection contract).

## dc-1

Cards keep their coverage chips' existing texts (parent ac-2). The design's readiness mark (a dot, and an unresolved or
no stub chip, on a card a Focus next concern names) is added beside them instead of replacing the uncovered coverage
chip as the handoff drew it.

## dc-2

The design's type picker lists six edge types. This story offers only the types legal for the source and target kinds,
with each type's consequence label and the confirmation on gate-bearing types (parent co-7; 05 element taxonomy); the
six rows are the styling.

## dc-3

Parent dc-13's four closed-story readings bind here: the parked sticky's handwriting, the trash and Delete refusing a
declared stub, the stub pin as an anchor but not a handle, and the sticky's attribution yarn and graduation drops.

## dc-4

boardspecrender.go is split by region with wall-strip-and-drawer: this story owns the canvas region, and
e2e/tests/helpers.ts changes only in this story (plan step 2, shared files).
