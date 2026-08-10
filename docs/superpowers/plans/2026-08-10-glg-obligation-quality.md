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
strings, one closed producer kind (`test|checker|authenticated-human`), one
closed source kind (`ci-job|governed-attestation`), one or more unique
declaration-ordered invalidators from
`spec|code|dependency|environment|policy`, and a nonblank rule. Structural
validation never claims the prose is substantively sufficient.

The implementation pins `ObligationQualityAdoptionCommit` to the exact
first-parent owner-merge commit that makes this plan and SI-71..SI-74 reachable.
An absent `quality` block is `legacy-unelaborated`, a valid compatibility
decode but never elaborated. Exact-tree audit at the planning base found zero
persisted bodies carrying the unauthored marker, correcting the earlier
planning count of 27. Marker-bearing bodies still assess unresolved, and every
new scaffold writer emits the marker plus explicit unresolved state. Frozen
legacy or unresolved obligations are remedied only by a new replacement
obligation on a successor story/spec through the normal specification
amendment ladder; this unit does not mutate them in place or invent an
obligation-supersession edge.

Adoption is prospective, not retroactive. When an authoritative fold is
explicitly evaluated at a commit that is an ancestor of or equal to the
adoption commit, absent-quality legacy obligations preserve the incumbent fold
meaning byte-for-byte. At any later evaluation commit, legacy absence can no
longer positively satisfy a kind: failing evidence remains a violation, while
pass/abstain stays pending with `legacy-unelaborated`. Evidence first produced
after adoption is never grandfathered merely because its obligation file is
old. Every post-adoption `build start` applies the new gate. An explicitly
present quality block always uses the new semantics, even during a historical
evaluation. Inability to prove the evaluation/adoption ancestry is operational.

Truth enforcement is unconditional; governance-profile adoption changes
ceremony, not proof meaning. This unit supplies no experimental escape because
there is no existing advisory `build start` surface. A later lifecycle-
governance unit may add a separately recorded advisory execution transition;
until then unresolved debt refuses all `build start` invocations.

The one `Assessment` is keyed per declared `(AC, evidence kind)` and first
returns `elaborated`, `unresolved-design-debt`, `legacy-unelaborated`,
`missing`, or an operational error for malformed/unreadable artifacts. An
elaborated declaration is only *eligible to match*; it is not proof by shape.
For every candidate evidence record, the same assessment also returns
`matched`, `violated-with-witness`, or `unproven` with one ordered reason code:
`producer-missing`, `producer-mismatch`, `source-mismatch`, `source-ref-missing`,
`source-ref-mismatch`, `freshness-stale`, or `freshness-unproven`.

Producer/source matching is exact and kind-aware:

- `static`, `behavioral`, and `runtime` obligations require producer kind
  `test|checker`, exact byte equality between `quality.producer.ref` and
  `Evidence.Producer`, authoritative source kind `ci-job`,
  `Evidence.Provenance.Source == ci`, and exact byte equality between
  `quality.authoritative_source.ref` and `Evidence.Provenance.Job`. An empty
  producer/job never matches.
- `attestation` requires `authenticated-human` plus
  `governed-attestation`. The current evidence record has no authenticated
  principal or governed-attestation identity; therefore this unit reports
  `source-ref-missing`/unproven and never turns an attestation pass into
  satisfaction. The later accountable-human-record unit may prove this through
  the same consumer port; this unit does not parse a witness string as identity.
- any other producer/source pairing is structurally invalid for that evidence
  kind and fails obligation decode.

Freshness uses the closed invalidator list mechanically; `rule` remains the
human-readable explanation and cannot weaken it. `code` requires the evidence
provenance commit to equal the evaluated commit. `spec` requires the evidence
commit to be a descendant of or equal to the obligation's source spec/story
first-parent acceptance landing commit, supplied by a consumer-local Git port.
The current evidence schema binds no dependency, environment or policy digest,
so each corresponding invalidator returns `freshness-unproven`; it can never
yield positive satisfaction in this unit. An unavailable landing/evaluation
commit or failed reachability query is operational, not unproven.

Missing/unresolved are verdict states. At authoritative fold time:

- a current failing record still yields `violated` with its witness;
- a waiver retains its existing higher precedence and is not evidence;
- otherwise an unelaborated/missing obligation or an elaborated-but-unmatched
  producer/source/freshness prevents a passing record from satisfying that
  kind, makes the kind pending and makes the story ineligible;
- preview/local folds remain advisory but display the same debt;
- malformed/I/O failure is operational.

`KindResult` gains an `ObligationQuality` projection with the structural state,
match state, ordered reason code and stable witness path. All fold consumers
receive it from the one fold; none re-derive quality.

`build start` checks all declared pairs after accepted/cascade proof and before
`RevParse`, branch creation or baseline work. Missing/unresolved/legacy-after-
adoption/unmatched returns exit 1 and names sorted pairs, states and reason
codes; malformed/unreadable/ancestry failure returns exit 2.

Journey adds reason code `obligation-design-unresolved`, blocker class
`mechanical`, and ID `obligation-quality/<ac-id>/<kind>`. One blocker is emitted
per non-elaborated or elaborated-but-unmatched pair, ordered by AC declaration
then evidence-kind declaration. `AffectedTransition` uses the registered CLI action identity
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
fields, illegal kind/source combinations, duplicates, whitespace, legacy
absence and marker bodies. Add the quality key to the obligation kernel
collision set. Pin the adoption commit and test ancestor/equal/after/divergent
classifications. Return structs; define the loader/Git interfaces at each
consumer.

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
precedence, pre-adoption legacy fold identity, post-adoption refusal, exact
producer and CI-job matching, missing producer/job, attestation unproven, every
freshness invalidator, preview disclosure, malformed/ancestry operational,
feature fold unchanged, and exact KindResult projection shared by every
consumer.

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
quality field assesses legacy-unelaborated, the exact base contains zero
persisted unauthored-marker bodies, every creation-surface scaffold emits the
marker and assesses unresolved, historical folds at/before the adoption commit
retain incumbent results, and post-adoption positive evidence cannot borrow
legacy meaning. Run fresh, unpiped:

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

This table is the auditable **26/26** source-to-destination witness. `Tn` means
the numbered task above.

| # | Source artifact | Destination | Test or intentional omission |
|---:|---|---|---|
| 1 | Workspace `AGENTS.md` | all tasks | TDD/full gates/no frozen edits |
| 2 | Workspace `CLAUDE.md` | launch/controller rules | exact-base handoff |
| 3 | Repository `CLAUDE.md` | package/verb boundaries | no duplicate verb/seam |
| 4 | `PLAN.md` | T6 gates | phase evidence preserved |
| 5 | `08-revision-notes` | adoption provenance | historical meaning preserved |
| 6 | Binding spec 00 | shared vocabulary | no lifecycle invention |
| 7 | Binding spec 01 | no new store root | existing obligation paths |
| 8 | Binding spec 02 | strict artifact union; T1 | codec negative tables |
| 9 | Binding spec 03 | fold/source/waiver; T4 | positive/negative precedence |
| 10 | Binding spec 04 | consumer-owned ports | loader/Git interfaces |
| 11 | Binding spec 05 | CLI exit/effects; T3 | zero-mutation refusal tests |
| 12 | GLG v3 | contract; T1-T5 | AC-2/DC-5/CO mapping |
| 13 | Context Integrity v2 | source/policy relationship | no second policy semantics |
| 14 | Four-feature orchestration | unit boundary/T6 | one bounded lane |
| 15 | Owner adjudications | custody/adoption | no authority duplication |
| 16 | Successor invention ledger | SI-71..SI-74 contract | row-to-test mapping |
| 17 | Active store-layout spec | existing paths/writer | no new root |
| 18 | Merge-signaled acceptance design | adoption commit | first-parent ancestry tests |
| 19 | Merge-signaled implementation plan | historical baseline | legacy fold identity |
| 20 | Operating model | `build:start` action | no catalog transition |
| 21 | Evidence-obligations record | exact pair semantics | per-kind assessment |
| 22 | Obligation-artifact record | quality union; T1 | strict decode/kernel tests |
| 23 | Obligation-gate record | T3/T4 | build/evidence gates |
| 24 | Obligation-seam record | T2 | every writer converges |
| 25 | Obligation-wall record | T3/T5 | deterministic blockers |
| 26 | Creation-surfaces record | T2 | scaffold/author/batch/workbench parity |

Transformations: the six prose dimensions become one closed quality union plus
an executable record match; frozen legacy absence becomes explicit
`legacy-unelaborated` under a prospective adoption cutoff; “cannot support
evidence” preserves negative witnesses while withholding unmatched positive
satisfaction; `build start` uses an action identity without entering the
operating-model catalog. Intentional omissions are experimental advisory
execution, authenticated-human receipt integration, frontend display, feature
obligations and frozen in-place migration. No source is silently grandfathered
or called complete.
