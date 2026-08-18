# Wave 3.5 UX Readiness Pilot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking. When Claude executes this plan, its
> main agent MUST remain on Fable and begin with `/fable-orchestration`; Sonnet
> workers implement non-frontend tasks, FABLE implements every frontend task,
> Opus repairs accepted defects, and the main agent adjudicates every finding
> against authority.

**Goal:** Test whether one solo spec author can understand a feature or story's
authoring position, identify the most important unresolved matter, inspect all
remaining concerns, and reach the correct existing editing or CLI action before
Verdi proceeds to controlled execution.

**Architecture:** Add one pure `internal/readinesspilot` projection over
already-derived spec, ASD provenance/mutation posture, scratch-board, GLG
journey, and CI conflict facts. Build that projection exactly once during
`verdi serve --context-request <path>` and pass the immutable in-memory value
into one server-rendered workbench cockpit.
The cockpit combines a four-area process rail with a deterministic attention
queue, but performs no mutation: it links to the existing spec board or renders
an exact tokenized CLI fallback. Browser instrumentation is bounded,
memory-only, content-free, and never lifecycle authority.

**Tech Stack:** Go, `internal/artifact`, `internal/designprovenance`,
`internal/journey`, `internal/contextcompile`, `internal/policyconflict`,
`internal/workbench`, server-rendered HTML, dependency-free JavaScript, the
shared dex stylesheet, hermetic Git fixtures, built-binary Go tests, and
Playwright.

**Spec:** Guided Lifecycle and Governance AC-1/AC-6/AC-8,
DC-1–DC-4/DC-10/DC-13, CO-1–CO-4; Context Integrity AC-3; AI-assisted spec
design's one-canonical-draft and direct-Markdown rules; orchestration plan Wave
3.5; invention-ledger SI-123–SI-124; owner-approved full Wave 3.5 design
captured in this plan's source-coverage witness.

**Execution status:** Blocked until the policy-conflict predecessor is merged
and this consolidated spec-only exact head has passed its required independent
cross-model review and owner adjudication.

## Global Constraints

- This plan is stacked on the exact policy-conflict completion head. Do not
  implement it until that predecessor has merged and this substantial
  spec-only tranche has completed its one independent cross-model review.
- V1 is design-phase and acceptance-candidate only. The supplied strict
  context request must name `phase: design`, the selected feature or story, and
  the current design branch/HEAD (or omit `expected` so startup can bind those
  exact current values). Build and review requests fail operationally.
- `--context-request` is additive and optional for the existing `serve` mode.
  Without it, `verdi serve` retains its exact current startup behavior and
  `/readiness` returns the disclosed unavailable page; only the pilot requires
  the flag.
- The projection is read-only and in-memory. Do not add a readiness file,
  readiness cache, database row, status field, transition, stamp, receipt,
  event log, or artifact kind.
- The four areas are presentation concepts, not lifecycle states:
  `shape-proposal`, `show-success`, `check-context`, and `request-review`.
- Three-valued honesty is exact: `proven`, `violated-with-witness`, or
  `unproven`. A missing, malformed, stale, mismatched, or unavailable operand
  never becomes `proven` and is never omitted from the complete concern list.
- `CurrentFocus` is the first non-proven area in the fixed four-area order. It
  changes no state and never suppresses a later violation or unknown.
- The attention queue contains every unresolved concern. Ordering changes
  prominence only; `AllConcerns` remains lossless and includes proven rows.
- The cockpit registers GET routes only. It does not call a board mutation
  endpoint, wrap a CLI mutation, create a new browser action, or reinterpret a
  board response. Existing board behavior is unchanged.
- A board destination is emitted only when the target and branch produce an
  existing `/b/{branch}/board/spec/{name}` address. Every other concern carries
  an exact CLI token vector; neither backend nor frontend emits shell text.
- The startup snapshot is computed before the writer lock, listener, MCP
  socket, or HTTP server is created. Any inability to build a complete honest
  snapshot is operational exit 2 with no server process left running.
- Every cockpit page says that it is a startup snapshot, names its exact HEAD,
  and directs the author to restart `verdi serve` after an edit. No refresh,
  polling, invalidation, websocket, or recomputation endpoint enters V1.
- Browser instrumentation records only a bounded sequence of closed event IDs
  and concern/area IDs in page memory. It records no clock time, source text,
  prompt, hidden reasoning, path outside the canonical target, secret, user
  identity, or network request; it never drives readiness.
- Frontend production, CSS, client JavaScript, rendered cockpit markup, and
  Playwright work are owned by a FABLE agent. Non-frontend implementation is
  owned by Sonnet. Repairs are owned by Opus.
- No controlled execution, evaluator, observer, scheduler, resume, retention,
  receipt, sealed runner, or CSE behavior is implemented here.
- TDD is mandatory: named RED, smallest GREEN, focused refactor, exact diff
  review, then a small imperative commit.
- The consolidated delivery unit closes with fresh `make verify` and
  `go test -race ./...`; each parallel lane runs only its proportionate owned
  gates before integration.

## Fixed Projection Contract

`internal/readinesspilot` owns these in-memory types. They are not a wire
artifact and have no encoder, decoder, schema ID, persistence path, or public
JSON endpoint.

```go
type State string

const (
    StateProven   State = "proven"
    StateViolated State = "violated-with-witness"
    StateUnproven State = "unproven"
)

type AreaID string

const (
    AreaShape   AreaID = "shape-proposal"
    AreaSuccess AreaID = "show-success"
    AreaContext AreaID = "check-context"
    AreaReview  AreaID = "request-review"
)

type Timing string

const (
    TimingCurrent  Timing = "current"
    TimingOther    Timing = "other"
    TimingEventual Timing = "eventual"
)

type Destination struct {
    BoardPath string
    CLI       []string
}

type Concern struct {
    ID          string
    Area        AreaID
    State       State
    Blocking    bool
    Timing      Timing
    Summary     string
    Witnesses   []string
    Destination Destination
}

type Area struct {
    ID    AreaID
    Label string
    State State
}

type Snapshot struct {
    TargetRef     string
    TargetClass   string
    Branch        string
    Head          string
    RequestDigest string
    Areas         []Area
    CurrentFocus  AreaID
    Attention     []Concern
    AllConcerns   []Concern
    StaleNotice   string
}
```

Every slice is non-nil. IDs and enums are closed and validated. Witnesses are
sorted and deduplicated. An unresolved concern's `Destination` requires exactly
one of a nonempty root-relative `BoardPath` or a nonempty CLI token vector; a
proven concern carries neither because it needs no corrective action. Tokens
reject control characters and empty strings.

Concern identity is source-prefixed rather than invented display text:
`shape/problem`, `shape/outcome`, `shape/question/<id>`, `shape/provenance`,
`shape/mutation`, `shape/board`, `shape/board/<kind>/<id>`,
`success/contributor/<id>`, `success/blocker/<journey-id>`,
`context/verdict`, `context/mechanical/<row-id>`,
`context/semantic/<row-id>`, `context/disclosure/<code>`,
`review/blocker/<journey-id>`, `review/role/<transition>/<obligation>`, and
`review/action`. Source IDs retain their exact bytes; validation rejects empty
or control-bearing components.

### Derivation and ordering

The projection receives decoded operands, never paths or raw bytes:

```go
type TargetFacts struct {
    Ref       string
    Class     string
    Branch    string
    Head      string
    BoardPath string
}

type ShapeFacts struct {
    ProblemPresent    bool
    OutcomePresent    bool
    DeclaredObjectIDs []string
    OpenQuestionIDs   []string
}

type ProvenanceFacts struct {
    ChainState        State
    ChainWitnesses    []string
    MutationState     State
    MutationWitnesses []string
}

type BoardItem struct {
    ID   string
    Kind string // question | agent-task
}

type BoardFacts struct {
    State     State
    OpenItems []BoardItem
    Witnesses []string
}

type Fallbacks struct {
    Shape   []string
    Success []string
    Context []string
    Review  []string
}

type Input struct {
    Target        TargetFacts
    Shape         ShapeFacts
    Provenance    ProvenanceFacts
    Board         BoardFacts
    Journey       journey.Record
    Conflict      policyconflict.Report
    Fallbacks     Fallbacks
    RequestDigest string
}

func Derive(input Input) (Snapshot, error)
```

The adapter supplies one explicit positive or unresolved concern for each area,
so an empty area cannot become a vacuous pass.

- `shape-proposal`: exact decoded problem/outcome/declared objects plus the ASD
  sidecar posture. Missing required content and unresolved declared open
  questions are blocking; missing provenance, an unclassified direct-Markdown
  gap, unavailable provenance context, a present mutation journal/staging
  directory, unavailable scratch-board enumeration, or an open scratch
  question/task is nonblocking unproven, never violated merely because direct
  Markdown or scratch annotations are supported. The mutation operand reports
  presence only; it does not decode, recover, or judge a transaction.
- `show-success`: journey evidence contributors plus every current/eventual
  blocker whose ID begins `obligation-quality/`. Contributor resolution maps
  exactly to the three readiness states but is nonblocking detail; current
  obligation-quality blockers are blocking and eventual ones are not.
- `check-context`: the policy-conflict verdict and every report reason or
  disclosure. `pass` is proven, `blocked-violated` is violated, and
  `blocked-unproven` is unproven. That top-level verdict is the sole blocking
  context row; reasons and disclosures remain nonblocking detail so readiness
  never reinterprets SI-120's verdict relevance.
- `request-review`: all remaining journey blockers, principal requirements,
  lifecycle posture, and the existence of a proven safe action that advances
  toward review. Current blockers and the action anchor are blocking; eventual
  blockers are not. An absent or unproven safe action is unproven, never ready.

An area's state is violated if any blocking row is violated, otherwise
unproven if any blocking row is unproven, otherwise proven. The adapter must
emit an explicit positive or unresolved anchor for each area, so the proven
case is never vacuous. A nonblocking unproven disclosure remains visible but
does not silently acquire gate authority. `CurrentFocus` is the first
non-proven area in the fixed area order; it is empty only when all four areas
are proven.

`AllConcerns` sorts by area order, then concern ID. `Attention` contains the
same unresolved concern values, without copies that differ semantically. It
sorts lexicographically by four fixed keys: blocking before nonblocking;
`current` before `other` before `eventual`; violated before unproven; then area
order and concern ID. This is the complete prioritization logic; do not add
scores, weights, heuristics, personalization, or AI ranking.

## Sequencing and Parallel Ownership

```text
policy-conflict merged
        |
        v
Task 1: shared projection contract (serialized)
        |
        +-------------------------+
        |                         |
        v                         v
Task 2: startup builder     Task 3: FABLE cockpit
backend-only                frontend-only
        |                         |
        +------------+------------+
                     v
Task 4: serve integration + Playwright (serialized)
                     |
                     v
Task 5: solo-author pilot and evaluation (serialized)
                     |
                     v
owner accepts Wave 3B re-entry
```

Task 2 and Task 3 may run concurrently only after Task 1 is committed. Their
changed-file inventories below are disjoint. Neither lane edits the umbrella
plan, invention ledger, `Makefile`, a schema registry, or the other's tests.
Task 4 begins only after both lane heads are integrated.

## Task 1: Define the pure readiness projection

**Owner:** Sonnet, serialized shared-contract task.

**Files:**

- Create: `internal/readinesspilot/schema.go`
- Create: `internal/readinesspilot/derive.go`
- Create: `internal/readinesspilot/schema_test.go`
- Create: `internal/readinesspilot/derive_test.go`

- [ ] Write table-driven validation REDs for every enum, nil slice, invalid
  ID, unsorted/duplicate witness, invalid destination union, control-bearing
  CLI token, invalid board item kind, violated mutation/board posture,
  inconsistent area state, missing focus, queue omission, queue duplication,
  and queue-order case.
- [ ] Write derivation REDs covering all three states in every area; an
  unclassified direct-Markdown gap; missing provenance; mutation residue; open
  and unavailable board facts; every journey evidence resolution;
  obligation-quality versus non-quality blocker routing; all three conflict
  verdicts; missing principal/action facts; eventual blockers; a later
  violation retained when an earlier area is current; and input-order
  determinism.
- [ ] Run the RED:

  ```bash
  go test ./internal/readinesspilot -run 'Test(Validate|Derive)' -count=1
  ```

- [ ] Implement only the fixed contract and six-rung comparator above. Reuse
  `journey.Record.Validate` and `policyconflict.Report.Validate`; do not copy
  either validator or recompute their proof semantics.
- [ ] Run the focused GREEN and race package gate:

  ```bash
  go test -race ./internal/readinesspilot -count=1
  go vet ./internal/readinesspilot
  ```

- [ ] Review for favorable empty defaults, missing later concerns, copied proof
  logic, or an accidental persistence/codec surface; then commit:

  ```bash
  git add internal/readinesspilot
  git commit -m "Define the readiness pilot projection"
  ```

## Task 2: Build the exact startup snapshot

**Owner:** Sonnet backend lane. May run concurrently with Task 3 after Task 1.

**Files:**

- Create: `cmd/verdi/readiness_snapshot.go`
- Create: `cmd/verdi/readiness_snapshot_test.go`
- Create: `cmd/verdi/readiness_snapshot_integration_test.go`
- Do not modify: `cmd/verdi/serve.go`, `internal/workbench/**`, `e2e/**`, or
  `internal/dex/assets/style.css` in this lane.

- [ ] Define one consumer-owned builder seam:

  ```go
  type readinessSnapshotBuilder interface {
      Build(ctx context.Context, root, requestPath string) (readinesspilot.Snapshot, error)
  }
  ```

  Production `localReadinessSnapshotBuilder` reuses
  `validatedConflictRequestPath`, `contextcompile.DecodeRequest`,
  `newLocalContextConflictProvider`, `journey.NewProjector`,
  `artifact.DecodeSpec`, `designprovenance.DecodeLog`, `boardio`'s existing
  annotation reader, and `store`'s existing draft-mutation paths. It does not
  add a second request decoder, policy resolver, journey derivation, Git fact
  model, board parser, or recovery interpreter.
- [ ] Write REDs proving: exactly one strict request read; symlink and `..`
  refusal; design-phase-only refusal; feature and story targets; absent expected
  identity bound to current design branch/HEAD; supplied identity mismatch;
  journey/report/spec identity mismatch; missing/malformed provenance mapped to
  explicit unproven when absence is legal and operational when bytes are
  malformed; direct-Markdown gap; present mutation staging/journal disclosed
  without recovery; open and unavailable scratch-board state; conflict
  pass/violated/unproven; and no file write under `.verdi/data` attributable to
  readiness.
- [ ] Run the RED:

  ```bash
  go test ./cmd/verdi -run '^TestReadinessSnapshot' -count=1
  ```

- [ ] Implement the minimum adapter. Compute SHA-256 over the exact canonical
  request bytes for `RequestDigest`; obtain branch and HEAD from the same root;
  reject any cross-source target/branch/HEAD mismatch before `Derive`. Require
  `contextcompile.PhaseDesign`, bind `Expected` to that branch/HEAD, and build
  exactly one `policyconflict.TargetAcceptanceCandidate` by copying the decoded
  request's adapter, grants, scope, and spec plus the computed expected value—the
  same construction already used by `runConflictGate`, not a new target rule.
- [ ] Build destinations in the adapter, not the renderer. For the selected
  design branch use `/b/<path-escaped-branch>/board/spec/<name>` only for
  board-correctable shape/evidence concerns. Otherwise provide an exact CLI
  vector beginning with `verdi`; no shell quoting or executable lookup occurs.
- [ ] Run focused and package gates:

  ```bash
  go test -race ./cmd/verdi -run '^TestReadinessSnapshot' -count=1
  go vet ./cmd/verdi
  ```

- [ ] Review for a second resolver, worktree/expected drift, reads after the
  snapshot returns, hidden absence, and any readiness persistence; then commit:

  ```bash
  git add cmd/verdi/readiness_snapshot.go cmd/verdi/readiness_snapshot_test.go cmd/verdi/readiness_snapshot_integration_test.go
  git commit -m "Build the readiness startup snapshot"
  ```

## Task 3: Build the FABLE-owned hybrid cockpit

**Owner:** FABLE frontend lane. May run concurrently with Task 2 after Task 1.

**Files:**

- Create: `internal/workbench/readiness.go`
- Create: `internal/workbench/readinessrender.go`
- Create: `internal/workbench/readiness_test.go`
- Create: `internal/workbench/assets/readiness.js`
- Modify: `internal/workbench/assets.go`
- Modify: `internal/workbench/boardspec.go` (`Deps.Readiness` fixture/input
  field only)
- Modify: `internal/workbench/handler.go` (GET `/readiness` and JS asset routes)
- Modify: `internal/dex/assets/style.css`
- Do not modify: `cmd/verdi/**`, `internal/readinesspilot/**`, `e2e/**`, or the
  planning/ledger files in this lane.

- [ ] Use `/fable-orchestration` before frontend production. Write renderer
  REDs for the four ordered areas, state labels, current-focus marker, queue
  ordering, complete concern list, witnesses, exact board/CLI destination
  exclusivity, stale notice with HEAD, no mutation form/button/fetch, keyboard
  landmarks, reduced-motion behavior, narrow viewport, dark mode, and all
  three honest states.
- [ ] Register only `GET /readiness` and `GET /assets/readiness.js`. Missing
  injected snapshot returns a disclosed 503 page; wrong methods return 405.
- [ ] Implement a legible hybrid layout: the linear rail stays continuously
  visible; the attention queue is visually primary; “all concerns” is a
  separate complete section rather than a collapsed remainder; state is never
  communicated by color alone.
- [ ] Implement memory-only instrumentation in `readiness.js` with this closed
  event vocabulary:

  ```text
  readiness-opened
  area-inspected
  concern-inspected
  board-link-followed
  cli-fallback-copied
  stale-notice-inspected
  ```

  Each event is `{sequence, event, area_id, concern_id}`; sequence is a
  page-local integer, the last 200 events are kept in
  `window.__verdiReadinessPilotEvents`, and a same-page
  `verdi:readiness-pilot` `CustomEvent` is dispatched. No event leaves the
  browser and no event influences rendering.
- [ ] Run the lane gates:

  ```bash
  go test -race ./internal/workbench -run 'TestReadiness' -count=1
  go test ./internal/specalign -run 'TestVocabProseWitness' -count=1
  go vet ./internal/workbench
  ```

- [ ] Review with FABLE for visual hierarchy, rail/queue balance, complete-fact
  visibility, keyboard use, stale-state legibility, and absence of browser
  mutation. Commit:

  ```bash
  git add internal/workbench/readiness.go internal/workbench/readinessrender.go internal/workbench/readiness_test.go internal/workbench/assets/readiness.js internal/workbench/assets.go internal/workbench/boardspec.go internal/workbench/handler.go internal/dex/assets/style.css
  git commit -m "Present the readiness pilot cockpit"
  ```

## Task 4: Wire serve startup and browser coverage

**Owner:** Sonnet for CLI integration, FABLE for any frontend/e2e correction;
serialized after Tasks 2 and 3 are integrated.

**Files:**

- Modify: `cmd/verdi/serve.go`
- Modify: `cmd/verdi/serve_integration_test.go`
- Modify: `cmd/e2eharness/main.go`
- Create: `cmd/e2eharness/provision_readiness.go`
- Create: `cmd/e2eharness/provision_readiness_test.go`
- Create: `e2e/tests/49-readiness-pilot.spec.ts`
- Modify frontend-owned files only through FABLE if integration exposes a UI
  defect.

- [ ] Add parser REDs for one `--context-request <path>` in any flag position,
  duplicate/missing/`-` values, unknown flags, and compatibility of existing
  `--http` behavior.
- [ ] Add startup-order REDs proving the builder is called once and completes
  before data-directory creation, writer lock, Unix/TCP listen, handler
  construction, or MCP service; builder failure returns exit 2 and leaves no
  server artifact.
- [ ] Add immutable-use REDs proving repeated HTTP requests return the original
  HEAD and concerns after the underlying spec changes, with the stale notice
  still visible and no second builder call.
- [ ] Implement the smallest `cmdServeWithDeps` seam required to prove ordering;
  do not add package globals or background refresh. Pass the one snapshot via
  `workbench.Deps.Readiness`.
- [ ] Have FABLE add Playwright coverage for: linear orientation; priority
  without omission; violated and unproven states; board deep link; CLI copy;
  keyboard traversal; in-memory event shape/cap; editing through the existing
  board; unchanged cockpit after edit; and restart guidance.
- [ ] Run the integration gates:

  ```bash
  go test -race ./cmd/verdi ./cmd/e2eharness ./internal/workbench -run 'Test(Readiness|Serve.*ContextRequest)' -count=1
  make e2e
  ```

- [ ] Review the combined range for first-effect ordering, one snapshot, route
  method closure, exact identity, and absence of a shadow artifact; commit:

  ```bash
  git add cmd/verdi/serve.go cmd/verdi/serve_integration_test.go cmd/e2eharness/main.go cmd/e2eharness/provision_readiness.go cmd/e2eharness/provision_readiness_test.go e2e/tests/49-readiness-pilot.spec.ts
  git commit -m "Wire the readiness pilot at serve startup"
  ```

## Task 5: Run and record the solo-author pilot

**Owner:** Main agent observes and records; the human participant supplies only
their own experience and retains every human-only judgment.

**Files:**

- Create: `docs/superpowers/reports/2026-08-18-wave-3-5-pilot-evaluation.md`
- Modify runtime or frontend only in a separately adjudicated correction pass.

- [ ] Run one feature and one story scenario. For each, ask the participant to:
  identify the target; locate the current area; explain readiness; name the top
  concern; find another concern; distinguish mechanical work from human
  judgment; follow one board link or CLI fallback; make one supported board
  edit; and explain why restart is required.
- [ ] Record task completion, wrong turns, terminology questions, navigation
  friction, unnecessary steps, stale-state understanding, missing corrective
  seams, and the closed in-memory event sequence. Do not record source text,
  prompts, hidden reasoning, secrets, or individual productivity judgments.
- [ ] Give every finding one state: `accepted-wave-3.5`,
  `deferred-original-wave`, `rejected-out-of-scope`, or `unproven`. A deferred
  finding names its original delivery unit and wave.
- [ ] Record the exact branch/HEAD, request digest, snapshot HEAD, browser and
  CLI scenario, verification commands, and whether each success criterion was
  proven, violated with witness, or unproven.
- [ ] If an accepted Wave 3.5 finding changes runtime or UI, perform at most one
  correction pass and one closure review under the repository review rule.
- [ ] Obtain the owner's explicit Wave 3B re-entry decision in the durable pull
  request conversation. Do not encode that approval as a runtime field.

## Task 6: Close the delivery unit

- [ ] Run exact final gates from the consolidated head:

  ```bash
  make verify
  go test -race ./...
  git diff --check
  git status --short
  ```

- [ ] Assemble one delivery-unit handoff with exact base/head SHAs, complete
  changed-file inventory, requirement coverage, RED/GREEN evidence, Playwright
  evidence, pilot evaluation, disclosures, and revert posture.
- [ ] Request one independent consolidated exact-head implementation review.
  Adjudicate every finding; permit at most one correction pass and one closure
  check.
- [ ] Keep Wave 3B paused until the Wave 3.5 pull request, review, checks, pilot
  evaluation, and owner re-entry decision are all current and durable.

## Lossless Carve-out Matrix

| Original owner | Wave 3.5 treatment | What remains with the original owner |
|---|---|---|
| Wave 3 CI `context-compiler` | Consumed unchanged | All compiler semantics and manifests; readiness never recompiles context. |
| Wave 3 CI `policy-conflict-gate` | Consumed unchanged | All proof, judgment, exemption, disposition, lifecycle-gate, and report semantics. |
| Wave 3 CSE evaluator/isolated execution | Deferred unchanged to Wave 3B | Capability discovery, evaluator, observer, materialization, isolation, scheduling, resume, retention. |
| Wave 4 all five delivery units | Deferred unchanged | Sealed execution, receipts/review, lifecycle governance, accountable humans, committed authority. |
| Wave 5 ASD adapters/review paths | Deferred unchanged | Capability/context reads, MCP, proposal dogfood, draft-write, semantic review, provenance views. Existing board routes are only linked, not extended. |
| Wave 5 CSE adapters/ratification/retention | Deferred unchanged | Every CSE adapter and authoritative ratification/retention behavior. |
| Wave 5 GLG `continuous-readiness` | Provisional pilot slice | Full lifecycle-wide current/eventual readiness, feature attestation, authoritative consumers, and non-pilot surfaces. |
| Wave 5 GLG `lifecycle-recovery` | Deferred unchanged | Diagnosis and recovery projection; the cockpit does not recommend recovery. |
| Wave 5 GLG `journey-metrics` | Provisional observation vocabulary only | Canonical outcome events, provenance, stable metric definitions, aggregation, privacy enforcement, and every metric consumer. |
| Wave 6 ASD workbench | Consumed unchanged | Board synchronization, unsaved-edit protection, semantic review, provenance presentation. |
| Wave 6 CI constitution workbench | Deferred unchanged | Rule ledger, derivation trail, impact review, Git-backed proposal workflow. |
| Wave 6 GLG workbench journeys | Promoted: solo-author cockpit shell, four-area language, queue/rail composition, navigation wording | Live refresh, broader roles, authoritative actions, attestation, recovery UX, accessibility completion, responsiveness, performance, complete lifecycle presentation. |
| Wave 6 CSE workbench | Deferred unchanged | Registration, lock, execution, result, and ratification surfaces. |
| Wave 7 integrated dogfood | Deferred unchanged | The pilot is usability evidence, not an ASD/CSE/CI/GLG end-to-end acceptance journey or whole-program approval. |

No deferred unit is deleted from the umbrella completion ledger. “Promoted”
means its exact bounded bytes need not be rebuilt; it does not mark the parent
delivery unit complete. “Provisional” means the pilot implementation and
evidence inform later work but do not satisfy the later unit's exit gate.

## Source Coverage and Losslessness Witness

The owner-approved full Wave 3.5 source is carried in 11/11 groups:

| # | Source group | Destination | Transformation or omission |
|---:|---|---|---|
| 1 | Purpose | Goal, Architecture, Task 5 | Rephrased as an evidence-producing solo-author slice; no feature-completeness claim. |
| 2 | Placement | Umbrella Wave 3A/3.5/3B split; Sequencing diagram | Preserved exactly; Wave 3B re-entry gains an explicit owner gate. |
| 3 | Primary user | Goal, Task 5 | Solo spec author retained; collaborative roles stay Wave 6. |
| 4 | Hybrid primary experience | Fixed contract, Task 3 | Rail, priority queue, complete list, board links, and CLI fallbacks all retained. |
| 5 | Four process stages and current focus | Fixed contract and derivation | Converted to closed area IDs; explicitly non-authoritative and non-suppressing. |
| 6 | Readiness inputs and three-valued model | Architecture, Fixed contract, Tasks 1–2 | Spec, ASD provenance/mutation posture, scratch board, journey, and conflict facts are decoded or classified once; unknown never becomes favorable. |
| 7 | Snapshot behavior | Global constraints, Tasks 2 and 4 | Exact CLI retained; startup-only computation and stale guidance made testable. |
| 8 | Mutation boundary | Global constraints, Tasks 3–4 | Cockpit GET-only; board mutation remains existing board ownership; CLI stays fallback. |
| 9 | Ten pilot success criteria | Task 5 | Converted one-to-one into the feature/story scenario checklist; none omitted. |
| 10 | Eight implementation phases | Tasks 1–6 and umbrella exit gate | Policy-conflict predecessor is an entry gate; sequencing/design are this authority tranche; implementation phases are grouped by shared contract, parallel lanes, integration, pilot, and close. |
| 11 | Promoted/provisional/deferred accounting and correct next action | Carve-out matrix; review gate | Every original Wave 3–7 unit is classified; task execution is blocked until this authority is independently reviewed and accepted. |

No source decision is intentionally omitted. The only structural
transformations are: narrative stages become closed presentation IDs; the
implementation sequence becomes executable TDD tasks; and the likely
carve-outs become explicit bounded classifications that preserve all later
obligations.
