# ASD Draft-Mutation Core Implementation Plan

> For agentic workers: REQUIRED SUB-SKILL: use `superpowers:test-driven-development` for every task and `superpowers:verification-before-completion` before handoff.

**Goal:** deliver ASD Wave 2's model-neutral atomic draft-mutation core and one
structured CLI adapter, including deterministic provenance, policy enforcement,
semantic diff and crash recovery.

**Architecture:** `internal/draftmutation` owns the application service;
`internal/designprovenance` owns the shared sidecar schema/identity;
`internal/artifact/splice` owns byte-preserving edits; `cmd/verdi` is a thin
adapter. Workbench, MCP and context compilation remain later units.

**Tech stack:** Go 1.25, strict canonical JSON, existing artifact/policy/Git/
locking seams, fixturegit, built-binary CLI tests.

**Planning authority:** ASD blob `364c4888a21b9e234786c59c3ebfd42f2c1d8205`,
first-parent promotion `f72612d7bca3de2d37a6570240dc75dbbe728864`;
new SI-65..SI-70; orchestration Wave 2 and OD-3/5/8/10/12.

## Global constraints

- The launch base is the exact `origin/main` after this plan/ledger PR merges.
  Stop if the plan commit is not reachable from that base.
- No UI, workbench, MCP, Playwright, context compiler, review packet or policy
  schema changes.
- No Git branch/commit/push/PR/accept/align/evidence/close behavior.
- Do not extend `artifact.ClassifyPath`; the provenance sidecar is explicitly
  non-authoritative.
- All writes are under an exact active draft spec. Symlinks at the spec,
  sidecar, transaction directory, lock or any parent below the store root are
  operational refusals.

## Contract fixed by SI-65..SI-70

The CLI is `verdi design mutate --request <path|->`. Input and output are
strict canonical JSON. `-` means stdin; the maximum request is 1 MiB. Success
writes one `verdi.draftmutation-result/v1` object plus newline to stdout.
Refusals and operational errors write human-readable stderr only.

The adapter injects actor attribution. `VERDI_ACTOR_KIND` is exactly `human`
or `delegated-agent`; human requires `VERDI_PRINCIPAL_ID`, agent requires
`VERDI_HARNESS` and uses the kernel unauthenticated marker unless a verified
principal is supplied by the adapter. The request contains no actor field.

The closed operations are:

`set-problem`, `set-outcome`, `add-ac`, `edit-ac`, `remove-ac`, `reorder-ac`,
`set-ac-evidence`, `add-constraint`, `edit-constraint`, `remove-constraint`,
`add-decision`, `edit-decision`, `remove-decision`, `add-question`,
`edit-question`, `remove-question`, `add-link`, `remove-link`, `add-stub`,
`edit-stub`, `remove-stub`, `reorder-stub`, `add-context-ref`, and
`remove-context-ref`.

Every union arm rejects fields owned by another arm. IDs are stable semantic
IDs. A transaction may target one ID more than once; operations apply in wire
order and each step sees the previous step. Duplicate add, missing edit/remove,
illegal links and invalid reorder anchors are verdict refusals. V1 fills an
existing scaffold only and never creates `spec.md`.

The digest is `sha256:` over exact `spec.md` bytes. Object digests are
`canonjson.Digest` over the decoded semantic object. Policy digest is the
sealed effective-policy digest. Context identity is explicit
`unavailable-before-context-compiler` in v1, never caller-supplied proof.

The sidecar is canonical JSONL at the ratified active/archive paths. Each entry
contains schema, spec, previous/result digests, attribution, harness/session
when applicable, policy digest, context status, ordered operations, ordered
object digest changes, bounded excerpts and its own digest. Its digest excludes
only the `digest` field. Optional fields are omitted, never `null`; duplicate
entry digest or broken previous/result chain is operational. Excerpts are at
most 600 Unicode scalar values and three per target, and carry target, target
digest, classification, representation (`verbatim|paraphrase`) and text.
Removal/redaction appends a tombstone in a later transaction; history is never
rewritten.

Policy v1 consumes only the landed `mode` and `layout=false`. An unadopted
policy store or absent ASD payload means human structured writes are allowed
with an explicit ungoverned disclosure, while delegated-agent writes are
refused. `off` and `proposal-only` refuse agent writes; `draft-write` permits
them. Direct Markdown is disclosed when the current spec digest does not equal
the sidecar chain tip; the unclassified change is allowed in v1 because SI-30
contains no block field.

The transaction uses `.verdi/data/draft-mutation/<spec-name>/` with `lock`,
`journal.json`, `spec.new`, and `provenance.new`. The journal stores old/new
digests and phase. Under the per-spec lock, recovery validates every path and
digest and deterministically rolls forward: install provenance first, install
spec second, then remove the journal/staging files. Readers that require a
coherent typed state use this package's read seam and take the same lock. Raw
filesystem readers may observe the brief provenance-ahead state, but never a
typed spec edit without its provenance record. Parent directories are fsynced
after staging, each rename and cleanup on platforms that support it; failure is
operational and leaves a recoverable journal. A malformed journal is kept for
human attention and blocks further mutation.

Semantic changes sort by the first operation that touches the target, then
target ID. Kinds are `added`, `replaced`, `removed`, `reordered`,
`relationship-added`, and `relationship-removed`. Removed, reordered,
relationship and replacements over 1,000 Unicode scalar values add fixed
warning codes. A stale base is exit 1 and reports current digest only; changed
IDs are omitted because the stale bytes are unavailable.

## Task 1: Add the provenance contract

**Files:** create `internal/designprovenance/{doc,record,codec,identity}.go` and
tests; modify `internal/store/paths.go` and tests.

Write failing tests for strict decode, unknown/trailing/duplicate data,
attribution exclusivity, own-digest projection, JSONL newline, chain break,
active/archive identity and the fixed exclusion reason. Implement minimally.

Run: `go test -race ./internal/designprovenance ./internal/store -count=1`

Commit: `Define design provenance records`

## Task 2: Complete byte-preserving operations

**Files:** create `internal/artifact/splice/draftmutation.go` and tests.

Write table tests for every operation, illegal/missing/duplicate targets,
ordered batches and byte identity outside touched regions. Reuse existing
splice parsing and validation.

Run: `go test -race ./internal/artifact/splice -count=1`

Commit: `Complete draft mutation splices`

## Task 3: Add request, result and semantic diff

**Files:** create `internal/draftmutation/{schema,operation,apply,semanticdiff,errors}.go`
and tests/testdata.

Start with strict codec and golden RED tests, then pure apply/diff tests. Pin
every enum, result byte shape, warning, ordering and 1 MiB refusal.

Run: `go test -race ./internal/draftmutation -run 'Test(Decode|Apply|SemanticDiff)' -count=1`

Commit: `Define draft mutation transactions`

## Task 4: Enforce identity, state and policy

**Files:** create `internal/draftmutation/policy.go`, `identity.go` and tests.

Test exact checkout/branch/HEAD/spec, Git-derived draft state, sealed effective
policy, all actor/mode combinations, absent authority, layout=false and forged
policy failures. Define consumer-local ports for fakes.

Run: `go test -race ./internal/draftmutation -run 'Test(Identity|State|Policy)' -count=1`

Commit: `Enforce draft mutation authority`

## Task 5: Implement recovery before mutation

**Files:** create `internal/draftmutation/{transaction,recovery}.go` and tests;
add approved data paths to store helpers.

Fault-inject after each durable write, fsync, rename and cleanup. Test two
concurrent processes, stale lock takeover, malformed/tampered journals, symlink
refusals and old-or-new recovery. The first integration test must fail before
implementation and prove no reachable spec-ahead state.

Run: `go test -race ./internal/draftmutation -run 'Test(Transaction|Recovery|Concurrent)' -count=1`

Commit: `Recover coordinated draft mutations`

## Task 6: Compose the service

**Files:** create `internal/draftmutation/service.go`, `doc.go` and integration
tests.

Test stale base, batch rollback, direct-Markdown disclosure, context-unavailable
identity, policy/provenance binding and no write on every refusal.

Run: `go test -race ./internal/draftmutation -count=1`

Commit: `Apply atomic draft mutations`

## Task 7: Add the structured CLI

**Files:** create `cmd/verdi/designmutate.go` and tests; modify
`cmd/verdi/design.go` and dispatch tests.

Drive the built binary for stdin/file input, all actor modes, canonical result,
exit 0/1/2, stale/concurrent calls and malformed input. The adapter contains no
operation switch and no semantic validation.

Run: `go test -race ./cmd/verdi -run 'Test.*DesignMutate' -count=1`

Commit: `Expose structured draft mutation CLI`

## Task 8: Verify the Wave 2 boundary

Add static witnesses that no workbench/MCP adapter imports draftmutation yet,
the sidecar reason is consumable, and `artifact.ClassifyPath` is unchanged.
Run fresh, unpiped:

```sh
go test -race ./...
make verify
git diff --check
git status --short
```

Commit only test/doc corrections required by the gate. Report AC-1..4 as
partial exactly as follows: core/CLI delivered; workbench/MCP, capabilities,
context and semantic review remain pending.

## Source-coverage witness

Coverage is **51/51**: 29 Wave-2 feature units and 22 imposed/shared records.
The prior canonical promotion independently preserves **166/166** source
elements. Transformations: illustrative YAML becomes strict canonical JSON;
actor moves from caller payload to trusted adapter injection; context digest is
an explicit unavailable status until Wave 3. Intentional omissions are the
later adapters, compiler, review packet, UI and Git/governance verbs named
above. No source authority is silently dropped.
