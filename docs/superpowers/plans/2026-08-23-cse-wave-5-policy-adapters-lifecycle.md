# CSE Wave 5 Policy, Adapters, and Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` or
> `superpowers:executing-plans` task by task. Implementation-heavy tasks use
> Sonnet implementers; accepted defects use Opus fixers. The main agent owns
> authority interpretation, Markdown, ledger changes, review adjudication,
> final gates, and handoff. This plan contains no frontend work and therefore
> no FABLE task; Wave 6 remains FABLE-owned.

**Goal:** Deliver the complete non-UI CSE Wave 5 lifecycle through one typed
application core: concrete effective policy, registration, CLI and agent
adapters, authenticated ratification, reproduction, capsule retention,
workspace release, and existing spike closure.

**Architecture:** Extend the landed schema/decision/evaluator/execution stack
without duplicating it. `internal/experimentpolicy` owns one typed Context
Integrity payload and resolver. `internal/experimentapp` owns lifecycle
composition and immutable experiment mutations. `cmd/verdi` and
`internal/mcpserve` are thin adapters. Existing Git, governance-principal,
experimentrun, execworkspace, and close seams remain authoritative.

**Planning base:** `8cbb97aa738e34e4703f6d8d57892357b8cf2bd8`

**Authority:** `spec/comparative-spike-experiments-v3` AC-1–AC-7,
DC-1–DC-28, CO-1–CO-7; `spec/context-integrity-v2`;
`spec/execution-workspace`; CSE Wave 5 application/lifecycle design;
four-feature orchestration Wave 5; SI-12–SI-13, SI-17, SI-30, SI-37,
SI-40–SI-46, SI-58–SI-63, SI-75–SI-76, SI-126–SI-146.

## Contents

1. Global execution contract
2. Wave 5A — policy and application foundation (Tasks 1–3)
3. Wave 5B — registration and adapters (Tasks 4–7)
4. Wave 5C — ratification, release, and closure (Tasks 8–12)
5. Wave 5C final review and program handoff

## Global execution contract

- Deliver exactly three pull requests: Wave 5A, 5B, and 5C. Rebase each new
  worktree from merged main after its predecessor lands.
- Do not begin runtime until the independently reviewed planning/authority PR
  carrying this plan, its design, and SI-139–SI-146 is owner-merged.
- Before each task, reread its cited authority and inspect the named predecessor
  APIs. Stop on any authority or public-interface contradiction; do not guess.
- Use TDD: capture the exact RED, implement the smallest conforming change,
  capture GREEN, refactor only with tests green, and make an imperative commit.
- No task edits `docs/design/specs/` or accepted self-hosted authority. Any
  newly discovered semantic ambiguity returns to the main agent for a ledger
  decision before implementation.
- Preserve strict canonical codecs, three-valued honesty, sealed operand
  custody, one writer lock, no-network tests, and exact 0/1/2 behavior.
- The CLI verb and MCP tool registries are shared serialized resources. Tasks
  6, 7, and 11 must run with an explicit ownership check and no concurrent
  registry change from another feature lane.
- No UI, HTTP workbench action, JavaScript, CSS, Playwright CSE feature, new
  lifecycle engine, prototype promotion, or candidate-reported corroboration.
- Each task report lives under
  `.superpowers/sdd/2026-08-23-cse-wave-5-policy-adapters-lifecycle/` and is
  ignored by Git. It records exact base/head, files, RED/GREEN, gates,
  disclosures, and deviations.
- Each PR receives one independent exact-head Opus review. The main agent
  adjudicates every finding. Accepted runtime defects use one bounded Opus
  correction task and a closure check; authority changes remain main-authored.
- At each final reviewed head run `go test -race ./...`, `make verify`,
  `git diff --check <base>..HEAD`, and prove a clean tracked/index state.

---

# Wave 5A — policy and application foundation

**PR scope:** definition v2/reproduction, the generic Context Integrity
layered-payload selection and experiment exact-tree state seams, typed CSE
policy, provenance, and read-only/validation application operations. No CLI,
MCP, registration write, execution launch, ratification, cleanup, or closure
change.

## Task 1: Add experiment definition v2 and reproduction derivation

**Own:**

- Modify `internal/experiment/definition.go`
- Modify `internal/experiment/codec.go` and strict-decode helpers only as
  required by version dispatch
- Add `internal/experiment/reproduction.go`
- Add/update `internal/experiment/{definition,reproduction,codec}_test.go`
- Add v1/v2 fixtures and digest ratchets under
  `internal/experiment/testdata/`

**Authority:** design §4; CSE AC-1/AC-4, DC-2/DC-15/DC-28, CO-1–CO-3;
SI-139/SI-140.

**RED:** Add tables proving v2 requires canonical `class`, accepts absent or
valid reproduction, rejects `minimum_valid_runs < 2`, unknown/duplicate/null
fields, and preserves v1 decode-only compatibility. Add reproduction tables
over absent rule, too few results, agreeing winners, disagreement,
incomplete-only rows, malformed rows, and a ratified select-other record.

Run:

```bash
go test ./internal/experiment -run 'Test(DefinitionV2|Reproduction)' -count=1
```

Expected RED: missing v2/reproduction symbols or accepted invalid v2 cases.

**GREEN:** Implement schema dispatch without weakening v1, clone all slices,
and derive reproduction from complete validated run details without selecting
or filtering runs.

```bash
go test -race ./internal/experiment -run 'Test(DefinitionV2|Reproduction)' -count=1
go test -race ./internal/experiment -count=1
go vet ./internal/experiment
```

**Commit:** `Add registered experiment reproduction rules`

## Task 2: Add the typed experiment policy and monotone resolver

**Own:**

- Add `internal/experimentpolicy/payload.go`
- Add `internal/experimentpolicy/resolve.go`
- Add `internal/experimentpolicy/authorization.go`
- Add `internal/experimentpolicy/*_test.go`
- Add deterministic policy fixtures under
  `internal/experimentpolicy/testdata/`
- Modify `internal/policyartifact/payload.go` and tests to register closed
  singleton/layered cardinality while preserving singleton as the default
- Modify `internal/policyauthority/{store,resolve}.go` and tests so duplicate
  singleton payloads still fail, while every typed layer of a registered
  layered kind is sealed into the one effective policy
- Add the smallest generic applicable-payload selection in
  `internal/contextcompile` over the existing `EvaluateApplicability` seam,
  with tests; do not add a CSE-specific scope evaluator there
- Modify `internal/experimentrun/authorization.go`, service/resume request
  plumbing, and focused tests to carry and enforce the effective observation
  byte ceiling
- Modify `internal/experimentevaluator/adapter.go` and focused tests to accept
  one explicit positive response limit while retaining the existing hard
  ceiling as defense in depth

**Authority:** design §5; CSE AC-6, DC-10/DC-11, CO-1–CO-3;
Context Integrity DC-23; execution-workspace grant authority; SI-141.

**RED:** Pin strict payload grammar; singleton duplicate refusal; layered
duplicate acceptance and mutation-safe sealed transport; exact target-scope
selection; unknown applicability; all CSE monotone reduction operators;
missing payload; empty intersection; lower-layer environment restoration;
same-id grant/value mismatch refusal; unknown class/evaluator/protocol;
trusted sources; mandatory guards; positive observation/retained-artifact
limits; minimum limit refinement; exact policy-supplied environment values;
raw response and canonical observation below/at/above-limit refusal without
append; hard-ceiling non-weakening; and exact
`experimentrun.ExecutionAuthorization` projection.

```bash
go test ./internal/experimentpolicy ./internal/policyartifact ./internal/policyauthority ./internal/contextcompile ./internal/experimentrun ./internal/experimentevaluator -run 'Test.*(Experiment|LayeredPayload|ObservationLimit)' -count=1
```

Expected RED: missing payload/resolver symbols and unregistered payload kind.

**GREEN:** Register `experiment_execution` as layered at package
initialization. Context Integrity selects applicable layers with its existing
three-valued scope algebra and seals the complete selection. CSE performs only
commutative intersection/minimum/union/denial reduction over that sealed
ledger and returns an immutable decision plus the existing exact
authorization. Its
environment field is a policy-owned `map[string]string`; it never copies
ambient values. Named environment ids intersect, while every layer naming a
surviving id must carry byte-identical grant bytes and environment values;
Wave 5 does not invent a grant-refinement algorithm. Grants remain the sole
timeout/CPU/memory/network/filesystem/process authority. Minimum observation
and retained-artifact byte ceilings remain in the sealed policy decision.
Project the observation cap into `ExecutionAuthorization`; pass the smaller
of that cap and the existing hard ceiling to each evaluator run; reject raw
responses and encoded measured observations above it before durable append.
Task 9 consumes the retained-artifact cap at capsule construction. Never load
policy inside the reducer and never create a fallback.

```bash
go test -race ./internal/experimentpolicy -count=1
go test -race ./internal/policyartifact ./internal/policyauthority ./internal/contextcompile ./internal/experimentrun ./internal/experimentevaluator -run 'Test.*(Payload|Experiment|ObservationLimit)' -count=1
go vet ./internal/experimentpolicy ./internal/policyartifact ./internal/policyauthority ./internal/contextcompile ./internal/experimentrun ./internal/experimentevaluator
```

**Commit:** `Resolve effective experiment execution policy`

## Task 3: Add mutation provenance and the read-only application core

**Own:**

- Add `internal/experiment/provenance.go` and tests
- Modify `internal/experiment/state.go` and tests to extract one byte-source
  entry point while retaining the filesystem API as an adapter over the same
  state algorithm
- Add `internal/experimentapp/service.go`
- Add `internal/experimentapp/actor.go`
- Add `internal/experimentapp/inspect.go`
- Add `internal/experimentapp/validate.go`
- Add `internal/experimentapp/review.go`
- Add `internal/experimentapp/accepted.go` for exact default-branch tree
  enumeration and byte supply through consumer-owned Git ports
- Add `internal/experimentapp/*_test.go`
- Add hermetic fixtures under `internal/experimentapp/testdata/`

**Authority:** design §§2–3, 6–7, 10; CSE AC-1/AC-5/AC-6,
DC-2/DC-7/DC-10/DC-11/DC-16, CO-1–CO-5; SI-142/SI-143.

**RED:** Pin strict/canonical provenance, actor seal/mutation rejection,
single policy-resolution call per operation, v1 inspection compatibility,
v2 registration readiness, filesystem/source state parity, exact
default-branch tree enumeration, divergent worktree bytes, stale/mixed HEAD,
deterministic review packet, unreconciled-direct-edit disclosure, no writes
from read operations, and exact classification of verdict versus operational
errors.

```bash
go test ./internal/experiment ./internal/experimentapp -run 'Test(Provenance|Inspect|ValidateDraft|ReviewRegistration|Actor)' -count=1
```

Expected RED: missing application/provenance symbols.

**GREEN:** Define consumer-owned ports for policy, accepted Git facts,
capability discovery, and result verification. Resolve all accepted experiment
bytes from one commit, then call the source-backed `internal/experiment` state
algorithm; never call the spec-only projector and never duplicate the state
table in the application package. Return structs, keep blocking I/O
context-first, and deep-copy sealed inputs. `review-registration` reports an
unreconciled direct edit but writes nothing. Do not add mutations yet.

```bash
go test -race ./internal/experiment ./internal/experimentapp -count=1
go test -race ./internal/experimentpolicy -count=1
go vet ./internal/experiment ./internal/experimentpolicy ./internal/experimentapp
```

**Commit:** `Establish the experiment application core`

## Wave 5A review and gate

The main agent assembles one immutable base-to-head review package containing
the task reports, diff, design, plan, ledger, and focused gate evidence. One
Opus reviewer checks schema compatibility, policy non-weakening, sealed actor
custody, direct-edit honesty, state derivation, and overengineering. Accepted
findings use one bounded Opus correction pass; the same reviewer performs one
closure check.

Final gate:

```bash
go test -race ./...
make verify
git diff --check <wave-5a-base>..HEAD
git status --short
```

Open and merge Wave 5A only after owner approval.

---

# Wave 5B — registration and adapters

**PR scope:** experiment draft mutations, human registration proposal and
accepted lock, execution/status/explanation operations, one CLI namespace,
one agent-safe MCP tool, and conformance. No ratification, capsule, release,
closure, or UI behavior.

## Task 4: Add draft, candidate, and registration operations

**Own:**

- Add `internal/experimentapp/draft.go`
- Add `internal/experimentapp/registration.go`
- Add `internal/experimentapp/write.go`
- Add/update `internal/experimentapp/*_test.go`
- Add hermetic Git fixtures under `internal/experimentapp/testdata/`

**Authority:** design §§3, 6–7; CSE AC-1/AC-5/AC-6,
DC-1–DC-3/DC-7/DC-11; SI-142–SI-144.

**RED:** Prove unlocked canonical mutations, candidate patch capture, protected
path refusal, provenance atomicity, read-only review refusal for an unmatched
direct edit, explicit `reconcile-draft` append using the closed
`reconcile-direct-draft` id, zero content mutation during reconciliation,
actor custody, locked immutability, expected-HEAD mismatch, proposed lock
non-authority, accepted exact-tree lock, and agent refusal of reconciliation
and registration lock.

```bash
go test ./internal/experimentapp -run 'Test(Draft|Candidate|Registration|DirectEdit)' -count=1
```

**GREEN:** Add `reconcile-draft` as an explicit local-human application
mutation and the fifth provenance operation id; it appends one
unauthenticated exact-digest reconciliation record and does not alter draft
content. Keep review read-only and keep reconciliation absent from MCP.
Serialize proposal artifact plus provenance under the existing writer lock,
then require accepted HEAD to carry their complete pair before either becomes
authority; a locally interrupted pair remains a refused dirty proposal rather
than creating a second journal. Human registration accepts only a sealed
authenticated actor and exact review packet digest. Accepted resolution reads
one default-branch tree and never the worktree proposal.

```bash
go test -race ./internal/experimentapp -run 'Test(Draft|Candidate|Registration|DirectEdit)' -count=1
go test -race ./internal/experimentapp -count=1
```

**Commit:** `Register immutable experiment definitions`

## Task 5: Compose execution, status, and explanation operations

**Own:**

- Add `internal/experimentapp/execution.go`
- Add `internal/experimentapp/status.go`
- Add `internal/experimentapp/explain.go`
- Add/update `internal/experimentapp/*_test.go`
- Modify `internal/experimentrun` only if an exact reviewed composition defect
  proves a missing predecessor seam; stop for adjudication before doing so

**Authority:** design §§3, 5, 7, 10; CSE AC-2–AC-6,
DC-10–DC-15/DC-20–DC-28; SI-126–SI-143.

**RED:** Prove accepted-lock requirement, one policy decision, exact
authorization reuse, run-id identity, start/resume parity, all-run status,
deterministic explanations, no duplicate provenance append for machine
evidence, completed unproven exit 1, and operational preservation.

```bash
go test ./internal/experimentapp -run 'Test(Start|Resume|Status|Explain)' -count=1
```

**GREEN:** Delegate execution only to `experimentrun.Service`; never duplicate
schedule, receipt, observation, result, or recommendation logic.

```bash
go test -race ./internal/experimentapp ./internal/experimentrun -run 'Test(Start|Resume|Status|Explain)' -count=1
go test -race ./internal/experimentapp ./internal/experimentrun -count=1
```

**Commit:** `Expose experiment execution operations`

## Task 6: Add the built-binary CLI adapter

**Serialized registry ownership:** `cmd/verdi` and the CLI inventory are owned
exclusively for this task.

**Own:**

- Add `cmd/verdi/experiment.go`
- Add `cmd/verdi/experiment_test.go`
- Modify the top-level CLI dispatcher and usage files
- Modify CLI inventory/spec-alignment tests and showcase mappings
- Add built-binary fixtures under `cmd/verdi/testdata/` if needed

**Authority:** design §§3, 8, 10–11; CSE AC-5, CO-4/CO-7; SI-145.

**RED:** Built-binary tests pin the exact through-5B operation inventory,
grammar, canonical `--json`, human rendering, stdin/file boundaries where
applicable, explicit `reconcile-draft`, every operation exit, human actor
resolution, no output mutation on failure, and legacy usage byte stability.

```bash
go test ./cmd/verdi -run 'Test.*Experiment.*BuiltBinary' -count=1
```

**GREEN:** Add only `verdi experiment`; subcommands translate to typed core
requests implemented through 5B. Do not predeclare or stub Wave 5C operations,
and do not implement lifecycle logic in `cmd/verdi`.

```bash
go test -race ./cmd/verdi -run 'Test.*Experiment' -count=1
go test ./internal/specalign -run 'Test(CLI|Verb|Vocab)' -count=1
go vet ./cmd/verdi
```

**Commit:** `Expose comparative experiments through the CLI`

## Task 7: Add the agent-safe MCP adapter and parity suite

**Serialized registry ownership:** `internal/mcpserve`, MCP inventories, and
showcase MCP mappings are owned exclusively for this task.

**Own:**

- Modify `internal/mcpserve/tooldefs.go`
- Add `internal/mcpserve/tool_experiment.go`
- Add `internal/mcpserve/experiment_test.go`
- Modify MCP inventory/spec-alignment tests
- Modify showcase MCP mappings/tests
- Add `internal/experimentapp/conformance_test.go`

**Authority:** design §§2–3, 8, 10–11; CSE AC-5/AC-6, DC-7/DC-10/DC-11,
CO-4/CO-7; 05 MCP safety rule; SI-145.

**RED:** Prove strict operation union, data-never-instructions description,
allowed agent operations, explicit refusal of reconciliation and every
human/release/closure operation, no free-form argv or path escape, and
byte-identical semantic
results between CLI JSON and MCP for equivalent reads, mutations, execution,
and failures.

```bash
go test ./internal/mcpserve ./internal/experimentapp -run 'Test.*Experiment' -count=1
```

**GREEN:** Register exactly one `experiment` tool. Reuse one tool-call decoder
and application service. MCP errors retain typed application classification
without converting them into JSON-RPC framing errors.

```bash
go test -race ./internal/mcpserve ./internal/experimentapp -run 'Test.*Experiment' -count=1
go test -race ./internal/mcpserve -count=1
go test ./internal/specalign -run 'Test(MCP|Vocab)' -count=1
```

**Commit:** `Expose agent-safe experiment operations`

## Wave 5B review and gate

One independent Opus review checks core/adapter parity, human-operation
absence from MCP, accepted-versus-proposed identity, built-binary exits,
provenance atomicity, and registry completeness. Apply at most one bounded
correction pass and one closure check.

```bash
go test -race ./...
make verify
git diff --check <wave-5b-base>..HEAD
git status --short
```

Open and merge Wave 5B only after owner approval.

---

# Wave 5C — ratification, release, and closure

**PR scope:** human ratification, reproduction status, selected capsule,
workspace release, the serialized CLI extension for those operations, and
existing spike-close evidence. No MCP expansion, UI, or Wave 7 dogfood.

## Task 8: Add authenticated ratification and accepted-state resolution

**Own:**

- Add `internal/experimentapp/ratification.go`
- Add `internal/experimentapp/ratification_test.go`
- Modify `internal/experiment/ratification.go` and tests to add strict emitted
  v2 actor claim/id bytes while retaining v1 decode/state-history compatibility
- Add accepted Git fixtures under `internal/experimentapp/testdata/`
- Reuse the reviewed exact-tree state seam delivered in Task 3; do not add a
  second accepted resolver

**Authority:** design §§3, 7, 9–10; CSE AC-5, DC-7/DC-16, CO-1–CO-4;
governance-principal authority; SI-143/SI-146.

**RED:** Pin v1 decode-only compatibility; v2 strict/canonical actor claim and
principal-id grammar; all dispositions; exact result digest; candidate binding;
proposal construction only from an authenticated sealed resolution;
forged/mutated/unproven actor; accepted claim re-resolution through the
configured profile/trust-fact reader; derived-id mismatch; missing trust
source; proposed bytes; accepted exact-tree bytes; stale HEAD; cross-run
duplicate digest; and no authority from payload actor text.

```bash
go test ./internal/experimentapp -run 'Test.*Ratification' -count=1
```

**GREEN:** Emit v2 only. Build its actor block only from the claim and
principal id of a sealed authenticated resolution. At accepted use, pass the
persisted claim to the existing governance resolver, require a new sealed
authenticated resolution, and compare the kernel-derived id byte-exactly
before any release or closure effect. Never serialize or reconstruct the
in-memory seal, accept a prebuilt id from an adapter, or treat v1 as new
release authority.

```bash
go test -race ./internal/experimentapp -run 'Test.*Ratification' -count=1
go test -race ./internal/governanceprincipal ./internal/experiment -run 'Test.*(Ratification|Principal)' -count=1
```

**Commit:** `Bind experiment ratification to accepted authority`

## Task 9: Build capsules and release disposable workspaces

**Own:**

- Add `internal/experiment/capsule_binding.go` and tests
- Add `internal/experimentapp/release.go` and tests
- Modify `internal/execworkspace` only if an exact reviewed release seam is
  missing; stop before any predecessor edit

**Authority:** design §§3–5, 9–10; CSE AC-4–AC-6,
DC-8/DC-9/DC-15/DC-16, CO-1–CO-5; execution-workspace release authority;
SI-140/SI-141/SI-146.

**RED:** Pin selecting candidate derivation, non-selecting no-capsule behavior,
exact artifact inventory and digests, fixture ids, missing/extra/symlinked/
nonregular inputs, immutable same/conflict publication, capsule-before-release
ordering, effective retained-artifact ceiling below/at/above every closed
inventory member, exact oversized-artifact witness with no publication or
deletion, all-workspace release, partial failure, retry, and minimal-record
non-targeting.

```bash
go test ./internal/experiment ./internal/experimentapp -run 'Test(Capsule|Release|Reproduction)' -count=1
```

**GREEN:** Build one deterministic manifest from accepted bytes and the one
sealed policy decision, reject any inventory artifact whose raw bytes exceed
the effective `retained_artifact_bytes`, publish and re-decode the manifest,
then invoke the existing release operation. The manifest itself is not a
retained-input member. No filesystem walk may infer selection or broaden
cleanup scope.

```bash
go test -race ./internal/experiment ./internal/experimentapp -run 'Test(Capsule|Release|Reproduction)' -count=1
go test -race ./internal/execworkspace -run 'Test.*Release' -count=1
```

**Commit:** `Retain selected experiment evidence`

## Task 10: Integrate accepted experiment evidence into spike closure

**Own:**

- Add a consumer-owned experiment-evidence port in the existing close core
- Modify `cmd/verdi/close.go` and the narrow spike-close implementation
- Add focused close unit tests and built-binary tests
- Do not add a new close verb, status, edge, or automatic mutation

**Authority:** design §9; CSE AC-5/AC-7, DC-1/DC-7/DC-9/DC-16;
existing spike closure and `resolves` authority; SI-146.

**RED:** Prove comparison-backed spike detection, no-ratification block,
selecting-without-capsule block, identity mismatch, non-selecting refusal,
valid selecting evidence, ordinary non-CSE spike compatibility, exact
pre-effect ordering, and no parent-feature or edge mutation.

```bash
go test ./cmd/verdi -run 'TestClose.*Experiment' -count=1
```

**GREEN:** Add one evidence provider at the existing pre-effect gate. Closure
consumes the ratified answer; it never authors it.

```bash
go test -race ./cmd/verdi -run 'TestClose.*Experiment' -count=1
go test -race ./cmd/verdi -run 'TestClose' -count=1
```

**Commit:** `Gate spike closure on ratified experiments`

## Task 11: Extend the CLI with Wave 5C human operations

**Serialized registry ownership:** `cmd/verdi` and the CLI inventory are owned
exclusively for this task; do not overlap another registry change.

**Own:**

- Modify `cmd/verdi/experiment.go` and its tests
- Modify the CLI dispatcher/usage only if the existing namespace registration
  cannot dispatch the new closed subcommands
- Modify CLI inventory/spec-alignment/showcase mappings in the same commit
- Add built-binary fixtures under `cmd/verdi/testdata/` if needed

**Authority:** design §§3, 8–10; CSE AC-5/AC-7, DC-7/DC-11/DC-16,
CO-1–CO-4/CO-7; binding 05 CLI surface; SI-145/SI-146.

**RED:** Prove the exact Wave 5C `verdi experiment` subcommands for
ratification proposal, capsule publication, and workspace release/retry;
sealed human actor acquisition; canonical JSON/human parity; proposed versus
accepted refusal; capsule-before-release ordering; all 0/1/2 exits; no MCP
inventory change; existing `verdi close` integration from Task 10; and legacy
through-5B behavior/usage compatibility.

```bash
go test ./cmd/verdi -run 'Test.*Experiment.*(Ratification|Capsule|Release|Close).*BuiltBinary' -count=1
```

**GREEN:** Extend the existing `verdi experiment` dispatcher only after Tasks
8–10 core methods exist. Translate arguments to typed application requests;
do not duplicate lifecycle, actor, capsule, release, or closure logic in the
CLI. The MCP union remains byte-for-byte agent-safe and unchanged.

```bash
go test -race ./cmd/verdi -run 'Test.*Experiment' -count=1
go test -race ./internal/mcpserve -run 'Test.*Experiment' -count=1
go test ./internal/specalign -run 'Test(CLI|Verb|Vocab)' -count=1
go vet ./cmd/verdi ./internal/mcpserve
```

**Commit:** `Expose experiment ratification and release operations`

## Task 12: Prove the complete non-UI Wave 5 journey

**Own:**

- Add hermetic application-core journey tests
- Add built-binary CLI journey tests
- Add live MCP parity journey tests
- Add deterministic caching fixtures only if existing Wave 3B fixtures cannot
  exercise the new lifecycle without mutation
- Update spec-align/showcase inventories and guide claims as required
- No Wave 7 genuine comparison and no browser tests

**Authority:** design §§1–13; CSE AC-1–AC-6, CO-7; Wave 5 exit gate.

**RED:** Start from an unlocked comparison and pin the full sequence: draft,
candidate capture, read-only unreconciled review, explicit reconciliation,
authenticated lock proposal, accepted lock, run,
result, explanation, authenticated ratification proposal, accepted
ratification, reproduction posture, capsule, release, and closure evidence.
Add adversarial journeys for a faster incorrect candidate, agent human-action
attempts, organization/project layered-policy weakening, changed accepted
HEAD with a divergent worktree, ratification claim/id substitution,
inconclusive reruns, cleanup failure/retry, and direct Git draft
reconciliation. Exercise every 5B and 5C CLI subcommand; prove the MCP union
remains unchanged after 5C.

```bash
go test ./internal/experimentapp ./cmd/verdi ./internal/mcpserve -run 'Test.*Experiment.*Journey' -count=1
```

**GREEN and containment:**

```bash
go test -race ./internal/experiment ./internal/experimentpolicy ./internal/experimentapp ./internal/experimentrun ./cmd/verdi ./internal/mcpserve -count=1
go vet ./internal/experiment ./internal/experimentpolicy ./internal/experimentapp ./internal/experimentrun ./cmd/verdi ./internal/mcpserve
make spec-align
```

**Commit:** `Prove the experiment lifecycle adapters`

## Wave 5C final review and program handoff

One independent whole-range Opus review covers all Wave 5A–5C commits against
the consolidated design and source-coverage witness. It checks policy and
actor custody, adapter parity, reproduction non-favorability, capsule
losslessness, cleanup safety, closure non-duplication, public inventory, and
overengineering. One accepted correction pass and one closure check maximum;
any residual concern returns to the main agent for explicit adjudication.

Final commands at the reviewed exact head:

```bash
go test -race ./...
make verify
git diff --check <wave-5c-base>..HEAD
git status --short --branch
```

The final handoff names all three merged PRs, exact heads, test outputs,
disclosures, and deferred capabilities. Wave 6 CSE workbench implementation
may begin only after Wave 5C is merged. Wave 7 dogfood remains blocked until
both Wave 5C and the later Wave 6 CSE presentation are green.
