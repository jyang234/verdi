---
id: spec/evidence-slot
kind: spec
title: "Evidence Slot"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-16
problem: { text: "a story AC card already discloses what each declared evidence kind DEMANDS (the obligation rows) but not what each kind HOLDS: the fold already computes per-kind record presence, yet the wall renders none of it, so an author cannot see that a declared kind has no folded record until the matrix or the MR gate says so", anchor: "#problem" }
outcome: { text: "an acceptance-criterion card renders its declared evidence kinds with their fold-derived record state, and an empty slot — a declared kind with no current folded record, by the fold's own definition — badges with a full derivation record, disclosed and never blocking, extending the existing per-kind obligation row rather than duplicating it", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a story AC card's evidence slot renders one entry per declared evidence kind, and 'empty' is the real fold's definition — a declared kind for which the current record set holds no record of that kind (the fold's per-kind no-record state over evidence.Current of the loaded records; for the attestation kind, no attestation on disk) — never a wall-side reimplementation of the fold", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "an empty evidence slot badges through the badge compute layer with a complete derivation record (source fold:empty-slot, inputs naming the spec and the derived-tree location probed with their revisions, records disclosing what was found), and the badge is disclosure, never blocking: no authoring action is refused because a slot is empty", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "the evidence slot extends the card's existing per-kind obligation row rather than adding a second per-kind list: each declared kind reads as ONE row carrying both what the kind demands (the obligation) and what it holds (the record state), rendered by the one board renderer and proven visually coherent by Playwright", evidence: [static, behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/wall-receipts#ac-3" }
decisions:
  - { id: dc-1, text: "'empty' is defined against the real evidence fold and only there: records load through the fold's own loader from the derived tree, reduce through evidence.Current, and a kind is empty exactly when that current set holds no record of the kind (attestation: no attestation file on disk) — a story wall with no derived tree at all is the ordinary authoring state and renders every declared kind as a calm empty slot, not an alarm", anchor: "#dc-1" }
  - { id: dc-2, text: "the slot state joins the obligation row: the row's existing kind label and obligation content gain a record-state chip (empty slots wear the board's established dashed pending vocabulary, the no-stub/no-obligation register), so demand and holdings read as one line per kind and the obligation column ships unduplicated", anchor: "#dc-2" }
  - { id: dc-3, text: "the empty-slot badge rides the badge compute layer's canonical derivation record and attachment point — inputs are the spec (its content digest) and the derived-tree path probed (with the digests of any record files read), records disclose per-kind what was found or that nothing was — never a second attachment path or record shape", anchor: "#dc-3" }
  - { id: dc-4, text: "slot state is card-scoped disclosure, not an AC-status verdict: the wall shows per-kind record presence only and does not render the fold's evidenced/violated/pending AC verdicts — verdicts stay with matrix and the MR gate, keeping this story inside wall-receipts co-2 (readiness is ambient chrome, enforced only at MR time)", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "wall-receipts co-1 carried: the slot computes with no LLM over pinned inputs (the spec, the derived tree, attestation files) and its derivation cites input revisions, never wall-clock time", anchor: "#co-1" }
  - { id: co-2, text: "wall-receipts co-2 carried, this story's own AC text: an empty slot is disclosed, never blocking — no write path, gate, or lint verdict consumes slot state", anchor: "#co-2" }
  - { id: co-3, text: "one fold, one reader: the slot's record loading and per-kind reduction reuse the evidence package's existing loader and Current reduction — a wall-private record scan or a lookalike per-kind reduction is a defect", anchor: "#co-3" }
frozen: { at: 2026-07-14, commit: f81a043ba5dc42f49f05605abd97bfa351839e10, stub_matched: true }
---
# Evidence Slot

## Problem

A story AC card already discloses what each declared evidence kind
DEMANDS — the obligation rows the obligation-wall story shipped render
each kind's authored obligation or a disclosed "no obligation" badge.
But nothing on the wall says what each kind HOLDS. The fold already
computes per-kind record presence (internal/evidence: records load from
the derived tree, reduce through Current, and each declared kind's
no-record state is computed for every matrix row), yet the wall renders
none of it. An author cannot see that a declared kind has no folded
record until `verdi matrix` or the MR gate says so — readiness discovered
late, the exact failure wall-receipts exists to fix.

## Outcome

An acceptance-criterion card renders its declared evidence kinds with
their fold-derived record state. An empty slot — a declared kind with no
current folded record, by the fold's own definition, never a wall-side
approximation — badges with a full derivation record, disclosed and
never blocking. The slot extends the existing per-kind obligation row,
so demand and holdings read as one line per kind and nothing ships
duplicated.

## ac-1

A story AC card's evidence slot renders one entry per declared evidence
kind (the AC's `evidence:` list), and "empty" is the real fold's
definition: records are loaded from the derived tree by the fold's own
loader, reduced through evidence.Current (latest-per-identity, the same
reduction every fold consumer gets), and a kind is empty exactly when
that current set holds no record of the kind — the same per-kind
no-record state the fold's summary strings already expose as
"pending(no-record)". For the attestation kind, empty means no
attestation file on disk for (story, AC), the fold's own
AttestationExists check. Never a wall-side reimplementation: if the
fold's definition of "current" changes, the wall changes with it.

## ac-2

An empty evidence slot badges through the badge compute layer — the one
attachment point — with a complete derivation record: source
fold:empty-slot, inputs naming the spec (content digest) and the
derived-tree location probed (with digests of any record files actually
read), records disclosing per-kind what was found or that nothing was.
The badge is disclosure, never blocking: no authoring action — sticky,
yarn, graduation, commit — is refused because a slot is empty, and no
gate consumes slot state (co-2). A draft story mid-authoring wears its
empty slots calmly, as fact, not as failure.

## ac-3

The evidence slot extends the card's existing per-kind obligation row —
the rows writeObligations renders for each declared kind — rather than
adding a second per-kind list to the card. Each declared kind reads as
ONE row: what the kind demands (the obligation title/prose, already
there) and what it holds (the record-state chip, new). One board
renderer emits it (the page and the fragment stay one code path), and a
Playwright e2e proves the coherence visually: a kind with an obligation
and no record shows both halves on one row, never two disagreeing lists.

## dc-1

"Empty" is defined against the real evidence fold and only there.
Records load through the fold's own loader from the derived tree,
reduce through evidence.Current, and a kind is empty exactly when the
current set holds no record of that kind; attestation-kind emptiness is
AttestationExists' answer. A story wall with no derived tree at all —
the ordinary state during design-branch authoring, since derived records
land at build time — renders every declared kind as a calm empty slot,
not an alarm: the empty wall teaches, it does not scold. The derivation
record still names the location probed, so the receipt is honest about
what was looked at and found absent.

## dc-2

The slot state joins the obligation row. The row's existing kind label
and obligation content gain a record-state chip; empty slots wear the
board's established dashed pending vocabulary — the same register as the
coverage receipt's "no stub" and the obligation row's "no obligation" —
so the wall's whole disclosure vocabulary stays one visual language.
Demand and holdings read as one line per kind, and the obligation
column the obligation-wall story shipped is extended in place, never
duplicated beside itself.

## dc-3

The empty-slot badge rides the badge compute layer's canonical
derivation record and its one attachment point in the board's I/O
enrichment tier — never a second attachment path or a second record
shape. Inputs: the spec file with its content digest, and the
derived-tree path probed with the digests of any record files read.
Records: per-kind, what was found, or the explicit statement that
nothing was. Revisions are digests, never wall-clock (co-1).

## dc-4

Slot state is card-scoped disclosure, not an AC-status verdict. The wall
shows per-kind record presence only; it does not render the fold's
evidenced/violated/pending/waived AC verdicts, which stay with `verdi
matrix` and the MR gate. This keeps the story inside wall-receipts co-2
— readiness is ambient chrome during authoring, enforced only at MR time
— and inside its stub's scope: ac-3 is about slots and their emptiness,
not about projecting the whole verdict ladder onto the wall.

## co-1

Wall-receipts co-1, carried: the slot computes with no LLM over pinned
inputs — the spec, the derived tree's record files, attestation files —
and its derivation record cites input revisions (content digests), never
wall-clock time.

## co-2

Wall-receipts co-2, carried, and this story's own AC text: an empty slot
is disclosed, never blocking. No write path, gate, or lint verdict
consumes slot state; the badge exists so the author sees the hole while
it is cheap to fill, not so anything refuses.

## co-3

One fold, one reader: the slot's record loading and per-kind reduction
reuse the evidence package's existing loader and Current reduction — the
same seams every fold consumer uses. A wall-private scan of the derived
tree or a lookalike per-kind reduction is a defect even if its outputs
match today: two readers of one truth drift.
