---
id: spec/model-digest
kind: spec
title: "Model Digest"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-33
problem: { text: "no artifact records which operating model shaped it: Provenance has no model field, the unified stamp seam's StampProvenance (built in the stamp-seam pre-work, L-M4) still takes a digest every caller passes as empty, and model.Digest() (live since model-schema) has no consumer — so once model.yaml becomes editable, an archived artifact's interpretation under the model it lived in is unrecoverable", anchor: problem }
outcome: { text: "Provenance gains an optional model digest field; every mint routed through the stamp seam passes the resolved model's canonical-JSON digest, deterministically; artifacts stamped before the field existed still decode (schema-additive), committed fixtures stay byte-stable, and a newly produced artifact's stamp always names the exact model that governed its production", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "artifacts minted through the stamp seam carry model: sha256 digests equal to the resolved model's Digest(), identical across repeated runs, proven by behavioral tests over the mint paths", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "every production Frozen/Provenance mint routes its digest through StampProvenance — no mint site bypasses the seam, proven by source-witness checks", evidence: [static], anchor: ac-2 }
  - { id: ac-3, text: "artifacts and fixtures stamped before the field existed decode unchanged and no committed fixture regenerates, proven by the existing fixture gates running green without modification", evidence: [behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/operating-model#ac-5" }
---
# Model Digest

## Problem

Every other field on a generated artifact's `Provenance` block answers "how
was this produced" — `Generator`, `Version`, `Inputs`, a content `Digest` or
judged `Integrity` (`internal/artifact/common.go`'s own doc comment, 02
§Generated artifacts and digests: "Computed content carries Digest
(recomputable from Inputs); judged content carries Integrity"). None of them
answer "under which operating model." That gap was invisible until now
because the operating model itself had no declared existence to name.
model-schema gave it one — `internal/model.Model`, `verdi.model/v1` — and
closed the loop with `(*Model).Digest()` (`internal/model/model.go:149`), a
canonical-JSON sha256 whose own doc comment already names this exact
destination: "stamped into every artifact this model produces (ledger L-M5,
Task 10 — not this task's own consumer)." Nothing consumes it yet.

The stamp-seam pre-work (L-M4, `internal/artifact/stamp.go`) built the seam
this story is meant to complete without completing it:
`StampProvenance(p *Provenance, modelDigest string)` exists, is called at
zero production sites, and is a documented no-op — its own doc comment
states it plainly: "Provenance carries no Model field yet, so modelDigest is
unused... any caller that does adopt this seam passes ''." Four production
call sites still build `artifact.Provenance{...}` literals directly, none
routed through it: `internal/commitdesign/commitdesign.go:254` (board
freeze), and three in `internal/align` — `report.go:117` (deviation
reports), `decision_report.go:151` (decision-conflict reports), and
`diagram_report.go:137` (diagram sweeps). Each of the four already sets a
content `Digest`; none sets a model.

This is not cosmetic. `model.yaml` is on a strangler path toward becoming
genuinely editable — operating-model dc-1's frontier is deliberately narrow
today (canonical model only, modulo vocabulary and templates), but is
explicitly a first slice, not a permanent ceiling. The day a store's model
can legitimately differ from another store's, or from an earlier commit's
own model, every artifact this codebase has ever minted becomes ambiguous
about which rules governed its production: a `deviation-report.md` frozen
last month recorded findings against whatever the model was then, but
nothing on the artifact itself says what that was. An archived artifact is
supposed to stay forever interpretable under the model it lived in (the
integration guide's own §5.2, cited verbatim by `Digest()`'s doc comment
already); today that is only true by accident, because the model has not
yet had the chance to move.

## Outcome

`Provenance` (`internal/artifact/common.go:166`) gains one new optional
field, `Model string` (`yaml:"model,omitempty" json:"model,omitempty"`),
holding a `sha256:<64 hex>` digest in exactly the shape `Digest`/`Integrity`
already validate against (`sha256Re`, `common.go`'s existing `Validate`) —
`Validate` grows the identical shape check for `Model` when it is present,
no new validation vocabulary invented. `StampProvenance` stops being a
no-op: given a non-nil `*Provenance` and a `modelDigest` string, it writes
`p.Model = modelDigest`, and — mirroring `NewFrozen`'s own "a structurally
empty stamp is always a caller bug, never a runtime condition" posture
(`stamp.go:24-30`) — panics on an empty `modelDigest` rather than silently
minting an artifact with an absent model claim from a call site that should
always have a real one.

The four production mint sites are rewired to call it. Each already
resolves, or can trivially reach, a `*store.Config`: `store.Open`'s own
guarantee, settled by model-schema, is that `Config.Model` is never nil —
an absent `.verdi/model.yaml` resolves to the embedded canonical
(`internal/store/open.go`'s own doc comment) — so `cfg.Model.Digest()` is
always available. It is computed once, at the `cmd/verdi` call sites that
already call `store.Open` (directly, or through its `loadManifest` thin
wrapper, `cmd/verdi/forgeboot.go:29`) ahead of invoking
`align.Generate`/`GenerateDecisionConflict`/`GenerateDiagramSweep`/the board
freeze path, and threaded down as a plain string alongside the other
caller-resolved values (`Covers`, `FrozenAt`, `Spec`) those packages'
`Input` structs already carry — never re-derived deep inside
`internal/align`/`internal/commitdesign`, and never by importing
`internal/model` there. That last point is load-bearing, not stylistic:
`internal/model` already imports `internal/artifact` (`internal/model/
decode.go:6`), so the reverse import would be a cycle — the exact reason
`StampProvenance`'s signature takes a plain `modelDigest string` rather than
a `*model.Model`, a constraint the stamp-seam pre-work already built in
without spelling out the cycle by name.

Two properties hold simultaneously, and both are required for this to be
additive rather than a breaking change. First, determinism: `model.Digest()`
already is one — `canonjson.Marshal` sorts object keys and disables HTML
escaping, `model.go`'s own doc comment states this outright — so two runs
against an unchanged model and unchanged inputs produce the identical
`model:` value; no new source of nondeterminism enters what was already a
deterministic pipeline. Second, backward compatibility: because the field
is additive and `omitempty`, an artifact minted before this story decodes
exactly as it always has — strict decode never required `model`, so its
absence was always legal, and `KnownFields(true)` has never rejected a
document for lacking an optional field, only for carrying an unknown one.
Nothing in `make verify` regenerates and diffs an already-committed artifact
against fresh output: `make fixture`'s three governed suites (`fixturegit`,
`corpus`, `svcfixcanned`, `Makefile:91`) never touch a Provenance-minting
call site at all; `lint-store` runs `verdi lint` and `verdi model check`
only, never `verdi align` or the board-freeze path (`Makefile:110-113`);
and this repository's own committed generated artifacts — every
`.verdi/specs/archive/*/decision-conflict-report.md` and its sibling
diagram-sweep/deviation reports — are frozen historical records no gate
re-derives. The "committed fixtures stay byte-stable" half of the outcome
is therefore a property of what already doesn't run against them, not a
new guard this story has to build and maintain.

`attest.go`'s `NewFrozen(time.Now().UTC()..., head)` call
(`cmd/verdi/attest.go:101`) is untouched by any of this. It mints only a
`Frozen` stamp for an `AttestationScaffold` (`internal/evidence/
attestations.go:142`), which carries no `Provenance` field at all — an
attestation is a human claim, neither computed nor judged content, so it
was never in scope for a model digest. Its wall-clock `At` remains exactly
the deliberately-disclosed, already-adjudicated exception it already is
("a convenience the operator updates... legally mutable until this file's
first commit," dc-2/ADJ-30, `stamp.go:16-22`) — this story does not reopen
it. The only change this story makes anywhere is the new digest field on
`Provenance` and the four call sites that populate it; determinism-versus-
wall-clock stamping in general was L-M4's settled work, not this story's.

## Ac 1

Behavioral tests, extending each of the four packages' own existing
suites (`internal/commitdesign/commitdesign_test.go`;
`internal/align/report_test.go` for `Generate`;
`decision_report_test.go` for `GenerateDecisionConflict` — which already
has a directly analogous precedent, `TestGenerateDecisionConflict_
SweepProvenanceRecorded`, proving a different provenance-adjacent field is
actually recorded; `diagram_report_test.go` for `GenerateDiagramSweep`),
prove that for every one of the four mint sites the artifact a
`Generate`/freeze call actually produces carries a `provenance.model` equal
to `sha256:` plus the hex digest the same resolved model's `Digest()` call
independently produces over the fixture store's model — the embedded
canonical in the common case, and a fixture `model.yaml` in at least one
case per site to prove the stamped value tracks a *different* model's
*different* digest, not a constant that happens to look right against the
one model every other test already uses. "Identical across repeated runs"
extends each package's existing determinism convention —
`report_test.go`'s own `TestGenerate_ByteIdenticalAcrossRuns` is the named
precedent — to cover the new field: two fresh `Generate` calls against
unchanged inputs produce byte-identical `model:` lines, not merely two
separately-computed digests that happen to agree.

## Ac 2

Mirroring `spec/shared-homes` ac-1's own "one seam, no surviving copies"
static convention (`.verdi/obligations/shared-homes/ac-1--static.md`): the
static evidence must show all four production `artifact.Provenance{...}`
literals — `internal/commitdesign/commitdesign.go:254`,
`internal/align/report.go:117`, `internal/align/decision_report.go:151`,
`internal/align/diagram_report.go:137` — set their `Model` field only by
way of `StampProvenance`, never inline in the literal the way
`Digest:`/`Integrity:` are set today. Because `StampProvenance` takes a
`*Provenance` and mutates it after construction rather than returning one,
"routes through the seam" is a checkable source property: no file outside
`internal/artifact/stamp.go` itself assigns to the `.Model` field of a
`Provenance` value, and all four sites call `artifact.StampProvenance`
between constructing their literal and returning or persisting it.
`cmd/verdi/attest.go` is correctly absent from this four-site enumeration
— the Problem section's own accounting explains why: it mints a `Frozen`,
never a `Provenance` — so the check's enumeration is exactly four sites,
never five, and the obligation records why the count is four so that a
future fifth mint site is caught by the same check rather than requiring
this list to be rediscovered by hand.

## Ac 3

The evidence here is deliberately negative: `make fixture` (`go test -race
./internal/fixturegit/... ./internal/corpus/... ./internal/svcfixcanned/...`,
`Makefile:91`) and `make lint-store` (`verdi lint` then `verdi model check`,
`Makefile:110-113`) must pass exactly as they do today, green, with no test
file, testdata fixture, or Makefile target touched to make them pass — the
proof is that this story's own build diff never touches those two targets'
inputs. Any artifact committed before this story — every file already under
`.verdi/specs/archive/**` and `.verdi/specs/active/**` carrying a
`provenance:` block with no `model:` line — must decode unchanged through
its type's existing decoder (`artifact.DecodeDeviation`,
`DecodeDecisionConflict`, `DecodeDiagramSweep`, `DecodeBoard`), since
`KnownFields(true)` was never violated by a document that simply lacks an
optional field. Where a hand-render counterpart already exists for the
type (`align.RenderMarkdown`, `RenderDecisionMarkdown`,
`RenderDiagramSweepMarkdown` — each already byte-pinned, "hand-rendered,
never yaml.Marshal'd," per `internal/evidence/attestations.go`'s own stated
module-wide posture), decoding one such pre-existing committed artifact and
re-rendering it must reproduce the exact original bytes: no `model:` line
appears where the source had none, and an old artifact is never
retroactively re-stamped merely because the code that reads it now knows
the field exists.
