---
id: spec/derivation-drawer
kind: spec
title: "Derivation Drawer"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-15
problem: { text: "wall badges carry derivation records but nothing opens them — a chip with an unreadable receipt is still an unexplained verdict (wall-receipts dc-2) — and the decision-conflict report records its sweep's own inputs (covers, sweep_provenance) that no wall surface shows, so a stale or partial judged sweep is indistinguishable from a fresh, complete one", anchor: "#problem" }
outcome: { text: "every wall badge opens a derivation drawer naming the rule that fired, the pinned inputs with their revisions, and the records that fired it — server-rendered from the badge's own derivation record, never a second computation — and judged findings surface on the case file wearing their sweep provenance (covers, adr_corpus_digest, decisions_scanned), so a stale or partial sweep looks stale, proven by Playwright", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "every wall badge is an opener: activating a card chip or case-file stamp (pointer or keyboard) opens the derivation drawer naming the badge's source rule id, its pinned inputs with their revisions, and its firing records; closing restores the wall untouched — proven by a Playwright e2e that opens and inspects both a card badge's and a case-file badge's drawer", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "the drawer renders from the badge's attached derivation record alone: one server-side renderer emits each badge's drawer body from the canonical record schema, and no drawer content is recomputed — not by a second server derivation and not by client-side re-derivation; the drawer is a pure function of the record", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "judged findings from the spec's decision-conflict report render on the case file with their sweep provenance: the drawer names the report's covers sha, sweep_provenance.adr_corpus_digest, and decisions_scanned, plus each finding's disposition state — and when covers differs from the current spec revision or decisions_scanned misses a currently-declared decision id, the drawer discloses it visibly, so a stale or partial sweep looks stale", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "the drawer cites input revisions, never wall-clock time: every citation in drawer markup is a digest, sha, or pinned field read from the derivation record, and no drawer render path reads a clock", evidence: [static, behavioral], anchor: "#ac-4" }
links:
  - { type: implements, ref: "spec/wall-receipts#ac-2" }
  - { type: implements, ref: "spec/wall-receipts#ac-6" }
decisions:
  - { id: dc-1, text: "the drawer body is server-rendered per badge as a hidden sibling element of the badge button (the board's established writePlacardFull idiom): assets/boardspec.js only opens, positions, and closes it — the DOM stays the server's own projection, one renderer, no client-side templating of derivation data", anchor: "#dc-1" }
  - { id: dc-2, text: "judged findings enter the wall as ONE case-file chip — the surface wall-receipts ac-6 itself requires — reading the spec's own decision-conflict-report.md, an existing verdi align artifact, so no computation is invented (wall-receipts dc-1); the chip rides the same canonical derivation record as every badge (source align:judged-sweep, inputs = the report with its digest and covers, records = the findings with their disposition state or an explicit undispositioned disclosure) and its drawer additionally stamps the sweep provenance block", anchor: "#dc-2" }
  - { id: dc-3, text: "staleness legibility is comparison, not verdict: the drawer contrasts the report's covers against the current spec content digest and decisions_scanned against the currently-declared decision ids — deterministic equality/set comparisons over pinned inputs, rendered as disclosure lines, never a blocking rule and never a computed 'stale' verdict badge of its own", anchor: "#dc-3" }
  - { id: dc-4, text: "interaction shape: the badge is a button (the opener contract), the drawer is a role=dialog panel with keyboard open and Esc close, available in every board mode — reading a receipt is never a write, so review and read-only walls open drawers exactly as authoring does", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "wall-receipts co-1 carried: the drawer cites input revisions, never wall-clock time; nothing in the drawer computes with an LLM or reads an unpinned input", anchor: "#co-1" }
  - { id: co-2, text: "wall-receipts co-2 carried: the drawer is disclosure — opening and reading it never blocks or mutates anything, in any mode", anchor: "#co-2" }
  - { id: co-3, text: "the drawer consumes the badge compute layer's canonical derivation record schema as-is: if the drawer needs a field the record lacks, the record schema is amended at its one defining seam — the drawer never recomputes or side-channels the missing datum", anchor: "#co-3" }
---
# Derivation Drawer

## Problem

Wall badges carry derivation records, but nothing opens them. A chip
whose receipt cannot be read is still an unexplained verdict — exactly
the failure wall-receipts dc-2 names: an unexplained badge trains authors
to game the badge rather than fix the cause. And the decision-conflict
report already records its judged sweep's own inputs — the covers sha and
the sweep_provenance block (adr_corpus_digest, decisions_scanned) that 03
§Decision-conflict gate mandates precisely "so a partial or stale sweep
is detectable" — but no wall surface shows them, so a stale or partial
sweep is indistinguishable from a fresh, complete one.

## Outcome

Every wall badge opens a derivation drawer naming the rule that fired,
the pinned inputs with their revisions, and the records that fired it —
server-rendered from the badge's own derivation record, never a second
computation. Judged findings surface on the case file wearing their sweep
provenance, so a stale or partial sweep looks stale at a glance. Both
paths are proven by Playwright e2e.

## ac-1

Every wall badge is an opener. Activating a card chip or a case-file
stamp — pointer or keyboard — opens the derivation drawer, which names
the badge's namespaced source rule id, its pinned inputs each with its
revision, and its firing records. Closing the drawer restores the wall
untouched. A Playwright e2e under verdi/e2e/ drives the full loop on
both surfaces: open a card badge's drawer, read rule/inputs/records,
close; open a case-file badge's drawer, same inspection, close.

## ac-2

The drawer renders from the badge's attached derivation record alone —
the canonical schema the badge compute layer defines (badge-computes
dc-2: source, label, target, inputs with revisions, records,
disclosures). One server-side renderer emits each badge's drawer body
from that record; no drawer content is recomputed, not by a second
derivation on the server and not by client-side re-derivation in
boardspec.js. The drawer is a pure function of the record: same record,
same drawer bytes.

## ac-3

Judged findings from the spec's own decision-conflict report
(.verdi/specs/active/<name>/decision-conflict-report.md, strict-decoded
through artifact.DecodeDecisionConflict) render on the case file, and
their drawer stamps the sweep's provenance: the report's covers sha,
sweep_provenance.adr_corpus_digest, and decisions_scanned, plus each
finding's disposition state — a dispositioned finding shows its
disposition and note, an undispositioned one is disclosed as such. When
covers differs from the current spec revision, or decisions_scanned
misses a decision id the spec currently declares, the drawer discloses
the mismatch visibly (dc-3). That is ac-6's whole point: a stale or
partial sweep LOOKS stale, on the wall, without opening the report file.

## ac-4

The drawer cites input revisions, never wall-clock time. Every citation
in drawer markup is a digest, sha, or pinned field read from the
derivation record (the covers sha, adr_corpus_digest, content digests of
inputs read). No drawer render path reads a clock, and no timestamp
appears in drawer markup — the receipt is re-verifiable against its
pinned inputs, not datable against a wall clock (wall-receipts co-1).

## dc-1

The drawer body is server-rendered per badge as a hidden sibling element
of the badge button — the board's established writePlacardFull idiom
(boardspecrender.go: the case-file placards' hidden full-prose element,
toggled client-side, measured never). assets/boardspec.js only opens,
positions, and closes the drawer; it never templates derivation data.
This keeps the board's core invariant intact: the DOM is always the
server's own projection — one renderer, reused by the full page and the
post-mutation fragment, no client-side duplicate.

## dc-2

Judged findings enter the wall as ONE case-file chip — the surface
wall-receipts ac-6 itself requires (its stub assigns ac-6 to this story)
— that reads the spec's own decision-conflict-report.md, an artifact
`verdi align` already writes, so surfacing it invents no computation
(wall-receipts dc-1's rule constrains computation, not the reading of an
existing report). The chip rides the same canonical derivation record as
every other badge, so the parent drawer contract (wall-receipts dc-2:
rule id, pinned inputs with revisions, firing records) holds by
construction: source is the namespaced sweep id (align:judged-sweep),
the pinned inputs are the report itself with its digest and covers sha,
and the firing records are the findings — each listed with id, text,
disposition + note when dispositioned, an explicit undispositioned
disclosure when not, never a silently omitted finding. The sweep
provenance block renders once, at the drawer's head, for all findings it
covers. A spec with no report gets no chip: absence of a sweep is not a
finding, and inventing a "no sweep yet" verdict would be a new
computation.

## dc-3

Staleness legibility is comparison, not verdict. The drawer contrasts
the report's covers sha against the current spec content digest, and
decisions_scanned against the decision ids the spec currently declares —
deterministic equality and set comparisons over pinned inputs, rendered
as disclosure lines ("sweep covers <sha>; this wall renders <digest>",
"dc-6 is not in decisions_scanned"). No blocking rule, and no computed
"stale" verdict badge of its own: the reader judges staleness from the
disclosed contrast, which keeps this story inside wall-receipts dc-1
(nothing new is computed — both operands already exist) and co-2
(disclosure, never refusal).

## dc-4

Interaction shape. The badge is a button — the opener contract the badge
compute layer's visual grammar establishes — and the drawer is a
role=dialog panel: opened by click or Enter/Space, closed by Esc or its
close control, focus moved in on open and restored on close. It is
available in every board mode: reading a receipt is never a write, so
review and read-only walls open drawers exactly as authoring walls do,
the same posture the mode-independent notices already take.

## co-1

Wall-receipts co-1, carried: the drawer cites input revisions, never
wall-clock time. Nothing in the drawer computes with an LLM or reads an
unpinned input; ac-4 is this constraint made testable.

## co-2

Wall-receipts co-2, carried: the drawer is disclosure. Opening and
reading it never blocks or mutates anything, in any mode; no write path
consults drawer or badge state.

## co-3

The drawer consumes the badge compute layer's canonical derivation
record schema as-is. If the drawer needs a field the record lacks, the
schema is amended at its one defining seam and every badge constructor
supplies it — the drawer never recomputes the missing datum, reads a
side channel, or grows a private variant of the record. One schema, one
renderer, receipts that always mean the same thing.
