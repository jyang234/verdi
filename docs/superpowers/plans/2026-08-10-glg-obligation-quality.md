# GLG Obligation-Quality Implementation Plan

> For agentic workers: REQUIRED SUB-SKILL: use `superpowers:test-driven-development` for every task and `superpowers:verification-before-completion` before handoff.

**Goal:** make unresolved evidence-obligation design debt strict, visible and
incapable of supporting authoritative evidence or `build start`, while keeping
legacy frozen artifacts honest and preserving negative evidence.

**Architecture:** `internal/artifact` owns the wire schema;
`internal/evidence` owns one quality assessment consumed by the fold and every
writer; `cmd/verdi` gates build before mutation; `internal/journey` projects the
same assessment through a consumer-owned port. No second profile, renderer or
evidence-record identity is introduced.

**Tech stack:** Go 1.25, strict Markdown/YAML decode, existing evidence fold,
human-artifact renderer, journey record and fixturegit/built-binary tests.

**Planning authority:** GLG v3 blob
`c347668014d26f987d38fbd9dca0082228238694`; GLG AC-2/DC-5/CO-1..7;
new SI-71..SI-74; inherited obligation artifact/gate/seam/wall authority.

## Global constraints

- Launch from exact `origin/main` after this plan/ledger PR merges and stop if
  the plan commit is not reachable.
- Obligations remain story-only `(story AC, evidence kind)` artifacts. Feature
  ACs remain exempt. Evidence records gain no `obligation_id`.
- No new CLI verb, MCP tool, lifecycle state, store root or operating-model
  transition.
- No frontend markup/CSS/JS/Playwright change. Backend workbench writer tests
  may change; a later FABLE unit owns presentation.
- Do not edit 282 frozen legacy obligation files. Do not call missing fields
  elaborated. Do not make unresolved scaffolds fail acceptance-time VL-020.

## Contract fixed by SI-71..SI-74

`ObligationFrontmatter` gains optional `quality:`:

```yaml
quality:
  state: unresolved-design-debt
```

or:

```yaml
quality:
  state: elaborated
  claim: "..."
  falsifier: "..."
  scope: "..."
  producer: { kind: checker, ref: "verify:behavioral" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "rerun when the accepted spec or implementation changes"
```

State is closed: `unresolved-design-debt|elaborated`. Unresolved permits no
meaning fields. Elaborated requires all six dimensions, nonblank normalized
strings, one closed producer kind (`test|checker|reviewer|authenticated-human`),
one closed source kind (`ci-job|signed-record|forge-check|repository-path|governed-attestation`),
one or more unique declaration-ordered invalidators from
`spec|code|dependency|environment|policy`, and a nonblank rule. Structural
validation never claims the prose is substantively sufficient.

An absent `quality` block is `legacy-unelaborated`, a valid compatibility
decode but never elaborated. The 27 known marker bodies also assess unresolved.
Every new scaffold writer emits explicit unresolved state. Frozen legacy or
unresolved obligations are remedied only by a new replacement obligation on a
successor story/spec through the normal specification amendment ladder; this
unit does not mutate them in place or invent an obligation-supersession edge.

Truth enforcement is unconditional; governance-profile adoption changes
ceremony, not proof meaning. This unit supplies no experimental escape because
there is no existing advisory `build start` surface. A later lifecycle-
governance unit may add a separately recorded advisory execution transition;
until then unresolved debt refuses all `build start` invocations.

The one `Assessment` is keyed per declared `(AC, evidence kind)` and returns
`elaborated`, `unresolved-design-debt`, `legacy-unelaborated`, `missing`, or an
operational error for malformed/unreadable artifacts. Missing/unresolved are
verdict states. At authoritative fold time:

- a current failing record still yields `violated` with its witness;
- a waiver retains its existing higher precedence and is not evidence;
- otherwise an unelaborated or missing obligation prevents a passing record
  from satisfying that kind, makes the kind pending and makes the story
  ineligible;
- preview/local folds remain advisory but display the same debt;
- malformed/I/O failure is operational.

`KindResult` gains an `ObligationQuality` projection with the closed states
above and a stable witness path. All fold consumers receive it from the one
fold; none re-derive quality.

`build start` checks all declared pairs after accepted/cascade proof and before
`RevParse`, branch creation or baseline work. Missing/unresolved returns exit 1
and names sorted pairs and fields; malformed/unreadable returns exit 2.

Journey adds reason code `obligation-design-unresolved`, blocker class
`mechanical`, and ID `obligation-quality/<ac-id>/<kind>`. One blocker is emitted
per non-elaborated pair, ordered by AC declaration then evidence-kind
declaration. `AffectedTransition` uses the registered CLI action identity
`build:start` rather than inventing a lifecycle transition. The generic
`verdi.journey/v1` blocker shape is sufficient; no schema version changes.
Journey output remains exit 0 when it successfully reports blockers; malformed
authority is operational exit 2.

The existing `internal/evidence.RenderObligation`/`WriteObligationFile` remains
the compatibility bridge and sole writer for this unit. It must consume
`humanartifact.Contract` for kernel/default validation rather than grow a
second configurable renderer. VL-020 remains existence-only so scaffolds can
land for review; quality is enforced at fold/build/journey boundaries.

## Task 1: Define obligation quality

**Files:** modify `internal/artifact/obligation.go` and tests; create
`internal/evidence/obligationquality.go` and tests; modify
`internal/humanartifact/kernel.go` and tests.

Begin with RED tables for both states, all missing dimensions, unknown enums/
fields, duplicates, whitespace, legacy absence and marker bodies. Add the
quality key to the obligation kernel collision set. Return structs; define the
loader interface at each consumer.

Run: `go test -race ./internal/artifact ./internal/evidence ./internal/humanartifact -count=1`

Commit: `Define obligation quality states`

## Task 2: Emit unresolved scaffolds everywhere

**Files:** modify `internal/evidence/obligations.go` and renderer tests;
`cmd/verdi/obligation.go`, `acceptobligation.go` and tests;
`internal/workbench/obligationauthor.go` and backend tests;
`cmd/verdi/obligationseam_e2e_test.go`.

RED tests must prove scaffold/author/batch/workbench paths emit the same
explicit unresolved block, fabricate none of the six meanings, remain
idempotent and leave existing files byte-identical. Preserve I-44 partial
residue behavior.

Run: `go test -race ./cmd/verdi ./internal/workbench ./internal/evidence -run 'Test.*Obligation' -count=1`

Commit: `Mark obligation scaffolds unresolved`

## Task 3: Block authoritative build

**Files:** modify `cmd/verdi/buildstart.go` and tests.

Test every quality state, each missing dimension, multiple sorted debts,
malformed/I/O errors and zero Git mutation. Check after accepted/cascade proof
and before any effect. Preserve all proposed/closed/superseded behavior.

Run: `go test -race ./cmd/verdi -run 'TestRunBuildStart|Test.*ObligationQuality' -count=1`

Commit: `Gate builds on obligation quality`

## Task 4: Protect authoritative evidence

**Files:** modify `internal/evidence/fold.go`, `status.go` and tests; update
affected matrix/rollup/close/workbench/MCP tests without adding a second read.

RED cases: unelaborated pass cannot evidence, fail still violates, waiver
precedence, preview disclosure, malformed operational, feature fold unchanged,
and exact KindResult projection shared by every consumer.

Run:

```sh
go test -race ./internal/evidence -count=1
go test -race ./cmd/verdi ./internal/workbench ./internal/mcpserve -run 'Test.*(Fold|Matrix|Rollup|Close|Obligation)' -count=1
```

Commit: `Bind evidence to obligation quality`

## Task 5: Project deterministic blockers

**Files:** modify `internal/journey/{port,project,derive,reason}.go` and tests;
ratchet record/golden/parity tests; modify `cmd/verdi/journey_test.go`.

Test the exact reason, class, ID, witness, owner and `build:start` action;
deterministic pair ordering; no blocker when elaborated; malformed operational;
successful blocked journey exit 0; no invented safe action.

Run: `go test -race ./internal/journey ./cmd/verdi -run 'Test.*(Journey|Reason|Blocker)' -count=1`

Commit: `Project obligation quality blockers`

## Task 6: Prove compatibility and gates

Add a corpus witness: exactly 282 legacy files remain unmodified, every absent
quality field assesses legacy-unelaborated, and all 27 marker bodies remain
visible unresolved. Run fresh, unpiped:

```sh
make test
make fixture
make spec-align
go test -race ./...
make verify
git diff --check
git status --short
```

Commit only corrections required by these gates. Report the experimental
advisory transition, legacy remediation and UI as disclosed pending work.

## Source-coverage witness

Coverage is **26/26** implicated authority artifacts: five instruction/
provenance, six binding semantics, six four-feature authorities, three
lifecycle/migration and six inherited obligation records. Transformations:
the six prose dimensions become one closed quality union; frozen legacy absence
becomes explicit `legacy-unelaborated`; “cannot support evidence” preserves
negative witnesses while withholding positive satisfaction; `build start`
uses an action identity without entering the operating-model catalog.
Intentional omissions are experimental advisory execution, frontend display,
feature obligations and frozen in-place migration. No source is silently
grandfathered or called complete.
