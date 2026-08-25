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
    text: "verdi matrix accepts --json for story and feature targets, emits one canonical verdi.matrix/v1 record containing target identity, effective state, preview posture, ordered acceptance-criterion rows, obligation facts, violated, and eligible, while the legacy text formatter and MCP get_matrix consume that same typed projection"
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
    text: "verdi.matrix/v1 contains schema, story, spec_ref, class, status, preview, acs, violated, and eligible; each acceptance-criterion row contains id, status, evidence, text, and obligation, ordered exactly as the target specification"
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

`verdi.matrix/v1` has the closed record and AC-row fields declared above.

### DC-3

One `internal/matrixprojection` assembly feeds CLI text, CLI JSON, and MCP.

## Constraints

### CO-1

The existing canonical JSON seam owns deterministic encoding.

### CO-2

Preview remains advisory and projection work cannot alter fold semantics.
