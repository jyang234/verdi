# CSE Wave 3B Evaluator and Isolated Execution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` or
> `superpowers:executing-plans` to implement this plan task-by-task. When
> Claude executes it, the main agent MUST stay on Fable and begin with
> `/fable-orchestration`; Sonnet workers implement non-frontend tasks, Opus
> fixes accepted defects, and the main agent adjudicates authority and owns
> every Markdown/ledger correction. This unit has no frontend work.

**Goal:** Execute a locked comparative-spike experiment through one strict
generic evaluator protocol in disposable default-deny workspaces, preserve an
honest durable environment receipt, and resume only missing measured
observations under an unchanged run identity.

**Architecture:** Keep `internal/experiment` as the sole artifact-schema and
cross-record validation package. Add `internal/experimentevaluator` as the
single-command capability/run adapter and process observer. Add
`internal/experimentrun` as the application core that resolves one injected
authorization, materializes candidates through `internal/execworkspace`,
derives the deterministic schedule, publishes the run receipt and observations
under the checkout writer lock, and invokes the existing closed
`internal/experimentdecision` engine. No CLI, MCP, lifecycle adapter,
workbench, cleanup, or capsule selection enters Wave 3B.

**Tech stack:** Go, `internal/experiment`, `internal/experimentdecision`,
`internal/execworkspace`, `internal/canonjson`, `internal/atomicfile`,
`internal/filelock`, `internal/store`, hermetic process fixtures, and
`fixturegit`.

**Authority:** `spec/comparative-spike-experiments` AC-1–AC-4, DC-10,
DC-12–DC-15, CO-1–CO-3, CO-6–CO-7; `spec/execution-workspace`; the canonical
CSE design's evaluator, execution, failure, and retention sections; the
four-feature orchestration plan's Wave 3B contract; invention-ledger
SI-12–SI-13, SI-17, SI-30, SI-37, SI-40–SI-46, SI-58–SI-63, SI-75–SI-76,
and SI-126–SI-133.

**Execution status:** Runtime implementation is BLOCKED until this consolidated
spec-only head passes its one independent cross-model review, the owner accepts
the decisions, and Task 0's canonical CSE/store-layout amendment is ratified
and merged. The current explicit terminal tree has no multi-run or durable
receipt path, and the current observation/result schemas cannot honestly
represent candidate crash or timeout. No runtime producer may work around
those authority gaps.

## Scope and non-goals

Wave 3B includes:

- evaluator capability discovery;
- the closed generic evaluator run protocol;
- harness-owned observation envelopes and fixed process observations;
- exact base-plus-patch materialization through `execworkspace`;
- one deterministic warmup/measured schedule;
- one durable execution receipt per run;
- atomic append-only observation publication;
- interruption resume against unchanged inputs;
- complete-run decision emission and at-rest receipt verification; and
- retention of the minimal durable run record.

Wave 3B explicitly excludes:

- CSE CLI, MCP, agent, lifecycle, or workbench adapters (Wave 5/Wave 6);
- registration locking and human ratification surfaces (Wave 5);
- concrete project/organization experiment-policy resolution (Wave 5);
- post-ratification workspace release, rejected-candidate cleanup, and the
  selected capsule (Wave 5);
- candidate-reported corroboration (OQ-2 remains unresolved);
- any default network permission, any elevated execution, or any production
  network-enabled path before the concrete policy resolver (the internal core
  accepts network only from an explicit injected authorization and has no
  user-facing adapter);
- random schedules, adaptive stopping, retries that hide a failed attempt,
  scoring, automatic winner selection, or a workflow engine; and
- any product-source promotion from a candidate workspace.

The unit is internal-only. A green library test is not a user-visible command.
Wave 5 remains the only place that may connect real policy authority and expose
the application core.

## Gate A — prerequisite canonical amendment

The main agent authors this as a separate spec-only tranche through the
repository's ratification flow. Do not edit frozen accepted artifacts as an
ordinary implementation change. The amendment must receive its own required
cross-model review before any Sonnet runtime task starts.

The amendment makes these exact changes:

1. Replace the single-run terminal files with this first-real-artifact layout:

   ```text
   .verdi/specs/{active,archive}/<spike>/experiments/<experiment-id>/
   ├── experiment.yaml
   ├── candidates/<candidate-id>.patch
   ├── evaluator-capabilities.json
   ├── runs/
   │   └── <run-id>/
   │       ├── execution.json
   │       ├── observations.jsonl
   │       └── result.json
   ├── recommendation.md
   ├── ratification.yaml
   └── selected/capsule-manifest.json
   ```

   There are no persisted real experiment instances to migrate at the
   amendment head. Tests and canned fixtures are updated in the runtime tasks.
   The tree has no `latest`, `current`, or preferred-run pointer.

2. Register `verdi.experiment-evaluator/v1` as the run request/response
   protocol and close OQ-1 as specified below. Register
   `verdi.experiment-evaluator-capabilities/v2`, adding required nonempty
   `evaluator_version` and requiring protocol support for evaluator V1 and
   observation V2. Capabilities V1 remains read-compatible but is not
   registrable for a Wave 3B run because it cannot supply the required
   evaluator version or V2 support.

3. Revise observations to `verdi.experiment-observation/v2`, adding one
   required closed execution outcome. V1 remains decodeable only for canned
   predecessor compatibility and is never emitted by Wave 3B. A later adapter
   must not silently mix V1 and V2 within one run.

4. Revise results to `verdi.experiment-result/v2`, binding
   `execution_digest` and preserving each candidate's execution outcome. V1
   remains decodeable for predecessor compatibility but cannot prove the
   environment-policy conjunct and cannot be emitted by Wave 3B.

5. Register `verdi.experiment-execution/v1` as the canonical durable run
   receipt and name `runs/<run-id>/execution.json` as SI-42/SI-44's proof
   locus.

6. Define aggregate state without choosing a favorable run:

   - `registered`: no complete run;
   - `measured`: no result exists and at least one run has a complete valid
     observation set;
   - `recommended`: every result-bearing run is valid, every such result is
     `proven-winner`, and all name the same winner;
   - `inconclusive`: at least one valid result exists and the result-bearing
     runs are not unanimous on one proven winner; and
   - `ratified`: `ratification.yaml` binds one exact result digest and that
     result's run/receipt validate.

   Any malformed run directory, cross-run identity mismatch, invalid result,
   or ambiguous duplicate result digest is operational. Every run remains
   enumerable with its own `registered|measured|recommended|inconclusive`
   posture; an incomplete run neither erases nor upgrades completed evidence.
   The aggregate state label is not a run selector. An exact ratification is
   the only operation allowed to prefer one result.

7. Record the amendment and the 38/38 source-coverage matrix in the accepted
   revision history. Update SI-126–SI-133 from conditional to the exact
   ratified amendment head only after merge.

## Fixed evaluator protocol

### Operation transport

`experiment.yaml` retains `evaluator.argv` exactly as an argument vector. V1
adds one cross-field invariant: its final element is the literal `run`.
Discovery copies the vector and replaces only that final element with
`describe`. The executable and every preceding configuration argument remain
byte-identical. Neither operation uses a shell.

`describe` receives empty stdin. Its exit code must be zero and stdout must be
the exact canonical encoding of one strict
`verdi.experiment-evaluator-capabilities/v2` document. The digest of those
canonical bytes must equal `evaluator.capabilities_digest`. Registration
persists the same bytes as `evaluator-capabilities.json`; a locked run reads and
verifies that file rather than executing untrusted discovery before policy
authorization. Stderr is diagnostic only.

The discovery function accepts an already-constructed `execworkspace.Profile`
from its caller and launches through `Profile.Command`; it never creates an
ambient or advisory discovery path. Wave 5 will decide which policy authority
may construct that profile. Wave 3B's locked-run service does not call
discovery—it consumes the registered file.

`run` receives exactly one canonical request on stdin:

```json
{
  "schema": "verdi.experiment-evaluator/v1",
  "experiment_digest": "sha256:<hex>",
  "run": "<run-id>",
  "candidate": "<candidate-id>",
  "cycle": {"kind": "warmup|measured", "number": 1},
  "workload": {"id": "<id>", "path": "<repo-relative>", "digest": "sha256:<hex>"},
  "fixtures": [{"id": "<id>", "path": "<repo-relative>", "digest": "sha256:<hex>"}],
  "contract": {"id": "<id>", "path": "<repo-relative>", "digest": "sha256:<hex>"}
}
```

`number` is one-based within its own kind. Fixtures stay in locked definition
order. Unknown/missing fields, duplicate keys, noncanonical bytes, trailing
data, or an identity mismatch fail operationally before publication.

The evaluator returns exactly one canonical response on stdout:

```json
{
  "schema": "verdi.experiment-evaluator/v1",
  "outcome": {"kind": "completed|candidate-crash|candidate-timeout", "witness": "..."},
  "guards": [],
  "measurements": [],
  "disclosures": []
}
```

Rules:

- `completed` forbids `outcome.witness` and requires the definition-aware
  guard/primary/bound evidence already enforced by `ValidateObservations`.
- `candidate-crash` and `candidate-timeout` require a nonempty witness and
  require empty guards and measurements. They are candidate outcomes, not a
  fabricated failure of every registered guard.
- Evaluator measurements may be `evaluator-measured` or
  `candidate-reported`; `harness-measured` is rejected at this boundary.
- The two fixed `verdi-evaluator-*` observer IDs are reserved: the evaluator
  response may not supply them. If the definition registers one as its primary
  or bounded metric, the harness-supplied value satisfies that requirement.
- A nonzero evaluator exit, harness deadline, missing response, malformed or
  noncanonical response, more than 1 MiB stdout, or more than 1 MiB retained
  stderr is operational. Only a zero-exit strict response can name a candidate
  crash/timeout.
- A non-completed warmup is operational because warmups never enter the
  measured evidence set; the runner does not convert an unmeasured event into
  a candidate verdict or silently discard it and continue.

The harness creates every authoritative observation envelope. The evaluator
never supplies schema version, experiment digest, run, candidate, measured
round, or harness measurements.

Observation V2 decision semantics are exact:

- each measured candidate/round still has exactly one observation;
- a `completed` observation must satisfy all existing guard, primary, and
  bounded-measurement requirements;
- a crash/timeout observation makes that candidate ineligible and contributes
  no metric value or guard verdict;
- aggregates for an ineligible candidate may describe only its completed
  rounds and carry the exact completed-round count;
- any baseline crash/timeout ends the decision as
  `violated-with-witness` under the new closed reason code
  `baseline-candidate-failure`, with candidate, outcome kind, round, and
  witness preserved; and
- a non-baseline crash/timeout does not prevent another fully eligible
  candidate from winning; if no non-baseline candidate remains eligible, the
  existing `no-eligible-candidate` disclosed-unproven reason applies.

Result V2 candidate rows carry a sorted `execution_failures` list of
`{round, kind, witness}`. They never translate an execution failure into a
guard violation.

### Process observer

For every zero-exit `run` response the harness appends:

- `verdi-evaluator-wall-duration`, numeric nanoseconds,
  `source: harness-measured`; and
- on Linux when returned by the process API, `verdi-evaluator-peak-rss`,
  numeric bytes, `source: harness-measured`.

The IDs and units are fixed. The duration is diagnostic unless the locked
definition explicitly registers the same ID. Peak RSS absence is represented
by the attempt's observation disclosure `peak-rss-unavailable`; it is never
zero-filled or written retroactively into the immutable run receipt.
Exit status and harness timeout are control facts, not measurements: nonzero or
deadline expiry invalidates the evaluator attempt operationally.

## Fixed durable run contract

`execution.json` is canonical JSON with this semantic shape:

```text
schema                    verdi.experiment-execution/v1
experiment_digest         locked definition digest
run                       canonical caller-supplied run ID
environment_policy        exact definition execution policy ID
authority_digest          authorization resolver's exact authority digest
capabilities_digest       canonical describe-response digest
schedule_digest           digest of the complete derived schedule
grants_digest             digest of canonical execworkspace grant bytes
fingerprint               exact execworkspace collection projection
enforcement               exact feature-owned projection of every grant row
network                    mode/configured/reason
candidates                sorted registered candidate materialization identities
versions                  Verdi and recommendation-engine versions
disclosures               sorted unique closed disclosures
```

The candidate receipt row carries candidate ID, base commit, patch digest, and
the full `execworkspace.Identity`; path and truncated workspace ID are not
authority. The fingerprint includes evaluator/workload/fixture inputs,
declared environment names and values, OS/architecture, and the named tool
versions. CPU/memory allocation is disclosed unproven when no enforced
resource-ceiling fact exists. Network mode is always present.

`ExecutionReceiptDigest` is `canonjson.Digest` of the validated value. Every
V2 result carries that digest. At-rest verification receives the exact receipt,
requires its experiment/run to match the observation set and result, requires
its schedule/capability/input identities to match the locked definition, then
runs existing recompute-equality. A matching V2 receipt removes
`result-environment-policy-receipt`; a V1 result retains that disclosure.

The service never invents authorization. It requires a consumer-owned port:

```go
type AuthorizationResolver interface {
    ResolveExecutionAuthorization(
        ctx context.Context,
        def experiment.Definition,
        capabilities experiment.Capabilities,
    ) (ExecutionAuthorization, error)
}

type ExecutionAuthorization struct {
    EnvironmentPolicy string
    AuthorityDigest   string
    GrantBytes        []byte
    DeclaredEnv       map[string]string
}
```

The accepted definition's workload, fixture, and contract references do not
carry paths, so the service also requires one read-only resolution port:

```go
type InputResolver interface {
    ResolveExperimentInput(
        ctx context.Context,
        root string,
        ref experiment.ArtifactRef,
    ) (ResolvedInput, error)
}

type ResolvedInput struct {
    ID     string
    Path   string
    Digest string
}
```

For every reference the result must preserve the exact ID/digest, name a
canonical repository-relative path already present in the definition's
`protected_paths`, and resolve below the exact base/candidate root to one
non-symlink regular file whose raw-byte sha256 equals the registered digest.
Duplicate paths and extra/unresolved inputs are operational. The receipt and
evaluator request carry this resolved path; the definition schema remains
unchanged.

The service strict-decodes canonical `GrantBytes`, requires policy equality,
requires the process allowlist to contain the exact evaluator argv0, requires
the granted timeout to equal the definition's timeout, requires every
capability-declared environment name to be present and no undeclared name,
refuses `requires_elevated`, and requires a network grant iff
`requires_network` is true. Until Wave 5 provides the concrete resolver there
is no default/local constructor and no user-facing launch path.

Every `DeclaredEnv` value is explicitly approved by that resolver for both
process exposure and durable recording in the run fingerprint. Wave 3B never
copies ambient environment variables. A later policy adapter must refuse
secret-bearing values rather than pass a secret and expect the receipt writer
to guess that it needs redaction.

Every describe/run launch also verifies the actual executable before start:
after `Profile.Command` fixes the launch path and the caller fixes `Dir`, the
adapter refuses a symlink or nonregular executable, hashes the exact file the
command will execute, and requires equality with `evaluator.digest`. It reads
but never rewrites `Cmd.Path`.

## Fixed schedule and resume contract

The schedule is a pure ordered slice of `(kind, number, candidate)`:

1. cycles `warmup 1..Warmups`;
2. cycles `measured 1..Rounds`;
3. for zero-based global cycle `i`, rotate the locked candidate list left by
   `i % len(candidates)`; and
4. execute candidates in that rotated order.

The schedule digest binds the whole slice. Warmups execute and validate but are
not appended to observations. Measured observation order is the measured
subsequence of this schedule and is validated exactly; file order is not an
arbitrary set.

Each candidate workspace uses its reserved root-level
`.verdi-cse-environment` directory as `BuildProfile`'s caller-owned `envRoot`
for the whole run. The base tree and every candidate patch must leave that path
absent. The directory persists across interruption so a resume does not reset
candidate-local HOME/XDG/TMP state after measured evidence already exists. If
any observation exists and a required candidate environment root is missing,
resume refuses the changed environment. When no observation exists, a missing
root is recreated and all warmups restart. After the complete measured set is
validated, the runner removes only these reserved roots and proves their
absence before emitting a result; failure to clean is operational. This makes
an interrupted workspace dirty and conservatively unreclaimable, while a
successful run leaves no profile-state obstacle for Wave 5's later release.

The first start:

1. validates the locked definition and patches;
2. strict-decodes `evaluator-capabilities.json` and proves digest parity;
3. resolves every workload/fixture/contract identity and verifies its protected
   base-tree bytes;
4. resolves and cross-checks authorization;
5. derives the receipt and full schedule;
6. creates `runs/<run-id>/execution.json` immutably under `writer.lock`; then
7. materializes and evaluates in schedule order.

Resume:

1. refuses an absent execution receipt (start is a separate explicit mode);
2. strict-decodes and fully re-verifies the receipt against the current locked
   definition, described capabilities, authorization, grants, fingerprint, and
   schedule;
3. strict-decodes the whole observations file and verifies its byte order is
   the exact measured-schedule prefix;
4. rejects duplicate, out-of-order, unknown, altered, extra, or middle-gap
   records; and
5. executes only the missing measured tail in schedule order. Completed warmups
   are not resumable evidence, so a resume begins the warmup schedule again
   before executing missing measured keys.

Publishing one measured observation holds `writer.lock`, re-reads and verifies
the current file, and uses `atomicfile.Write` with exactly the existing bytes
plus one canonical JSON line and trailing newline. No cursor or journal is an
authority source. A crash leaves either the old complete file or the old file
plus one complete line.

A complete run invokes `experimentdecision.Evaluate` with the receipt's exact
environment policy, constructs a V2 result with `execution_digest`, verifies
it, and immutably publishes `result.json`. An existing byte-equal result is an
idempotent success; any different existing result is operational. The runner
never publishes a result for an incomplete run.

## Implementation sequence

Concurrency is one. Although SI-123 permits at most two lanes, evaluator
response semantics, observation V2, the execution receipt, and resume identity
form one dependency chain. Splitting them would require temporary duplicate
schemas or a speculative interface. Serialization is simpler and safer.

### Task 0: Ratify the prerequisite CSE/store amendment (main agent only)

**Authority files:** canonical CSE/store artifacts through their ratification
flow, the accepted revision history, this plan, and the invention ledger.

- [ ] Produce the exact amendment described by Gate A without direct-edit
      bypass.
- [ ] Add a 38/38 source-coverage witness and zero-existing-artifact migration
      witness.
- [ ] Run `make spec-align`.
- [ ] Obtain one independent cross-model review; adjudicate and author at most
      one correction pass; obtain the same reviewer's closure check.
- [ ] Owner-merge the amendment before Task 1.

### Task 1: Land the V2 artifact and protocol kernel

**Modify:** `internal/experiment/{definition,capabilities,observation,
observations_validation,result,ratification,state,state_disclosure,enums}.go`
and their tests.

**Create:** `internal/experiment/{evaluator_protocol,execution_receipt,
run_paths}.go` and tests/fixtures.

- [ ] RED: exact strict codecs, union presence, unknown/duplicate/trailing data,
      canonical-byte equality, command-operation invariant, candidate outcomes,
      receipt/result binding, multi-run enumeration, V1 disclosure retention,
      and V2 receipt proof.
- [ ] GREEN: implement only the fixed schemas and validators.
- [ ] Preserve old V1 decode/recompute tests; V1 is read compatibility, not a
      Wave 3B emission path.
- [ ] Add digest/fixture ratchets and prove no real persisted experiment needs
      migration.
- [ ] Run `go test -race ./internal/experiment ./internal/experimentdecision`.
- [ ] Commit: `Define CSE run evidence protocols`.

### Task 2: Implement evaluator discovery and one-attempt observation

**Create:** `internal/experimentevaluator/{adapter,observer,errors}.go`,
platform observer files, tests, and `testdata/` helper executables or canned
responses.

- [ ] RED: describe token replacement, exact stdin, canonical stdout, digest
      parity, closed outcome rules, trust-boundary rejection, stdout/stderr
      bounds, exit/timeout classification, context cancellation, duration,
      Linux RSS, and unavailable RSS disclosure.
- [ ] GREEN: construct every launch through `Profile.Command`; set only `Dir`,
      `Stdin`, `Stdout`, and `Stderr`; never mutate `Path`, `Args`, `Env`,
      `SysProcAttr`, or `ExtraFiles` afterwards.
- [ ] Use injected command/process seams for unit tests and hermetic built
      helper processes for integration; no network.
- [ ] Run `go test -race ./internal/experimentevaluator ./internal/execworkspace`.
- [ ] Commit: `Run strict experiment evaluators`.

### Task 3: Implement schedule, receipt, and authorization cross-checks

**Create:** `internal/experimentrun/{schedule,authorization,inputresolve,
receipt}.go` and tests.

- [ ] RED: complete rotation tables (2/3/4 candidates, zero/multiple warmups),
      digest determinism, clone safety, mismatched policy/digest/timeout/env,
      network/elevated requirements, unsupported grant mechanisms, exact
      input ID/path/digest/protected-path resolution, candidate materialization
      identities, reserved environment-root collisions, and receipt round trip.
- [ ] GREEN: one pure schedule derivation and one authorization validation path;
      do not copy grant or fingerprint semantics.
- [ ] Run `go test -race ./internal/experimentrun ./internal/experiment
      ./internal/execworkspace`.
- [ ] Commit: `Bind experiment schedules to execution receipts`.

### Task 4: Implement start and measured observation publication

**Create:** `internal/experimentrun/{service,storage}.go` and tests.

- [ ] RED: start ordering, immutable receipt before execution, base-plus-patch
      materialization, candidate-root command directory, warmup exclusion,
      candidate outcome preservation, process/evaluator operational failures,
      cancellation, run-scoped environment persistence/cleanup, and no
      observation/result on failure.
- [ ] RED: symlink/nonregular run parents and files, writer-lock contention,
      atomic prefix append, concurrent writers, partial/corrupt existing files,
      and per-step pre-effect snapshots.
- [ ] GREEN: compose existing ports and primitives; do not add a second Git,
      process, lock, or atomic-write implementation.
- [ ] Run `go test -race ./internal/experimentrun ./internal/experimentevaluator
      ./internal/execworkspace`.
- [ ] Commit: `Execute isolated experiment schedules`.

### Task 5: Implement resume and complete-run result emission

**Modify:** `internal/experimentdecision` only where V2 outcome/result semantics
require it.

**Modify/Create:** `internal/experimentrun/{resume,decision}.go` and tests.

- [ ] RED: unchanged resume; rejected missing-middle and accepted missing-tail
      observations;
      changed definition/capabilities/authorization/grants/env/fingerprint/
      schedule; duplicate/out-of-order/altered records; rerun ID separation;
      complete-run idempotency; candidate crash/timeout eligibility; receipt
      digest result binding; V2 at-rest verification; and no favorable run
      selection.
- [ ] GREEN: reuse `ErrObservationIncomplete`, existing decision computation,
      and canonical digest seams; add no second recommendation algorithm.
- [ ] Prove all complete reruns enumerate in canonical run-ID order and an
      explicit result digest is required for later ratification.
- [ ] Run `go test -race ./internal/experimentrun ./internal/experiment
      ./internal/experimentdecision`.
- [ ] Commit: `Resume and evaluate experiment runs`.

### Task 6: Consolidated integration, review, and gates

- [ ] Add a hermetic end-to-end Go journey: locked definition → describe → two
      candidates → interrupted measured schedule → unchanged resume → V2 result
      → at-rest receipt/recompute proof.
- [ ] Add negative journeys for changed environment, malformed evaluator
      response, unsupported isolation, candidate crash, candidate timeout, and
      evaluator timeout. Assert candidate versus operational classification.
- [ ] Audit that no `cmd/verdi`, MCP, workbench, browser, lifecycle, release,
      cleanup, capsule, or real policy adapter entered the diff.
- [ ] Run `go test -race ./...`.
- [ ] Run `make verify` and require the literal final `verify OK`.
- [ ] Build one immutable exact-head review package and obtain one independent
      Opus task review. The main agent adjudicates; an Opus fixer repairs accepted
      defects; the same reviewer performs one closure check only.
- [ ] Commit only small imperative commits with required provenance footers.

## Failure and exit semantics

Wave 3B returns typed application outcomes; it has no process exit adapter yet.
The future Wave 5 adapter maps:

- completed result, including a disclosed-unproven recommendation: exit 0;
- a future explicit experiment-policy/lifecycle verdict refusal: exit 1; and
- malformed authority, missing/mismatched receipt, unavailable control,
  evaluator/harness failure, materialization failure, lock/write failure, or
  changed resume input: exit 2.

Candidate guard failure, candidate crash, and candidate timeout are data inside
a successfully completed comparison, not application errors. An evaluator
crash or harness timeout is operational and publishes no observation for that
attempt.

## Overengineering review

The chosen design deliberately avoids:

- a workflow engine: one service executes one pure schedule;
- a database, cursor, or event log: receipt + canonical observation prefix are
  sufficient for resume;
- containers or a second sandbox: `execworkspace.Profile.Command` is the only
  launch boundary;
- a plugin SDK: one fixed stdin/stdout protocol closes OQ-1;
- a second observation schema owned by the adapter: the evaluator emits a
  non-authoritative body and `internal/experiment` owns the authoritative V2
  record;
- policy inference: an injected authorization is mandatory until Wave 5 owns
  the concrete resolver;
- a preferred-run pointer: all run directories remain visible and exact
  result digests carry later authority;
- per-candidate logs as durable evidence: bounded stderr is returned only in
  operational diagnostics; and
- premature release/capsule logic: Wave 5 remains the ratification boundary.

The additional V2 outcome and execution receipt are not ceremony. Without the
outcome, candidate crashes must be lied about as guard failures or treated as
evaluator failures. Without the receipt, the engine's environment-policy
precondition remains unprovable at rest. Without per-run directories, reruns
must overwrite evidence or be silently selected. Each added structure closes a
binding correctness gap and has one owner.

## Source coverage and losslessness witness

The Wave 3B slice maps 38/38 implicated authority units:

| # | Source unit | Destination |
|---:|---|---|
| 1 | AC-3 project-owned argv evaluator | fixed operation transport; Tasks 1–2 |
| 2 | AC-3 describe handshake | Task 2 |
| 3 | AC-3 capabilities contents | Tasks 1–3 |
| 4 | AC-3 strict observation | evaluator response + observation V2; Tasks 1–2 |
| 5 | AC-3 trust classes | SI-127; Tasks 1–2 |
| 6 | AC-3 append-only observation key | receipt/run paths; Tasks 1, 4–5 |
| 7 | AC-3 metric primitives/aggregations | unchanged predecessor validators |
| 8 | AC-4 base-plus-patch workspaces | Task 4 through `execworkspace` |
| 9 | AC-4 common evaluator/workload | fixed request; Tasks 2, 4 |
| 10 | AC-4 deterministic rotation | SI-129; Task 3 |
| 11 | AC-4 warmups excluded | fixed schedule; Tasks 3–4 |
| 12 | AC-4 environment fingerprint | receipt; Task 3 |
| 13 | AC-4 network default deny | authorization cross-check + `execworkspace`; Tasks 3–4 |
| 14 | AC-4 failure taxonomy | SI-127/SI-131; Tasks 2, 4–6 |
| 15 | AC-4 resume only missing | SI-129; Task 5 |
| 16 | AC-4 unchanged resume inputs | receipt re-verification; Task 5 |
| 17 | AC-4 distinct visible reruns | SI-128; Gate A, Tasks 1/5 |
| 18 | AC-4 reproduction rule | intentionally deferred; no reproduced claim in Wave 3B |
| 19 | AC-4 minimal durable retention | per-run receipt/observations/result |
| 20 | AC-4 cleanup after ratification | deferred unchanged to Wave 5 |
| 21 | DC-10 closed kernel + protocol revision | Gate A, Task 1 |
| 22 | DC-12 trust custody | SI-127; Tasks 1–2 |
| 23 | DC-13 schedule/fingerprint | SI-128/SI-129; Tasks 1/3 |
| 24 | DC-14 failure families | SI-131; Tasks 2/4/6 |
| 25 | DC-15 resume/rerun/reproduction | Tasks 1/5; reproduction deferred visibly |
| 26 | AC-6 project policy | mandatory authorization port; concrete resolver deferred to Wave 5 |
| 27 | CO-1 three-valued honesty | all validation/failure boundaries |
| 28 | CO-2 strict versioned schemas | Gate A, Task 1 |
| 29 | CO-3 canonical/append-only evidence | Tasks 1/4/5 |
| 30 | CO-6 unavailable control operational | Tasks 3/4/6 |
| 31 | CO-7 testing posture | RED/GREEN and hermetic integration in Tasks 1–6 |
| 32 | Delivery step 3 evaluator/observer | Tasks 1–2 |
| 33 | Delivery step 4 isolated execution | Tasks 3–6 |
| 34 | Execution-workspace ownership boundary | all materialization/profile/fingerprint/release mechanics reused, never copied |
| 35 | OQ-1 evaluator protocol scope | resolved by SI-126 and Gate A |
| 36 | OQ-2 candidate-report corroboration | explicitly remains unresolved and non-decision-eligible |
| 37 | Locked workload/fixture/contract digest proof | SI-132 read-only resolver; Tasks 3–6 |
| 38 | Resume-stable profile environment and cleanup handoff | SI-133; Tasks 3–6, final release deferred to Wave 5 |

Transformations: the undefined OQ-1 protocol becomes SI-126's fixed v1; the
single-run terminal tree becomes SI-128's lossless multi-run tree before any
real persisted instance exists; candidate failure prose becomes an explicit
closed outcome rather than a fabricated guard; and the in-memory SI-42
attestation gains a durable receipt. Intentional omissions are reproduction
claims, policy adapters, ratification, cleanup, capsule selection, CLI/MCP/UI,
and OQ-2 corroboration, each retained under its named original wave or open
question. No source unit is silently dropped.

## Handoff after Wave 3B

After Task 6 and owner merge, resume the original program at Wave 5. Wave 5
must provide the concrete experiment-policy resolver and user-facing adapters,
then registration/ratification and post-ratification release/capsule behavior.
Wave 6 owns the workbench. Wave 7 dogfood remains unchanged.
