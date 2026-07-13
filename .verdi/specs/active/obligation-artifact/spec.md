---
id: spec/obligation-artifact
kind: spec
title: "Obligation Artifact"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-5
problem: { text: "the evidence-obligations feature needs its load-bearing object — a first-class evidence-obligation artifact — before anything can gate on it or render it. Today no such kind exists: `internal/artifact` knows spec, attestation, adr, diagram, waiver, board, evidence, rollup, deviation, bindings — nothing that states what a story AC's declared evidence kind must specifically show. And there is no way to author one on the wall.", anchor: "#problem" }
outcome: { text: "a new `kind: obligation` markdown artifact exists, strict-decoded through the single `internal/artifact` seam: id `obligation/<story-slug>--<ac-id>--<for-kind>`, a `for_kind` evidence-kind, the obligation prose (title + body), a `verifies` edge to a STORY AC fragment, and a frozen stamp — living at `.verdi/obligations/<story-ref-slug>/<ac-id>--<for-kind>.md`. It validates that its id, `for_kind`, and path agree and that its `verifies` target is a STORY AC (a feature-AC or non-AC target is refused). And it is authored the way every wall object is: a board sticky graduates into one, bound to the AC it is dropped on.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a `kind: obligation` artifact strict-decodes and round-trips through the `internal/artifact` seam: id `obligation/<story-slug>--<ac-id>--<for-kind>`, `for_kind` one of the evidence kinds, an obligation title + body, a `verifies` edge to an AC fragment, and a frozen stamp; validation rejects an id/for_kind/path disagreement and a malformed id, and unknown frontmatter fields fail closed", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "an obligation whose `verifies` target resolves to a FEATURE AC, to a non-AC fragment, or to a whole spec is refused by lint — obligations attach to STORY ACs only (the feature-blind / story-scoped split, carried from 03 §The feature fold); the refusal names the offending target", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "an obligation is authored by graduating a board sticky: a sticky dropped on a story AC graduates into an obligation object bound to that AC (seeding its `verifies` edge and `for_kind`), exactly as an attestation sticky graduates today — proven on the board", evidence: [behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/evidence-obligations#ac-1" }
  - { type: implements, ref: "spec/evidence-obligations#ac-3" }
decisions:
  - { id: dc-1, text: "the obligation is a MARKDOWN artifact (frontmatter + prose body), decoded through `internal/artifact` exactly like an attestation — `kind: obligation`, no `schema:` line (that is for JSON artifacts). Frontmatter: id, kind, for_kind, title, owners, links (a single `verifies` edge), frozen. The prose body is the full obligation statement; the title is its one-line summary. Mirroring the attestation artifact keeps one decode/validate/freeze posture, not a second", anchor: "#dc-1" }
  - { id: dc-2, text: "on-disk home `.verdi/obligations/<story-ref-slug>/<ac-id>--<for-kind>.md`, mirroring attestations' `.verdi/attestations/<story-ref-slug>/<ac-id>.md` (D6-18's story-ref-slug convention, so an obligation and its story's attestations share a slug). The id `obligation/<story-slug>--<ac-id>--<for-kind>` and the path are two views of the same (story, ac, kind) triple; validation requires them to agree", anchor: "#dc-2" }
  - { id: dc-3, text: "`verifies`-a-story-AC-only is validated at the artifact seam AND surfaced as a lint finding (a new VL rule, next free number) so a mis-targeted obligation is caught at author time, not silently ignored — the D6-18 lesson (a mis-slugged attestation read as absent) applied preemptively. The target's class is resolved through the index the same way `supersedesTargetsStory`/`supersedesTargetsFeature` (accept.go) already resolve a ref's class", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "no network in any test: decode/validate/round-trip is table-driven (happy + every negative: malformed id, id/for_kind/path disagreement, unknown field, missing verifies, verifies a feature AC / non-AC / whole spec); the board graduation is a Playwright e2e over a hermetic fixture wall", anchor: "#co-1" }
  - { id: co-2, text: "this story adds the artifact + its authoring only — it does NOT wire the activation gate (obligation-gate, ac-2 of the feature) or the wall/matrix render (obligation-wall, ac-4). A declared kind with no obligation is not yet refused here; graduation and decode are the whole scope, so the two downstream stories build on a real, frozen artifact", anchor: "#co-2" }
---
# Obligation Artifact

## Problem

The evidence-obligations feature needs its load-bearing object before anything
can gate on it or render it: a first-class evidence **obligation** artifact.
Today `internal/artifact` knows spec, attestation, adr, diagram, waiver, board,
evidence, rollup, deviation, and bindings — nothing that states what a story
AC's declared evidence kind must specifically show, and no way to author one on
the wall.

## Outcome

A new `kind: obligation` markdown artifact, strict-decoded through the single
`internal/artifact` seam: id `obligation/<story-slug>--<ac-id>--<for-kind>`, a
`for_kind` evidence kind, the obligation prose (title + body), a `verifies` edge
to a STORY AC fragment, and a frozen stamp — living at
`.verdi/obligations/<story-ref-slug>/<ac-id>--<for-kind>.md`. It validates that
its id, `for_kind`, and path agree and that its `verifies` target is a story AC
(a feature-AC or non-AC target is refused), and it is authored the way every
wall object is — a board sticky graduates into one, bound to the AC it is
dropped on.

## AC-1

A `kind: obligation` artifact strict-decodes and round-trips through
`internal/artifact`: id `obligation/<story-slug>--<ac-id>--<for-kind>`,
`for_kind` one of the evidence kinds (static/behavioral/runtime/attestation), an
obligation title and body, a `verifies` edge to an AC fragment, and a frozen
stamp. Validation rejects an id/`for_kind`/path disagreement and a malformed id,
and unknown frontmatter fields fail closed (the dialect's `KnownFields(true)`).
Evidence: static (the schema is declared and strict-decoded) + behavioral (a
decode/round-trip test over real fixtures).

## AC-2

An obligation whose `verifies` target resolves to a FEATURE AC, to a non-AC
fragment, or to a whole spec is refused by lint — obligations attach to STORY
ACs only, the feature-blind / story-scoped split carried from 03 §The feature
fold. The refusal names the offending target so the author sees exactly what is
wrong (the D6-18 lesson: never a silent absence). Evidence: static + behavioral.

## AC-3

An obligation is authored by graduating a board sticky. A sticky dropped on a
story AC graduates into an obligation object bound to that AC — seeding its
`verifies` edge and `for_kind` — exactly as an attestation sticky graduates
today (boardspecapi.go's `actionStickyGraduate`). Proven on the board.
Evidence: behavioral (a Playwright e2e over a hermetic fixture wall).

## DC-1

The obligation is a **markdown** artifact (frontmatter + prose body), decoded
through `internal/artifact` exactly like an attestation: `kind: obligation`, no
`schema:` line (that is for JSON artifacts). Frontmatter carries id, kind,
`for_kind`, title, owners, a single `verifies` link, and frozen. The body is the
full obligation statement; the title its one-line summary. Mirroring the
attestation artifact keeps one decode/validate/freeze posture, not a second.

## DC-2

On-disk home `.verdi/obligations/<story-ref-slug>/<ac-id>--<for-kind>.md`,
mirroring attestations' `.verdi/attestations/<story-ref-slug>/<ac-id>.md`
(D6-18's story-ref-slug convention, so an obligation and its story's
attestations share a slug). The id `obligation/<story-slug>--<ac-id>--<for-kind>`
and the path are two views of the same (story, ac, kind) triple; validation
requires them to agree.

## DC-3

`verifies`-a-story-AC-only is validated at the artifact seam AND surfaced as a
lint finding (a new VL rule, the next free number) so a mis-targeted obligation
is caught at author time, not silently ignored — the D6-18 lesson applied
preemptively. The target's class is resolved through the index the same way
`supersedesTargetsStory` / `supersedesTargetsFeature` (accept.go) already
resolve a ref's class.

## CO-1

No network in any test. Decode/validate/round-trip is table-driven — happy plus
every negative (malformed id, id/`for_kind`/path disagreement, unknown field,
missing `verifies`, `verifies` a feature AC / non-AC / whole spec). The board
graduation is a Playwright e2e over a hermetic fixture wall.

## CO-2

This story adds the artifact and its authoring only. It does NOT wire the
activation gate (the obligation-gate story, feature ac-2) or the wall/matrix
render (the obligation-wall story, feature ac-4). A declared kind with no
obligation is not yet refused here; graduation and decode are the whole scope,
so the two downstream stories build on a real, frozen artifact.
