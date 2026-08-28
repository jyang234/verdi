---
id: spec/vatc-machine-projections
kind: spec
title: "Canonical machine projections for Verdi-ATC"
owners: [platform-team]
class: story
story: jira:VERDI-ATC-1
problem: { text: "Verdi-ATC must currently scrape matrix text and rely on journey's undocumented default JSON behavior, so its lifecycle inputs are not one explicit, strict, parity-tested machine contract", anchor: problem }
outcome: { text: "matrix and journey expose explicit canonical JSON modes, matrix CLI and MCP share one projection, and successful reports preserve existing exit semantics even when the projected state contains blockers or violations", anchor: outcome }
acceptance_criteria:
  - id: ac-1
    text: "verdi matrix accepts --json for story and feature targets and emits one canonical tagged verdi.matrix/v1 union: a common target, preview, and violated envelope plus exactly one story or feature body preserving that fold's complete native acceptance-criterion facts; the legacy text formatter and MCP get_matrix consume that same typed projection"
    evidence: [static, behavioral]
    anchor: ac-1
  - id: ac-2
    text: "verdi journey accepts an explicit --json flag and emits bytes identical to its legacy no-flag canonical JSON for the same target, without creating a second projector or changing its read-only behavior"
    evidence: [behavioral]
    anchor: ac-2
  - id: ac-3
    text: "malformed flags and unresolvable inputs exit 2, a successfully produced report exits 0 even when it reports a violated or blocked state, and built-binary tests prove deterministic CLI and MCP parity"
    evidence: [behavioral]
    anchor: ac-3
links:
  - { type: implements, ref: "spec/verdi-atc-prerequisites#ac-1" }
decisions:
  - id: dc-1
    text: "the public invocations are verdi matrix [--preview] --json <story-or-feature-ref> and verdi journey --json <feature-or-story-ref>; legacy matrix text and legacy journey JSON remain backward compatible"
    anchor: dc-1
  - id: dc-2
    text: "verdi.matrix/v1 contains schema, target, preview, violated, and exactly one of story or feature: target contains class, spec_ref, and effective_state; the story body contains story_ref, eligible, and ordered story AC rows with id, text, status, summary, and the fold's ordered kind projections; the feature body contains ordered feature AC rows with id, text, status, summary, implementing_stories, and the complete outcome_floor projection; story-only fields never appear on feature targets and feature-only fields never appear on story targets"
    anchor: dc-2
  - id: dc-3
    text: "internal/matrixprojection is the sole assembly seam for matrix CLI text, matrix CLI JSON, and MCP get_matrix; MCP returns the typed record and does not shell out to the CLI"
    anchor: dc-3
constraints:
  - id: co-1
    text: "canonical output uses the existing canonjson package, one trailing newline, empty arrays instead of null, and no location-, clock-, or process-dependent fields"
    anchor: co-1
  - id: co-2
    text: "preview remains explicit and advisory, authoritative-only remains the default, and adding --json cannot alter fold, feature-fold, obligation, or lifecycle-state semantics"
    anchor: co-2
---
# Canonical machine projections for Verdi-ATC

## Problem

ATC cannot treat a human-formatted table as lifecycle authority. Journey is
already canonical JSON, but a named mode is required so consumers do not rely
on an accidental default while matrix remains text-only.

## Outcome

One matrix record feeds all matrix adapters, and both verbs advertise explicit
canonical JSON behavior. The change is projection-only: it neither gates nor
mutates lifecycle state.

## AC-1

The matrix JSON record follows the existing target-resolution, fold, effective
state, preview, and obligation rules. Story and feature projections use the
same closed schema. MCP and CLI results compare equal after protocol-envelope
removal.

## AC-2

`journey --json` delegates to the existing projector and canonical encoder.
The no-flag invocation remains valid and byte-identical.

## AC-3

Unit, MCP, and built-binary tests cover success, adverse reported state,
preview, feature and story targets, unknown or duplicated flags, missing refs,
canonical encoding, and operational failure.

## Decisions

### DC-1

The exact invocations are the existing verbs plus `--json`; legacy modes stay
compatible.

### DC-2

`verdi.matrix/v1` is a class-tagged union. Its exact nested fields and absence
rules are fixed in the Matrix wire contract below.

### DC-3

One `internal/matrixprojection` assembly feeds CLI text, CLI JSON, and MCP.

## Constraints

### CO-1

The existing canonical JSON seam owns deterministic encoding.

### CO-2

Preview remains advisory and projection work cannot alter fold semantics.

## Matrix wire contract

The common envelope is:

```text
schema: "verdi.matrix/v1"
target: { class, spec_ref, effective_state }
preview: boolean
violated: boolean
story: StoryBody | absent
feature: FeatureBody | absent
```

`target.class` is the closed `story | feature` discriminator. Exactly the
matching body is present. The other key is absent, not `null`. Collections are
present arrays, including when empty.

`StoryBody` is:

```text
story_ref: string
eligible: boolean
acs[]: {
  id, text, status, summary,
  kinds[]: {
    kind, satisfied, attestation_state, violating_witness,
    obligation_quality: { structural_state, match_state, reason, witness_path }
  }
}
```

The AC and kind arrays retain declaration order. `attestation_state` is the
fold's closed absent/unauthored/authored value and is `not-applicable` for a
non-attestation kind. `violating_witness` is the canonical evidence identity
or the empty string when none exists. The obligation-quality values and
witness path are copied from the fold's own projection, never re-derived.

`FeatureBody` is:

```text
acs[]: {
  id, text, status, summary,
  implementing_stories[],
  outcome_floor: {
    satisfied, declares_attestation, attestation_state, violating_witness
  }
}
```

Feature ACs retain specification order; `implementing_stories` uses the
feature fold's canonical sorted order. The outcome-floor fields preserve its
OR-across-signals semantics. There is no feature `eligible` value and no
invented story reference. Unknown keys or a discriminator/body mismatch fail
closed in every decoder.
