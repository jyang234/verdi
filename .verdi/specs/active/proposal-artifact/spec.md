---
id: spec/proposal-artifact
kind: spec
title: "Proposal Artifact"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-8
problem: { text: "the ratified 02 §Diagram proposals schema — class: proposal, the proposed→accepted authored enum, scope, derived_from + base digest — exists only as prose. internal/artifact/diagram.go's DiagramFrontmatter has no class discriminator, no scope/derived_from fields, and its status enum (active/superseded) has no room for proposed/accepted; nothing enforces that a diagram's mermaid body survives every write path byte-for-byte; nothing accepts a proposal at merge the way a spec is accepted; nothing computes the disclosed realized/stale states without writing them. diagram-proposals#ac-2 and #ac-6 are unreachable until this artifact is real.", anchor: problem }
outcome: { text: "DiagramFrontmatter gains an optional class discriminator, a class-conditioned status enum, and optional scope/derived_from fields, strict-decoded through the single internal/artifact seam with unknown fields failing closed; the mermaid body is byte-preserved by every write path; verdi accept admits a class: proposal diagram's proposed→accepted transition at the merge of its own design MR, exactly as it accepts a spec; and a pure, never-written computed-status function discloses the four-value proposed/accepted/realized/stale vocabulary from an externally supplied truth comparison.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "DiagramFrontmatter strict-decodes class, scope, and derived_from{ref,digest}; Validate enforces the class-conditioned status enum (class: proposal → proposed/accepted only; class absent → active/superseded, unchanged) and the class-conditioned frozen requirement (frozen present iff accepted); unknown fields fail closed", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "the mermaid body of every diagram artifact — proposal or incumbent — is byte-preserved across every write path this repo has: no path ever re-serializes, reformats, or round-trips it through any intermediate graph representation", evidence: [static, behavioral], anchor: ac-2 }
  - { id: ac-3, text: "verdi accept diagram/<name> flips a proposed, class: proposal diagram to accepted and writes its frozen stamp, mirroring the spec acceptance ritual's core mechanical flip (merge of the diagram's own design MR is acceptance); it refuses a non-proposal diagram, a non-proposed status, and any target that is not a diagram ref, naming the refusal", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "the four-value disclosed status (proposed/accepted/realized/stale) is computed by a pure function from an accepted proposal plus an externally supplied residual-diff outcome, never written to the artifact; strict decode itself refuses realized/stale as authored frontmatter values", evidence: [static, behavioral], anchor: ac-4 }
  - { id: ac-5, text: "a new lint rule refuses a class: proposal diagram whose derived_from.ref does not resolve to a real diagram in the corpus, or whose derived_from.digest is not sha256:<64-hex>, naming the offending field; id/path agreement for the diagrams/ kind is already generic (VL-002) and needs no new coverage", evidence: [static, behavioral], anchor: ac-5 }
links:
  - { type: implements, ref: "spec/diagram-proposals#ac-2" }
  - { type: implements, ref: "spec/diagram-proposals#ac-6" }
decisions:
  - { id: dc-1, text: "DiagramFrontmatter gains three fields mirroring the ratified 02 shape verbatim: Class (string, omitempty; the only non-empty value is \"proposal\"), Scope (string, omitempty; the flowmap selector, opaque to this story), and DerivedFrom (*DiagramDerivedFrom{Ref, Digest string}, omitempty). Validate branches on Class exactly the way spec.go already branches on a spec's Class (feature/story/component) — an established pattern, not a new one: class: proposal uses a distinct proposalStatuses enum {proposed, accepted} and requires Frozen iff status is accepted; class absent keeps today's diagramStatuses{active, superseded} and requireFrozen(..., false, ...) exactly as it decodes now, so every existing incumbent diagram fixture keeps decoding unchanged", anchor: dc-1 }
  - { id: dc-2, text: "verdi accept dispatches on the target ref's kind: a spec/... ref keeps its existing full ritual untouched; a diagram/... ref is new and narrower — it requires class: proposal and status: proposed, flips status to accepted, and writes frozen: {at, commit} (no stub-match, no CODEOWNERS, no supersedes cascade: a diagram carries no ACs or stubs to match against). AC-6's \"accepts at merge like any spec content\" is read as a LIFECYCLE match (merge of its own design MR is the acceptance event, frozen at that moment) rather than a literal reuse of the story-class stub-matching machinery, which has nothing to bind to on a diagram", anchor: dc-2 }
  - { id: dc-3, text: "the disclosed-vocabulary boundary: this story owns the pure mapping DiagramDisclosedStatus(fm DiagramFrontmatter, residual *ResidualDiff) Status — proposed/accepted pass through unchanged when residual is nil (verification has not run). Once a residual is supplied for an accepted proposal, an empty residual renders realized and a non-empty one renders stale. This story does NOT compute the residual itself (that is verification-extractor's ac-1 diff, consumed here through its own return type, not reimplemented — co-1 of the parent feature: no duplicate graph-semantics code). realized/stale are never legal AUTHORED values: they are absent from proposalStatuses, so strict decode itself fails closed the moment either appears in frontmatter, which is the enforcement mechanism for \"never written\" — no separate runtime guard is needed beyond the decode boundary plus the accept ritual (dc-2) never writing them", anchor: dc-3 }
  - { id: dc-4, text: "\"computed the way spec-stale is\" (parent ac-6) is read as a POSTURE match, not a shared code path: spec-stale (03 §The amendment ladder) has no implementation in this repo yet (verdi audit remains a v1-scoped stub, 05 §CLI) and its own mechanism — counting accepted-deviation dispositions on a story's alignment report — has no bearing on a diagram's truth divergence. diagram-stale instead shares dc-4 of the PARENT feature (\"realization is detected by regeneration diff against current truth\"): stale is simply the non-empty-residual case of that same diff, dc-3's realized/stale pair being the two computed outcomes of one comparison, exactly as spec-stale and realized/stale alike share only the posture \"computed, disclosed, never a written status\"", anchor: dc-4 }
constraints:
  - { id: co-1, text: "no LLM anywhere in this story's code (parent co-1): schema validation, the accept-ritual extension, and the disclosed-status mapping are pure, deterministic Go", anchor: co-1 }
  - { id: co-2, text: "no network in any test: decode/validate is table-driven (happy path plus every negative — unknown field, wrong status for class, missing/extra frozen, malformed digest); the accept-ritual extension is exercised over a fixturegit checkout; the disclosed-status mapping is a pure-function unit test needing no fixture at all", anchor: co-2 }
---
# Proposal Artifact

## Problem

The ratified 02 §Diagram proposals schema — `class: proposal`, the
`proposed → accepted` authored enum, `scope`, `derived_from` plus a base
digest — exists only as ratified prose (dc-7's amendment batch, merged
ahead of this build). `internal/artifact/diagram.go`'s `DiagramFrontmatter`
carries none of it: no class discriminator, no `scope` or `derived_from`
fields, and a status enum (`active`/`superseded`) with no room for
`proposed`/`accepted`. Nothing enforces that a diagram's mermaid body
survives every write path byte-for-byte. Nothing accepts a proposal at the
merge of its design MR the way a spec is accepted. Nothing computes the
disclosed `realized`/`stale` states without illegally writing them.
`diagram-proposals#ac-2` and `#ac-6` are unreachable until this artifact is
real.

## Outcome

`DiagramFrontmatter` gains an optional class discriminator, a
class-conditioned status enum, and optional `scope`/`derived_from` fields,
strict-decoded through the single `internal/artifact` seam with unknown
fields failing closed. The mermaid body is byte-preserved by every write
path. `verdi accept` admits a `class: proposal` diagram's
`proposed → accepted` transition at the merge of its own design MR, exactly
as it accepts a spec. And a pure, never-written computed-status function
discloses the four-value `proposed`/`accepted`/`realized`/`stale`
vocabulary from an externally supplied truth comparison.

## AC-1

`DiagramFrontmatter` strict-decodes `class`, `scope`, and
`derived_from: {ref, digest}`. `Validate` enforces a class-conditioned
status enum: `class: proposal` admits only `proposed`/`accepted`; a diagram
with no `class` keeps today's `active`/`superseded` enum unchanged. Frozen
presence is likewise class-conditioned: required iff a `class: proposal`
diagram is `accepted`, forbidden otherwise (matching today's
"never frozen" rule for incumbent diagrams exactly). Unknown frontmatter
fields fail closed via the existing `KnownFields(true)` dialect. Evidence:
static (the schema is declared and strict-decoded through the one seam) +
behavioral (table-driven decode/round-trip tests, happy path and every
negative: unknown field, wrong status for the class, frozen present when
forbidden, frozen absent when required, malformed `derived_from.digest`).

## AC-2

The mermaid body — the bytes below the frontmatter fence — is byte-
preserved across every write path this repo has, for a proposal and an
incumbent diagram alike: no path ever re-serializes it, reformats it, or
round-trips it through any intermediate graph representation (dc-1 of the
parent feature: "there is no board→graph→mermaid round-trip anywhere").
Concretely: the frontmatter-only edits a save path performs (e.g. a status
flip) must not touch a single byte of the body, proven by a regression test
that SHA-256s the body before and after a save round trip over a fixture
carrying idiosyncratic whitespace, comments, and line endings. Evidence:
static (the write path is inventoried and shown to touch frontmatter bytes
only) + behavioral (the byte-identity regression test).

## AC-3

`verdi accept diagram/<name>` flips a `proposed`, `class: proposal` diagram
to `accepted` and writes its `frozen: {at, commit}` stamp — the merge of
the diagram's own design MR is the acceptance event, mirroring the spec
ritual's core mechanical flip (dc-2). It refuses, naming the target and the
reason: a diagram with no `class` (an incumbent, authored-living diagram —
there is no "acceptance" for it); a `class: proposal` diagram whose status
is already `accepted` or otherwise not `proposed`; and any ref that does
not resolve to a diagram at all. Evidence: behavioral (a CLI test over a
fixturegit checkout exercising the happy path and each refusal).

## AC-4

The four-value disclosed status — `proposed`/`accepted`/`realized`/`stale`
— is computed by a pure function of an accepted proposal plus an
externally supplied residual-diff outcome (nil when no verification has
run), never written back to the artifact. Strict decode itself refuses
`realized`/`stale` as authored frontmatter values — they are absent from
the class: proposal status enum (AC-1), so the enforcement is the decode
boundary itself, not a separate runtime guard. Evidence: static (the
mapping function's signature and the absent-from-the-enum invariant) +
behavioral (a table-driven test over every input combination: proposed/no
residual, accepted/nil residual, accepted/empty residual → realized,
accepted/non-empty residual → stale, plus a decode test proving
`status: realized` and `status: stale` are rejected as authored input).

## AC-5

A new lint rule (next free number after VL-020, i.e. VL-021) refuses a
`class: proposal` diagram whose `derived_from.ref` does not resolve to a
real diagram artifact in the corpus, or whose `derived_from.digest` is not
`sha256:<64-hex>`, naming the offending field so the author sees exactly
what is wrong (the D6-18 lesson: never a silent absence). id/path agreement
for the `diagrams/` kind is already generic — `VL-002`'s
`singleFileKindDir` map already covers `diagram: {"diagrams", ".mermaid"}`
regardless of class — so this story adds no new coverage there; the
class/status enum agreement AC-1 declares is enforced at strict-decode
time (a `DecodeErr`, already surfaced by the baseline decode-cleanliness
check every kind gets), so it likewise needs no dedicated new VL number.
The one genuinely new, corpus-aware check this class needs is
`derived_from` resolution, and that is the whole of this rule's scope.
Evidence: static (the rule is declared, and the "no new coverage needed"
claims above are shown true by reading VL-002 and the existing
decode-cleanliness check) + behavioral (a lint fixture test: a proposal
with a dangling `derived_from.ref`, one with a malformed digest, and a
clean proposal that passes).

## DC-1

`DiagramFrontmatter` gains three fields mirroring the ratified 02 shape
verbatim: `Class` (string, `omitempty`; the only non-empty value is
`"proposal"`), `Scope` (string, `omitempty`; the flowmap selector, opaque
to this story — verification-extractor owns what it means), and
`DerivedFrom` (`*DiagramDerivedFrom{Ref, Digest string}`, `omitempty`).
`Validate` branches on `Class` exactly the way `spec.go` already branches
on a spec's `Class` (feature/story/component) — an established pattern in
this codebase, not a new one: `class: proposal` uses a distinct
`proposalStatuses` enum (`proposed`, `accepted`) and requires `Frozen` iff
`status: accepted`; a diagram with no `class` keeps today's
`diagramStatuses{active, superseded}` and `requireFrozen(..., false, ...)`
exactly as it decodes now, so every existing incumbent-diagram fixture
(including `testdata/corpus/.verdi/diagrams/loansvc-topology.mermaid`)
keeps decoding unchanged.

## DC-2

`verdi accept` dispatches on the target ref's kind: a `spec/...` ref keeps
its existing full ritual untouched; a `diagram/...` ref is new and
narrower — it requires `class: proposal` and `status: proposed`, flips
status to `accepted`, and writes `frozen: {at, commit}` — no stub-match, no
CODEOWNERS routing, no supersedes cascade, since a diagram carries no ACs
or stubs to match against. AC-6's "accepts at merge like any spec content"
is read as a LIFECYCLE match (merge of its own design MR is the acceptance
event, frozen at that moment) rather than a literal reuse of the
story-class stub-matching machinery, which has nothing to bind to on a
diagram.

## DC-3

The disclosed-vocabulary boundary: this story owns the pure mapping
`DiagramDisclosedStatus(fm DiagramFrontmatter, residual *ResidualDiff) Status`
— `proposed`/`accepted` pass through unchanged when `residual` is `nil`
(verification has not run yet); once a residual is supplied for an
accepted proposal, an empty residual renders `realized` and a non-empty one
renders `stale`. This story does NOT compute the residual itself — that is
verification-extractor's `ac-1` three-way diff, consumed here through its
own result type rather than reimplemented (parent `co-1`: no duplicate
graph-semantics code, one source of truth). `realized`/`stale` are never
legal AUTHORED values: they are absent from `proposalStatuses` (DC-1), so
strict decode itself fails closed the moment either appears in
frontmatter — the enforcement mechanism for "never written" is the decode
boundary plus the accept ritual (DC-2) never writing them; no separate
runtime guard is needed.

## DC-4

"Computed the way spec-stale is" (parent `ac-6`) is read as a POSTURE
match, not a shared code path. `spec-stale` (03 §The amendment ladder) has
no implementation in this repo yet — `verdi audit` remains a v1-scoped
stub (05 §CLI: "audit ADR exemptions and mid-build deviations ... v1-scoped
alongside waivers") — and its own mechanism (counting
`accepted-deviation` dispositions accumulated on a story's alignment
report) has no bearing on a diagram's truth divergence: the two flags
share no code and cannot honestly be made to. `diagram-stale` instead
shares the PARENT feature's own `dc-4` ("realization is detected by
regeneration diff against current truth"): `stale` is simply the
non-empty-residual case of that same diff, DC-3's `realized`/`stale` pair
being the two computed outcomes of one comparison. What `diagram-stale`
and `spec-stale` genuinely share is posture alone — computed, disclosed,
never a written status — and this decision records that explicitly rather
than let the parent's analogy be silently over-read as a shared
implementation that does not exist.

## CO-1

No LLM anywhere in this story's code (parent `co-1`): schema validation,
the accept-ritual extension, and the disclosed-status mapping are pure,
deterministic Go.

## CO-2

No network in any test: decode/validate is table-driven (happy path plus
every negative — unknown field, wrong status for the class, missing/extra
frozen, malformed digest). The accept-ritual extension is exercised over a
fixturegit checkout. The disclosed-status mapping is a pure-function unit
test needing no fixture at all.
