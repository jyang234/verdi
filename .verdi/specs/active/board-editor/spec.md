---
id: spec/board-editor
kind: spec
title: "Board Editor"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-10
problem: { text: "diagram-proposals ratified an authoring surface (ac-7) and mechanical before-peek/reset (ac-3), but the workbench has no diagram surface at all: the board renders spec projections only, a class: proposal artifact (ratified 02 §Diagram proposals) has no route, no editor, no preview, and nothing binds structural-operations-only, byte preservation, or position refusal to an interactive surface", anchor: problem }
outcome: { text: "a drafting-focus editor on the board for class: proposal diagrams — code pane plus live preview under the one pinned vendored mermaid, failing visible on render errors; a verification rail consuming the extractor's tier and findings; structural operations landing as deterministic source-text edits with no positions anywhere; and mechanical before-peek and reset reproducing a derived proposal's pinned base from digest-verified inputs", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "GET /board/diagram/{name} serves the drafting surface for a class: proposal diagram: a code pane holding the artifact's mermaid source and a live preview rendered client-side by the vendored pinned mermaid asset; source the renderer rejects paints a visible render-error state carrying the renderer's own message — never a blank preview, never a silently retained last-good picture", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "each structural operation — add node, connect (click-click or drag-to-connect), rename inline, delete — POSTs to the editor API and lands as exactly the deterministic source-text edit dc-2's grammar declares (same source + same operation = same bytes); on source outside the operation grammar's flowchart subset the operations are disclosed unavailable while the code pane stays fully live; no operation accepts or writes a position", evidence: [static, behavioral], anchor: ac-2 }
  - { id: ac-3, text: "every editor write path byte-preserves the source (feature ac-2 bound to this surface): a code-pane save stores the pane's bytes exactly, a structural operation changes only the lines its grammar names and leaves every other byte bit-identical, and no write path normalizes, reflows, or round-trips the text through a graph", evidence: [static, behavioral], anchor: ac-3 }
  - { id: ac-4, text: "a derived proposal's before-peek renders the pinned base reproduced from its digest-verified inputs, and reset replaces the working source with that base byte-for-byte through the ordinary save path; a digest mismatch fails visible with no write; neither affordance carries state of its own", evidence: [static, behavioral], anchor: ac-4 }
  - { id: ac-5, text: "the verification rail renders the artifact's coverage tier and per-element findings as consumed from the verification extractor's seam, renders a disclosed verification-unavailable state when the extractor is absent or errors, and never blocks an edit or a save", evidence: [behavioral], anchor: ac-5 }
links:
  - { type: implements, ref: "spec/diagram-proposals#ac-3" }
  - { type: implements, ref: "spec/diagram-proposals#ac-7" }
decisions:
  - { id: dc-1, text: "placement: the editor is its own board surface at /board/diagram/{name} — page, fragment, and POST /board/diagram/{name}/api/{action} — the same routing grammar as /board/spec/{name}; reachable from a spec board's pinned diagram reference card and the corpus page; every write sits behind the same authoring-mode gate as spec-board writes", anchor: dc-1 }
  - { id: dc-2, text: "the structural-op → text-edit grammar is server-computed and line-oriented: add-node appends one line `<id>[\"<label>\"]` with id the lowest unused n<k>; connect appends one line `<from> --> <to>`; rename rewrites only the label text at the node's defining occurrence (identity ids are immutable through the ops, keeping feature dc-5's remove+add diff honest); delete removes the node's defining line plus every edge line naming it (edge delete removes that one line); ops recognize only the flowchart subset this grammar names — any other source the pinned renderer accepts stays code-pane-editable with ops disclosed unavailable, never guessed at, never rewritten", anchor: dc-2 }
  - { id: dc-3, text: "pinned-mermaid vendoring shape: the one existing vendored asset — mermaid 10.9.1 at internal/dex/assets/mermaid/mermaid.min.js with README provenance — served to the workbench through the existing /assets/mermaid.min.js route; the editor vendors no second copy and never touches a CDN; the live preview is a client-side render of the pane text under that asset", anchor: dc-3 }
  - { id: dc-4, text: "the rail consumes, never computes: the workbench defines the consumer-side port (04 §port pattern) whose implementation is the verification-extractor story's deliverable; until wired, or on any extractor error, the rail renders the disclosed unavailable state — verification informs, it never blocks (feature dc-4)", anchor: dc-4 }
  - { id: dc-5, text: "before-peek/reset mechanics: the base is recovered from git history at derived_from.ref's pinned commit; sha256 over the base's canonical graph JSON must equal derived_from.digest before either affordance proceeds; a mismatch renders a disclosed failure and writes nothing; reset writes the recovered base bytes through the same byte-preserving save path — both affordances are pure functions of provenance, with no state of their own (feature ac-3)", anchor: dc-5 }
constraints:
  - { id: co-1, text: "byte preservation (feature ac-2) binds every write path this story adds; no board→graph→mermaid round-trip exists anywhere on this surface (feature dc-1)", anchor: co-1 }
  - { id: co-2, text: "no author-positioned layout: no position field in any editor request, record, or artifact write; drag connects, it never places (feature dc-2, co-3)", anchor: co-2 }
  - { id: co-3, text: "no LLM in any editor computation; the rail consumes extractor findings through its seam and never reimplements flowmap's graph semantics (feature co-1, co-2)", anchor: co-3 }
  - { id: co-4, text: "no network in any test: unit and integration tests are hermetic; Playwright e2e under e2e/ drives the built binary against the vendored asset only — the behavioral register for every editor path", anchor: co-4 }
---
# Board Editor

## Problem

diagram-proposals ratified an authoring surface (its ac-7) and the
mechanical before-peek/reset pair (its ac-3), and dc-7's amendment batch
froze the `class: proposal` schema into 02 §Diagram proposals — but the
workbench has no diagram surface at all. The board renders spec
projections only (`/board/spec/{name}`); a proposal artifact has no
route, no editor, and no preview; and nothing yet binds the feature's
hard authoring rules — structural-operations-only (dc-2), byte
preservation on every write path (ac-2), position refusal (co-3) — to an
interactive surface. Until this story lands, a designer can only author
a proposal by editing a file by hand, with none of the disclosure the
feature exists to provide.

## Outcome

A drafting-focus editor surface on the board for `class: proposal`
diagrams. A code pane accepts any mermaid the pinned renderer accepts; a
live preview renders it under the one vendored mermaid asset and fails
visible on render errors. A verification rail shows the artifact's
coverage tier and per-element findings, consumed from the verification
extractor's seam. Structural operations — add node, connect, rename
inline, delete — each land as a deterministic edit to the source text,
with no positions accepted or stored anywhere. A derived proposal offers
mechanical before-peek and reset, each reproducing the pinned base from
its digest-verified inputs. Every write path byte-preserves the source.

## AC-1

The drafting surface itself. `GET /board/diagram/{name}` renders the
editor page for a `class: proposal` diagram artifact: a code pane
holding the artifact's mermaid source (the body below the frontmatter,
02 §Diagram proposals) and a live preview rendered client-side by the
vendored pinned mermaid asset (dc-3). The pane accepts ANY text; when
the renderer rejects the source, the preview paints a visible
render-error state carrying the renderer's own message — never a blank
preview, and never a silently retained last-good picture, which would
show the author a diagram their source no longer describes. Valid
source renders to an SVG. Behavioral register: Playwright e2e types
valid and invalid source into the pane and asserts the SVG and the
visible error state respectively.

## AC-2

Structural operations as deterministic text edits. Each operation — add
node, connect (click-click or drag-to-connect), rename inline, delete —
POSTs to the editor API (dc-1) and the server computes exactly the
source-text edit dc-2's grammar declares: same source + same operation =
same bytes, every time. On source outside the operation grammar's
flowchart subset (any other mermaid the renderer accepts — sequence,
state, exotic flowchart syntax), the operations are disclosed
unavailable while the code pane stays fully live: the ops never guess
at source they do not parse and never rewrite it to something they do.
No operation accepts or writes a position — a drag that connects two
nodes produces an edge line, and a drag that connects nothing produces
nothing (co-2). Static register: the op transform is a pure function
with table-driven happy- and negative-path unit tests, including the
determinism property. Behavioral register: e2e performs each operation
and asserts the resulting source text in the pane.

## AC-3

Byte preservation, the feature's ac-2 bound to this surface. A
code-pane save stores the pane's bytes exactly. A structural operation
changes only the lines its grammar names and leaves every other byte —
indentation, comments, blank lines, ordering, trailing whitespace —
bit-identical. No write path normalizes, reflows, or round-trips the
text through a graph representation (co-1). Static register: unit tests
prove op edits over adversarial fixtures (pasted diagrams with unusual
but renderer-legal formatting) preserve all untouched bytes, and the
save handler is proven to write its input verbatim. Behavioral
register: e2e saves a pasted diagram and re-reads it bit-identical
through the page.

## AC-4

The mechanical pair for derived proposals (feature ac-3). Before-peek
renders the pinned base — recovered from git history at
`derived_from.ref`'s pinned commit and verified against
`derived_from.digest` (dc-5) — beside the working preview, read-only.
Reset replaces the working source with that base byte-for-byte, written
through the ordinary byte-preserving save path. When the recovered
inputs do not hash to the pinned digest, both affordances fail visible
with a disclosed mismatch and write nothing — a wrong base silently
peeked or reset would be worse than no affordance. Neither affordance
carries state of its own: both are pure functions of the artifact's
provenance fields. Static register: the digest-verified base recovery
is a unit-tested pure function with a table-driven mismatch case.
Behavioral register: e2e peeks and resets a derived fixture proposal
and asserts the reproduced base, plus the disclosed-failure state on a
corrupted-digest fixture.

## AC-5

The verification rail. Alongside the preview, the rail renders the
artifact's coverage tier (full / partial / illustrative, feature dc-3)
and its per-element findings (exists / proposed-new / contradicted /
stale-base, feature ac-1) — consumed from the verification extractor's
seam through the consumer-defined port (dc-4). The verification-
extractor story computes them; this story renders them and never
duplicates the computation (co-3). When the extractor is absent or
errors, the rail renders a disclosed verification-unavailable state —
never a fabricated tier, never a silent blank. The rail never blocks an
edit or a save: verification informs scrutiny, it does not gate
authoring (feature dc-4). Behavioral register: e2e asserts the rail
renders a canned extractor report and the disclosed unavailable state
without one.

## DC-1

Placement and route. The editor is its own board surface at
`/board/diagram/{name}` — page, fragment, and
`POST /board/diagram/{name}/api/{action}` — deliberately the same
routing grammar as the spec board's `/board/spec/{name}` trio, so the
workbench keeps one routing idiom. It is reachable from a spec board's
pinned diagram reference card and from the corpus page. Every write
sits behind the same authoring-mode gate as spec-board writes: a
proposal is authored on a design branch, and a read-only or review
checkout refuses mutations exactly as the spec board does. Rejected
alternative: an editor mode embedded inside `/board/spec/{name}` — a
proposal is a first-class artifact (feature dc-8), not a spec object,
and embedding would force the spec projection to carry a second
document's state.

## DC-2

The structural-op → text-edit grammar — the decision feature ac-7 asks
this story to pre-make. Server-computed (one writer, no client-side
duplicate of the edit logic) and line-oriented:

- **add-node** appends one line `<id>["<label>"]`, at the source's
  prevailing indentation, with `<id>` the lowest unused `n<k>`
  identifier (n1, n2, …) not present in the source.
- **connect** appends one line `<from> --> <to>`.
- **rename** rewrites only the label text inside the brackets at the
  node's defining occurrence (a bare node `A` gains one: `A["label"]`).
  Identity ids are immutable through the ops — renaming never touches
  `<id>`, so feature dc-5's remove+add diff semantics stay honest and
  no op ever cascades through the document.
- **delete** of a node removes the node's defining line and every edge
  line naming it; delete of an edge removes that one line.

The ops recognize only the flowchart subset this grammar names. Any
other source the pinned renderer accepts stays fully code-pane-editable
with the ops disclosed unavailable (ac-2) — the grammar never guesses,
and a partial parse never rewrites what it did not understand.

## DC-3

Pinned-mermaid vendoring shape. The pin already exists: mermaid 10.9.1
vendored at `internal/dex/assets/mermaid/mermaid.min.js` with README
provenance (version, source URL, sha256, license), embedded by
`internal/dex` and served to the workbench through the existing
`/assets/mermaid.min.js` route (one copy — two surfaces). The editor
vendors no second copy, pins no second version, and never touches a
CDN — artifacts and tests stay hermetic. The live preview is a
client-side render of the pane text under that asset, the same client
the dex and board pages already load; "same source, same picture" holds
across the editor, the board, and the dex because it is literally the
same renderer bytes (feature co-3).

## DC-4

The rail consumes, never computes. The workbench defines the
consumer-side port for verification reports (04 §port pattern —
interfaces at the consumer), and the verification-extractor story's
deliverable implements it. Until it is wired, or on any extractor
error, the rail renders the disclosed verification-unavailable state.
This keeps the two stories buildable in either order, keeps flowmap's
graph semantics out of this story entirely (feature co-2), and keeps
verification non-blocking (feature dc-4): the only hard failures on
this surface are strict-decode failures of the artifact itself.

## DC-5

Before-peek/reset mechanics. The base source is recovered from git
history at `derived_from.ref`'s pinned commit; sha256 over the base's
canonical graph JSON (02 §Generated artifacts and digests) must equal
`derived_from.digest` before either affordance proceeds. A mismatch —
rewritten history, wrong pin, corrupted input — renders a disclosed
failure and writes nothing. Reset writes the recovered base bytes
through the same byte-preserving save path every other write uses
(co-1), so "reproduces the base exactly" is byte-exact, not
render-equivalent. Both affordances are mechanical consequences of the
provenance fields (feature ac-3): no peek cache, no reset history, no
state of their own.

## CO-1

Byte preservation (feature ac-2) binds every write path this story
adds — the save handler, every structural op, and reset. There is no
board→graph→mermaid round-trip anywhere on this surface (feature dc-1):
nothing ever normalizes or regenerates the author's text.

## CO-2

No author-positioned layout, inherited verbatim (feature dc-2, co-3):
no position field in any editor request, record, or artifact write; no
layout sidecar for diagram content; drag connects, it never places.
Layout is renderer-owned under the pinned mermaid version.

## CO-3

No LLM in any editor computation (feature co-1). The rail consumes
extractor findings through its seam and never reimplements flowmap's
graph semantics (feature co-2) — this story contains zero graph
analysis.

## CO-4

No network in any test. Unit and integration tests are hermetic;
Playwright e2e under `e2e/` drives the built binary against the
vendored mermaid asset only — no CDN, no fetch. e2e is the behavioral
register for every editor path this story claims.
