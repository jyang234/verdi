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
The stale refusal writes its canonical refusal object to stdout. All other
verdict refusals and operational errors write a fixed code plus human-readable
detail to stderr only.

The wire request contains no actor field. The v1 CLI adapter is always a
`delegated-agent`/script adapter and always injects the kernel unauthenticated
attribution plus required `--harness <id>` and optional `--session <id>`
metadata. Environment variables and request bytes never select actor kind or
principal identity. The service's in-process `Actor` operand may carry a
principal attribution only when constructed from a sealed
`governanceprincipal.PrincipalResolution` by
`AttributionFromResolution`; authenticated yields a principal while sealed
violated/unproven resolution yields the explicit unauthenticated marker, and an
invalid/forged seal is operational. Human structured CLI attribution is
explicitly not delivered in this unit. A later trusted workbench or
authenticated harness adapter may construct that operand; merely naming a
principal grants nothing.

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
entry digest or an unexplained broken previous/result chain is operational. A
legal raw-Markdown gap is represented only on the next typed entry as
`unclassified_gap:{from_digest,to_digest}`: `from_digest` must equal the prior
entry's result and `to_digest` must equal this entry's previous digest. The gap
contains no actor, operations or excerpts and is not a retroactive provenance
record. Any other mismatch fails operationally. Excerpts are at
most 600 Unicode scalar values and three per target, and carry target, target
digest, classification, representation (`verbatim|paraphrase`) and text.
Removal/redaction appends a tombstone in a later transaction; history is never
rewritten.

Policy v1 consumes only the landed `mode` and `layout=false`. An unadopted
policy store or absent ASD payload refuses the v1 delegated-agent CLI; future
trusted human adapters are outside this unit. `off` and `proposal-only` refuse
agent writes; `draft-write` permits them. Direct Markdown is disclosed when the
current spec digest does not equal the sidecar chain tip, encoded by the exact
unclassified-gap rule above, and allowed because SI-30 contains no block field.

The transaction uses the ratified store-layout root
`.verdi/data/draft-mutation/<spec-name>/` with `journal.json`, `spec.new`, and
`provenance.new`. Before recovery or mutation, the standalone CLI acquires the
existing checkout-wide `.verdi/data/writer.lock`; if `verdi serve` or another
writer holds it, the CLI refuses operationally and performs no write. There is
no independent per-spec lock. The journal stores old/new digests and phase.
Under the global writer lock, recovery validates every path and digest and
deterministically rolls forward: install provenance first, install spec second,
then remove the journal/staging files. Readers inside the future migrated
writer use the same global lock; until workbench migration, the lock prevents
the incumbent workbench writer and CLI from running concurrently. Raw
filesystem readers may observe the brief provenance-ahead state, but never a
typed spec edit without its provenance record. Parent directories are fsynced
after staging, each rename and cleanup where supported; failure is operational
and leaves a recoverable journal. A malformed journal is kept for human
attention and blocks further mutation.

Semantic changes sort by the first operation that touches the target, then
target ID. Kinds are `added`, `replaced`, `removed`, `reordered`,
`relationship-added`, and `relationship-removed`. Removed, reordered,
relationship and replacements over 1,000 Unicode scalar values add fixed
warning codes `destructive-removal`, `semantic-reorder`,
`relationship-change`, and `large-replacement`. A stale base is exit 1 and
emits the canonical refusal record below with the current digest and every
added/changed/removed semantic target derived by comparing the required base
snapshot to current bytes.

## Exact wire grammar

All JSON object fields below are in the written order, unknown fields fail,
strings must be valid UTF-8 and nonempty unless explicitly optional, arrays
preserve order and reject duplicates where the semantic collection is a set.
The entire request, including the base snapshot, is at most 1 MiB.

```text
Request = {
  "schema":"verdi.draftmutation/v1",
  "spec":"spec/<name>",
  "base_digest":"sha256:<64-lower-hex>",
  "base_spec_b64":"<standard padded base64 of exact prior spec.md bytes>",
  "expected":{"checkout":"<canonical checkout root>","branch":"<exact branch>","head":"<40-lower-hex>"},
  "operations":[Operation, ...],
  "excerpts":[Excerpt, ...]                 // optional, omitted when empty
}
Excerpt = {
  "target":"<problem|outcome|object-id>",
  "classification":"human-stated|ai-synthesized|ai-inferred|unresolved",
  "representation":"verbatim|paraphrase",
  "text":"<1..600 Unicode scalars>"
}
```

`base_spec_b64` must decode to valid exact prior bytes whose SHA-256 equals
`base_digest`; it is used only for stale changed-target comparison and is never
stored in provenance. There are at most three excerpts per target. The core
computes each excerpt's target digest from the resulting object; callers never
supply it.

`checkout` is the exact worktree root returned by
`git rev-parse --show-toplevel`, made absolute and clean, resolved through
`filepath.EvalSymlinks`, then rendered with `/` separators. Empty, relative,
unresolvable, non-root-equivalent or non-POSIX forms are operationally invalid;
v1 supports the repository's Darwin/Linux platform set. The request's expected
checkout, branch and HEAD must byte-equal the three resolved operands before
any mutation. The shared response identity is:

```text
Identity = {
  "checkout":"<canonical checkout root>",
  "branch":"<exact short local branch>|DETACHED",
  "head":"<40-lower-hex>",
  "spec":"spec/<name>"
}
```

Detached HEAD is not a design branch and yields a state refusal whose exact
branch value is the literal `DETACHED`; the field is never empty. The service
constructs `Identity` once after request decode and threads that same value
through success and every typed refusal; adapters never reconstruct it.

Every `Operation` has `op` first and exactly the fields in its row:

| `op` | Remaining fields |
|---|---|
| `set-problem`, `set-outcome` | `text`, `anchor` |
| `add-ac`, `edit-ac` | `id`, `text`, `evidence` (nonempty unique list of shared evidence kinds), `anchor` |
| `remove-ac` | `id` |
| `reorder-ac` | `id`, optional `after_id` (omitted means first) |
| `set-ac-evidence` | `id`, `evidence` |
| `add-constraint`, `edit-constraint` | `id`, `text`, `anchor` |
| `remove-constraint` | `id` |
| `add-decision`, `edit-decision` | `id`, `text`, `anchor` |
| `remove-decision` | `id` |
| `add-question`, `edit-question` | `id`, `text`, `anchor` |
| `remove-question` | `id` |
| `add-link`, `remove-link` | `source` (`spec` or a declared decision id), `type`, `ref`, optional `note`; removal matches all supplied bytes exactly |
| `add-stub`, `edit-stub` | plain arm: `slug`, `acceptance_criteria`; spike arm: `slug`, `spike:true`, `resolves`; incompatible fields are omitted and rejected |
| `remove-stub` | `slug` |
| `reorder-stub` | `slug`, optional `after_slug` (omitted means first) |
| `add-context-ref`, `remove-context-ref` | `ref` |

An edit replaces the complete row named by its ID/slug; it is never a partial
patch. Object body prose outside the named anchor is left byte-identical.
Removal deletes the structured declaration only and leaves body prose for
human cleanup, adding `destructive-removal`. Context refs and exact link tuples
are sets and reject duplicates. Operation text and anchor fields are bounded by
the existing artifact decoder; the request-level 1 MiB ceiling is the v1
resource bound.

The result unions are exact:

```text
Change = {
  "target":"problem|outcome|<object-id>|stub/<slug>|context/<ref>|link/<source>/<type>/<ref>",
  "change":"added|replaced|removed|reordered|relationship-added|relationship-removed",
  "before_digest":"sha256:<64-lower-hex>",  // omitted for added/relationship-added
  "after_digest":"sha256:<64-lower-hex>"    // omitted for removed/relationship-removed
}
Warning = {
  "code":"destructive-removal|semantic-reorder|relationship-change|large-replacement",
  "target":"<same target grammar as Change>"
}
Disclosure =
  {"code":"unclassified-direct-edit","from_digest":"sha256:<64-lower-hex>","to_digest":"sha256:<64-lower-hex>"}
| {"code":"context-unavailable","reason":"unavailable-before-context-compiler"}
Result = {
  "schema":"verdi.draftmutation-result/v1",
  "identity":Identity,
  "previous_digest":"sha256:<64-lower-hex>",
  "result_digest":"sha256:<64-lower-hex>",
  "changes":[Change, ...],
  "warnings":[Warning, ...],
  "disclosures":[Disclosure, ...]
}
```

Empty changes/warnings/disclosures are encoded as `[]`. Changes and warnings
use operation-first/target-second ordering; disclosures use the order shown.
Each target component uses RFC 3986 percent encoding over UTF-8 bytes with
uppercase hex, leaves only ASCII unreserved bytes (`ALPHA / DIGIT / -._~`)
literal, and is then joined with `/`, so target identity is reversible. Stale
refusal is the only verdict record written to stdout:

```text
{"schema":"verdi.draftmutation-refusal/v1","identity":Identity,"code":"stale-base","current_digest":"sha256:<64-lower-hex>","changed_targets":["<Change target>","..."]}
```

Changed targets are sorted canonical semantic IDs and include `problem` and
`outcome`; deletion is represented by the absent current target still named in
the list. Every internal typed refusal contains `identity:Identity`. Other
verdicts use fixed stderr codes `state-forbidden`, `policy-forbidden`,
`actor-forbidden`, `operation-invalid`, and `result-invalid`; the CLI renders
the code and canonical identity before human detail. Operational failures use
`input-invalid`, `identity-invalid`, `authority-invalid`, `recovery-invalid`,
or `io-failure`; after strict request decode they carry/render the same exact
identity. A failure before the request yields a valid spec is an input-decoder
diagnostic rather than a mutation response and cannot truthfully name a spec.

The sidecar entry is also exact. `Operation` is the same strict union decoded
from the request; `Change` is the result union above. The CLI arm always writes
`attribution:{"unauthenticated":true}` and `harness`; trusted future adapters
may instead write `attribution:{"principal_id":"principal/<id>"}` and omit
`harness`/`session`. The two attribution arms are exclusive.

```text
ProvenanceExcerpt = {
  "target":"<Change target>",
  "target_digest":"sha256:<64-lower-hex>",
  "classification":"human-stated|ai-synthesized|ai-inferred|unresolved",
  "representation":"verbatim|paraphrase",
  "text":"<1..600 Unicode scalars>"
}
ProvenanceEntry = {
  "schema":"verdi.design-provenance/v1",
  "spec":"spec/<name>",
  "previous_digest":"sha256:<64-lower-hex>",
  "result_digest":"sha256:<64-lower-hex>",
  "unclassified_gap":{"from_digest":"sha256:<64-lower-hex>","to_digest":"sha256:<64-lower-hex>"}, // optional
  "attribution":{"principal_id":"principal/<id>"}|{"unauthenticated":true},
  "harness":"<nonblank>",              // optional; required for CLI arm
  "session":"<nonblank>",              // optional; CLI only
  "policy_digest":"sha256:<64-lower-hex>",
  "context":{"state":"unavailable","reason":"unavailable-before-context-compiler"},
  "operations":[Operation, ...],
  "changes":[Change, ...],
  "excerpts":[ProvenanceExcerpt, ...],
  "digest":"sha256:<64-lower-hex>"
}
```

`unclassified_gap` is omitted when the prior sidecar tip equals
`previous_digest`. Empty changes/excerpts are `[]`; a normal mutation requires
at least one operation. The own digest is canonical JSON of the full entry with
only `digest` omitted. `policy_digest` is required because the delivered CLI
cannot mutate without adopted, sealed policy.

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
every enum, per-operation field set, result/refusal byte shape, warning,
ordering, base-snapshot digest check, stale changed-target computation and 1
MiB refusal. Pin `Identity` on success and every typed refusal. Reject
request-supplied attribution and unknown optional fields.

Run: `go test -race ./internal/draftmutation -run 'Test(Decode|Apply|SemanticDiff)' -count=1`

Commit: `Define draft mutation transactions`

## Task 4: Enforce identity, state and policy

**Files:** create `internal/draftmutation/policy.go`, `identity.go` and tests.

Test canonical symlink-resolved checkout representation and exact expected
checkout/branch/HEAD/spec, Git-derived draft state, sealed effective
policy, delegated-agent/harness inputs, sealed-resolution-only in-process
attribution, all mode combinations, absent authority, layout=false and forged
policy failures. Define consumer-local ports for fakes. The CLI never exercises
a human actor path.

Run: `go test -race ./internal/draftmutation -run 'Test(Identity|State|Policy)' -count=1`

Commit: `Enforce draft mutation authority`

## Task 5: Implement recovery before mutation

**Files:** create `internal/draftmutation/{transaction,recovery}.go` and tests;
add approved data paths to store helpers.

Fault-inject after each durable write, fsync, rename and cleanup. Use only the
existing checkout-wide `data/writer.lock`; test contention with `verdi serve`,
two concurrent processes, stale global-lock takeover, malformed/tampered
journals, symlink refusals and old-or-new recovery. Prove no per-spec lock is
created. The first integration test must fail before implementation and prove
no reachable spec-ahead state.

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

Drive the built binary for stdin/file input, required `--harness`, optional
`--session`, rejection of actor/principal environment spoofing, canonical
result/refusal identity, identity-bearing post-decode stderr, exit 0/1/2,
stale/concurrent calls and malformed input. The adapter contains no operation
switch and no semantic validation.

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

The table is the auditable **51/51** source-to-destination witness: 29 Wave-2
feature units and 22 imposed/shared records. `Tn` means the numbered task above;
`contract` means the exact grammar/constraints in this plan. The prior
canonical promotion independently preserves **166/166** source elements.

| # | Source unit | Destination | Test or intentional omission |
|---:|---|---|---|
| 1 | ASD AC-1 atomic mutation | contract; T2/T5/T6 | fault and rollback tests |
| 2 | ASD AC-2 attribution | contract; T1/T4/T7 | spoofing and sealed-resolution tests |
| 3 | ASD AC-3 policy control | contract; T4 | mode/absence tables |
| 4 | ASD AC-4 provenance | contract; T1/T6 | chain/digest/excerpt tests |
| 5 | ASD DC-1 one coordinated mutation | T5/T6 | old-or-new recovery witness |
| 6 | ASD DC-2 actor attribution | trusted-actor contract | CLI cannot claim human |
| 7 | ASD DC-3 semantic operations | wire table; T2/T3 | every arm positive/negative |
| 8 | ASD DC-4 provenance chain | gap grammar; T1 | explained/broken-chain tables |
| 9 | ASD DC-5 policy custody | T4 | sealed policy only |
| 10 | ASD DC-6 durability | T5 | every write/fsync/rename fault |
| 11 | ASD DC-7 mode enforcement | T4 | closed mode matrix |
| 12 | ASD DC-8 request boundary | exact wire; T3/T7 | strict codec/built binary |
| 13 | ASD DC-9 identity | request/response Identity; T3/T4/T7 | canonical checkout/branch/HEAD/spec mismatch and echo |
| 14 | ASD DC-10 stable IDs | operations; T2/T3 | duplicate/missing target tests |
| 15 | ASD DC-11 ordered batch | operations; T2/T6 | order and rollback tests |
| 16 | ASD DC-12 semantic diff | result grammar; T3 | deterministic change golden |
| 17 | ASD DC-13 digest projections | contract; T1/T3 | exact bytes/canonical object tests |
| 18 | ASD DC-16 recovery | global-lock journal; T5 | crash matrix |
| 19 | ASD DC-17 path safety | constraints; T5 | symlink/path substitution refusals |
| 20 | ASD CO-1 single draft writer | global writer lock; T5 | serve/contention integration |
| 21 | ASD CO-2 no partial transaction | provenance-first recovery; T5 | no spec-ahead witness |
| 22 | ASD CO-3 draft-only | T4 | accepted/closed refusal |
| 23 | ASD CO-4 caller cannot self-promote | trusted actor; T3/T7 | actor fields/env rejected |
| 24 | ASD CO-5 provenance honesty | gap/unavailable contract; T1/T6 | no fabricated context/chain |
| 25 | ASD CO-6 governed assistance | policy contract; T4 | absent/off/proposal/draft-write |
| 26 | ASD CO-9 fail-closed recovery | T5 | malformed journal retained |
| 27 | ASD relationship to CI | policy digest/port; T4/T6 | one sealed policy seam |
| 28 | ASD delivery step 1 | T1-T8 | core plus structured CLI only |
| 29 | ASD non-goals | global constraints | adapters/UI/publication intentionally omitted |
| 30 | Orchestration Wave-2 slot | launch packet A | one bounded implementation unit |
| 31 | Orchestration shared artifact ownership | `internal/artifact/splice`; T2 | no duplicate decoder/model |
| 32 | Orchestration shared authority ownership | policy/principal ports; T4 | no feature-local authority |
| 33 | Orchestration gate discipline | T8 | full required gates |
| 34 | OD-3 attribution custody | trusted actor contract | sealed resolution only |
| 35 | OD-5 three-valued honesty | result/disclosure grammar | unavailable remains disclosed |
| 36 | OD-8 sidecar custody | T1 | design-provenance path/codec |
| 37 | OD-10 instruction/CLI custody | T7 | thin registered adapter |
| 38 | OD-12 store ownership | ratified store amendment; T5 | named data root/global lock |
| 39 | SI-30 assistance mode | policy contract; T4 | use landed mode only |
| 40 | SI-31 merge semantics | T4/T8 | Git-derived state, no retired accept |
| 41 | SI-33 policy/profile provenance | T1/T4 | sealed effective digest |
| 42 | SI-34 journey/shared identity | omissions/T8 | no parallel lifecycle identity |
| 43 | SI-37 direct-edit posture | gap grammar; T1/T6 | explicit unclassified gap |
| 44 | Store layout committed/data split | store amendment; T1/T5 | sidecar committed, journal ignored |
| 45 | Store layout D3 one writer | global lock; T5 | no second lock |
| 46 | Artifact strict decode | T1/T3 | unknown/duplicate/trailing fail |
| 47 | Policy-authority contract | T4 | one load/resolve/digest seam |
| 48 | Governance-principal contract | T1/T4 | kernel attribution constructor |
| 49 | Human-artifact contract | T2/T8 | existing spec model/render seam preserved |
| 50 | Repository AGENTS.md | all tasks | TDD, no network, full gates |
| 51 | Repository CLAUDE.md | launch/T8 | exact-head authority and serialized verbs |

Transformations: illustrative YAML becomes strict canonical JSON; actor moves
from caller payload to a delegated-agent CLI and a sealed-resolution-only
in-process operand; direct edits become an explicit `unclassified_gap`; context
digest is an explicit unavailable status until Wave 3. Intentional omissions
are the later workbench/MCP adapters, compiler, review packet, UI and
Git/governance verbs named above. No source authority is silently dropped.
