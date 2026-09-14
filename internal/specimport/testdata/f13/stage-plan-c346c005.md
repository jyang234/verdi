# Verdi-ATC Stage 1 Orchestration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development under `/fable-orchestration` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement isolated runways, deterministic scheduling, scoped claims, sealed lane execution, bounded review, forge landing, and resumable Verdi closure.

**Architecture:** Pure scheduler and gatekeeper reducers decide allowed transitions from committed facts. Effectful runway, Verdi, forge, and sealed-lane adapters execute one command at a time and append results before the next transition. Every attempt uses a unique epoch and candidate binding; every dead end routes to G2.

**Tech Stack:** Go 1.25.0, Git CLI behind a hermetic port, Stage 0.5 Verdi sealed-execution/event/receipt surfaces, official Claude Code and Codex adapters through Verdi, JSON-RPC MCP, forge HTTP adapters.

**Spec:** `docs/design/specs/00-verdi-atc-v0.5-ste.md` §§6–10, Amendment 001,
and accepted Amendment 003

## Global Constraints

- F01-F07 and all Stage 0.5 surfaces must pass before integration acceptance.
- The daemon never writes tracked `.verdi/` artifact content.
- Only `spec.md` under `.verdi/specs/**` becomes non-writable; Verdi sidecars remain writable.
- R0 once, optional R1 once, Align after every candidate change, R2 once; no third automatic round.
- No terminal `Failed`; all dead ends append a G2 decision request.
- Every implementation handoff begins `/fable-orchestration`.
- F11A and F11B use genuine Claude Opus 5 as the main implementation agent,
  per the owner's explicit routing instruction; Codex retains authority
  adjudication, commit acceptance, and final-gate ownership.
- F12 uses genuine Claude Opus 5 as the main implementation agent, independent
  reviewer, and accepted-fix agent through `/fable-orchestration`; Codex
  authors authority, adjudicates findings, owns Git, and runs final gates.
- Every task's exact pre-commit candidate runs `make verify` and
  `go test -race ./...`; focused package checks never replace either full gate.

---

### Task F08: Manage runway epochs and BuildStart reconciliation

**Files:**
- Create: `internal/gitproto/runner.go`
- Create: `internal/gitproto/client.go`
- Create: `internal/gitproto/client_test.go`
- Create: `internal/gitproto/fixturegit_test.go`
- Create: `internal/runway/manager.go`
- Create: `internal/runway/buildstart.go`
- Create: `internal/runway/protect.go`
- Create: `internal/runway/quarantine.go`
- Create: `internal/runway/manager_test.go`
- Create: `internal/runway/buildstart_test.go`
- Create: `internal/runway/protect_test.go`
- Create: `internal/runway/quarantine_test.go`

**Interfaces:**
- Consumes: F03 Verdi client, F05 ledger, fixture Git repositories.
- Produces: epoch worktree manager, candidate identity, write-ahead BuildStart, deterministic reconciliation, lease/quarantine operations.

- [ ] **Step 1: Write the Git port and injection tests**

```go
type Flight struct { ID, StoryRef, FeatureName, ExpectedBase string }
type Candidate struct { CommitSHA, TreeSHA, Branch string; Clean bool }

type Git interface {
	AddDetachedWorktree(ctx context.Context, repo, path, commit string) error
	CheckoutBranch(ctx context.Context, path, branch string) error
	BranchHead(ctx context.Context, repo, branch string) (string, bool, error)
	MergeBase(ctx context.Context, repo, a, b string) (string, error)
	Candidate(ctx context.Context, path string) (Candidate, error)
	MoveWorktree(ctx context.Context, repo, oldPath, newPath string) error
}
```

Reject shell metacharacters as data, preserve spaces in paths, classify Git
exit failures, and prove stable fixture SHAs across two builds.

- [ ] **Step 2: Write BuildStart ordering tests and observe RED**

```bash
go test ./internal/gitproto ./internal/runway -run 'Test(Git|BuildStart|Epoch|Protect|Quarantine)'
```

The fake ledger must durably acknowledge `build_start.intent` before the
instrumented BuildStart mutex is acquired and before any Git or Verdi call.
The fake Git client must observe the detached worktree before
`verdi build start`.

- [ ] **Step 3: Implement the exact procedure**

```go
func (m *Manager) BuildStart(ctx context.Context, flight Flight, epoch uint64) (Candidate, error)
```

Append and durably acknowledge `build_start.intent`; only then take the
process-wide BuildStart mutex. Create the path returned by
`filepath.Join(".vatc", "worktrees", storySlug, fmt.Sprintf("a%d", epoch))`,
run build start inside it, and verify `"feature/" + featureName` is checked out
there. If the branch exists, adopt only when
its base equals the intent's expected base; otherwise append a G2 witness.

- [ ] **Step 4: Protect and quarantine**

Walk `.verdi/specs/**/spec.md` without following symlinks and remove write bits.
Do not change sidecars. A new epoch moves the old worktree to the path returned
by `filepath.Join(".vatc", "quarantine", storySlug, fmt.Sprintf("a%d", epoch))`
and creates a fresh worktree at the last bound candidate. Never reuse the
directory.

- [ ] **Step 5: Run gates and commit**

```bash
go test -race ./internal/gitproto ./internal/runway
make fixture
make verify
git add internal/gitproto internal/runway
git commit -m "Manage isolated runway epochs"
```

### Task F09: Schedule dependencies, claims, leases, and budget stops

**Files:**
- Create: `internal/scheduler/state.go`
- Create: `internal/scheduler/claims.go`
- Create: `internal/scheduler/ready.go`
- Create: `internal/scheduler/leases.go`
- Create: `internal/scheduler/scheduler_test.go`
- Create: `internal/budget/governor.go`
- Create: `internal/budget/governor_test.go`
- Modify: `internal/board/state.go`
- Modify: `internal/board/reduce.go`
- Modify: `internal/board/reduce_test.go`

**Interfaces:**
- Consumes: approved run-plan snapshot and committed events.
- Produces: pure readiness decisions, claim index, flight-lifetime leases, pool limit, and ground-stop decisions.

- [ ] **Step 1: Write scheduler model tests**

```go
type Decision struct {
	FlightID string
	Action   string
	Witness  []string
}

type Limits struct { PoolSize int; Grounded bool; ActiveLeases map[string]string }

func Decide(board board.State, limits Limits) ([]Decision, error)
```

Table cases cover dependency order, disjoint same-wave claims, intersecting
claims, empty-claim serialization, shared registry resources, pool exhaustion,
ground stop, terminal lease release, invalidating replan release, and retained
lease through Blocked/NeedsDecision/retry. A committed undeclared-write event
that intersects another active flight's claim invalidates both candidates and
emits two G2 witnesses even if prevention failed.

- [ ] **Step 2: Run tests and observe RED**

```bash
go test ./internal/scheduler ./internal/budget ./internal/board -run 'Test(Scheduler|Claims|Lease|Budget)'
```

- [ ] **Step 3: Implement normalized claim intersection**

```go
type Claim struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}
```

File claims use repository-relative slash paths. Directory claims end in `/`
and match descendants on segment boundaries. Resources use closed names,
including `registry/cli-verbs` and `registry/mcp-tools`. Unknown kinds fail.
The scheduler consumes recorder write facts as a mandatory backstop and applies
the two-flight invalidation rule before either candidate may advance.

- [ ] **Step 4: Implement budget governor**

Meter lane and Verdi-judge usage from committed events. A configured limit may
lower pool capacity to zero and append a ground-stop fact; it cannot change a
Verdi verdict or release active leases. Missing usage is disclosed unproven.

- [ ] **Step 5: Run gates and commit**

```bash
go test -race ./internal/scheduler ./internal/budget ./internal/board
make verify
git add internal/scheduler internal/budget internal/board
git commit -m "Schedule claims and flight leases"
```

### Task F10: Serve VATC claims and register scoped context

**Files:**
- Create: `internal/mcpserve/protocol.go`
- Create: `internal/mcpserve/server.go`
- Create: `internal/mcpserve/claim_paths.go`
- Create: `internal/mcpserve/server_test.go`
- Create: `internal/mcpserve/claim_paths_test.go`
- Create: `internal/mcpserve/transport_test.go`
- Create: `cmd/vatc/mcp.go`
- Create: `cmd/vatc/mcp_test.go`
- Modify: `cmd/vatc/dispatch.go`

**Interfaces:**
- Consumes: F09 scheduler claim service and F05 ledger.
- Produces: one VATC MCP tool `claim_paths(paths[])` and registration metadata for the separate Verdi scoped-context server.

- [ ] **Step 1: Write strict JSON-RPC tests**

Prove initialize, tools/list, tools/call, unknown method, unknown tool, unknown
argument, trailing JSON, oversized line, cancellation, and dropped connection.
`tools/list` must contain exactly `claim_paths`.

- [ ] **Step 2: Write claim decision tests and observe RED**

```bash
go test ./internal/mcpserve ./cmd/vatc -run 'Test(MCP|ClaimPaths)'
```

Grant appends a claim event before success. Queue keeps the call pending until
the conflict releases or context cancels. Deny returns a typed tool error and
requires the agent to stop/report. Every path is normalized and repository
bounded.

- [ ] **Step 3: Implement the server**

```go
type ClaimDecision struct { State string; Claims []scheduler.Claim; Witness string }

type ClaimService interface {
	Claim(ctx context.Context, flightID string, paths []string) (ClaimDecision, error)
}
```

The server receives flight identity from a sealed, unforgeable launch
configuration rather than accepting `flight_id` from tool arguments.

- [ ] **Step 4: Register two distinct servers**

Lane launch metadata registers VATC `claim_paths` and the Stage 0.5
Verdi-owned scoped context server. Never register unrestricted `verdi mcp` and
never expose `claim_paths` through Verdi.

- [ ] **Step 5: Run gates and commit**

```bash
go test -race ./internal/mcpserve ./cmd/vatc
make verify
git add internal/mcpserve cmd/vatc
git commit -m "Serve flight-scoped path claims"
```

### Task F11A: Extend Verdi for dual required scoped MCP

This sibling-Verdi prerequisite is one separately reviewed commit and pin
update. It does not grant the ATC implementation agent access to unlisted
Verdi files.

**Files in sibling `../verdi`:**
- Create: `internal/sealedexec/scoped_mcp.go`
- Create: `internal/sealedexec/scoped_mcp_test.go`
- Modify: `internal/sealedexec/controller_schema.go`
- Modify: `internal/sealedexec/controller_codec.go`
- Modify: `internal/sealedexec/controller_client.go`
- Modify: `internal/sealedexec/controller_contract_test.go`
- Modify: `internal/sealedexec/service.go`
- Modify: `internal/sealedexec/service_test.go`
- Modify: `internal/sealedexec/claude/mcp.go`
- Modify: `internal/sealedexec/claude/mcp_test.go`
- Modify: `internal/sealedexec/claude/adapter.go`
- Modify: `internal/sealedexec/claude/adapter_test.go`
- Modify: `internal/sealedexec/codex/adapter.go`
- Modify: `internal/sealedexec/codex/adapter_test.go`
- Modify: `cmd/verdi/context_execution.go`
- Modify: `cmd/verdi/context_execution_contract_test.go`
- Modify: `cmd/verdi/claude_execution_e2e_test.go`

**Interfaces:**
- Consumes: accepted Amendment 003, canonical
  `sealedexec.ExecutionRequest`, the typed FD3 client, and the existing
  two-tool scoped-context HTTP handler.
- Produces: operation 23 and one provider-neutral dual-MCP launch projection.

```go
type ClaimMCPQuery struct {
	RequestDigest string
}

type ClaimMCPRegistration struct {
	Name          string
	Type          string
	URL           string
	Tools         []string
	RequestDigest string
}

func (c *ControllerClient) ResolveClaimMCP(
	ctx context.Context,
	query ClaimMCPQuery,
) (ClaimMCPRegistration, error)

type RequiredMCP struct {
	Name          string
	URL           string
	Authorization string
	Tools         []string
}

type RequiredMCPSet struct {
	Claim   RequiredMCP
	Context RequiredMCP
}
```

- [ ] **Step 1: Add RED subtests to the existing producer owners**

Extend subtests inside `TestContextControllerWireContract_Static`,
`TestClaudeAdapterParityContract_Static`,
`TestClaudeAdapterParityContract_Behavioral`,
`TestAdapterStartUsesPinnedIsolationAndTypedInput`,
`TestAdapterResumeTargetsExplicitVerifiedSession`,
`TestContextExecutionPublicContract_Behavioral`, and
`TestClaudeBuiltBinaryLifecycle_Behavioral`. Do not add an eighth U6 top-level
producer. The subtests must fail on the absent operation 23, one-row Claude
file/init projection, missing Codex `-c` operands, missing cross-token refusal,
and missing protected-value membership.

- [ ] **Step 2: Run the exact RED command**

```bash
go test ./internal/sealedexec ./internal/sealedexec/claude ./internal/sealedexec/codex ./cmd/verdi \
  -run '^(TestContextControllerWireContract_Static|TestClaudeAdapterParityContract_Static|TestClaudeAdapterParityContract_Behavioral|TestAdapterStartUsesPinnedIsolationAndTypedInput|TestAdapterResumeTargetsExplicitVerifiedSession|TestContextExecutionPublicContract_Behavioral|TestClaudeBuiltBinaryLifecycle_Behavioral)$' \
  -count=1
```

Expected RED: the controller registry has 22 operations and no
`ResolveClaimMCP`; Claude emits only `verdi-context`; Codex emits no dynamic
MCP operands.

- [ ] **Step 3: Add the strict controller arm**

Add `resolve-claim-mcp` as operation 23 with Amendment 003 §4's exact
operation-derived request and result schemas. Strict-decode, canonical
re-encode, and cross-match `request_digest`; require name `vatc`, type `http`,
exact `["claim_paths"]`, IPv4 loopback, positive port, and `/mcp` with no other
URL component. The result never carries an authorization value.

- [ ] **Step 4: Factor the common scoped-context HTTP lifecycle**

Move listener validation, context-capability derivation, HTTP serving,
terminal delivery, provider-reap ordering, and idempotent targeted shutdown
from the Claude-only helper into `sealedexec/scoped_mcp.go`. Keep the accepted
context capability preimage and handler bytes unchanged. Return a
`RequiredMCP` for `verdi-context`; derive the separate VATC capability from the
same canonical LF-terminated request and the validated controller
registration. Add both capability strings and both full `Bearer ` values to
the protected set before the first provider observation.

- [ ] **Step 5: Emit the exact Claude and Codex configurations**

Claude atomically writes Amendment 003 §5's exact mode-0600 two-row JSON plus
one LF, accepts either provider init row order, sorts by name before the
provider-summary digest, and refuses missing/extra/duplicate/disconnected/
renamed rows. Codex inserts Amendment 003 §6's twelve ordered `-c` pairs at the
fixed start and resume positions, keeps review `--model` ordering intact, and
admits `claim_paths` only as foreign telemetry. Provider-reported tool success
is never claim authority.

- [ ] **Step 6: Prove lifecycle and mutation coverage GREEN**

Use hermetic HTTP listeners and fake providers to prove both required
initialize handshakes before useful work, each token refused by the other
server and by a changed request, no FD3/listener inheritance, prelaunch
failure teardown, provider reap before context teardown, and config removal.
Mutate every Claude row and every Codex `-c` operand individually.

Run:

```bash
go test ./internal/sealedexec ./internal/sealedexec/claude ./internal/sealedexec/codex ./cmd/verdi -count=1
go test -race ./internal/sealedexec ./internal/sealedexec/claude ./internal/sealedexec/codex ./cmd/verdi -count=1
make verify
go test -race ./...
```

- [ ] **Step 7: Commit and hand back the new Verdi pin**

```bash
git add internal/sealedexec cmd/verdi/context_execution.go \
  cmd/verdi/context_execution_contract_test.go cmd/verdi/claude_execution_e2e_test.go
git commit -m "Require dual scoped MCP servers"
```

Hand back the exact commit, tree, built-binary SHA-256, changed paths, existing
producer outputs, `make verify`, and separate race output. F11B must pin that
accepted binary before its first GREEN claim.

### Task F11B: Bind sealed envelopes and lane adapters

**Files:**
- Create: `internal/envelope/schema.go`
- Create: `internal/envelope/codec.go`
- Create: `internal/envelope/render.go`
- Create: `internal/envelope/envelope_test.go`
- Create: `internal/lane/port.go`
- Create: `internal/claudeproto/adapter.go`
- Create: `internal/claudeproto/adapter_test.go`
- Create: `internal/codexproto/adapter.go`
- Create: `internal/codexproto/adapter_test.go`
- Create: `internal/laneconfig/launch.go`
- Create: `internal/laneconfig/launch_test.go`
- Create: `internal/laneconfig/policy.go`
- Create: `internal/laneconfig/policy_test.go`
- Create: `internal/mcpserve/http.go`
- Create: `internal/mcpserve/http_test.go`
- Create: `internal/mcpserve/recording.go`
- Create: `internal/mcpserve/recording_test.go`
- Create: `internal/verdiproto/controller.go`
- Create: `internal/verdiproto/controller_test.go`
- Modify: `internal/verdiproto/client.go`
- Modify: `internal/verdiproto/client_test.go`
- Modify: `internal/verdiproto/integration_test.go`
- Modify: `internal/verdiproto/runner.go`
- Modify: `internal/verdiproto/runner_test.go`

**Interfaces:**
- Consumes: F02's accepted F11A Verdi pin; F05 ledger; F06 recorder and
  segment store; F07 approved plan; F08 candidate/runway identity; F09
  scheduler/leases; F10 one-flight claim server and registration inventory;
  F11A's 23-operation wire.
- Produces: authenticated VATC claim HTTP, one complete FD3 controller session,
  canonical dispatch/result envelopes, and one harness-neutral lane port.

- [ ] **Step 1: Write HTTP and claim-recorder RED tests**

Require exact `POST /mcp`, no query, one exact Authorization and
`application/json` header, body smaller than `MaxFrameBytes`, HTTP 200 for one
framed request/error, and HTTP 202 empty for a notification. Freeze Amendment
003 §4's retry digest over sorted paths and `RQ`. Prove one request, optional
wait, exactly one grant-or-deny, and later release under the derived identity
whose only changed field is `lane:"vatc-claims"`; prove identical retry emits
no second event and a conflicting replay is operational.

- [ ] **Step 2: Run the transport RED**

```bash
go test ./internal/mcpserve -run 'Test(HTTP|Recording|ClaimRetry|ClaimIdentity)' -count=1
```

Expected RED: no HTTP wrapper or recorder-backed claim service exists.

- [ ] **Step 3: Implement the ATC claim boundary**

Build the loopback wrapper around the F10 server without GET, SSE, batching,
cookies, redirects, OAuth, or a second catalogue. `recording.go` cross-matches
the bound flight, appends the five existing recorder kinds in Amendment 003
§7 order, and keeps its idempotency state in the durable claim-decision
boundary rather than the HTTP request. `vatc-claims` is only a recorder
producer discriminator and is never accepted by run-plan lane validation.

- [ ] **Step 4: Write the 23-operation controller and FD3 RED tests**

Drive a built fake child through a socketpair on inherited FD3. Require strict
adjacent call sequence, operation-derived schemas, typed success/error unions,
the 22 accepted operations delegated to their existing ATC ports, and
operation 23 returning only the request-matched claim registration. Prove the
child and its subprocess inherit neither the parent endpoint nor a listener.

```bash
go test ./internal/verdiproto -run 'Test(Controller|StartSealed|ResumeSealed|Descriptor)' -count=1
```

Expected RED: `StartSealed` and `ResumeSealed` still return
`ErrControllerUnavailable`.

- [ ] **Step 5: Implement the controller-backed sealed client**

`runner.go` creates one socketpair, passes only the child endpoint as FD3,
serves the typed controller concurrently, closes the parent's duplicate child
endpoint immediately after start, and joins command/controller completion.
`controller.go` maps every accepted operation to its F02/F05/F06/F07–F10 owner
and emits only the existing operational error union for unavailable or
contradictory facts. `client.go` canonicalizes the exact start/resume request,
streams only canonical Verdi events through `IngestEvent`, and returns only a
final result/session cross-matched to the dispatch.

- [ ] **Step 6: Write envelope, policy, and lane RED tests**

```go
type Session struct {
	ID       string
	Identity protocol.ExecutionIdentity
}

type Continuity struct {
	ManifestDigest string
	Profile        string
	Worktree       string
	CandidateSHA   string
	LastSequence   uint64
	ExpansionRoot  string
}

type Adapter interface {
	Start(context.Context, envelope.Dispatch, recorder.Sink) (Session, error)
	Resume(context.Context, envelope.Dispatch, Continuity, recorder.Sink) (Session, error)
	Stop(context.Context, Session) error
}

type HTTPRegistration struct {
	Name  string
	URL   string
	Tools []string
}

type ClaimRuntime struct {
	Identity      protocol.ExecutionIdentity
	RequestDigest string
	Registration mcpserve.HTTPRegistration
	Capability   string
}
```

Test stale epoch, stale manifest revision, wrong runway, wrong branch, wrong
head/tree, dirty candidate, missing session, duplicate result, unknown status,
secret value in payload, a changed reviewer identity between R0 and R2, and a
builder conversation supplied to a review lane. Also mutate either scoped
registration's name, tools, URL, request digest, and capability. Prove the
launch policy denies every edit/write to `.verdi/specs/**/spec.md` while
allowing Verdi-managed sidecars.

- [ ] **Step 7: Run the lane RED**

```bash
go test ./internal/envelope ./internal/laneconfig ./internal/claudeproto ./internal/codexproto \
  -run 'Test(Envelope|Launch|Policy|Claude|Codex|DualMCP)' -count=1
```

- [ ] **Step 8: Implement canonical envelopes and both lane ports**

The canonical dispatch embeds or references the byte-identical Verdi manifest
and instruction projection, session-bound identity, and both request-bound MCP
registrations. Its digest covers every layer. The rendered self-check names
flight, lane, epoch, manifest revision, session, runway, branch, commit, tree,
and the two disjoint catalogues. Before useful work, the working directory,
branch, clean head/tree, frozen spec revision, and both registrations must
match the signed dispatch.

`laneconfig.Policy` carries the closed denial pattern
`.verdi/specs/**/spec.md` into each sealed launch. The provider edit/write
capability must enforce it; the filesystem write-bit protection in F08 is a
separate layer and cannot substitute for this launch policy.

`claudeproto` and `codexproto` invoke only F11A's pinned
`StartSealed`/`ResumeSealed`; they never reconstruct provider argv. They bind
each normalized event to the dispatch, call the shared
`protocol.ExecutionEventInput` bridge with the next source sequence, and wait
for the recorder's exact committed acknowledgment before advancing.

- [ ] **Step 9: Prove the built cross-repository lifecycle**

Use the accepted F11A binary with hermetic controller, HTTP, provider, Git,
ledger, recorder, scheduler, and segment fakes. Cover Claude and Codex start
and resume; both initialize handshakes; cross-token and cross-request refusal;
queued cancellation/retry; durable grant before undeclared write;
provider-reported success without grant; controller/provider failures;
prelaunch and post-reap shutdown; and exact request/wait/decision/release order
without a Verdi-sequence collision.

- [ ] **Step 10: Run final F11 gates and commit**

```bash
go test ./internal/mcpserve ./internal/verdiproto ./internal/envelope ./internal/laneconfig ./internal/claudeproto ./internal/codexproto -count=1
go test -race ./internal/mcpserve ./internal/verdiproto ./internal/envelope ./internal/laneconfig ./internal/claudeproto ./internal/codexproto -count=1
make verify
go test -race ./...
git add internal/envelope internal/lane internal/laneconfig internal/claudeproto \
  internal/codexproto internal/mcpserve internal/verdiproto
git commit -m "Bind dual scoped lane execution"
```

Hand back the exact commit/tree, accepted Verdi pin and binary digest, changed
paths, RED/GREEN transcripts, claim-event sequence, descriptor inventory,
`make verify`, separate race, and every disclosed-unproven fact. Do not begin
F12 until Codex accepts both F11A and F11B.

### Task F12: Execute, retry, suspend, and resume flights

**Ratified design:**
`docs/superpowers/specs/2026-09-01-vatc-f12-one-shot-executor.md`

**Executable plan:**
`docs/superpowers/plans/2026-09-01-vatc-f12-one-shot-executor.md`

F12 now owns the complete one-shot public path from committed G1 plan through
durable candidate handoff. The earlier eight-file sketch omitted required
run-plan v2 authority, store observation, runway identity, shared claim
grammar, cross-process reservation, controller-owner assembly, public command,
and built-binary evidence. The executable plan replaces that sketch and splits
the expanded work into four bounded TDD slices: authority projections,
production assembly, execution, and public proof. No slice is independently an
F12 completion boundary.

### Task F13: Implement the bounded gatekeeper state machine

**Files:**
- Create: `internal/gatekeeper/state.go`
- Create: `internal/gatekeeper/event.go`
- Create: `internal/gatekeeper/transition.go`
- Create: `internal/gatekeeper/transition_test.go`
- Create: `internal/gatekeeper/review.go`
- Create: `internal/gatekeeper/review_test.go`
- Create: `internal/gatekeeper/invalidation_test.go`
- Modify: `internal/board/reduce.go`

**Interfaces:**
- Consumes: committed candidate, align, review, adjudication, receipt, and G2 facts.
- Produces: pure next-state decisions and required effects.

- [ ] **Step 1: Define the closed state catalog**

```go
const (
	Planned State = "Planned"
	Ready State = "Ready"
	BuildStart State = "BuildStart"
	Implementing State = "Implementing"
	Aligning State = "Aligning"
	AlignDisposition State = "AlignDisposition"
	ReviewR0 State = "ReviewR0"
	AuthorAdjudication State = "AuthorAdjudication"
	CorrectionR1 State = "CorrectionR1"
	ClosureCheckR2 State = "ClosureCheckR2"
	Countersigning State = "Countersigning"
	MergeReady State = "MergeReady"
	Landing State = "Landing"
	ClosePrepared State = "ClosePrepared"
	ClosurePublished State = "ClosurePublished"
	AwaitingUAT State = "AwaitingUAT"
	Done State = "Done"
	Blocked State = "Blocked"
	NeedsDecision State = "NeedsDecision"
)
```

- [ ] **Step 2: Write model tests and observe RED**

```bash
go test ./internal/gatekeeper ./internal/board -run 'Test(Transition|ReviewBound|Invalidation)'
```

Enumerate every allowed edge and representative forbidden edges. Prove R0
cannot repeat, R1 count cannot exceed one, R1 candidate re-enters Aligning,
R2 cannot produce another automatic correction, any tree change invalidates
bound results, all stories Done moves the feature to AwaitingUAT, two
consecutive operational exits from Align route only that flight to G2, provider
summaries never satisfy a gate, and there is no `Failed` state.

- [ ] **Step 3: Implement review finding validation**

A blocking finding requires nonempty binding-authority cite, reachable-state
witness, concrete incorrect result, and threat-model fit. The author lane
adjudicates each finding. Conflicting blocking findings route to G2.

- [ ] **Step 4: Run gates and commit**

```bash
go test -race ./internal/gatekeeper ./internal/board
make verify
git add internal/gatekeeper internal/board
git commit -m "Enforce bounded flight review"
```

### Task F14: Land, countersign, and close through CI

**Files:**
- Create: `internal/gatekeeper/effects.go`
- Create: `internal/gatekeeper/service.go`
- Create: `internal/gatekeeper/service_test.go`
- Create: `internal/forgeflow/countersign.go`
- Create: `internal/forgeflow/landing.go`
- Create: `internal/forgeflow/closure.go`
- Create: `internal/forgeflow/forgeflow_test.go`
- Create: `internal/recovery/reconcile.go`
- Create: `internal/recovery/reconcile_test.go`
- Modify: `cmd/vatc/dispatch.go`
- Create: `cmd/vatc/run_test.go`

**Interfaces:**
- Consumes: F03 Verdi, F04 forge, F13 gatekeeper, approved landing policy.
- Produces: story countersign, lint/gate, implementation landing, prepare/CI close/push/MR/merge, feature G3 closure, restart reconciliation.

- [ ] **Step 1: Write effect-order tests**

Prove R2 and reviewer forge approval precede lint/gate; gate green on the exact
candidate precedes implementation merge; merge precedes close prepare; bound
preparation precedes CI close; the CI-created commit precedes its CI-side push
and ATC closure MR; reviewed closure merge, required checks and fresh
default-branch closed state precede Done (Amendment 004). No candidate-tree write
occurs during countersign.

- [ ] **Step 2: Run tests and observe RED**

```bash
go test ./internal/gatekeeper ./internal/forgeflow ./internal/recovery ./cmd/vatc -run 'Test(Countersign|Landing|Closure|Reconcile|Run)'
```

- [ ] **Step 3: Implement story landing policies**

`auto-merge` executes after all prerequisites. `clearance` appends a G2 request
and waits for the configured human clearance. `landing: wave` fails run-plan
decode and cannot reach this package.

After countersign, the service calls F03 `Lint` on the exact candidate and only
then calls F03 `Gate`. Both typed verdicts must be green and candidate-bound
before the forge merge operation is enabled.

Alignment findings append G2 decision items. Each item requires either a human
fix or an accepted-deviation disposition with rationale. The service runs
`verdi disposition`, reruns Align against the resulting fresh candidate, and
advances only when every finding is dispositioned and the new Align result is
green.

- [ ] **Step 4: Implement closure and recovery**

Follow Amendment 004: run `verdi close --prepare` for readiness, bind the
approved close input, run real close in CI, observe the CI-side push of its
exact closure commit, and open/reuse the closure MR through the ATC forge port.
Poll the required checks and reviewed closure merge, then query fresh
default-branch `verdi spec state` before Done. CI failure, rejected closure,
unexpected advance or branch-base mismatch produces a G2 witness. Keep commit
and tracker-publication outcomes separate and reconcile before retrying.
Feature G3 still requires human forge approval; reconcile the disclosed feature
publication discrepancy before feature integration. Exact machine/CI contracts
remain prerequisites. Build and test this flow with local hermetic fixtures
under IL-157; no hosted operation is authorized by the plan.

- [ ] **Step 5: Add restart reconciliation**

Replay the plan snapshot and events, query Verdi/forge fresh, compare every
observed advance with the recorded expected chain, reacquire active leases,
and resume only when execution continuity proves green.

- [ ] **Step 6: Run gates and commit**

```bash
go test -race ./internal/gatekeeper ./internal/forgeflow ./internal/recovery ./cmd/vatc
make verify
git add internal/gatekeeper internal/forgeflow internal/recovery cmd/vatc
git commit -m "Land and close Verdi flights"
```

### Orchestration completion gate

```bash
make verify
go test -race ./...
```

Expected: a hermetic non-UI flight can run through Done; every forbidden review
loop, stale identity, missing countersign, recorder gap, and unexpected
repository advance fails closed with a witness.
