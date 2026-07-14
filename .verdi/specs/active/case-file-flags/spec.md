---
id: spec/case-file-flags
kind: spec
title: "Case File Flags"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-17
problem: { text: "the case file — the wall's spec-level surface — wears no spec-level state: the spec-stale and pending-supersession ladder flags exist (the dex story-lens renders them) but the board's case file does not, and nothing observes an acceptance-criteria column outgrowing a screen, so the two spec-level truths an author most needs while authoring are invisible exactly where authoring happens", anchor: "#problem" }
outcome: { text: "spec-stale and pending-supersession render as case-file stamps computed by the exact code path the dex story-lens uses (three-valued: flagged, unflagged, or disclosed-unproven), and an acceptance-criteria column whose estimated rendered height at the declared reference-viewport constant exceeds it raises a size-smell badge — an observation, never a rule, with a derivation that discloses its proxy and never cites a client viewport measurement", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "spec-stale and pending-supersession render as stamps on the case file, computed by the same exported entry points the dex story-lens calls — decisionsweep.ScanSpecStale over lint.BuildSnapshot and evidence.PendingSupersession fed by evidence.LoadPendingSupersessionCandidates and evidence.ImplementsByFeature — through the badge compute layer, three-valued: flagged-with-witness, proven-unflagged, or disclosed-unproven when open MRs cannot be enumerated", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "an acceptance-criteria column exceeding the viewport raises the size-smell badge on the case file, where 'exceeding the viewport' is resolved deterministically (dc-1): the declared AC count times the board's declared card geometry, measured against the declared reference-viewport constant — a pure function of pinned inputs whose derivation record discloses the constants, the count, and the computed estimate; an observation, never a rule", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "size-smell is invariant to the client: the badge's presence and its drawer content are identical across different browser viewport sizes, no client measurement feeds the compute, and the drawer never cites an actual client viewport — proven by e2e at two distinct viewport sizes", evidence: [static, behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/wall-receipts#ac-4" }
  - { type: implements, ref: "spec/wall-receipts#ac-5" }
decisions:
  - { id: dc-1, text: "the ac-5 determinism trap is resolved by proxy: 'exceeding the viewport' means the AC column's estimated rendered height — the AC zone's top offset plus the declared AC count times the board layout's declared row pitch (card height + gap) — exceeds the declared reference-viewport-height constant (900, a laptop-class viewport); the estimate reads the spec frontmatter's AC count and the layout package's declared geometry only, never dragged positions and never a measured client viewport, and the derivation record discloses every operand", anchor: "#dc-1" }
  - { id: dc-2, text: "observation-never-a-rule, made concrete: the size-smell badge speaks in the wall's observation register (the multi-claim observation's voice — 'worth a look', never an error), no gate, lint verdict, or write path consumes it, and the reference constant is a declared constant, not configuration — the smallest reversible v1, tunable only by amending this decision", anchor: "#dc-2" }
  - { id: dc-3, text: "division of labor with the badge compute layer: the layer (badge-computes' story) owns computing the ladder-flag values and the one attachment point; this story owns the case-file surface contract — which walls wear which stamps (ladder stamps on story walls, the story-scoped computes they are; size-smell on any spec wall that declares acceptance criteria), their placement and register, and the unproven disclosure's rendering", anchor: "#dc-3" }
  - { id: dc-4, text: "one vocabulary across surfaces: the case-file stamps wear the same flag names the dex story-lens badges wear (spec-stale, pending-supersession), and a disclosed-unproven pending-supersession renders as a case-file disclosure line in the board's notice vocabulary, not as a stamp — unproven is never dressed as a verdict in either direction", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "wall-receipts co-1 carried: the flags compute with no LLM over pinned inputs, and every drawer citation is an input revision, never wall-clock time and never a client measurement", anchor: "#co-1" }
  - { id: co-2, text: "wall-receipts co-2 carried, sharpened by ac-5's own words: size-smell is an observation, never a rule — nothing blocks, gates, or refuses on it, at authoring or MR time", anchor: "#co-2" }
  - { id: co-3, text: "the ac-4 trap: the case file's spec-stale and pending-supersession values MUST come from the same code path the dex story-lens uses (internal/dex lens.go/ladder.go's decisionsweep and evidence entry points) — a lookalike reimplementation is a defect, and static evidence must witness the shared call sites", anchor: "#co-3" }
frozen: { at: 2026-07-14, commit: 22d30fda57e76b240d6e2930b9b4653a290c8bad, stub_matched: true }
---
# Case File Flags

## Problem

The case file — the board-placards lockup that opens every wall, the
board's spec-level surface — wears no spec-level state. The spec-stale
and pending-supersession ladder flags exist and are already rendered by
the dex story-lens (computed from decisionsweep and evidence scans), but
the board's case file shows neither. And nothing anywhere observes an
acceptance-criteria column outgrowing a screen — the oldest smell a
murder board has. The two spec-level truths an author most needs while
authoring are invisible exactly where authoring happens; they surface
later, on the dex page or at MR time, as somebody else's news.

## Outcome

Spec-stale and pending-supersession render as case-file stamps computed
by the exact code path the dex story-lens uses — three-valued: flagged
with witness, proven unflagged, or disclosed-unproven. An
acceptance-criteria column whose estimated rendered height at the
declared reference-viewport constant exceeds it raises a size-smell
badge: an observation, never a rule, with a derivation that discloses
its proxy and never cites a client viewport measurement.

## ac-1

Spec-stale and pending-supersession render as stamps on the case file,
computed by the same exported entry points the dex story-lens calls
(internal/dex/lens.go computeLensData, ladder.go storyLadder):
decisionsweep.ScanSpecStale over a lint.BuildSnapshot, and
evidence.PendingSupersession fed by
evidence.LoadPendingSupersessionCandidates and
evidence.ImplementsByFeature. The values arrive through the badge
compute layer's one attachment point, with the lens's own three-valued
posture: flagged-with-witness (finding ids and counts; MR ids and
touched object ids — the stamps' firing records), proven-unflagged (no
stamp), or disclosed-unproven when no forge or default branch was
available to enumerate open MRs — rendered as a disclosure, never as a
stamp and never as silence.

## ac-2

An acceptance-criteria column exceeding the viewport raises the
size-smell badge on the case file. "Exceeding the viewport" is resolved
deterministically by dc-1's proxy: the declared AC count times the board
layout's declared row pitch, plus the AC zone's top offset, measured
against the declared reference-viewport-height constant. The result is a
pure function of pinned inputs — the spec frontmatter and declared
layout constants — and the badge's derivation record discloses every
operand: the constants by name and value, the AC count, and the computed
estimate. An observation, never a rule (co-2): its copy observes, and
nothing consumes it.

## ac-3

Size-smell is invariant to the client. The badge's presence and its
drawer content are byte-identical across different browser viewport
sizes; no client measurement feeds the compute (it runs server-side over
pinned inputs) and no script injects one afterward; the drawer never
cites an actual client viewport — it cites the declared reference
constant, disclosed as such. Proven by an e2e that loads the same badged
wall at two distinct viewport sizes and asserts the same badge and the
same derivation content.

## dc-1

The ac-5 determinism trap, resolved in this spec. "Exceeding the
viewport" cannot be a client measurement — a badge that appears on a
small laptop and vanishes on a tall monitor is not a pure function of
pinned inputs (wall-receipts co-1) and would make the wall's receipts
unreproducible. The proxy: estimated AC-column height = the AC zone's
top offset + (declared AC count x the layout package's declared row
pitch, card height plus gap); the badge raises when the estimate exceeds
the declared reference-viewport-height constant of 900 — a laptop-class
viewport. The estimate reads the spec frontmatter's AC count and the
board layout's declared geometry constants only: never dragged card
positions (dragging paper around must not create or destroy an
observation about the spec's size) and never a measured client viewport.
The derivation record discloses every operand — the constant names and
values, the count, the estimate — so the proxy is legible in the drawer,
not hidden behind the badge.

## dc-2

Observation-never-a-rule, made concrete. The size-smell badge speaks in
the wall's observation register — the same voice as the open-question
multi-claim observation ("one spike answering many questions is normal;
many spikes on one question is worth a look"): it notes that the AC
column has outgrown a screen and that outcome-shaped ACs this numerous
are worth a scoping look. No gate, lint verdict, or write path consumes
it. The reference constant is a declared constant in code, not
configuration: the smallest reversible v1 choice, tunable only by
amending this decision — a config knob would invite tuning the smell
away instead of reading it.

## dc-3

Division of labor with the badge compute layer, so implementers never
collide: the layer (the badge-computes story) owns computing the
ladder-flag values and the one attachment point in the board's I/O
enrichment tier; this story owns the case-file surface contract — which
walls wear which stamps (the ladder stamps on story walls, since
spec-stale and pending-supersession are story-scoped computes; the
size-smell badge on any spec wall that declares acceptance criteria,
feature and story alike), their placement in the case-file lockup, their
register, and how the disclosed-unproven state renders.

## dc-4

One vocabulary across surfaces. The case-file stamps wear the same flag
names the dex story-lens badges wear — spec-stale, pending-supersession
— so a flag reads identically on the wall and on the dex page (they are
the same computation; they must wear the same name). A
disclosed-unproven pending-supersession renders as a case-file
disclosure line in the board's existing notice vocabulary, not as a
stamp: unproven is never dressed as a verdict in either direction —
neither a scary stamp nor a silent pass.

## co-1

Wall-receipts co-1, carried: the flags compute with no LLM over pinned
inputs — the deviation report, the lint snapshot, the enumerated open
MRs, the spec frontmatter, declared layout constants. Every drawer
citation is an input revision (digest or pinned sha), never wall-clock
time and never a client measurement.

## co-2

Wall-receipts co-2, carried, sharpened by ac-5's own words: size-smell
is an observation, never a rule. Nothing blocks, gates, or refuses on
it, at authoring or MR time; its entire effect is to be seen.

## co-3

The ac-4 trap, named again where it binds: the case file's spec-stale
and pending-supersession values MUST come from the same code path the
dex story-lens uses — the decisionsweep.ScanSpecStale and
evidence.PendingSupersession entry points internal/dex/lens.go and
ladder.go call. A lookalike reimplementation inside the wall is a defect
even if its outputs match today: two logic paths drift, and the flag
would stop meaning the same thing on the two surfaces that both claim to
render it. Static evidence must witness the shared call sites.
