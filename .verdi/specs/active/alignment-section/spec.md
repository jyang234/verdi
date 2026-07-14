---
id: spec/alignment-section
kind: spec
title: "Alignment Section"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-12
problem: { text: "internal/align's build-branch deviation report reconciles a spec's declared boundaries against regenerated reality, but a diagram proposal has no reconciliation at all: an accepted future-state flowchart could sit realized or badly diverged from what actually shipped and the pre-review verdict would never say so — the one ruling that turns diagram-proposals from a drawing feature into a loop (feature ac-5) does not exist yet.", anchor: problem }
outcome: { text: "verdi align's computed section gains a diagram-alignment subsection: every accepted class: proposal diagram in the corpus is regenerated and diffed via verification-extractor's shared comparison — an empty residual renders realized, a non-empty one renders divergent with each delta's witness — folded into the SAME computed findings/digest machinery every other computed finding already uses; every illustrative diagram living in the spec's own body is listed as unverifiable rather than silently dropped. Never judged, never blocking merge on its own — surfaced for review exactly as a boundary deviation is.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "the diagram-alignment computation discovers every accepted class: proposal diagram in the corpus (unowned by any single story, corpus-wide by the schema's own silence on a diagram-to-spec edge) and every illustrative body figure living in the CURRENT spec's own body (dc-8's containment tie), never silently dropping either set", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "each discovered accepted proposal is regenerated and diffed via verification-extractor's shared comparison and stale-base functions (never reimplemented); an empty residual yields one computed Finding disclosing realized, a non-empty residual yields one computed Finding disclosing divergent with every delta's candidate witness folded into its text; every Finding also discloses the proposal's own full/partial coverage tier (verification-extractor ac-1) alongside realized/divergent, so a partial-coverage realized claim never reads as a fully-verified one", evidence: [static, behavioral], anchor: ac-2 }
  - { id: ac-3, text: "RenderBody gains a \"### Diagram alignment\" subsection under \"## Computed\", mirroring the existing boundary-diff subsection's shape exactly: one line per accepted proposal (its full/partial coverage tier plus realized or divergent, with deltas), one line per illustrative diagram (unverifiable, never a finding needing disposition — informational context, mirroring the boundary-diff's own undispositioned-context precedent) — never omitted, even when the set is empty, and the coverage tier is never dropped from the rendered line even when it would otherwise read identically to a full-coverage realized proposal", evidence: [static, behavioral], anchor: ac-3 }
  - { id: ac-4, text: "diagram findings ride the SAME findings list ComputeDigest already covers — no second digest field — and a reviewer can disposition a real divergence (fixed/accepted-deviation) through the existing mechanism exactly as a boundary finding, proving the review loop closes end to end for a genuinely accepted, genuinely diverged proposal", evidence: [static, behavioral, attestation], anchor: ac-4 }
links:
  - { type: implements, ref: "spec/diagram-proposals#ac-5" }
decisions:
  - { id: dc-1, text: "scope asymmetry, schema-driven: a class: proposal diagram carries no ownership edge to any spec in the ratified 02 diagram-artifact FRONTMATTER FIELDS (id/kind/class/status/scope/derived_from — the last already nesting its own base-digest sub-field, {ref, digest}, not a separate top-level key — what the artifact itself decodes). dc-7 ratifies four items as one amendment batch, but only three are frontmatter fields (status vocabulary, derived_from-plus-digest as the one nested field, and scope); the fourth, the illustrative marker, is dc-8's own explicit reading of itself: 'deliberately NOT a new edge type... a body figure, not a first-class artifact' — dc-7 groups a document-location convention into its batch banner, it does not thereby make that convention a diagram-artifact field, so this enumeration's omission of it follows dc-8's own words, not a reinterpretation of dc-7. ac-1 therefore treats every accepted proposal in the corpus as in-scope for every alignment run, corpus-wide, rather than inventing an ownership relationship the schema does not define (the smallest-invention reading; a future scope-to-impacts intersection is a disclosed possible refinement, out of v1). An illustrative diagram DOES have a defined tie — dc-8's body-figure containment, restated by illustrative-class's own dc-4 (\"a fenced body figure is tied to its spec by living in that spec's body\") — so illustrative discovery is correctly scoped to the CURRENT spec's own body only, and is a wholly distinct concept from a class: proposal artifact's own full/partial coverage tier (dc-3 below): illustrative-class's own dc-2 (already accepted, spec/illustrative-class) settles this precisely — illustrative is decided BY CLASS, before any extractor call ('a diagram-kind artifact without class: proposal is illustrative BY CLASS; no extractor call'), so a class: proposal artifact never reaches an illustrative outcome by construction; it is routed to the extractor, which verification-extractor's own dc-1 defines as returning full or partial only. A proposal is always full or partial, never illustrative — not this story's own invention, but the composed effect of two already-ratified sibling decisions", anchor: dc-1 }
  - { id: dc-2, text: "regeneration and comparison are never reimplemented here: this story calls verification-extractor's shared extraction/comparison function (the exists/proposed-new/kept-but-gone three-way result) and its stale-base digest comparison directly, over the SAME upstream.Runner seam internal/align's existing Compute already threads through (no second exec path, no second graph-JSON decode) — CLAUDE.md's one-source-of-truth rule applied to this feature's own co-1/co-2 (no LLM, no reimplemented graph semantics)", anchor: dc-2 }
  - { id: dc-3, text: "one computed Finding per accepted proposal (kind: computed, id \"diagram-<name>\", disposition-eligible exactly like a boundary finding — including a positive/no-issue realized outcome, mirroring declaredBoundaryFinding's existing precedent of producing a Finding even for the holds/no-problem case) folds every divergence delta and its candidate witness into Finding.Text (mirroring how a violated boundary folds its reason into Text — the schema carries no separate witness field, by the same existing precedent). Every Finding's text ALSO states the proposal's full/partial coverage tier alongside realized/divergent (e.g. \"realized (full coverage)\" vs \"realized (partial coverage — 2 elements excluded from comparison)\") — parent dc-3's three-valued coverage disclosure applies to this section exactly as it applies to rendering (illustrative-class ac-3's precedent, restated here for text rather than badges): a partial-coverage proposal's clean diff must never read identically to a fully-verified one. Illustrative diagrams are NOT findings — no deviation claim is being made about them, so nothing needs a fixed/accepted-deviation disposition — they render as supporting, undispositioned context exactly the way ServiceBoundaryDiff already does; no new Finding kind, no new disposition value", anchor: dc-3 }
  - { id: dc-4, text: "attestation is scoped to ac-4 alone, not the whole story: the human-judgment floor this slice adds beyond what deviation.go's existing fixed/accepted-deviation machinery already provides is not a new KIND of judgment (dispositioning a diagram finding is mechanically identical to dispositioning a boundary finding, already proven elsewhere) — it is the END-TO-END proof that a real reviewer, faced with a genuinely accepted proposal that has genuinely diverged, can see it surfaced here and act on it through the pre-existing mechanism. That is an attestation claim (an operator affirms the loop actually closed on a real case), not a static or behavioral one, so it rides ac-4 only", anchor: dc-4 }
constraints:
  - { id: co-1, text: "no LLM anywhere in this story's code (parent co-1): the diagram-alignment subsection is entirely computed, added to the SAME digest-covered findings list, never touching RunJudged/JudgedInput at all", anchor: co-1 }
  - { id: co-2, text: "no network in any test (parent co-2): the discovery, regeneration, and rendering are exercised over a fixture corpus (accepted proposals with a known realized case and a known divergent case, plus a fixture spec body carrying an illustrative fenced block) through internal/upstream's existing fake-Runner seam", anchor: co-2 }
---
# Alignment Section

## Problem

`internal/align`'s build-branch deviation report reconciles a spec's
declared boundaries against regenerated reality, but a diagram proposal
has no reconciliation at all: an accepted future-state flowchart could sit
realized or badly diverged from what actually shipped and the pre-review
verdict would never say so. The one ruling that turns diagram-proposals
from a drawing feature into a loop (feature `ac-5`) does not exist yet.

## Outcome

`verdi align`'s computed section gains a diagram-alignment subsection:
every accepted `class: proposal` diagram in the corpus is regenerated and
diffed via verification-extractor's shared comparison — an empty residual
renders `realized`, a non-empty one renders `divergent` with each delta's
witness — folded into the SAME computed findings/digest machinery every
other computed finding already uses. Every illustrative diagram living in
the spec's own body is listed as unverifiable rather than silently
dropped. Never judged, never blocking merge on its own — surfaced for
review exactly as a boundary deviation is.

## AC-1

The diagram-alignment computation discovers every accepted
`class: proposal` diagram in the corpus (unowned by any single story,
corpus-wide by the schema's own silence on a diagram-to-spec edge) and
every illustrative body figure living in the CURRENT spec's own body
(`dc-8`'s containment tie). Neither set is ever silently dropped — an
empty set of either kind still renders its subsection, explicitly empty,
never omitted (constitution: silence is never a pass). Evidence: static
(the two discovery functions are named and their scopes documented) +
behavioral (a fixture corpus with two accepted proposals and a fixture
spec body with one fenced illustrative block, asserting both are
discovered and neither an unrelated proposal nor an unrelated spec's body
figure leaks in).

## AC-2

Each discovered accepted proposal is regenerated and diffed via
verification-extractor's shared comparison and stale-base functions —
never reimplemented (`dc-2`). An empty residual yields one computed
Finding disclosing `realized`; a non-empty residual yields one computed
Finding disclosing `divergent`, with every delta's candidate witness
folded into its text. Every Finding also discloses the proposal's own
full/partial coverage tier (verification-extractor `ac-1`) alongside
realized/divergent (`dc-3`), so a partial-coverage proposal's clean diff
is never presented as indistinguishable from a fully-verified one.
Evidence: static (the call site invoking verification-extractor's
exported comparison/digest functions, with no parallel graph-diff logic
in this story's own code) + behavioral (a fixture proposal whose truth is
unchanged since acceptance disclosing `realized (full coverage)`, one
with a fixture-scripted removed node disclosing `divergent` with that
node's candidate witness commit named, and one with a deliberately
partial-coverage source disclosing its clean diff as
`realized (partial coverage — N elements excluded)`, never as a bare
`realized`).

## AC-3

`RenderBody` gains a `### Diagram alignment` subsection under
`## Computed`, mirroring the existing `### Boundary diff vs acceptance
baseline` subsection's shape exactly: one line per accepted proposal (its
full/partial coverage tier plus `realized` or `divergent`, with deltas),
one line per illustrative diagram (`unverifiable`, never a finding
needing disposition — informational context, mirroring the
boundary-diff's own undispositioned-context precedent, `dc-3`). The
coverage tier is never dropped from the rendered line, even when it would
otherwise read identically to a full-coverage realized proposal. The
subsection renders even when both sets are empty, reading e.g. "(no
accepted proposals)" / "(no illustrative diagrams in this spec's body)"
rather than vanishing. Evidence: static (the render function's shape,
parallel to `renderBaselineDiffs`) + behavioral (a golden-text test over
a fixture report with one full-coverage realized proposal, one divergent
proposal, one partial-coverage realized proposal, and one illustrative
diagram, asserting the exact rendered subsection distinguishes all four).

## AC-4

Diagram findings ride the SAME `findings:` list `ComputeDigest` already
covers — no second digest field, no parallel provenance record — so the
diagram-alignment section is exactly as recomputable and tamper-evident
as every other computed finding. A reviewer can disposition a real
divergence (`fixed`/`accepted-deviation`) through the existing mechanism
exactly as a boundary finding. Evidence: static (the digest call site
passes the extended findings slice, unchanged signature) + behavioral (a
round-trip test: generate, disposition the divergent finding, regenerate,
confirm the disposition survives via the existing `PreserveDispositions`
path) + attestation (an operator affirms that, given a real accepted
proposal deliberately diverged from a real build, `verdi align` surfaces
the divergence in this section and the operator's own
`fixed`/`accepted-deviation` disposition is honored end to end — the
review loop closes on a genuine case, not just a fixture).

## DC-1

Scope asymmetry, schema-driven. A `class: proposal` diagram carries no
ownership edge to any spec in the ratified 02 diagram-artifact FRONTMATTER
FIELDS (id/kind/class/status/scope/`derived_from` — the last already
nesting its own base-digest sub-field, `{ref, digest}`, not a separate
top-level key — what the artifact itself decodes). `dc-7` ratifies four
items as one amendment batch, but only three are frontmatter fields
(status vocabulary, `derived_from`-plus-digest as the one nested field,
and scope); the fourth, the illustrative marker, is
`dc-8`'s own explicit reading of itself: "deliberately NOT a new edge
type... a body figure, not a first-class artifact" — `dc-7` groups a
document-location convention into its batch banner, it does not thereby
make that convention a diagram-artifact field, so this enumeration's
omission of it follows `dc-8`'s own words, not a reinterpretation of
`dc-7`. `AC-1` therefore treats every accepted proposal in the corpus as
in-scope for every alignment run, corpus-wide, rather than inventing an
ownership relationship the schema does not define (the smallest-invention
reading; a future scope-to-`impacts` intersection is a disclosed possible
refinement, out of v1). An illustrative diagram DOES have a defined tie —
`dc-8`'s body-figure containment, restated by illustrative-class's own
`dc-4` ("a fenced body figure is tied to its spec by living in that
spec's body") — so illustrative discovery is correctly scoped to the
CURRENT spec's own body only, and is a wholly distinct concept from a
`class: proposal` artifact's own full/partial coverage tier (`DC-3`
below): illustrative-class's own `dc-2` (already accepted,
`spec/illustrative-class`) settles this precisely — illustrative is
decided BY CLASS, before any extractor call ("a diagram-kind artifact
without `class: proposal` is illustrative BY CLASS; no extractor call"),
so a `class: proposal` artifact never reaches an illustrative outcome by
construction; it is routed to the extractor, which verification-extractor's
own `dc-1` defines as returning full or partial only. A proposal is always
full or partial, never illustrative — not this story's own invention, but
the composed effect of two already-ratified sibling decisions.

## DC-2

Regeneration and comparison are never reimplemented here: this story
calls verification-extractor's shared extraction/comparison function (the
exists/proposed-new/kept-but-gone three-way result) and its stale-base
digest comparison directly, over the SAME `upstream.Runner` seam
`internal/align`'s existing `Compute` already threads through (no second
exec path, no second graph-JSON decode) — CLAUDE.md's one-source-of-truth
rule applied to this feature's own `co-1`/`co-2` (no LLM, no reimplemented
graph semantics).

## DC-3

One computed Finding per accepted proposal (`kind: computed`, id
`diagram-<name>`, disposition-eligible exactly like a boundary finding —
including a positive/no-issue `realized` outcome, mirroring
`declaredBoundaryFinding`'s existing precedent of producing a Finding even
for the holds/no-problem case) folds every divergence delta and its
candidate witness into `Finding.Text` (mirroring how a violated boundary
folds its reason into `Text` — the schema carries no separate witness
field, by the same existing precedent). Every Finding's text ALSO states
the proposal's full/partial coverage tier alongside realized/divergent
(e.g. "realized (full coverage)" vs. "realized (partial coverage — 2
elements excluded from comparison)") — the parent feature's `dc-3`
three-valued coverage disclosure applies to this section exactly as it
already applies to rendering (illustrative-class `ac-3`'s precedent,
restated here for text rather than badges): a partial-coverage proposal's
clean diff must never read identically to a fully-verified one.
Illustrative diagrams are NOT findings — no deviation claim is being made
about them, so nothing needs a `fixed`/`accepted-deviation` disposition —
they render as supporting, undispositioned context exactly the way
`ServiceBoundaryDiff` already does; no new Finding kind, no new
disposition value.

## DC-4

Attestation is scoped to `AC-4` alone, not the whole story. The
human-judgment floor this slice adds beyond what `deviation.go`'s
existing `fixed`/`accepted-deviation` machinery already provides is not a
new KIND of judgment (dispositioning a diagram finding is mechanically
identical to dispositioning a boundary finding, already proven elsewhere)
— it is the END-TO-END proof that a real reviewer, faced with a genuinely
accepted proposal that has genuinely diverged, can see it surfaced here
and act on it through the pre-existing mechanism. That is an attestation
claim (an operator affirms the loop actually closed on a real case), not
a static or behavioral one, so it rides `AC-4` only.

## CO-1

No LLM anywhere in this story's code (parent `co-1`): the
diagram-alignment subsection is entirely computed, added to the SAME
digest-covered findings list, never touching `RunJudged`/`JudgedInput` at
all.

## CO-2

No network in any test (parent `co-2`): the discovery, regeneration, and
rendering are exercised over a fixture corpus (accepted proposals with a
known realized case and a known divergent case, plus a fixture spec body
carrying an illustrative fenced block) through `internal/upstream`'s
existing fake-`Runner` seam.
