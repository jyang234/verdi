# F13 review journal and restart replay

> One Sonnet implementation flight under `/fable-orchestration`, TDD, one
> independent Opus review, main adjudication, and at most one correction and
> same-reviewer closure. Main authors this contract and runs final gates.

**Goal:** Persist and reconstruct the accepted candidate-review core without
serializing its private state or refunding consumed rounds after restart.

**Authority:** v0.5 §§4.5, 6(3), 7 invariants, 13(4,6); Amendment 001 §§6–8;
PLAN F13 and IL-160–161; the accepted transition-core contract at `8952e42` and
runtime at `9ddeb04`. This is a bounded F13 prerequisite, not full integration.

**Architecture:** `internal/reviewlog` owns a strict internal record codec,
pure replay through `gatekeeper.New` and its public transitions, and a small
compare-and-append adapter over the existing `ledger.Store`. No other package
changes. Go 1.25, existing dependencies, no network or cgo.

## Boundary and trust

F12's one `vatc.flight-handoff/v1` remains the immutable implementation result.
The new review history is a successor stream anchored to that result, not a
second `vatc.candidate.produced` row. Candidate changes in the successor are
inputs to `Machine.ChangeCandidate`; they cannot alter the original handoff or
F12's retry budget. This chooses the representation for the successor, not the
production bridge between the two owners.

The caller supplies an expected Anchor after independently checking the F12
handoff, approved plan, clean Git candidate, recorder and Verdi authority.
Loading compares the stored initialization with that complete expected value.
Anchor digests are references, not authentication; no copied history can prove
fresh Git, Verdi, receipt or recorder authority by itself. Proof.Proven and
freshness fields retain the core's trusted-caller meaning. Stored Decisions
are checked calculations, never accomplished effects. Returned actions must
not be dispatched merely because Load returned them: a future effect owner
must reconcile durable intent/completion and fresh evidence first.

The adapter requires a dedicated supplied ledger stream containing only this
one review history. It does not open a database or select a path. Tests may
instantiate the existing persistence/ledger implementation in a temporary
database. Production shared-ledger routing, board vocabulary/projection,
recorder mirroring, CLI/API input, machine Verdi contracts, effect dispatch,
G2 resets, new epochs and F14 remain excluded. In particular, do not insert
these records into the current production orchestration ledger: its board
does not yet recognize them. No migration, second runtime append owner,
installed-binary change or hosted action is introduced by this slice.

The threat model covers valid caller inputs, malformed or conflicting stored
records, concurrent appends, errors with uncertain commit outcomes and process
restart. The existing ledger's integrity/whole-replay contract is trusted.
An attacker replacing the entire database and expected Anchor together, or
forging all caller-verified evidence, is outside this adapter's authority.
Hash links detect broken retained history; they do not authenticate it or
detect removal of a complete suffix from an externally replaced store.

## Internal representation

Reuse every gatekeeper domain type and validator. No JSON tags or runtime
behavior change in gatekeeper. Nested domain objects use `encoding/json`'s
default treatment: existing tags where present (for example
`protocol.ExecutionIdentity`), otherwise Go field names (for example
`Candidate.Identity` and `Proof.Proven`). Do not hand-write a domain marshaller.
This explicit private representation avoids a second set of candidate,
finding, reviewer or receipt DTOs. It is not a public Verdi/provider protocol.

```go
type Anchor struct {
    PlanDigest string `json:"plan_digest"`
    HandoffDigest string `json:"handoff_digest"`
    Candidate gatekeeper.Candidate `json:"candidate"`
    Reviewer gatekeeper.Reviewer `json:"reviewer"`
}
type Head struct { Sequence uint64; Digest string }
type AlignInput struct {
    Proof gatekeeper.Proof `json:"proof"`
    ExitCode int `json:"exit_code"`
    HasFindings bool `json:"has_findings"`
}
type DispositionInput struct {
    Proof gatekeeper.Proof `json:"proof"`
    FullyDispositioned bool `json:"fully_dispositioned"`
}
type ReturnInput struct {
    Result gatekeeper.ReviewResult `json:"result"`
    Findings []gatekeeper.Finding `json:"findings"`
}
type AdjudicateInput struct {
    Proof gatekeeper.Proof `json:"proof"`
    Records []gatekeeper.Adjudication `json:"records"`
    RecordDigest string `json:"record_digest"`
    CorrectionRequested bool `json:"correction_requested"`
    ConflictingBlockers bool `json:"conflicting_blockers"`
}
type EscalateInput struct { Witness string `json:"witness"` }
type Input struct {
    Initialize *Anchor `json:"initialize,omitempty"`
    Align *AlignInput `json:"align,omitempty"`
    Disposition *DispositionInput `json:"disposition,omitempty"`
    OpenReview *gatekeeper.ReviewPacket `json:"open_review,omitempty"`
    ReturnReview *ReturnInput `json:"return_review,omitempty"`
    Adjudicate *AdjudicateInput `json:"adjudicate,omitempty"`
    ChangeCandidate *gatekeeper.Candidate `json:"change_candidate,omitempty"`
    Escalate *EscalateInput `json:"escalate,omitempty"`
}
type Record struct {
    Schema string `json:"schema"`
    PreviousDigest string `json:"previous_digest"`
    Input Input `json:"input"`
    Decision gatekeeper.Decision `json:"decision"`
}
```

Exactly one nonnil Input arm is required. The record schema is
`vatc.review.record/v1`; its ledger kind is `vatc.review.transition`.
PreviousDigest is empty only on initialization and otherwise a valid
`sha256:` digest of the immediately preceding record's exact canonical payload
bytes, including LF. PlanDigest/HandoffDigest must be valid digests. Anchor
Candidate/Reviewer validation delegates to `gatekeeper.New`.

`Encode(record Record) ([]byte,error)` and `Decode(raw []byte) (Record,error)`
own this schema. Both require supported schema, one arm and the corresponding
previous-digest shape. Encode rejects every invalid-UTF-8 string anywhere in
the record as ErrRecord before marshaling, including findings, rationales,
identities and witnesses. JSON replacement characters must never silently
change an input or Anchor. Prepare consequently refuses these inputs before
append; valid Unicode, including a genuine U+FFFD, remains allowed.
Decode uses `artifact.DecodeJSON`, then compares bytes
against its canonical encoding: exact keys/case, all non-optional members
(including false/zero members), sorted keys, no HTML escaping, one LF, no
unknown/duplicate/trailing values. Null cannot stand for a required scalar or
object; null list and empty list are both allowed representations of empty
Findings/Records. Encode does not decide stateful transition validity; Replay
does. Never persist a Machine or trust caller-supplied counts/state.

## Replay and append API

```go
func Replay(anchor Anchor, events []ledger.Event) (History, error)
func (h History) Initialized() bool
func (h History) Head() Head
func (h History) Snapshot() gatekeeper.Snapshot
func (h History) Decision() gatekeeper.Decision
func (h History) Prepare(input Input, actor string, at time.Time) (ledger.Event, error)

func New(store ledger.Store, anchor Anchor) (*Journal, error)
func (j *Journal) Load(ctx context.Context) (History, error)
func (j *Journal) Append(ctx context.Context, expected Head, input Input,
    actor string, at time.Time) (History, error)
```

History holds private machine, anchor, initialized flag, head and last decision.
Head.Digest is empty before initialization and otherwise the `sha256:` digest
of the last record's exact canonical payload bytes including LF: the same
value the next record's PreviousDigest carries. Head.Sequence is the last
committed ledger sequence, zero before initialization.
It is immutable to callers; no retained slice/pointer alias to caller inputs or
stored event bytes may change later results. Zero History is unusable for
Prepare. `Replay(validAnchor,nil)` returns an uninitialized anchored History,
zero Head and zero Snapshot/Decision without error. New rejects nil store or
invalid Anchor; a Journal is constructed only through New.

Replay validates the complete supplied stream before returning success. Events
start at sequence 1 and are contiguous; schema, kind and flight must match;
flight equals expected Anchor.Candidate.Identity.FlightID. Validate other event
metadata through the existing ledger event validator on a sequence-zero copy.
The first record must initialize with exactly the expected Anchor and empty
PreviousDigest. Initialize is forbidden thereafter, even after NeedsDecision.
An empty history cannot consume another operation. Subsequent links must match
the prior payload digest. Interleaved flights, unrelated kinds and gaps are
errors; this API explicitly requires a dedicated whole stream.

Initialization calls `gatekeeper.New`, with computed Decision
`{Action: ActionRunAlign, Witness: "review history initialized at the bound candidate"}`.
Every other input calls the corresponding unchanged core method exactly once.
An error refuses the history; no prefix or partial History is returned. Each
record's Decision must equal the recomputed Action and Witness exactly. A
core quarantine, NeedsDecision, or valid no-op is a valid journaled outcome;
it advances the log head while preserving the core's prescribed state/budgets.
Shape errors and forbidden source transitions are not journaled.

Prepare computes the next record and returns a validated, uncommitted
ledger.Event (Sequence=0), with caller actor/time, fixed kind/schema/flight and
canonical payload. It never changes History. Initialize must equal the bound
Anchor. It uses the same internal operation application routine as Replay;
never duplicate the gatekeeper transition table. No wall-clock reads.

Load obtains the complete stream with `store.Replay(ctx,0)` and runs Replay.
Append loads fresh, requires exact expected Head (sequence AND digest), prepares
the input, then invokes `store.Append(ctx, expected.Sequence, event)` once.
Mismatch returns an error wrapping `ledger.ErrConflict`; no automatic retry.
The store's global compare-and-append resolves the remaining race. Do not cache
a head or machine in Journal. On a durable-append error, return an error and
zero History even if the write might have committed; the caller must Load
before deciding its next action. Never infer that an error means no commit.
On success, require the acknowledgement to match the offered schema, kind,
flight, actor, instant and exact payload, with Sequence=expected.Sequence+1
(refuse sequence overflow before append). Time-zone normalization may preserve
the same instant. Validate the returned committed event and fold it with the loaded
history through the same replay rules; return only that accepted History.
An invalid store acknowledgement is an error, not a success claim or rollback.

Use contextual errors and preserve wrapped ledger/store errors. Distinguish
record/history validation from an ordinary compare-and-append conflict without
parsing prose. Package-local `ErrRecord` and `ErrHistory` sentinels suffice.
No new external error taxonomy or exit code is invented.

## Implementation and evidence

Write set: new `internal/reviewlog/schema.go`, `codec.go`, `replay.go`,
`journal.go`, and focused `*_test.go`; committed fixture bytes only under
`internal/reviewlog/testdata/` if needed. Existing gatekeeper, executor, board,
ledger, recorder, dependencies and frozen authority remain unchanged.

- [ ] Write a failing restart test: initialize → Align → R0 open/return →
  adjudicate correction → changed candidate → Align → R2 open/return → author
  adjudication → Countersigning. Capture RED before implementation.
- [ ] Implement strict record encoding and replay. For every prefix, reopen
  from bytes and compare the result and next operation with the continuously
  applied core; include restart with pending findings and pending packet.
- [ ] Prove no-correction path, all eight operations, repeated R0/R1/R2 denial,
  invalidation at each round, streak survival across candidate changes,
  historical R0 binding in R2, quarantine/no-op persistence and terminal G2.
- [ ] Reject wrong anchors/flights/kinds/schema, reordered/missing/duplicate
  rows, bad links, second initialization, forged Decisions, malformed union,
  invalid UTF-8 before append (including Anchor strings and witnesses), valid
  Unicode round trips, omitted/null required fields, unknown keys/case/enums, duplicates and
  trailing/noncanonical JSON. Test caller slice/byte alias isolation.
- [ ] Implement Journal with table-driven fake store tests for read failure,
  stale sequence/digest, append refusal, malformed acknowledgement, and
  commit-then-error. A fresh Load after the latter must expose the committed
  transition; no duplicate round or automatic retry follows.
- [ ] Use a real temporary SQLite ledger: close/reopen after each review
  boundary; two concurrent writers from one Head yield one success and one
  conflict, with exactly one new committed row. No filesystem snapshot or
  conversation state is a recovery input.
- [ ] Worker runs `go test -count=1 -v ./internal/reviewlog`,
  `go test -race -count=1 ./internal/reviewlog`, gofmt, vet and focused lint.
  Main adjudicates the consolidated Opus review, routes accepted code defects
  to Opus, then runs `make verify` and `go test -race -count=1 ./...` on the
  exact final candidate before a local imperative commit.

## Coverage witness

| Source obligation | Destination / transformation |
|---|---|
| v0.5 §4.5 and §6(3), bounded author/reviewer cycle | Reuse accepted core transitions; replay never restores caller counters |
| v0.5 §7 candidate binding and invalidation | Expected Anchor plus ChangeCandidate input; current proofs still caller-verified |
| v0.5 §13(4), replay without memory | Full persisted inputs through New/transition methods; SQLite reopen tests |
| v0.5 §13(6), witnessed gate decisions | Exact recomputed Decision persisted with typed input and ledger metadata |
| Amendment 001 §7, retained R0/R2 context and continuity | Replay pending packets/findings and historical digests; fresh runtime verification deferred explicitly |
| Amendment 001 §6, durable append before projection | No success History before append acknowledgement; uncertainty requires Load; recorder/effect integration excluded |
| Amendment 001 §8, fail-closed outcomes | Core quarantine and G2 retained; malformed history/load errors refuse progress; recorder, context/write-access and runtime suspension integration explicitly deferred |
| IL-160, immutable core/private state and F12 boundary | Separate anchored successor, unchanged core/F12 and no serialized Machine |
| IL-161, journal representation scope | Dedicated supplied stream, unchanged F12 handoff, exact expected head, and explicit exclusion of production/shared-ledger wiring |

Coverage: 9/9 named source obligations mapped. This adds internal representation;
it promotes no new canonical specification and omits no source authority.
