---
id: spec/operating-model
kind: spec
title: "Operating Model"
owners: [platform-team]
class: feature
status: closed
problem: { text: "the operating model — lifecycle states, transitions and their obligations, class hierarchy, display vocabulary, and scaffold content — is hard-coded in Go across ~45 files (extensibility audit @ 24214fd): teams cannot see or reshape it, every rename or template change is a code change, and the same process facts are re-encoded independently in up to seven subsystems with no shared seam", anchor: problem }
outcome: { text: "the canonical operating model is declared in a strict-decoded .verdi/model.yaml (verdi.model/v1) with an embedded canonical default so absent config changes nothing; verdi model check validates it fail-closed with pinned frontier errors; scaffolds render from editable templates with a custom: opaque namespace; display vocabulary is configurable and reaches CLI, workbench, dex, and MCP surfaces; and every produced artifact's provenance carries the model digest — with the entire pre-existing fixture and e2e suite green against the canonical default", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "with no model.yaml present, the full verify gate (build, tests, fixtures, e2e) passes unchanged against the embedded canonical model — byte-identical CLI output and unchanged committed fixtures", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "verdi model check exits 2 on operational trouble, 1 with the pinned frontier error text on a structurally-deviant manifest, and 0 with an OK line naming schema, counts, and digest on a valid one", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "a store with an edited template (added body section and a custom: field) scaffolds new specs from it, the custom field survives strict decode and re-emit, and model check round-trips every template fail-closed", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "a store with vocabulary display renames shows the renamed state, verb, and class labels on CLI verdicts, board, dex, and MCP tool descriptions, while ids in refs, branches, and history stay unchanged", evidence: [behavioral], anchor: ac-4 }
  - { id: ac-5, text: "every frozen/provenance mint routes through a single stamping seam, obligation stamps derive from the covered commit rather than wall-clock, and stamps carry the resolved model's canonical-JSON digest", evidence: [static], anchor: ac-5 }
stubs:
  - { slug: model-schema, acceptance_criteria: [ac-1, ac-2] }
  - { slug: scaffold-templates, acceptance_criteria: [ac-3] }
  - { slug: vocabulary-surfaces, acceptance_criteria: [ac-4] }
  - { slug: model-digest, acceptance_criteria: [ac-5] }
decisions:
  - { id: dc-1, text: "v1 frontier: verdi.model/v1 accepts only the canonical model modulo vocabulary and templates; structural deviation is rejected with a pinned error naming the frontier — smallest reversible slice of the strangler", anchor: dc-1 }
  - { id: dc-2, text: "custom: is KnownFields-exempt but the YAML dialect wall (anchors/aliases/tags rejection) still applies inside it — loosening later is additive, tightening would break stores", anchor: dc-2 }
frozen: { at: 2026-07-17, commit: 98f8c4208f7308356be67c80aecbe7cb73f97424 }
---
# Operating Model

## Problem

The operating model — every lifecycle state, every transition and the
obligations that gate it, the class hierarchy, the display vocabulary, and
the content a scaffold produces — is hard-coded in Go across roughly 45
files (extensibility audit @ 24214fd). A team cannot see the model as a
single artifact, let alone reshape it: renaming a state, adding a
transition, or changing what a scaffolded spec looks like is a Go change
and a recompile, not a config edit.

The duplication is structural, not cosmetic. The "is this the
accepted-pending-build status line" check alone is independently
re-implemented as at least four near-identical regexes —
`cmd/verdi/close.go:331`, `cmd/verdi/supersede.go:27`,
`cmd/verdi/accept.go:32`, and `internal/lint/vl010.go:38-41` (which alone
carries four more, one per status) — each hand-written against the same
literal string. The `accepted-pending-build` token itself is re-encoded as
a Go string literal across roughly seven distinct subsystems: `cmd/verdi`
(`blastradius.go`, `gate.go`, `buildstart.go`), `cmd/e2eharness`
(`provision_board.go`), `internal/workbench` (`boardspecrender.go`,
`boardspecapi.go`), `internal/refindex` (`status.go`, `entry.go`), and
`internal/artifact` itself (`status.go`, `spec.go`) — with no shared seam a
team could read once to see the whole lifecycle. The closure gate's own
condition lists — `runClosureGate` and `runFeatureClosureGate`
(`cmd/verdi/closuregate.go`, `closuregatefeature.go`) — carry their
PASS/FAIL conditions as Go code, not data, for the same reason.

Scaffolds have the identical problem one layer up: `Feature` and `Story` in
`internal/designscaffold/designscaffold.go` assemble a new spec's
frontmatter and body with `fmt.Sprintf`/`strings.Builder` — editing what a
scaffolded spec looks like means editing Go source. And even the parts of
the system that are supposed to be deterministic have a hole: obligation
stamps in `internal/workbench/obligationauthor.go:103` are minted from
`time.Now().UTC()` rather than the covered commit — the one wall-clock read
among otherwise commit-derived mints, in direct tension with the house rule
that generated artifacts carry no wall-clock or randomness except declared
stamps.

## Outcome

The canonical operating model becomes a single declared artifact: a
strict-decoded `.verdi/model.yaml` (`verdi.model/v1`), with an embedded
canonical default so a store with no `model.yaml` at all changes nothing —
the model was always implicitly "the canonical one," now it is also
explicit. `verdi model check` validates a manifest fail-closed: a
structurally deviant model is rejected with a pinned frontier error naming
exactly what is out of bounds (dc-1), never a silent partial acceptance.

Scaffolds move from Go string builders to editable template files, with a
`custom:` namespace a team can populate without touching Go at all (dc-2
keeps the strict-decode dialect wall active even inside that namespace, so
the escape hatch cannot become a smuggling vector for anchors, aliases, or
tags). Display vocabulary becomes configurable and reaches every surface
that renders a state, verb, or class name — CLI verdicts, the workbench
board, dex, and MCP tool descriptions — while the underlying ids in refs,
branches, and history stay exactly as they are today, so a rename is
cosmetic by construction. Every produced artifact's provenance carries the
resolved model's digest, and the wall-clock stamp in `obligationauthor` is
replaced by the same commit-derived stamping seam `align/freeze.go`
already uses, closing that determinism gap as a side effect rather than a
separate fix.

None of this is free until it is proven inert: the entire pre-existing
fixture and e2e suite must stay green, byte-for-byte, against the embedded
canonical default. That parity is what makes this a strangler slice rather
than a rewrite — stage 1 makes the model *describe* today's transitions and
obligations exactly; it deliberately does not yet move *enforcement* off
the hard-coded condition slices (dc-1's frontier), so nothing about how a
gate actually decides changes this phase.

## AC-1

With no `model.yaml` present, the full verify gate — build, tests,
fixtures, e2e — passes unchanged against the embedded canonical model:
byte-identical CLI output, unchanged committed fixtures. This is the
load-bearing proof for the whole approach: the embedded
`internal/model/canonical.yaml` must express today's hard-coded model
exactly, or nothing built on top of it can be trusted to be additive
rather than a silent behavior change.

## AC-2

`verdi model check` exits 2 on operational trouble (an unreadable store, a
decode error), 1 with the pinned frontier error text on a
structurally-deviant manifest, and 0 with an OK line naming the schema,
object counts, and digest on a valid one. This extends the store's
existing three-valued exit discipline to the model manifest itself, so a
team gets the same fail-closed feedback on a hand-edited `model.yaml` that
`verdi lint` already gives on a spec.

## AC-3

A store with an edited template — an added body section and a `custom:`
field — scaffolds new specs from it; the custom field survives strict
decode and re-emit; `model check` round-trips every template fail-closed.
Templates are the first genuinely user-editable surface of the model: a
team adds a section or a field with no Go change, and the round-trip check
is what keeps that editability from silently corrupting specs it did not
mean to touch.

## AC-4

A store with vocabulary display renames shows the renamed state, verb, and
class labels on CLI verdicts, the board, dex, and MCP tool descriptions,
while ids in refs, branches, and history stay unchanged. Vocabulary is
deliberately cosmetic-only: what a state is *called* changes; what it *is*
— its id, its branch prefix, its position in history — does not, which is
the split that makes a rename safe to ship without touching every existing
ref.

## AC-5

Every frozen/provenance mint routes through a single stamping seam,
obligation stamps derive from the covered commit rather than wall-clock,
and stamps carry the resolved model's canonical-JSON digest. This makes
"which model shaped this artifact" a static, checkable property of the
artifact itself, and is also the fix for the wall-clock read in
`obligationauthor` named in the Problem section — the same seam closes both
at once rather than needing a separate patch.

## DC-1

v1's frontier is deliberately narrow: `verdi.model/v1` accepts only the
canonical model, modulo vocabulary and template differences. Any
structural deviation — a different state set, a different transition set,
a different class set, different obligations — is rejected outright, with
a pinned error naming exactly which frontier the manifest crossed, rather
than partially honored.

This is the smallest reversible slice of the strangler pattern (reference
guide: `docs/design/concepts/2026-07-17-integration-startup-guide.md`,
Appendix B sequencing): stage 1 makes the model *describe* today's
transitions and obligations exactly, but deliberately leaves *enforcement*
— the gate's own condition slices — hard-coded, unmoved, this phase. That
narrowness is what makes AC-1's byte-identical parity possible at all:
because nothing about how a gate actually decides changes yet, there is
nothing for the existing fixture and e2e suite to disagree with. Widening
the frontier to accept real structural configuration, and lifting
enforcement to read the manifest rather than the hard-coded slices, is
later-phase work building on this declared shape, not this phase's.

## DC-2

`custom:` is exempt from `KnownFields` — a team may add fields under it
without strict decode rejecting the document — but the YAML dialect wall
(rejection of anchors, aliases, and tags) still applies inside it, with no
carve-out.

This closes a sharp edge the audit flagged directly: an opaque namespace
that is *fully* unchecked would be exactly the kind of loophole that lets
a store-specific dialect quietly diverge from the rest of the corpus,
undermining the one property strict decode exists to guarantee everywhere
else. Keeping the wall active inside `custom:` costs nothing today's use
cases need, and the asymmetry matters for what happens later: loosening
this rule further is an additive change (any document valid today stays
valid), while tightening it after stores have come to depend on looser
behavior would break them outright. Choosing the stricter default now
keeps the reversible direction open and closes off the irreversible one.
