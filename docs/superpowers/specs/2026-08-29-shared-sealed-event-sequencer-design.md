# Shared sealed event sequencer design

**Status:** Owner-approved design direction on 2026-08-29; implementation is
pending review of these written bytes.

## Problem

The sealed execution service and Claude's embedded scoped MCP server can both
append VATC events for one `(flight,lane,epoch,manifest_revision)` identity.
They currently own separate mutable counters. The service owns lifecycle
events such as `adapter-start`, `resume`, provider observations, and
`execution-result`; scoped MCP owns `context-request`, `context-decision`, and
`child-manifest`.

This split has two concrete failures:

1. A start can let both components allocate the same source sequence. The
   fixture recorder does not currently prosecute that duplicate, so existing
   success evidence can remain green.
2. A resume checkpoint that the execution service accepts cannot initialize
   embedded MCP. Standalone MCP correctly refuses a completed checkpoint with
   no active tail, while service restart must activate that authenticated
   checkpoint and continue its source sequence. Trying to fabricate an active
   tail contradicts the recorder codec's revision-successor and expansion-root
   rules.

Amendment 002 §7 requires one never-resetting acknowledged event order, exact
replay, and no invented state. Amendment 002 §9 requires a real built Claude
resume. Both requirements need one append owner.

## Decision

The sealed execution service owns one mutable `FlightState` for the entire
active execution. It constructs the state only after it has strictly decoded
the request and proven authority, runway, workspace, profile, conflict,
recorder checkpoint, expansion ledger, and—on resume—continuity and provider
session facts.

The service passes the same state pointer through `AdapterCheck`. The embedded
Claude adapter gives that pointer to `NewScopedMCP`; it does not query the
controller for a second checkpoint and does not construct a second state.
Standalone `verdi context mcp` remains independent and retains I-86's refusal
of a completed checkpoint without an active tail.

This is the only semantic change. Controller wires, public execution request
bytes, standalone MCP grammar, the two-tool registry, and Codex provider bytes
remain unchanged.

## State and ownership

The shared state carries the current:

- execution request and key;
- workspace and candidate identity;
- manifest revision and digest;
- projection and expansion roots;
- next source sequence;
- prior event digest and optional revision bridge;
- last acknowledged global sequence; and
- the complete canonical acknowledgment stream, including authenticated
  restart history and both service- and MCP-owned appends; and
- invalidation state.

Its mutex is the sole in-process serialization boundary. A valid recorder
acknowledgment is the sole operation that advances source, prior-event, or
global state. An expansion installation advances manifest revision/digest and
expansion root only after the child-manifest acknowledgment and the existing
atomic install succeed. Failure leaves the last acknowledged state intact and
invalidates where the existing MCP contract requires it.

The service must stop storing an independently authoritative sequence tuple.
It may cache values for reporting, but every event build must take the current
state under the shared lock and every successful append must update that same
state before releasing the lock.

`ExecutionRun` carries an immutable terminal snapshot and copy of that complete
acknowledgment stream. Completion validates the stream against the snapshot and
builds `execution-result` from the snapshot's current manifest revision,
manifest digest, next source sequence, prior-event digest, and last global
sequence. The completed recorder checkpoint, receipt, and public result must
cross-match that actual terminal revision. Completion does not poll the
controller to reconstruct state and does not fall back to the original request
revision after an expansion.

## Initialization

### Start

After proving the recorder is pristine, the service initializes the public
request revision and manifest at source sequence 1, with no prior event,
global sequence zero, no expansion root, and no bridge.

### Resume

After `validateResumeFacts`, `planRestart`, and provider-session verification
succeed, the service initializes the shared state from the authenticated
restart plan:

- the request's current manifest revision/digest;
- `plan.sequence`, `plan.priorDigest`, and `plan.priorGlobal`;
- the continuity's verified expansion root;
- the verified workspace/candidate identity; and
- the exact invalidation and bridge state represented by the restart plan.

The service then appends any required recorder gap and the `resume` event
through that state before provider observations are consumed. This is not a
public reopening of a terminal checkpoint: it occurs only inside the already
validated execution call. Standalone MCP continues to reject the same
completed-null checkpoint.

## Event flow

Service lifecycle append:

1. Lock shared state.
2. Build the event from its current revision, manifest, sequence, prior digest,
   and workspace identity.
3. Append through the recorder.
4. Validate the returned acknowledgment against the event and current global
   sequence.
5. Advance shared state and unlock.

Scoped MCP already holds the state lock for a complete context request
transition. It continues to append request, decision, and child-manifest in
order, then installs the expansion and advances the same shared manifest
state. The next service-owned provider observation therefore uses the child
revision rather than the original request revision.

The provider process may call MCP while the service is blocked waiting for the
next provider frame. The shared mutex orders that MCP transition before the
subsequent provider observation. Interruption uses the same append boundary,
so it cannot race a context transition into duplicate event identities.

## Errors and cleanup

- Any state/request/checkpoint/continuity contradiction refuses before provider
  launch.
- Duplicate or contradictory recorder acknowledgment is operational and does
  not advance shared state.
- MCP invalidation remains terminal and prevents later result or receipt.
- Provider reap still precedes HTTP MCP close and scoped-config removal.
- A failed MCP response write still does not publish a terminal.
- No error path resets source order, creates a second state, or falls back to
  controller polling.
- If an error after expansion requires I-88 preservation, the unchanged partial
  schema represents the shared state's current manifest revision/digest and its
  complete acknowledgment stream. Validation uses the same per-revision source
  continuity and strictly increasing global order as successful completion;
  it never discards the MCP acknowledgments to fit the original request.
- The atomic child-install/first-child-append boundary is a closed partial-only
  exception: the stream may end on the parent while the terminal snapshot opens
  exactly the next revision at source one, with empty prior-event digest,
  unchanged last-global order, and the exact non-null bridge to that parent ack.
  A failure there preserves the installed child manifest plus the parent prefix;
  successful completion still requires its terminal event on the child.

## Test contract

The correction must first preserve the genuine red built-resume witness. It
then proves:

1. the real built Claude resume request reaches `commandClaudeAdapter.Resume`;
2. provider argv ends in exact `--resume <verified-session>`;
3. acknowledged events begin `resume`, `adapter-start` at the continued source
   sequence;
4. start with a successful `request_context` call allocates no duplicate event
   key and the next provider event uses the installed child revision;
5. resume with a successful context request has the same property;
6. a fake recorder that rejects duplicate identities makes any independent
   counter implementation fail;
7. standalone `context mcp` still refuses a completed-null checkpoint;
8. Codex start/resume bytes and existing service/package behavior remain
   unchanged; and
9. completion after either embedded expansion consumes the complete shared
   acknowledgment stream and emits result/receipt at the child revision; and
10. the exact seven frozen U6 producers remain the only producer set.

Focused TDD and race checks belong to genuine Opus. Full `go test ./...`, full
race, and `make verify` remain Codex-owned final gates.

## Rejected alternatives

Refreshing checkpoints independently before each append adds controller I/O
without an atomic read-build-append transaction and retains races. Depending
on duplicate refusal leaves valid built execution unable to use the required
tool. Disabling `request_context` during execution contradicts the exact
two-tool surface. Weakening completed-null or continuity validation would make
resume reachable by discarding authority rather than by sharing it.
