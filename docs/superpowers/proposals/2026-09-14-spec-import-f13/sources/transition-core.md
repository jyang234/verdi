# F13 candidate-bound review transition core

> Implementation uses the existing Stage 1 F13 plan and TDD, with one focused
> implementation flight and independent review. Main owns this contract.

**Goal:** Exercise the bounded review cycle locally, from a committed candidate
through permission to request a countersign, with explicit G2 and quarantine
outcomes. This is the next F13 slice, not full F13/F14 completion.

**Authority:** v0.5 §§4.5, 6(3), 7 Phase A and invariants, 15 D5/D7;
Amendment 001 §§5, 7–8; PLAN §§5–6 and IL-158–160. Amendment 004's closure
order remains intact and outside this slice.

**Architecture:** A pure, in-memory transition machine in `internal/gatekeeper`.
It consumes typed observations whose authentication and durable provenance
remain the caller's obligation. It produces next state and required action;
it launches no process, writes no ledger, and asserts no real gate proof.
Go 1.25.0, existing module and standard library only, no cgo or network.

## Boundary and representation adjudication

The existing F12 `vatc.flight-handoff/v1` stays unchanged. F12 recovery rejects
a second candidate record for one flight. Consequently this slice does not
append candidate records, change board replay or wire the machine to F12.
Future integration must reconcile that durable candidate-chain/replay boundary
before connecting the machine; this may require more than an adapter.
Likewise fresh Verdi alignment and receipt verification still require their
machine contracts; typed observations here do not substitute for them.

Start at `Aligning` after a caller has checked the committed F12 handoff,
clean Git state, recorder continuity and Verdi authority. Earlier scheduler
and BuildStart behavior and later countersign/gate/landing behavior are not
reimplemented. Publish the complete F13 state catalog for consistent naming,
but expose no transition into deferred states. All-stories-Done → AwaitingUAT
and human G3 stay outside this candidate review machine.

Reuse `protocol.ExecutionIdentity`, `runplan.ValidDigest`,
`gitproto.ValidateObjectID`, existing Finding/Adjudication types and validators.
Do not duplicate receipt schemas or import sibling repositories. Existing
review validators remain behaviorally unchanged.

Types and names below are the internal contract. None has JSON/YAML tags or a
wire codec. No generic map, provider text or serialized event is accepted.

```go
type Candidate struct {
    Identity protocol.ExecutionIdentity // author candidate identity
    HeadDigest string // opaque expected head binding, not a newly invented hash algorithm
    Clean bool
}
type Reviewer struct { Lane, Adapter, Model, Version, Profile string }
type Proof struct {
    Candidate Candidate
    Digest string // reference to the caller-verified durable evidence
    Proven bool // caller assertion, never authentication performed by this core
}
type ReviewPacket struct {
    Proof Proof
    Reviewer Reviewer
    Execution protocol.ExecutionIdentity // reviewer execution, separate from author identity
    PacketDigest string
    Fresh bool
    BuilderConversationExcluded bool
    AdjudicationDigest string // empty for R0; exact retained author record for R2
}
type ReviewResult struct {
    Proof Proof
    PacketDigest string
    Execution protocol.ExecutionIdentity
}
```

Candidate validation requires all identity strings nonblank, lane in
`runplan.Lanes()`, epoch positive, valid commit/tree object IDs, valid head
digest and Clean=true. ManifestRevision may be zero. Reviewer fields are all
nonblank and its lane must be known and different from the author's lane.
The head digest is supplied by the caller's accepted binding; this contract
neither derives one nor equates it with a manifest or packet digest.
Proofs require a valid digest and exact equality of the entire current
Candidate. Validate all input shape requirements first, then source-state and
round ordering, then candidate/packet binding, then proven/freshness flags,
then semantic routing. Proof candidate shape permits Clean=false so it can
receive the required quarantine outcome; invalid identity/digest syntax is an
error. A well-formed dirty or stale binding with an invalid evidence digest
therefore returns a shape error, not quarantine. A stale/dirty/wrong-epoch result is quarantined: no progress, no
review budget spent, state unchanged, a witnessed quarantine action. A valid
current but unproven proof routes to NeedsDecision. Malformed types, unknown
enums, impossible calls and missing required fields return contextual errors
with the original machine unchanged and no action. No error silently passes.

`Machine` holds private state, current candidate, configured reviewer, review
counts, alignment operational streak, retained findings and adjudication
record, R0 packet identity, and whether R2 has returned. Construct it only via
`New(candidate Candidate, reviewer Reviewer) (Machine, error)`; zero Machine
is unusable and all transitions refuse it. `Snapshot() Snapshot` returns a
value summary with State, Candidate, R0Count/R1Count/R2Count (uint8),
AlignOperationalFailures (uint8), R0AdjudicationDigest and
R2AdjudicationDigest (string). No mutable
findings or internal slices escape. Every transition has signature
`(Machine, Decision, error)` and returns independent values; it must neither
mutate its receiver nor retain aliases to caller slices. Findings are copied
on ingestion and never modified in place; branching two transitions from the
same Machine value must leave both branches and the original independent.

`Decision` has `Action Action` and `Witness string`; Action is a closed string
set: `none`, `run-align`, `request-disposition`, `open-r0`, `adjudicate`,
`correct-r1`, `open-r2`, `request-countersign`, `request-g2`, `quarantine`.
These are suggestions to a future effect owner, not accomplished effects.
Every non-none decision carries a nonblank deterministic witness. Successful
OpenReview also carries a witness identifying the round and packet despite
action none. Caller-supplied witness text must be scrubbed before entering this
model; the future recorder must still apply its required redaction.

## Allowed transitions

Expose methods below on Machine. Signatures use the existing review types.
Calls are allowed only in the stated source states, except candidate change
and escalation as specified below. Returning a review is distinct from
opening it; count the budget at opening so invalidation cannot refund it.

| Method and inputs | Source | Result |
|---|---|---|
| `Align(proof Proof, exitCode int, hasFindings bool)` | Aligning | Exit 2 increments consecutive operational streak: first stays Aligning/run-align, second NeedsDecision/request-g2. Exit 0 with no findings routes to the next review-opening state. Exit 1 requires hasFindings=true and routes AlignDisposition/request-disposition. Exit 0 with findings also routes AlignDisposition. Unknown exits and exit 2 with findings are malformed; exit 1 without findings is a witnessed dead end → G2. Any nonoperational align resets the streak. |
| `Disposition(proof Proof, fullyDispositioned bool)` | AlignDisposition | False stays requesting disposition. True means the caller proves fresh, fully dispositioned current Verdi report; route to the next review-opening state. No local author decision replaces the human disposition. |
| `OpenReview(packet ReviewPacket)` | ReviewR0 or ClosureCheckR2 | Validate fresh sealed packet, configured reviewer, candidate and execution; spend the appropriate count once, stay in state with action none. A second opening is forbidden. |
| `ReturnReview(result ReviewResult, findings []Finding)` | ReviewR0 or ClosureCheckR2, with that round opened and not returned | Validate findings, result proof and exact PacketDigest/Execution against the active opened packet; a binding mismatch quarantines without consuming or completing a round. Retain an independent copy. R0 → AuthorAdjudication/adjudicate. R2 → AuthorAdjudication/adjudicate while retaining that it is the closure result; every finding still requires author adjudication. |
| `Adjudicate(proof Proof, records []Adjudication, recordDigest string, correctionRequested bool, conflictingBlockers bool)` | AuthorAdjudication | Validate exact per-finding coverage and nonblank rationales, valid record digest. Retain the author record digest in the corresponding R0 or R2 slot. Conflicting blocking findings route G2 after complete author adjudication; a conflict flag requires at least two Blocking findings. After R0, explicit correctionRequested with at least one accepted finding opens the single R1 → CorrectionR1/correct-r1, spending R1 once; otherwise → ClosureCheckR2/open-r2. An accepted Blocking finding with no requested correction routes G2. After R2, any Blocking finding or any correctionRequested routes G2 regardless of accept/reject; otherwise → Countersigning/request-countersign. No automatic R3 or second correction exists. |
| `ChangeCandidate(candidate Candidate)` | Any active state in this slice, including Countersigning (v0.5 §7 invariant 2 detects external changes; countersigning itself still makes none), but not NeedsDecision | Same flight, author lane, epoch, session, runway required; manifest revision may stay or increase. Exact unchanged candidate is a no-op. Changed commit, tree, head digest or revision invalidates all candidate-bound proofs and clears current findings and the active pending packet, but retains every round count, reviewer identity, historical R0 packet digest, historical R0 adjudication digest/completion marker, and any historical R2 adjudication digest. It preserves the operational streak until a nonoperational align result resets it, and enters Aligning/run-align. Candidate syntax errors are refused; dirty candidates, identity continuity mismatch and manifest-revision regression quarantine without adopting the candidate or changing state. G2/epoch replacement is a separate deferred command. |
| `Escalate(witness string)` | Any initialized state | NeedsDecision/request-g2 with nonblank witness; preserves counts and candidate. |

The consecutive operational streak counts align results, not candidate
versions: candidate changes cannot reset a failure because no successful or
verdict align result has intervened. The Stage 1 F13 model-test requirement
explicitly uses two consecutive operational exits. A subsequent nonoperational
exit breaks the streak; no lifetime failure quota is introduced.

The next review-opening state/action after alignment is ReviewR0/open-r0
when R0Count=0; ClosureCheckR2/open-r2 when completed historical R0
adjudication is available and R2Count=0; otherwise NeedsDecision/request-g2. During CorrectionR1, only a changed candidate may
advance (a same-candidate no-op never skips realignment). Retain the R0 author
record for the R1 → R2 packet as historical input, but never retain it as proof
about the new candidate. Candidate invalidation before R0 adjudication or
after R2 opens therefore reaches G2 after re-alignment instead of repeating a
spent review. The historical R0 packet digest also survives invalidation so R2 freshness
comparison remains meaningful. None of these historical records proves the
new candidate. Candidate invalidation while R2 is pending or after it finishes
cannot earn another R2. This is a conservative dead end under v0.5 §6(3), not
an authorization to start a new cycle.

A new-cycle G2 command, new epoch, replacement session, retry and resume are
explicitly not exposed yet: they require durable human decision/replay and
fresh-context contracts. NeedsDecision cannot be escaped through this API.
Constructing an unrelated New value is not a runtime reset API; no runtime
caller exists. Runtime recovery must eventually preserve counts from durable
facts and may not reconstruct a consumed cycle as New.

## Review packet checks and trust limits

A packet's Proof must be current and proven. Returned results additionally bind the active opened packet digest and the
full reviewer execution identity, including its manifest revision and session.
A current-candidate proof from a different round is insufficient.
All five Reviewer fields must
match exactly for R0 and R2. Its execution uses the reviewer lane, current
flight/epoch/runway/commit/tree, a nonblank session, and a valid identity;
its manifest revision belongs to that review execution and need not equal
the author's revision. Fresh and BuilderConversationExcluded must both be
true. PacketDigest must be valid. R0 has no adjudication digest. R2 requires
the exact retained R0 adjudication digest and a PacketDigest distinct from
R0's; fresh packet compilation remains an upstream proof obligation, since
different digest bytes alone cannot prove isolation. Reusing the reviewer
session is not forbidden by this core if the fresh-context proof is supplied.
An identity/continuity mismatch quarantines without spending a round.
Missing isolation/freshness proof routes G2. Input shape errors remain errors.

The threat model is a correctly integrated, trusted orchestrator calling this
internal pure model with separately verified facts. Forged structs from an
untrusted runtime boundary are outside this slice: it has no such boundary.
Testing Proven=true exercises a conditional rule, not real authentication.
Provider summaries cannot satisfy any method: no narrative-to-proof adapter
exists, and unproven observations cannot progress. Receipt authentication,
sealed packet compilation, source sequence continuity, Git freshness and
current Verdi state must be verified before future runtime calls. Those
integration proofs remain disclosed as unproven here.

## Implementation flight and verification

The worker owns only new `internal/gatekeeper/state.go`, `candidate.go`,
`packet.go`, `transition.go`, `adjudication.go` and associated `_test.go` files;
it may update the package comment in `review.go` for the expanded purpose.
Other agents are present; preserve their changes. Main owns PLAN and this
record. No board, executor, provider, CLI, wire format, frozen authority or
installed binary changes. No push, PR, merge or hosted test.

Use the required Sonnet implementation route, Opus fixes/review, and the
owner's previously authorized Codex fallback only if Claude is unavailable.
Do not call a Codex role named Sonnet/Opus a Claude execution. Record actual
provider provenance. Only this plan, cited authority excerpts, existing
review.go/tests, shared identity/validation seams, module and lint settings
are task inputs; no unrelated provider conversations.

- [ ] Write table-driven tests and retain a real failing run before production
  implementation. Exercise each allowed edge, forbidden ordering, both clean
  paths (no correction and R1), complete R0/R2 adjudication, every missing
  blocking witness, malformed adjudications, contradictory blockers, post-R2
  blockers, incomplete disposition, both operational failures and reset,
  quarantine on every identity mismatch, unknown exits, unproven proofs,
  reviewer continuity, fresh packets, exact adjudication binding, and aliases.
- [ ] Add minimal pure implementation and run focused green/race checks.
- [ ] Exercise candidate invalidation at every review boundary, with a same-tree
  new commit, a new tree, head/revision change, regression, wrong flight/epoch,
  and dirty candidate; verify round counts never decrease and no R0/R2 repeats.
- [ ] Independently review the consolidated code and adjudicate every finding.
- [ ] Main runs the commands below, captures exit codes/output, confirms no
  focused skips, commits locally, and records remaining F13/F14 boundaries.

```sh
go test -count=1 -v ./internal/gatekeeper
go test -race -count=1 ./internal/gatekeeper
make verify
go test -race -count=1 ./...
git diff --check
```

Main verifies that the source hashes are stable across the full gates; no
other worker may mutate the tested checkout during those runs.

Existing optional real-pair environment tests remain disclosed when not
configured. They do not waive any new core test. No full F13, recorder-backed
review, real forge/CI or end-to-end local workflow success is claimed.

## Coverage witness

This is an internal implementation record, not canonical promotion. The F13
requirements are accounted for as follows (11/11 groups, no silent omissions):

| Requirement | Treatment |
|---|---|
| Closed state vocabulary, no Failed | Full catalog; active subset explicit |
| Initial candidate and clean identity binding | New, Candidate and Proof checks |
| Align and two operational failures scoped to one flight | Align and per-machine streak |
| Human dispositions and fresh report | Disposition's caller proof boundary |
| R0 once, R1 at most once, R2 once | Opening counters and closed transitions |
| Four-field blockers and every finding adjudicated | Existing validators at both rounds |
| Conflicts and post-R2 dead ends | Explicit G2; no automatic new cycle |
| Candidate invalidation and Align re-entry | ChangeCandidate and retained counters |
| Same reviewer and fresh sealed context | ReviewPacket checks plus disclosed upstream proof |
| Candidate/epoch/receipt/recorder authority; no provider summaries | Typed conditional input and quarantine; runtime verification deferred |
| Board projection, G2 recovery and all-stories-Done/AwaitingUAT | Explicitly deferred; F12 compatibility prevents premature event wiring |
