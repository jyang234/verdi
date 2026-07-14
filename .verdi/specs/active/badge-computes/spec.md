---
id: spec/badge-computes
kind: spec
title: "Badge Computes"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-14
problem: { text: "the store already computes what the wall should wear — VL lint findings, the spec-stale and pending-supersession ladder flags — but the board projection carries none of them: no compute layer runs those existing computations at render time, so no card or case file can badge, and any badge added without a derivation record would be an unexplained verdict (wall-receipts dc-2: exactly what trains authors to game it)", anchor: "#problem" }
outcome: { text: "a badge compute layer in the board's I/O enrichment tier runs the existing computes — VL findings scoped to this spec through the dc-3 partition, spec-stale and pending-supersession through the exact code path the dex story-lens uses — and attaches every result to the projection as a badge carrying its full derivation record (rule id, pinned inputs with revisions, firing records), rendered as chips on cards and stamps on the case file, disclosed and never blocking", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the badge compute layer attaches computed badges to the board projection in loadBoard's I/O enrichment tier (the attachObligations posture): the projector stays a pure function of its in-memory inputs, badges are attached after buildProjection, and the same enrichment feeds the full page, the post-mutation fragment, and get_board's LoadProjection", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "VL findings scoped to this spec partition by self-classification (wall-receipts dc-3): a finding that anchors to a rendered object badges that object's card, a spec-level finding badges the case file, and store-structural/plumbing findings and decode failures never reach the projection — the classification is declared at the lint seam by the finding itself, never by a wall-side rule allowlist", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "the spec-stale and pending-supersession badge values are computed by the same exported entry points the dex story-lens calls — decisionsweep.ScanSpecStale over lint.BuildSnapshot, and evidence.PendingSupersession fed by evidence.LoadPendingSupersessionCandidates and evidence.ImplementsByFeature — with the same three-valued outcome: flagged-with-witness, proven-unflagged, or disclosed-unproven when open MRs cannot be enumerated", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "every badge carries a complete derivation record in the canonical schema (dc-2) — source rule id, pinned inputs with their revisions, the firing records — attached at compute time, sufficient for the derivation drawer to render receipts without recomputation, and byte-identical across renders over identical inputs", evidence: [static, behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "badges render as chips on their cards and stamps on the case file in every board mode, and never block: no board write path refuses an action because a badge is present", evidence: [behavioral], anchor: "#ac-5" }
links:
  - { type: implements, ref: "spec/wall-receipts#ac-2" }
  - { type: implements, ref: "spec/wall-receipts#ac-4" }
decisions:
  - { id: dc-1, text: "the compute layer lives in loadBoard's I/O enrichment tier and is the ONE attachment point for every wall badge: this story delivers the layer, the VL-finding badges, and the ladder-flag values; evidence-slot and case-file-flags add their computes (empty slots, size-smell) and surface polish through this same layer and record schema — never a second attachment path", anchor: "#dc-1" }
  - { id: dc-2, text: "the derivation record schema is { source, label, target, inputs: [{name, path, revision}], records: [...], disclosures: [...] } — source is a namespaced rule id (lint:VL-006, ladder:spec-stale, ladder:pending-supersession, fold:empty-slot, observe:size-smell), target is an object id for a card badge or empty for a case-file badge, every input carries a revision, and disclosures carry the unproven inputs", anchor: "#dc-2" }
  - { id: dc-3, text: "finding self-classification is realized as an optional wall-locus declaration on lint.Finding populated by the rule that raises it — an object anchor (that object's card) or a spec-level marker (the case file); a finding that declares no locus stays off the wall even when its Path lies inside this spec's directory, so plumbing rules (status-path, dangling layout keys, data-tracking) and decode failures are excluded fail-closed, and the wall additionally keeps only declared-locus findings scoped to this spec's directory — a new rule classifies itself, exactly wall-receipts dc-3", anchor: "#dc-3" }
  - { id: dc-4, text: "badge visual grammar: card badges are compact chips in the card's existing receipt-row vocabulary (the coverage-chip/obligation idiom); case-file badges are stamps on the case-file lockup beside the class tag; every badge element is a button carrying data-badge-source and its serialized derivation record — the derivation-drawer story's opener contract", anchor: "#dc-4" }
  - { id: dc-5, text: "an input's revision is a content digest (sha256 over the exact bytes read) or an already-pinned digest/sha field the compute carries (the deviation report's covers sha, sweep_provenance.adr_corpus_digest, the digest of each candidate superseding spec fetched from an open MR) — a mutable ref like a bare MR id is a firing record (dc-2's records), never a revision, and no revision is ever wall-clock time: the live wall reads the working tree, which has no commit for dirty state, so the digest is the honest revision", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "wall-receipts co-1 carried: badges compute with no LLM and read only pinned inputs; every revision a derivation record cites is an input revision, never wall-clock time", anchor: "#co-1" }
  - { id: co-2, text: "wall-receipts co-2 carried: badges never block authoring — disclosure, not refusal; no board write path, gate, or lint verdict consumes a wall badge", anchor: "#co-2" }
  - { id: co-3, text: "the ac-4 trap: spec-stale and pending-supersession MUST be computed by the same code path the dex story-lens uses (internal/dex lens.go/ladder.go's calls into decisionsweep and evidence) — a lookalike reimplementation of either computation inside the wall is a defect, and static evidence must witness the shared call sites", anchor: "#co-3" }
frozen: { at: 2026-07-14, commit: b8a2002dcced29c5455e69d6103cafb1a97712fb, stub_matched: true }
---
# Badge Computes

## Problem

The store already computes what the wall should wear — VL lint findings
(internal/lint's engine), the spec-stale and pending-supersession ladder
flags (evidence.SpecStale via decisionsweep.ScanSpecStale;
evidence.PendingSupersession over open MRs) — but the board projection
carries none of them. No compute layer runs those existing computations at
board render time, so no card or case file can badge. And a badge without
a derivation record would be an unexplained verdict — exactly what
wall-receipts dc-2 forbids, because it trains authors to game the badge
rather than fix the cause.

## Outcome

A badge compute layer in the board's I/O enrichment tier (the tier
attachObligations established) runs the existing computes: VL findings
scoped to this spec, partitioned by wall-receipts dc-3, and the two ladder
flags through the exact code path the dex story-lens uses. Every result
attaches to the projection as a badge carrying its full derivation record
— rule id, pinned inputs with revisions, firing records — and renders as
a chip on its card or a stamp on the case file. Disclosed, never blocking.

## ac-1

The badge compute layer attaches computed badges to the board projection
in loadBoard's I/O enrichment tier — the posture attachObligations
(internal/workbench/boardspec.go) established: buildProjection stays a
pure function of its in-memory inputs, and store-derived enrichment runs
after it, in the I/O layer. Because loadBoard feeds the full page render,
the post-mutation fragment, and the exported LoadProjection that
mcpserve's get_board consumes, one attachment point means every surface
sees the same badges — never a page-only or human-only enrichment.

## ac-2

VL findings scoped to this spec partition by self-classification, the
wall-receipts dc-3 contract: a finding that anchors to a rendered object
(an AC, decision, constraint, or open-question card) badges that card; a
spec-level finding with no single object anchor badges the case file;
store-structural and cross-file plumbing findings (gitattributes,
data-tracking, status-in-path, dangling layout keys) and decode failures
(unparsed-island territory) never reach the projection. The
classification is declared at the lint seam by the finding itself (dc-3),
never by a wall-side allowlist of rule ids that would rot as rules are
added.

## ac-3

The spec-stale and pending-supersession badge values are computed by the
same exported entry points the dex story-lens calls (internal/dex/lens.go
computeLensData and ladder.go storyLadder): decisionsweep.ScanSpecStale
over a lint.BuildSnapshot for spec-stale, and evidence.PendingSupersession
fed by evidence.LoadPendingSupersessionCandidates and
evidence.ImplementsByFeature for the race-window flag. The outcome is the
same three-valued record the lens keeps: flagged-with-witness (the finding
ids, MR ids, touched object ids), proven-unflagged, or disclosed-unproven
when no forge (or no default branch) was available to enumerate open MRs —
unproven renders as a disclosure, never as a badge and never as silence.

## ac-4

Every badge carries a complete derivation record in dc-2's canonical
schema — the namespaced source rule id, the pinned inputs with their
revisions, and the records that fired it — attached at compute time.
The record is sufficient for the derivation-drawer story to render
receipts without recomputing anything, and it is deterministic:
byte-identical across renders over identical inputs, with sorted
orderings and no wall-clock or randomness (dc-5, co-1).

## ac-5

Badges render as chips on their cards and stamps on the case file in
every board mode — authoring, review, and read-only alike, the same way
notices render in every mode — and never block: no board write path
(sticky, yarn, graduate, commit) refuses an action because a badge is
present. Disclosure, not refusal (co-2).

## dc-1

The compute layer lives in loadBoard's I/O enrichment tier and is the ONE
attachment point for every wall badge. Division of labor across the
wall-receipts stories, so implementers never collide: this story delivers
the layer itself, the VL-finding badges, and the ladder-flag values with
their derivation records; evidence-slot adds the empty-slot compute and
case-file-flags adds the size-smell compute and the case-file surface's
presentation contract — all through this same layer and dc-2's record
schema, never a second attachment path or a second record shape.

## dc-2

The derivation record schema, load-bearing for every sibling story:

    source:      namespaced rule id — "lint:VL-006", "ladder:spec-stale",
                 "ladder:pending-supersession", "fold:empty-slot",
                 "observe:size-smell"
    label:       the chip's short text
    target:      object id for a card badge; empty for a case-file badge
    inputs:      [ { name, path, revision } ] — every pinned input the
                 compute read, each with its revision (dc-5)
    records:     [ ... ] — one entry per firing record (finding ids and
                 messages, MR ids, touched object ids)
    disclosures: [ ... ] — the unproven inputs, disclosed (three-valued
                 honesty: an unenumerable input is named, never silent)

Receipts, not verdicts (wall-receipts dc-2): the record names what fired
and from what, so the drawer can show the whole derivation.

## dc-3

Finding self-classification is realized as an optional wall-locus
declaration on lint.Finding, populated by the rule that raises it — the
smallest seam that makes wall-receipts dc-3's partition self-maintaining.
A rule declares one of two loci for its findings: an object anchor (the
finding badges that object's card) or a spec-level marker (the finding
badges the case file). A finding that declares NO locus stays off the
wall — fail-closed — even when its Path lies inside this spec's
directory: that is what keeps spec-local plumbing findings (a dangling
layout.json key in this spec's own directory, status-in-path,
data-tracking) and decode failures (unparsed-island territory) in
`verdi lint`/CI and off the wall, reproducing wall-receipts dc-3's third
bucket exactly rather than approximating it by path. On top of the
declared locus, the wall keeps only findings whose Path lies within this
spec's directory (wall-receipts dc-1: "VL lint findings scoped to this
spec"). A new rule classifies itself by what it declares; the wall
changes not at all.

## dc-4

Badge visual grammar. Card badges are compact chips riding the card's
existing receipt-row vocabulary — the same visual register as the
coverage chip and the obligation rows (writeScopingReceipts /
writeObligations in boardspecrender.go), so a card reads as one column of
receipts. Case-file badges are stamps on the case-file lockup, beside the
class tag (writeCaseClassTag's position), so the spec-level surface wears
spec-level state. Every badge element is a button carrying
data-badge-source and its serialized derivation record: that is the
opener contract the derivation-drawer story binds to.

## dc-5

An input's revision is a content digest — sha256 over the exact bytes the
compute read — or an already-pinned digest/sha field the compute carries:
the deviation report's covers sha, sweep_provenance.adr_corpus_digest,
and for pending-supersession the digest of each candidate superseding
spec's bytes as fetched from the open MR's source branch. A mutable ref
is not a revision: a bare MR id names WHICH record fired (dc-2's records
array), while the revision of that input is the digest of the candidate
spec bytes actually read, so the receipt stays re-verifiable against the
state that fired it even after the MR moves. Never a wall-clock time
(co-1): the live wall reads the working tree, and dirty state has no
commit sha, so the content digest is the honest, recomputable revision.

## co-1

Wall-receipts co-1, carried: badges compute with no LLM and read only
pinned inputs — the spec file, layout.json, the deviation and
decision-conflict reports, the lint snapshot, the enumerated open MRs.
Every revision a derivation record cites is an input revision (dc-5),
never wall-clock time.

## co-2

Wall-receipts co-2, carried: badges never block authoring. Disclosure,
not refusal — readiness is ambient chrome, enforced only at MR time. No
board write path, no gate, and no lint verdict consumes a wall badge.

## co-3

The ac-4 trap, named: spec-stale and pending-supersession MUST be
computed by the same code path the dex story-lens uses — the
decisionsweep.ScanSpecStale and evidence.PendingSupersession entry points
internal/dex/lens.go and ladder.go call. A lookalike reimplementation of
either computation inside internal/workbench is a defect even if its
outputs match today: two logic paths drift. Static evidence must witness
the shared call sites.
