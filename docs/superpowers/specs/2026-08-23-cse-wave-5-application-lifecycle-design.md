# CSE Wave 5 Application and Lifecycle Authority Design

**Status:** Owner-approved design; repository authority becomes effective when
the reviewed commit carrying this document and SI-139 through SI-146 reaches
the configured default branch.

**Planning base:** `8cbb97aa738e34e4703f6d8d57892357b8cf2bd8`

**Owners:** platform-team

**Delivery units:** CSE Wave 5A policy/application foundation, Wave 5B
registration and adapters, and Wave 5C ratification/release integration

**Frontend boundary:** none. The CSE workbench remains the FABLE-owned Wave 6
consumer of the application core defined here.

## 1. Decision and scope

Wave 5 delivers the non-UI CSE lifecycle through one typed application core.
The core composes the already-landed experiment schemas, decision engine,
strict evaluator adapter, isolated execution service, Context Integrity
effective-policy system, governance-principal kernel, Git-derived acceptance
state, and existing spike closure flow. CLI and MCP are adapters over that
core; neither owns policy, authority, state, or experiment semantics.

The work is split into three serialized units:

1. **Wave 5A — policy and application foundation:** experiment-definition v2,
   registered reproduction rules, typed CSE policy, mutation provenance, and
   read/validate/review application operations.
2. **Wave 5B — registration and adapters:** draft/candidate mutation,
   registration lock, run start/resume/status/explanation, and parity across
   the CLI and agent-safe MCP surface.
3. **Wave 5C — ratification and release:** authenticated ratification,
   reproduction derivation, selected capsule construction, workspace release,
   and existing spike-close integration.

The split does not create three cores. Every unit extends one
`internal/experimentapp` consumer. Package boundaries keep schemas, policy,
execution, and application orchestration separate.

This design does not add:

- a workflow engine, mutable lifecycle status, cursor, current-run pointer, or
  preferred-run pointer;
- a second policy store, hierarchy, resolver, or execution-grant vocabulary;
- automatic design selection, ratification, open-question resolution, or
  prototype promotion;
- a new spec kind, link type, evidence kind, or closure path;
- a feature-local identity provider or self-declared human actor;
- a second agent protocol merely to avoid the existing MCP inventory; or
- any browser markup, CSS, JavaScript, HTTP action, or workbench behavior.

## 2. Architecture and ownership

```text
CLI adapter ---------+
                     |
MCP agent adapter ---+--> experimentapp
                           |-- experiment schemas and state derivation
                           |-- experimentpolicy effective resolver
                           |-- governance-principal authority
                           |-- experimentrun execution service
                           |-- Git accepted/proposed identity
                           `-- existing spike closure consumer
```

`internal/experiment` remains the sole schema and cross-record-validation
owner. `internal/experimentpolicy` owns the CSE typed payload and its monotone
interpretation of one sealed `policyauthority.EffectivePolicy`.
`internal/experimentrun` remains the sole runner and receipt/result publisher.
`internal/experimentapp` owns lifecycle preconditions, actor custody,
provenance, immutable experiment-directory writes, and composition of those
predecessors.

Adapters may:

- strict-decode one typed request;
- obtain adapter-controlled actor evidence;
- invoke one application operation; and
- render the returned typed result and exit classification.

Adapters may not:

- reinterpret policy or a disclosure;
- construct or modify sealed authority operands;
- synthesize a human principal from request text;
- change recommendation, reproduction, retention, or closure meaning;
- write experiment authority directly; or
- expose an operation the adapter class is forbidden to invoke.

## 3. Closed operation set

The application core exposes a closed operation vocabulary. Operations share
one identity envelope: checkout root, spike ref, experiment id, expected
accepted HEAD when authority-bearing, and adapter-controlled actor.

### 3.1 Read and planning operations

- `inspect` derives the experiment aggregate state, every visible run, fixed
  disclosures, effective-policy posture, reproduction posture, and permitted
  next actions. It never selects a run.
- `discover-capabilities` runs the existing strict `describe` adapter, returns
  canonical capabilities bytes and digest, and writes nothing.
- `validate-draft` validates definition, candidate patches, protected inputs,
  evaluator identity, policy membership, and path ownership without locking.
- `review-registration` derives a deterministic human review packet containing
  question, class, candidates and patch digests, contract, workload, fixtures,
  evaluator and capabilities, metric, guards, schedule, environment policy,
  reproduction rule, and retention effect. The packet is a projection, never
  lifecycle authority.
- `explain-result` returns a deterministic explanation of one exact result,
  its scope, witnesses, run identity, recommendation, and reproduction status.

### 3.2 Agent-permitted mutations

An adapter-controlled agent may create or amend an unlocked draft, capture or
replace candidate patches while exploratory, and start or resume an already
accepted locked run. It may never lock, ratify, close, weaken policy, change a
locked definition, or release evidence.

Every agent operation carries an explicit unauthenticated attribution with
harness identity and optional session identity. Attribution is not authority.

### 3.3 Human-only mutations

Registration lock and ratification require a sealed authenticated principal
resolution produced by the adapter's existing trust boundary. A payload actor
string never satisfies the operation.

The registration lock binds the canonical definition digest after the human
reviews the registration packet. The resulting bytes are only a proposal
until merge-signaled accepted HEAD proves the human transport event.

Ratification binds one exact valid result digest and one of the existing
closed dispositions. Its actor is copied from the authenticated principal
resolution, never from request data. The resulting bytes are likewise a
proposal until accepted HEAD proves the transport event.

### 3.4 Post-ratification operations

Only accepted ratification bytes may trigger release processing. A selecting
disposition (`select-recommended` or `select-other`) first creates one
immutable selected capsule, verifies it, and only then releases disposable
workspaces. A non-selecting disposition (`reject-all`, `misframed`, or
`request-new-revision`) creates no selected capsule and may release all bulky
workspaces after the ratification is durable.

Cleanup failure is operational. It never changes the ratification, removes
the minimal durable record, or rewrites decision state. All workspaces remain
eligible for a later idempotent retry. No selecting disposition can enter
spike closure without the valid capsule; non-selecting dispositions cannot
close the linked question and instead leave the spike open for reframing or a
new experiment revision.

## 4. Experiment definition v2 and reproduction

`verdi.experiment/v2` is the Wave 5 emitted and registrable definition schema.
V1 remains strict decode-only compatibility and can still derive historical
state, but it cannot be newly locked through Wave 5.

V2 carries all v1 fields unchanged and adds:

```yaml
class: request-path-performance
reproduction:
  minimum_valid_runs: 2
```

`class` is a required canonical ID. It selects no behavior by itself; it is
only an exact policy matching operand for allowed classes and mandatory
guards. `reproduction` is optional. When present,
`minimum_valid_runs` is an integer of at least two. No environment diversity,
statistical score, confidence level, retry suppression, or favorable run
filter is implied.

A result set is reproduced exactly when:

1. the locked v2 definition declares a reproduction rule;
2. at least `minimum_valid_runs` complete valid result-bearing runs exist;
3. every visible result-bearing run is valid and binds that definition;
4. every result-bearing run proves the same winner; and
5. no malformed or ambiguous run state exists.

Incomplete runs remain visible and do not count. They neither improve nor
erase complete evidence. A recommended or ratified result without the rule,
or below its minimum, remains valid but is never described as reproduced.
Ratifying a candidate other than the proven winner does not make that human
selection reproduced.

## 5. Typed CSE policy

`internal/experimentpolicy` registers the `experiment_execution` payload kind
with `policyartifact`. Context Integrity continues to own storage,
inheritance, applicability, effective resolution, identity, and digest. The
CSE package receives one sealed effective policy and has no fallback store.

The v1 payload contains:

```yaml
experiment_execution:
  experiment_paths: [".verdi/specs/active/**/experiments/**"]
  candidate_paths: ["spikes/**"]
  classes: [request-path-performance]
  evaluators:
    - argv0: ./tools/cache-evaluator
      protocols: [verdi.experiment-evaluator/v1]
  environments:
    - id: default-deny
      grants: <exact spec/execution-workspace grant-set shape>
      declared_environment: [GOMAXPROCS]
  limits:
    timeout_seconds: 120
    observation_bytes: 1048576
    retained_artifact_bytes: 16777216
  trusted_measurement_sources: [harness-measured, evaluator-measured]
  mandatory_guards:
    - class: request-path-performance
      guards: [contract-correct]
```

Every list is present, sorted, and duplicate-free. Numeric limits are positive.
The grant field strict-decodes through the one shared execution-workspace
grammar; CSE does not restate network, filesystem, process, CPU, or memory
semantics.

The CSE resolver selects all applicable effective-policy entries that carry
this payload and refines them monotonically:

- allowlists intersect;
- maximum limits take the minimum;
- required guards union;
- any denial dominates;
- a lower layer cannot restore an excluded path, class, evaluator, protocol,
  measurement source, environment value, or grant.

The selected definition environment-policy id must resolve to one exact
effective environment entry. Its canonical grant bytes, declared environment,
and effective-policy digest become the existing
`experimentrun.ExecutionAuthorization`. Missing applicable CSE policy,
duplicate environment ownership, empty intersections, malformed refinement,
or an input outside the final allowance refuses fail-closed.

Policy is resolved once per application operation. The same sealed result and
digest drive validation, execution authorization, and mutation provenance;
an adapter cannot ask two resolvers and choose a favorable answer.

## 6. Mutation provenance

Each experiment directory may carry `mutation-provenance.jsonl`, a strict,
canonical, append-only sequence. It is not lifecycle state and never grants
authority. Each record binds:

```text
schema            verdi.experiment-mutation-provenance/v1
experiment        spike ref + experiment id
operation         closed CSE operation id
previous_digest   exact prior artifact-set digest
result_digest     exact resulting artifact-set digest
policy_digest     exact sealed effective-policy digest
  policy_decision   allowed, with closed reason ids
attribution       canonical principal or explicit unauthenticated marker
harness/session    present only for delegated-agent attribution
paths             sorted canonical affected paths
digest            record self-digest
```

The closed successful mutation ids are `draft-definition`,
`capture-candidate`, `propose-registration`, and `propose-ratification`.
Machine evidence production already carries its own receipt, observation,
result, capsule, and release identities and is not duplicated into a second
provenance algorithm. Refused attempts are returned to the caller and are not
written into committed authority merely to create an audit side effect.

Direct Markdown/Git edits cannot truthfully identify every editing event.
During registration review the application core compares the accepted base
with proposed bytes. If changes lack an exact matching typed-operation chain,
it appends one `reconcile-direct-draft` record with explicit unauthenticated
attribution and the exact prior/result digests. It never invents a person,
agent, session, or intermediate operation. The human registration review then
judges those resulting bytes normally.

Typed proposal writes serialize the artifact and provenance append under the
checkout writer lock. They remain non-authoritative working-tree bytes until
Git commits them together and the accepted resolver proves their complete
pair at default-branch HEAD. A crash between local files therefore leaves a
dirty, incomplete proposal that later validation refuses or reconciles; it
never creates a partially authoritative state and does not justify a second
transaction journal. Existing accepted canonical artifact bytes are never
rewritten to repair a missing provenance record.

## 7. Proposed versus accepted authority

Working-tree or review-branch lock and ratification bytes are proposals. The
application core has separate proposed and accepted resolvers:

- proposed operations validate exact caller-selected bytes and expected HEAD;
- authority-bearing inspection and lifecycle operations resolve the exact
  default-branch HEAD tree and reuse the existing merge-signaled state
  projector.

Registration is proven only when the locked definition's exact bytes and
digest are present in the accepted tree. Ratification is proven only when the
exact ratification bytes, bound result, definition, receipt, observations, and
candidate patches resolve from one accepted tree. An unmerged proposal never
enables execution, cleanup, or closure.

The actor resolution embedded in `ratification.yaml` is independently
re-verified through the governance-principal kernel at the accepted revision.
Unproven authentication is a blocking verdict. Malformed or internally
inconsistent identity evidence is operational.

## 8. CLI and MCP adapters

Wave 5B adds one top-level CLI namespace:

```text
verdi experiment <operation> ...
```

Subcommands map one-to-one to the application operations. Machine output is
canonical JSON when `--json` is selected; human output is a rendering of the
same typed result. Standard streams and the repository exit contract remain
unchanged: `0` clean/proven, `1` completed refusal or unproven verdict, `2`
operational.

Wave 5B also adds one MCP tool named `experiment`. Its request is a strict
closed union over the agent-permitted operation subset. This is one typed tool,
not a free-form command tunnel. It never accepts `propose-registration`,
`propose-ratification`, `publish-capsule`, `release-workspaces`, or closure.
Unknown operations and human-only operation names fail explicitly.

The CLI verb and MCP tool inventories are serialized shared registries. Wave
5B cannot overlap any ASD, CI, or GLG task that changes either registry.
Spec-alignment and showcase inventories change in the same commit as their
live registrations.

CLI and MCP conformance tests feed semantically identical requests through
both adapters and require byte-identical core result projections. The MCP
wrapper retains the existing data-never-instructions safety note; free text
from definitions, witnesses, or provenance is always returned as untrusted
data.

## 9. Ratification, capsule, release, and closure

The human adapter obtains one sealed authenticated principal resolution before
building a ratification proposal. It sets `ratification.actor` from that
resolution and runs the existing record and binding validators. The core
cannot accept an actor field supplied by the request.

For a selecting ratification, the selected candidate is:

- the bound result's winner for `select-recommended`; or
- the explicitly named registered candidate for `select-other`.

The capsule builder deterministically produces the existing
`verdi.experiment-capsule/v1` manifest. Its complete required artifact set is:

- locked definition;
- selected candidate patch;
- evaluator capabilities;
- behavioral contract;
- workload;
- every fixture;
- selected run execution receipt;
- the selected run's complete observations;
- selected result;
- ratification; and
- recommendation explanation when present.

Artifact ids are closed and deterministically derived; fixture ids are
namespaced by registered fixture id. Digests are recomputed from exact bytes.
The builder rejects missing, extra, duplicate, mutable, symlinked,
non-regular, or digest-mismatched inputs. The manifest is published immutably
under the writer lock and re-decoded before release begins.

Release calls the existing execution-workspace release operation for each
known experiment-scoped candidate workspace. Selecting ratification releases
all disposable workspaces after the capsule is safe; the capsule, not a dirty
prototype checkout, is the retained reproduction set. Non-selecting
ratification releases every disposable workspace without producing a capsule.
Minimal experiment artifacts are never release targets.

The existing spike-close service receives an additive CSE evidence provider.
For a comparison-backed spike it requires accepted ratification. A selecting
disposition additionally requires a valid capsule whose definition, result,
candidate, and ratification identities match. It then uses the ratified answer
through the spike's existing `resolves` edge. It adds no edge and never edits
the parent feature. Non-selecting dispositions remain honest terminal human
responses to an experiment but do not satisfy open-question closure.

## 10. Failure and exit semantics

Application results classify exactly three outcomes:

- `clean`: the operation completed; for a comparison, a proven winner exists;
- `verdict`: a well-formed operation completed with an unsatisfied policy,
  authority, lifecycle, or evidence condition; or
- `operational`: required bytes or external facts could not be safely
  interpreted or changed.

CLI maps them to exits 0, 1, and 2. MCP returns a typed tool failure carrying
the same classification and witness; JSON-RPC framing errors remain protocol
errors.

Examples of verdict outcomes include policy refusal, unauthenticated human
authority, an agent request for a human-only operation, unreproduced posture,
an inconclusive comparison, and unsatisfied closure evidence. Examples of
operational outcomes include malformed or noncanonical bytes, ambiguous Git
state, unsafe filesystem shape, lock contention, evaluator/harness failure,
and cleanup failure.

No operational error becomes a comparison verdict. No cleanup error rewrites
ratification. No missing fact is interpreted favorably.

## 11. Verification and adversarial coverage

Every unit uses TDD and receives an independent task review before the next
unit crosses its authority boundary. Required evidence includes:

- strict decode, canonical encode, digest, clone-safety, and version migration
  tests for every new record;
- policy refinement tables proving intersection/minimum/union/deny precedence
  and lower-layer non-weakening;
- application tests proving one effective-policy resolution per operation;
- hermetic Git histories distinguishing proposed from accepted bytes;
- authenticated-principal and unauthenticated-agent custody tests;
- CLI built-binary and live in-process MCP conformance tests;
- explicit MCP refusal of every human-only operation;
- registration immutability and direct-edit reconciliation tests;
- reproduction tables over zero, incomplete, agreeing, disagreeing,
  malformed, and extra visible runs;
- capsule exact-inventory/digest tests;
- cleanup tests for symlink, nonregular, missing, partial, contended, retried,
  selecting, and non-selecting states;
- closure tests proving no new edge or automatic question resolution;
- `go test -race ./...`; and
- `make verify`, including spec-align, showcase, and existing Playwright gates.

Wave 5 adds no browser behavior, so it adds no CSE Playwright suite. Wave 6
must exercise the same core through FABLE-owned workbench tests.

## 12. Delivery gates

Wave 5A may begin only after this authority and its ledger entries merge.
Wave 5B begins only after Wave 5A's exact head receives independent review and
the CLI/MCP registries are free. Wave 5C begins only after Wave 5B adapter
conformance is green. No unit may absorb another Wave 5 feature lane merely
because up to three orchestration slots exist.

Each unit lands in its own pull request. Each PR is based on current main,
contains only its owned packages and authority-aligned inventory changes, and
passes the full gate at its final reviewed head. Wave 6 remains blocked until
Wave 5C lands.

## 13. Lossless source-coverage witness

This design maps every deferred CSE Wave 5 obligation from the original
orchestration and the Wave 3B handoff:

| Source obligation | Destination | Transformation or omission |
|---|---|---|
| Wave 5 CSE CLI and agent adapters | §§2, 3, 8, 11 | One CLI namespace and one agent-safe MCP union over one core |
| AC-5 same typed operations | §§2–3, 8 | Adapter parsing/rendering separated from core semantics |
| AC-5 human registration lock | §§3, 7 | Proposal bytes separated from merge-signaled accepted authority |
| AC-5 authenticated ratification | §§3, 7, 9 | Existing ratification schema plus sealed principal resolution |
| AC-5 spike closure | §9 | Additive evidence provider into the existing closure path; no new edge |
| AC-4 reproduction rule deferred by Wave 3B | §4 | Minimal registered run-count rule over existing unanimous aggregate |
| AC-4 cleanup deferred by Wave 3B | §§3, 9 | Release only after durable ratification; minimal evidence excluded |
| AC-4 selected capsule deferred by Wave 3B | §9 | Exact closed inventory over existing capsule v1 manifest |
| AC-6 concrete policy resolver deferred by Wave 3B | §5 | Typed Context Integrity payload; no feature-local hierarchy |
| AC-6 policy constraints | §5 | Paths, classes, evaluators, grants, limits, sources, mandatory guards |
| AC-6 mutation provenance | §6 | CSE-specific strict append-only record; honest direct-edit reconciliation |
| DC-2 derived state | §§3–4, 7 | No new state artifact or preferred-run pointer |
| DC-7 human-only decisions | §§3, 7–9 | Agent surface structurally omits authority operations |
| DC-8 durable reasoning/disposable prototypes | §9 | Capsule first, then workspace release |
| DC-9 no patch promotion | §§1, 9 | Capsule is evidence only; product source untouched |
| DC-11 no privileged adapter | §§2, 8, 11 | CLI/MCP parity against one core |
| DC-15 visible reruns and reproduction | §4 | Every visible result participates; no favorable selector |
| DC-16 ratification dispositions | §§3, 7, 9 | Selecting and non-selecting consequences made explicit |
| CO-1 three-valued honesty | §§4, 10 | Missing/ambiguous facts never pass |
| CO-2 strict schemas | §§4–6, 8–9 | Versioned closed records and unions |
| CO-3 deterministic artifacts | §§4, 6, 9 | Canonical bytes, digests, sorted inventories, immutable writes |
| CO-4 process exit mapping | §10 | Exact 0/1/2 adapter mapping |
| CO-5 scoped claims | §§4, 8–9 | Explanation and reproduction remain definition/run scoped |
| CO-7 test requirements | §11 | Core, Git, binary CLI, MCP, adversarial filesystem, full gates |
| Wave 6 CSE workbench | Explicitly omitted in §§1, 8, 11–12 | Deferred unchanged to FABLE-owned Wave 6 |
| Wave 7 genuine comparison | Explicitly omitted in §12 | Deferred unchanged until Wave 5C lands |

Coverage: **25 of 25 source obligations mapped**. No source capability is
silently removed. Candidate-reported corroboration remains the canonical
unresolved OQ-2 and is intentionally omitted from decision eligibility.
