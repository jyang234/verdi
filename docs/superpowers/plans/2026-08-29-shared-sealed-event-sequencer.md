# Shared Sealed Event Sequencer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` task-by-task. This correction additionally requires genuine first-party `claude-opus-5` through `/fable-orchestration`, with no subagents.

**Goal:** Give sealed execution and embedded Claude scoped MCP one authenticated append state so real built start/resume cannot allocate duplicate VATC event keys or consume stale manifest state.

**Architecture:** `sealedexec.Service` constructs one `*FlightState` after prerequisite and resume-continuity verification. It passes that pointer through `AdapterCheck`; lifecycle and MCP context transitions serialize on the same mutex and advance only after a valid recorder acknowledgment. Standalone `verdi context mcp` keeps its completed-null refusal.

**Tech Stack:** Go 1.24, `internal/sealedexec`, `cmd/verdi`, canonical JSON, hermetic FD-3 controller fakes, `go test`, race detector.

**Spec:** `docs/superpowers/specs/2026-08-29-shared-sealed-event-sequencer-design.md`

## Global Constraints

- Binding authority: Amendment 002 §§4, 7, and 9; I-86, I-110, I-115; IL-079; SI-171.
- Every Claude invocation begins with `/fable-orchestration`, selects first-party `claude-opus-5`, and uses no delegation or subagent.
- Preserve the exact seven frozen U6 producer names and cardinality.
- Preserve standalone `verdi context mcp` completed-null refusal.
- Preserve public request/controller bytes and Codex start/resume bytes. Stop for Codex adjudication rather than widening a wire.
- Only a valid recorder acknowledgment advances source, prior-event, global, revision, or expansion state.
- The current uncommitted `cmd/verdi/claude_execution_e2e_test.go` diff is the genuine RED. Refine it; do not discard it.
- Opus runs focused checks only. Codex owns full `go test ./...`, full race, and `make verify`.

---

### Task 1: Freeze shared-state false greens

**Files:**
- Modify: `internal/sealedexec/mcp_test.go`
- Modify: `internal/sealedexec/service_test.go`
- Modify: `cmd/verdi/context_execution_contract_test.go`
- Modify: `cmd/verdi/claude_execution_e2e_test.go`

**Interfaces:**
- Consumes: current `FlightState`, `ScopedMCP`, `Service`, `AdapterCheck`, and the red `claudeLifecycleOptions.resume` fixture.
- Produces: failing oracles for one append owner, service-to-adapter state identity, and real built Claude resume.

- [ ] **Step 1: Preserve the built-resume RED**

Run:

```bash
go test ./cmd/verdi -run '^TestClaudeBuiltBinaryLifecycle_Behavioral/sealed_resume_drives_the_public_claude_assembly$' -count=1 -v
```

Expected: FAIL before provider launch while the decoded request is `ActionResume`. Record the exact refusal.

- [ ] **Step 2: Add duplicate-allocation oracle**

Add `TestSharedFlightStateSerializesServiceAndMCPAppends`. Its recorder fake is keyed by `(manifest_revision,source_sequence)`: byte-identical replay returns the original ack; different bytes at an existing key fail operationally without allocating global order. Drive service-shaped `adapter-start`, a proven `request_context` transition, and a service-shaped provider event through one state.

Assert exact kinds:

```go
[]contextevent.Kind{
    contextevent.KindAdapterStart,
    contextevent.KindContextRequest,
    contextevent.KindContextDecision,
    contextevent.KindChildManifest,
    contextevent.KindProviderMessage,
}
```

Assert unique event keys, strictly increasing global acks, and final provider event at child revision/source sequence 1 with the exact bridge.

- [ ] **Step 3: Add service ownership oracle**

Extend the service adapter fake to retain `AdapterCheck.State`. Start and resume must each receive one nonnil exact pointer. The first service acknowledgment must be visible through that pointer. Mutation: substitute a second `NewFlightState`; the duplicate-key recorder must fail.

- [ ] **Step 4: Add command ownership oracle**

Assert `commandClaudeAdapter.init` consumes `AdapterCheck.State` and makes no second `recorder-checkpoint` or `verify-expansion` controller call. Keep all standalone `buildMCPFlightSnapshot` mutations, including completed-null refusal.

- [ ] **Step 5: Run RED set**

```bash
go test ./internal/sealedexec ./cmd/verdi -run 'TestSharedFlightStateSerializesServiceAndMCPAppends|TestContextExecutionContract_Static|TestClaudeBuiltBinaryLifecycle_Behavioral/sealed_resume_drives_the_public_claude_assembly' -count=1 -v
```

Expected: FAIL because `AdapterCheck` has no shared state and service/MCP allocate independently.

---

### Task 2: Make `FlightState` the append transaction

**Files:**
- Modify: `internal/sealedexec/mcp.go`
- Modify: `internal/sealedexec/mcp_test.go`

**Interfaces:**
- Consumes: `FlightStateSnapshot`, `StampSource`, `WorkspaceFacts`, and `buildEvent`.
- Produces: package-private `eventAppender`, `appendLocked`, and locking `append` methods on `*FlightState`.

- [ ] **Step 1: Define append result**

```go
type flightAppendResult struct {
    Event contextevent.Event
    Ack   contextevent.EventAck
}

type eventAppender interface {
    Append(context.Context, contextevent.Event) (contextevent.EventAck, error)
}
```

- [ ] **Step 2: Add locked primitive**

```go
func (s *FlightState) appendLocked(
    ctx context.Context, recorder eventAppender, stamps StampSource,
    workspace WorkspaceFacts, kind contextevent.Kind, payload any,
) (flightAppendResult, error)
```

It builds from the current snapshot, validates ack against `LastGlobalSequence`, advances next/prior/global facts, and clears a consumed bridge. Stamp/build/append/ack failure leaves the snapshot unchanged.

- [ ] **Step 3: Add locking wrapper**

Add the same signature as `appendLocked` under method name `append`; it acquires/releases `s.mu`. Nil context/state/dependencies retain existing operational classification.

- [ ] **Step 4: Refactor MCP appends**

Keep the existing whole `requestContext` state lock. Delegate request, decision, and child-manifest appends to `appendLocked`. After child ack and successful install, advance revision/manifest/expansion, reset source sequence to 1, install the exact predecessor bridge, and preserve last-global facts.

- [ ] **Step 5: Verify and commit**

```bash
go test ./internal/sealedexec -run 'TestScopedContextMCPContract_Static|TestSharedFlightStateSerializesServiceAndMCPAppends' -count=1 -v
go test -race ./internal/sealedexec -run 'TestScopedContextMCPContract_Static|TestSharedFlightStateSerializesServiceAndMCPAppends' -count=1
git add internal/sealedexec/mcp.go internal/sealedexec/mcp_test.go
git commit -m 'Share sealed event append state' -m 'Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>'
```

Expected: PASS, no race, one scoped commit.

---

### Task 3: Give `Service` sole state ownership

**Files:**
- Modify: `internal/sealedexec/service.go`
- Modify: `internal/sealedexec/service_test.go`
- Modify: `internal/sealedexec/mcpcompile.go`
- Modify: `internal/sealedexec/mcpcompile_test.go`

**Interfaces:**
- Consumes: Task 2 `*FlightState` and append primitive.
- Produces: `AdapterCheck.State *FlightState`, verified start/resume initialization, and lifecycle append sharing.

- [ ] **Step 1: Extend AdapterCheck**

```go
type AdapterCheck struct {
    Request ExecutionRequest
    Profile ResolvedProfile
    Workspace WorkspaceFacts
    Review *ReviewLaunch
    State *FlightState
}
```

All adapter fakes require the exact nonnil pointer; adapters never create a default.

- [ ] **Step 2: Initialize start state**

After pristine checkpoint proof, construct request/key/workspace/candidate, request revision/manifest/projection, empty expansion root, sequence 1, empty prior/bridge, and global zero.

- [ ] **Step 3: Initialize resume state**

Only after `validateResumeFacts`, `planRestart`, and provider-session proof, construct from verified request/workspace, exact expansion root, and `plan.sequence/priorDigest/priorGlobal`. Do not invoke standalone snapshot reconstruction and do not synthesize an `ActiveRevision` result.

- [ ] **Step 4: Make child compilation action-aware**

For `ActionStart`, current request revision still requires empty prior expansion root. For `ActionResume`, require the exact canonical nonempty root already cross-matched to `request.Resume.Continuity.ExpansionLedgerRoot`. Add start-empty, start-nonempty, resume-exact, resume-empty, and resume-mismatch rows.

- [ ] **Step 5: Route service events through state**

Store the pointer on `activeExecution`. Replace authoritative use of independent `sequence`, `priorDigest`, and `priorGlobal` for recorder gap, resume, provider observations, suspension, adapter-stop, and every service-built event. Event identity must use the shared current revision/manifest after child expansion. Any reporting cache is copied only after a valid ack.

- [ ] **Step 6: Prove failure immutability**

Mutate stamp, append, ack, and expansion-install failures. The snapshot stays exact except the already-ratified MCP invalidation bit where applicable.

- [ ] **Step 7: Verify and commit**

```bash
go test ./internal/sealedexec -run 'TestContextExecutionContract_Static|TestScopedContextMCPContract_Static|TestCanonicalChildCompiler|TestSharedFlightStateSerializesServiceAndMCPAppends' -count=1 -v
go test -race ./internal/sealedexec -run 'TestContextExecutionContract_Static|TestScopedContextMCPContract_Static|TestSharedFlightStateSerializesServiceAndMCPAppends' -count=1
git add internal/sealedexec/service.go internal/sealedexec/service_test.go internal/sealedexec/mcpcompile.go internal/sealedexec/mcpcompile_test.go
git commit -m 'Own sealed flight state in execution' -m 'Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>'
```

Expected: PASS and a scoped service commit.

---

### Task 4: Consume shared state in Claude assembly

**Files:**
- Modify: `cmd/verdi/context_execution.go`
- Modify: `cmd/verdi/context_execution_contract_test.go`
- Modify: `cmd/verdi/claude_execution_e2e_test.go`

**Interfaces:**
- Consumes: `AdapterCheck.State` from Task 3.
- Produces: embedded Claude MCP over the same state plus real built start/resume evidence.

- [ ] **Step 1: Remove independent reconstruction**

In `commandClaudeAdapter.init`, require `check.State`, pass it to `NewScopedMCP`, and remove controller checkpoint/expansion queries and `buildMCPFlightSnapshot`. Keep controller ports for resolver, append, install, stamps, and segments. Do not change standalone `context_mcp.go`.

- [ ] **Step 2: Prosecute duplicate keys in the built fake**

Retain canonical event bytes and original acks by key. Exact replay returns the original; contradictory bytes fail without global allocation.

- [ ] **Step 3: Complete real resume**

Keep the Opus-authored resume option, continuity, `ActionResume`, session ref, `--resume` assertion, and `resume`/`adapter-start` prefix. Match checkpoint/expansion facts to service validation without fabricating an MCP active tail. Keep `outFile` orthogonal.

- [ ] **Step 4: Exercise context transition in start and resume**

Have fake Claude call `request_context` in both rows. Assert unique keys, child-manifest transition, next provider event on child revision, exactly one result/receipt, zero quarantine, process reap, terminal observation, and scoped cleanup. Mutation: replace shared state with a fresh pointer and require duplicate refusal before result/receipt.

- [ ] **Step 5: Verify real rows and standalone refusal**

```bash
go test ./cmd/verdi -run '^TestClaudeBuiltBinaryLifecycle_Behavioral/(sealed_start_drives_the_public_claude_assembly|sealed_resume_drives_the_public_claude_assembly)$' -count=1 -v
go test -race ./cmd/verdi -run '^TestClaudeBuiltBinaryLifecycle_Behavioral/(sealed_start_drives_the_public_claude_assembly|sealed_resume_drives_the_public_claude_assembly)$' -count=1
go test ./cmd/verdi -run '^TestContextExecutionPublicContract_Behavioral$' -count=1
```

Expected: start and resume PASS; standalone completed-null mutation remains refused.

- [ ] **Step 6: Run exactly seven frozen producers**

```bash
go test ./internal/sealedexec/claude ./internal/contextevent -run '^(TestClaudeAdapterParityContract_(Static|Behavioral)|TestContextEventEnvelopeContract_(Static|Behavioral)|TestContextEventRegistryContract_(Static|Behavioral)|TestContextEventRedactionContinuityContract_Behavioral)$' -count=1 -v
```

Expected: exactly seven top-level PASS lines.

- [ ] **Step 7: Run focused package/static checks**

```bash
gofmt -w cmd/verdi/context_execution.go cmd/verdi/context_execution_contract_test.go cmd/verdi/claude_execution_e2e_test.go internal/sealedexec/mcp.go internal/sealedexec/mcp_test.go internal/sealedexec/service.go internal/sealedexec/service_test.go internal/sealedexec/mcpcompile.go internal/sealedexec/mcpcompile_test.go
go vet ./internal/sealedexec ./internal/sealedexec/claude ./cmd/verdi
go test ./internal/sealedexec ./internal/sealedexec/claude ./cmd/verdi -count=1
go test -race ./internal/sealedexec ./internal/sealedexec/claude ./cmd/verdi -count=1
git diff --check
```

Expected: all exit 0. On sandbox listener denial, preserve the terminal witness and report it; do not alter product code or run full gates.

- [ ] **Step 8: Commit assembly**

```bash
git add cmd/verdi/context_execution.go cmd/verdi/context_execution_contract_test.go cmd/verdi/claude_execution_e2e_test.go
git commit -m 'Complete shared Claude event sequencing' -m 'Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>'
```

---

### Task 5: Evidence handoff

**Files:**
- Modify ignored: `.superpowers/sdd/2026-08-29-verdi-atc-u6-sealed-claude-events/task-6-report.md`
- Modify ignored: `.superpowers/sdd/2026-08-29-verdi-atc-u6-sealed-claude-events/progress.md`

**Interfaces:**
- Consumes: exact Task 1–4 RED/GREEN/commit evidence.
- Produces: truthful local provenance for Codex.

- [ ] **Step 1: Record exact evidence**

Append REDs, mutations, commands, exit codes, commit SHAs, changed paths, residual disclosures, canonical model usage, explicit `/fable-orchestration`, and no-subagent/no-network facts.

- [ ] **Step 2: Preserve gate boundary**

State that Opus did not run or claim full normal, full race, or `make verify`; Codex owns them. Include exact line:

```text
telemetry unavailable — local recorder only; bootstrap/advisory
```

- [ ] **Step 3: Hand back without closure**

Return exact HEAD/tree/parent/status, commits, focused results, and blockers. Do not claim U6 closure, push, merge, PR, promotion, or release.
