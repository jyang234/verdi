# Comparative spike experiments

Date: 2026-07-30
Status: approved in design session; awaiting document review

## Contents

1. [Purpose](#purpose)
2. [Current-state grounding](#current-state-grounding)
3. [Scope and success criteria](#scope-and-success-criteria)
4. [Design principles](#design-principles)
5. [Decision-proof model](#decision-proof-model)
6. [Architecture and artifact lifecycle](#architecture-and-artifact-lifecycle)
7. [Explore, register, and lock](#explore-register-and-lock)
8. [Experiment definition](#experiment-definition)
9. [Evaluator and observation protocol](#evaluator-and-observation-protocol)
10. [Deterministic recommendation](#deterministic-recommendation)
11. [Execution, isolation, and recovery](#execution-isolation-and-recovery)
12. [Human and agent interaction](#human-and-agent-interaction)
13. [Ratification and existing spike closure](#ratification-and-existing-spike-closure)
14. [Retention and reproducibility](#retention-and-reproducibility)
15. [Extensibility](#extensibility)
16. [Policy, abuse resistance, and honest limits](#policy-abuse-resistance-and-honest-limits)
17. [Caching example](#caching-example)
18. [Verification strategy](#verification-strategy)
19. [Rollout](#rollout)
20. [Non-goals](#non-goals)
21. [Decisions and rejected alternatives](#decisions-and-rejected-alternatives)

## Purpose

Verdi should help a team answer a practical feature-design question:

> Given several viable technical approaches, which path should this feature
> take to produce the most desirable accepted outcome?

Comparative spike experiments extend an existing question-resolving spike
with an immutable, executable comparison of candidate technical designs. The
comparison preserves a common behavioral contract, measures a preregistered
outcome, rejects candidates that violate required constraints, and produces
either a scoped recommendation or an honest inconclusive result.

Verdi recommends; a human ratifies. Prototype code remains disposable and can
never be promoted into product source. If the team selects an approach, it is
implemented afresh under a normal story spec and the ordinary build,
verification, and pull-request process.

This is a lightweight, high-trust decision mechanism. It is more rigorous
than an informal local benchmark but intentionally does not duplicate
Verdi-bench's benchmark-grade evaluation of agents, models, and engineering
harnesses.

## Current-state grounding

The binding artifact and evidence specifications already define the parent
lifecycle:

- A spike is a story subtype with `spike: true`.
- It must carry at least one `resolves` edge to a declared open-question
  fragment and carries no `implements` edges.
- It may attach to a draft or accepted feature.
- It is exempt from the implementation evidence model.
- VL-016 fences its committed branch diff to configured non-product
  `spike_paths`.
- Its deliverable is an answer rather than shippable product code.
- A frozen feature is never edited when the question is answered. The
  accepted spike's `resolves` edge produces the computed `resolved-by`
  backlink.

Comparative experiments preserve those semantics. They are subordinate
evidence used to choose and justify the spike's answer, not a new spec kind,
edge vocabulary, evidence kind, or alternate closure path.

Verdi-bench already owns controlled comparisons of agent stacks, models,
prompts, and harness configurations. Comparative spike experiments instead
compare technical implementations of the same feature behavior. Chronicle
process telemetry may report how ergonomic the experiment journey is, but it
does not decide which candidate implementation wins.

## Scope and success criteria

The first version supports technical alternatives that can be exercised
against one common behavioral contract and compared using machine-produced
observations. Typical examples include:

- caching at different points in a request path;
- choosing between storage, indexing, or invalidation strategies;
- comparing algorithms under a fixed correctness contract;
- selecting a protocol or serialization approach under compatibility and
  resource constraints.

A successful journey:

1. begins with a real open question owned by a spike;
2. permits unconstrained, disposable exploration;
3. freezes a fair comparison before measurements count;
4. evaluates every candidate against the same contract and conditions;
5. distinguishes correctness failures from operational failures;
6. produces a reproducible recommendation or an explicit unproven result;
7. requires a human to ratify the path;
8. preserves enough evidence to audit the decision later; and
9. discards prototype machinery rather than smuggling it into product code.

The v1 is successful when Verdi dogfoods one genuine design choice through
this complete path and the chronicle can identify the human effort, failures,
and decision confidence without relying on an informal transcript.

## Design principles

1. **The desired outcome comes first.** Candidates are compared against the
   outcome the feature is intended to produce, not against whatever each
   prototype happens to measure well.
2. **Explore freely, then lock.** Work before registration is explicitly
   exploratory. Evidence counts only after a human freezes the question,
   candidates, workload, metrics, constraints, environment, and decision
   rule.
3. **Correctness precedes optimization.** A fast candidate that violates the
   common behavioral or safety contract is ineligible.
4. **One primary metric.** A preregistered primary outcome determines the
   ranking. Secondary metrics are bounded guardrails, never adjustable
   weights in a composite score.
5. **Deterministic interpretation of disclosed noise.** Verdi makes the
   execution schedule, aggregation, thresholds, and recommendation mechanical.
   It does not claim that raw performance observations are mathematically
   repeatable.
6. **Recommend, never decide.** Verdi cannot ratify an experiment, resolve an
   open question, or choose a feature design on a human's behalf.
7. **Disposable code, durable reasoning.** Prototype workspaces disappear.
   Registered candidate diffs, normalized observations, and the human
   decision remain.
8. **Closed kernel, open typed adapters.** Provenance, trust, and decision
   invariants are not configurable. Domain-specific evaluators and observers
   extend the system through versioned strict protocols.
9. **No privileged agent path.** Interactive adapters invoke the same typed
   operations, while direct artifact edits must pass the same strict
   validators and policy checks before they can change lifecycle state.
10. **Three-valued honesty.** The result is proven, violated with a witness,
    or disclosed as unproven. Missing or ambiguous evidence is never a pass.

## Decision-proof model

"Best" is not an intrinsic property of a prototype. It is a scoped claim
defined by an immutable decision contract:

- the accepted behavior to preserve or produce;
- the registered candidates;
- the primary outcome to optimize;
- constraints that cannot be sacrificed;
- the minimum improvement that matters;
- the workloads and environments over which the result applies; and
- the deterministic rule that turns observations into a recommendation.

The decision sequence is:

```text
behaviorally valid candidates
    intersect candidates satisfying every guardrail
    intersect candidates materially improving on the baseline
    compare by the registered primary metric
    => proven winner or disclosed-unproven result
```

Across candidate and comparison levels, a completed valid run expresses the
three-valued result:

- **Proven winner:** one candidate passes every required guard, materially
  improves on the baseline, and materially outperforms every other eligible
  candidate under the registered comparison.
- **Violated with witness:** a candidate is ineligible because a named
  correctness, safety, or resource guard failed with a preserved witness.
- **Disclosed as unproven:** the run is complete and valid, but no candidate
  qualifies or noise, ties, or conflicting constraints prevent the evidence
  from distinguishing a winner.

An operational harness or evaluator failure is not a fourth decision verdict
and is not silently converted into evidence. It records an operational error,
produces no recommendation, and must be resumed or rerun before the comparison
can make a decision claim.

The experiment verb follows Verdi's existing exit contract: `0` for a
completed comparison with a proven winner, `1` for a completed comparison
whose verdict is unproven, and `2` for an operational failure.

A proven result must state its boundary prominently:

> Candidate B is the best demonstrated path among the registered candidates
> for this desired outcome, workload, environment, and comparison revision.

Verdi does not claim that the candidate is universally superior to
unregistered designs or production conditions that the experiment did not
represent. The human registration review determines whether the declared
outcome and conditions are a faithful decision proxy. The harness then makes
the comparison mechanical.

## Architecture and artifact lifecycle

The spike remains the question and lifecycle container. Each locked
comparison is an immutable child experiment:

```text
.verdi/specs/active/<spike>/experiments/<experiment-id>/
├── experiment.yaml
├── candidates/
│   ├── baseline.patch
│   ├── candidate-a.patch
│   └── candidate-b.patch
├── observations.jsonl
├── result.json
├── recommendation.md
├── ratification.yaml
└── selected/
    └── capsule-manifest.json
```

The layout shows the complete terminal shape. Observation, result,
ratification, and selected-capsule artifacts appear only as the lifecycle
reaches those states. The experiment directory follows its parent spike when
the existing lifecycle moves that spike from active to archive.

The child does not mutate the spike schema or introduce a second lifecycle
status. State is derived from the presence and validity of artifacts:

| Derived state | Required facts |
|---|---|
| exploratory | no locked definition digest exists |
| registered | definition and candidate patches are complete and locked |
| measured | every registered observation exists and validates |
| recommended | a complete valid run produced a proven winner |
| inconclusive | a valid decision evaluation produced no proven winner |
| ratified | a human recorded a decision against the immutable result |

One spike may contain multiple experiment revisions, such as `latency-v1` and
`latency-v2`. A revision is never edited after lock. A changed candidate,
metric, threshold, workload, evaluator, or environment becomes a new child
experiment so history cannot be rewritten.

The experiment directory and captured patches live inside the spike's own
spec directory, which VL-016 already permits. Additional durable support
files may live only under configured non-product `spike_paths`. A registered
patch may describe ephemeral changes to product source inside an isolated
candidate workspace, but those product changes are never committed on the
spike branch. VL-016 therefore continues to fence the durable spike diff.

## Explore, register, and lock

Before registration, a human or agent may create, modify, run, or discard any
number of candidate prototypes in disposable workspaces. Results from this
phase are explicitly exploratory and cannot be copied into the experiment as
decision evidence.

Registration captures:

- one common base commit;
- every candidate as a canonical binary-capable patch, including added files;
- the digest of each patch and its base;
- the common behavioral contract;
- evaluator executable, configuration, capabilities, and content identity;
- workload and fixture identities;
- correctness and safety guards;
- the primary metric, direction, aggregation, and significance threshold;
- secondary guardrail bounds;
- warmups, measured rounds, execution order, timeouts, and environment policy;
- the recommendation algorithm version; and
- the retention policy.

All candidates must apply to the same base. Protected paths include the
experiment definition, evaluator, workload, fixtures, and governing policy.
A candidate touching a protected path cannot be registered.

Verdi derives a concise semantic review packet from the proposed
registration. It highlights the question, candidate diffs, common contract,
metric, constraints, execution schedule, environment, and retention effect.
A human explicitly locks the packet. Locking assigns the definition digest
and makes the revision immutable.

Only this lock is a new pre-execution human checkpoint. It replaces
line-by-line ceremony with one review of the consequential decision contract.

## Experiment definition

`experiment.yaml` uses the strict, versioned
`verdi.experiment/v1` schema. Unknown fields, enum values, metric types,
aggregations, and versions fail closed. Its conceptual sections are:

```yaml
schema: verdi.experiment/v1
id: cache-placement-v1
spike: spec/cache-placement-spike
question: spec/request-path#oq-cache-placement
base_commit: <full-git-sha>

candidates:
  - id: baseline
    patch: candidates/baseline.patch
    digest: sha256:<digest>
  - id: final-cache
    patch: candidates/final-cache.patch
    digest: sha256:<digest>
  - id: facts-cache
    patch: candidates/facts-cache.patch
    digest: sha256:<digest>

evaluator:
  argv: ["./tools/cache-evaluator", "run"]
  digest: sha256:<digest>
  capabilities_digest: sha256:<digest>

workload:
  id: representative-request-mix
  digest: sha256:<digest>

decision:
  primary_metric:
    id: request-latency
    type: duration
    unit: ms
    aggregation: p95
    direction: lower
  baseline_improvement:
    relative: 0.25
  candidate_separation:
    relative: 0.05
  guards:
    - id: behavioral-equivalence
    - id: tenant-isolation
    - id: invalidation-deadline
    - id: peak-rss
      maximum_relative_to_baseline: 0.15

execution:
  warmups: 3
  rounds: 10
  order: deterministic-rotation
  timeout_per_round: 30s
  environment_policy: local-isolated-v1
```

The example defines the v1 field names relevant to the decision contract.
Schema elaboration may add required provenance fields that already follow
established Verdi conventions, but it may not rename, reinterpret, or weaken
the fields shown here without revising this design.

## Evaluator and observation protocol

An evaluator is a project-owned argument-vector command. Verdi never invokes
it through shell text. Registration records its executable identity,
configuration, and capabilities.

Before registration, Verdi invokes a `describe` operation that returns strict
`verdi.experiment-evaluator-capabilities/v1` JSON. The response declares:

- supported protocol versions;
- metric identifiers, primitive types, units, and directions;
- available correctness and safety guards;
- available observers;
- required workload inputs;
- environment dependencies; and
- whether any capability requires network or elevated access.

During execution, the same base-tree evaluator runs against each isolated
candidate root. It emits strict `verdi.experiment-observation/v1` records:

```json
{
  "schema": "verdi.experiment-observation/v1",
  "candidate": "facts-cache",
  "round": 4,
  "guards": [
    {
      "id": "behavioral-equivalence",
      "verdict": "pass",
      "witness": null
    }
  ],
  "measurements": [
    {
      "id": "request-latency",
      "value": 18.0,
      "unit": "ms",
      "source": "evaluator-measured"
    }
  ],
  "disclosures": []
}
```

Every measurement carries one trust classification:

- `harness-measured`: facts Verdi observes outside candidate control, such as
  process duration, exit status, timeout, and peak memory;
- `evaluator-measured`: facts produced by a locked project evaluator with a
  declared capability; or
- `candidate-reported`: internal counters such as cache hits, misses,
  evictions, or compute calls.

Only harness- and evaluator-measured values may determine eligibility or the
recommendation. Candidate-reported values are diagnostic unless a registered
independent observer corroborates them.

Raw observations are append-only and keyed by experiment digest, run
identity, candidate, and round. Normalized values preserve units and the
original witness needed to explain failures.

## Deterministic recommendation

The recommendation engine is part of Verdi's closed kernel. Given the same
locked definition and complete normalized observations, it must emit
byte-identical canonical JSON and the same decision.

It evaluates in this order:

1. Prove that the run is complete and matches the locked digests and
   environment policy.
2. Mark a candidate ineligible if any required correctness or safety guard
   fails.
3. Aggregate the primary metric using the registered function.
4. Apply every secondary resource or performance bound.
5. Require the registered practical improvement over the baseline.
6. Require the registered practical separation from the next eligible
   candidate.
7. Apply the registered variability rule.
8. Emit one proven winner only if all conditions hold; otherwise emit the
   precise violated or unproven reason.

There is no weighted score, dynamic metric selection, post-run threshold
editing, or automatic tie-breaker. Secondary guardrails cannot compensate for
a worse primary outcome, and a strong primary metric cannot compensate for a
failed correctness guard.

Raw performance values may vary because operating systems and runtimes are
not perfectly deterministic. High trust comes from a fixed schedule,
registered aggregation and variability rules, practical thresholds, and
honest inconclusive outcomes—not from pretending measurement noise does not
exist.

## Execution, isolation, and recovery

Each candidate runs in a disposable workspace derived from the registered
base commit. Verdi applies the captured patch, verifies the resulting
identity, and executes the common evaluator and workload under the registered
environment policy.

The schedule is deterministic and declared before execution. The default is
a fixed rotation across candidates rather than runtime randomization. Warmups
never enter the measured observation set.

The environment fingerprint includes at least:

- operating system and architecture;
- runtime and evaluator versions;
- CPU and memory allocation where controllable;
- relevant environment variables;
- workload and fixture digests;
- network policy; and
- the Verdi and recommendation-engine versions.

Network is disabled by default. An experiment that declares network access
may run only when policy permits and must disclose its weaker isolation in
the result.

Failures retain their meaning:

- A correctness or safety failure is a valid candidate verdict with a
  witness.
- A candidate crash or timeout is a candidate result if the harness and
  workload remained healthy.
- An evaluator crash, malformed response, protected-input mismatch, missing
  round, environment mismatch, or unavailable required isolation control
  invalidates the run and returns an operational error.
- Excessive variance, a practical tie, conflicting bounds, or no eligible
  candidate yields disclosed-unproven.

An interrupted execution may resume only missing observations. Existing
observations are immutable. The harness rejects a resume if any registered
input or environment requirement changed.

Reruns have separate identities even when they share an experiment revision.
Verdi shows all complete reruns and never selects the most favorable one
silently. A result may be called reproduced only under an explicit registered
reproduction rule.

## Human and agent interaction

The workbench, CLI, and agent-facing interface use one typed experiment
application core. Direct Markdown/Git editing remains a first-class draft
workflow: it writes the same canonical artifacts and passes through the same
strict validation and policy checks before registration can lock. It does not
receive a privileged lifecycle path merely because it bypasses the interactive
adapters.

The journey is:

```text
open question
    |
    v
spike created ------------------------------------------------ human intent
    |
    v
free prototype exploration --------------------------- human and/or agent
    |
    v
experiment draft + candidate registration ------------ human and/or agent
    |
    v
semantic registration review + lock ------------------ HUMAN REQUIRED
    |
    v
deterministic execution and recommendation ------------ Verdi
    |
    v
decision packet review and ratification --------------- HUMAN REQUIRED
    |
    v
existing spike answer and closure --------------------- existing lifecycle
    |
    v
fresh story-spec design and implementation ------------ normal build flow
```

An agent may:

- brainstorm candidate approaches;
- create and discard exploratory workspaces;
- draft the experiment definition;
- produce and register candidate patches;
- invoke a locked run;
- inspect observations;
- explain the recommendation; and
- propose the next action.

An agent may not:

- lock a registration on a human's behalf;
- alter locked inputs;
- ratify or reject a recommendation;
- resolve the linked open question;
- weaken project or organization policy;
- classify candidate-reported measurements as trusted; or
- promote prototype code into a product branch.

Normal agent context contains the refined spike, locked experiment,
applicable project decisions and policy, evaluator capabilities, and
resulting evidence. It does not include the complete exploratory transcript.
Optional supporting excerpts may jog memory but remain non-authoritative and
outside normal build context.

The two required human moments are consequential rather than ceremonial:

1. the registration lock confirms that the experiment represents a fair and
   useful decision; and
2. ratification confirms that the resulting recommendation should determine
   the feature's chosen path.

The normal pull-request approval remains the implementation governance gate.
Verdi adds no competing approval ceremony and does not recreate Codex,
Claude Code, or Verdi-go integrations.

## Ratification and existing spike closure

`ratification.yaml` records a human response to one immutable result. It may:

- select the recommended candidate;
- select a different candidate with an explicit reason;
- reject all candidates;
- declare that the desired outcome or experiment was misframed; or
- request a new experiment revision.

The record names the result digest, actor identity, selected disposition, and
reason where required. An adapter authenticates the actor; a payload cannot
self-declare human authority.

Ratification does not directly edit the parent feature or introduce a new
resolution edge. For a comparison-backed spike, the ratified decision becomes
the answer used by the existing spike acceptance and closure flow. The
spike's already-declared `resolves` edge remains the sole mechanism connecting
the answer to the open question. On an accepted feature, the computed
`resolved-by` backlink surfaces resolution without modifying the frozen spec.

The ratification record should be incorporated into the normal spike closure
review rather than requiring a redundant approval. A result without human
ratification can never close the question automatically.

Selection authorizes a future story spec to implement the chosen design. It
does not authorize copying, cherry-picking, or otherwise promoting the
prototype patch.

## Retention and reproducibility

All registered candidates retain:

- the canonical patch captured at lock;
- patch and base digests;
- normalized observations;
- guard verdicts and witnesses;
- evaluator, workload, fixture, environment, and recommendation-engine
  identities; and
- the recommendation and human ratification.

Rejected candidates do not retain bulky or executable material after
ratification:

- worktrees and temporary branches;
- containers;
- compiled binaries;
- profiles and traces;
- verbose logs; and
- transient caches.

The selected candidate receives one sealed capsule manifest describing the
complete retained reproduction set. The capsule contains or content-addresses
the registered patch, inputs, observations, identities, and required retained
artifacts. It is evidence of the selected design, not a product-source
delivery vehicle.

Cleanup runs only after the human decision is durably recorded. A cleanup
failure is operational and disclosed; it does not rewrite the decision.
Retention policy may keep less bulky diagnostic data, but it cannot remove
the minimal durable record or keep product-ready prototypes in a way that
bypasses fresh implementation.

## Extensibility

The architecture is a closed deterministic kernel with open typed adapters.

The kernel owns invariants that extensions cannot replace:

- common-base and candidate-digest verification;
- protected-path enforcement;
- correctness-before-optimization ordering;
- measurement-source trust classification;
- one primary metric and bounded secondary guards;
- practical-significance and variability evaluation;
- deterministic recommendation or inconclusive output;
- strict schemas and unknown-value failure;
- human-only lock and ratification; and
- immutable observations and run identities.

Extension protocols are independently versioned:

- `verdi.experiment/v1`;
- `verdi.experiment-evaluator-capabilities/v1`;
- `verdi.experiment-evaluator/v1`; and
- `verdi.experiment-observation/v1`.

The evaluator capability handshake allows projects to add domain-specific:

- correctness and safety guards with typed verdicts and witnesses;
- workload drivers;
- metrics composed from core primitive types;
- external observers;
- environment-fingerprint providers; and
- presentation or retention adapters that do not affect the decision.

Core metric primitives include duration, bytes, count, ratio, scalar, and
boolean, with explicit units and comparison direction. The initial closed
aggregation vocabulary includes operations such as p50, p95, maximum, mean,
and rate. A new primitive, aggregation, or trust class requires an explicit
protocol revision rather than arbitrary YAML.

Future adapters could wrap Go benchmarks, JMH, pytest-based measurement, or
telemetry collectors without teaching the kernel about caches, JVMs, Python,
or a particular observability vendor. Such adapters are examples of the
extension boundary, not v1 commitments.

Project configuration selects permitted evaluators and capabilities.
Organization policy may narrow project choices; a lower layer cannot weaken a
higher policy. Verdi exposes capability inspection so humans and agents can
draft valid experiments without guessing.

There are no in-process Go plugins, imported evaluator packages, untyped
hooks, or shell command strings. Adapters cannot ratify, resolve questions,
change recommendation semantics, or promote their own measurements to a
higher trust class.

V1 ships only the generic command evaluator and built-in process observer.
Specialized adapters arrive later without changing experiment lifecycle or
stored evidence.

## Policy, abuse resistance, and honest limits

Policy may constrain:

- allowed candidate and experiment paths;
- permitted evaluator executables and protocol versions;
- network, filesystem, process, CPU, memory, and timeout access;
- maximum observation and retained-artifact sizes;
- trusted measurement sources; and
- mandatory guards for named experiment classes.

Verdi runs evaluators as argument vectors, size-limits responses,
strict-decodes results, and refuses undeclared capabilities. Candidates cannot
modify protected comparison inputs. Mutation provenance records the actor,
operation, prior digest, resulting digest, and policy decision across every
surface.

Decision gaming is constrained by immutable registration, correctness-first
eligibility, one primary metric, bounded guardrails, visible failed
candidates, separate rerun identities, and versioned recommendation logic.

A prototype may still overfit a visible workload. The result therefore
applies only to the registered contract, candidates, fixtures, workload, and
environment. Candidate diffs remain reviewable for workload-specific
branching or suspicious behavior. A project may register several
deterministic workload profiles for broader confidence.

Hidden benchmark corpora, adversarial trial design, model blinding, and
benchmark anti-tamper machinery belong to Verdi-bench. This harness offers
high-trust technical decision evidence, not a claim of adversarially secure or
statistically universal proof.

## Caching example

Suppose a request currently follows:

```text
request -> load account facts -> evaluate policy -> render quote
```

The spike asks where caching should occur. Three candidates share the same
base:

- **baseline:** no cache;
- **final-cache:** cache the rendered quote; and
- **facts-cache:** cache account facts, then reevaluate policy and render for
  every request.

The common workload contains a deterministic request mix:

- 70 percent hot-account access;
- 20 percent warm-account access;
- 10 percent cold-account access;
- fixed concurrency;
- injected account updates; and
- injected policy updates.

The decision contract is:

- all outputs remain byte-identical to the uncached reference behavior;
- tenant isolation always holds;
- no response remains stale beyond the invalidation deadline;
- the primary p95 request latency improves by at least 25 percent over the
  baseline;
- peak RSS increases by no more than 15 percent; and
- miss-path p95 latency regresses by no more than 5 percent.

An illustrative normalized result is:

| Candidate | p95 latency | Peak RSS | Correctness | Eligibility |
|---|---:|---:|---|---|
| baseline | 40 ms | 100 MiB | pass | reference |
| final-cache | 12 ms | 108 MiB | stale after policy update | ineligible |
| facts-cache | 18 ms | 109 MiB | pass | eligible |

The final-cache candidate is fastest but fails the accepted behavior. The
facts-cache candidate improves the primary outcome by 55 percent, remains
inside the resource guard, and is therefore the proven winner among the
registered candidates under this experiment.

The decision packet recommends facts-cache and preserves the final-cache
staleness witness. A human ratifies or rejects that recommendation. If
ratified, the later story implements facts-cache afresh; it does not promote
the prototype.

The harness measures exit status, duration, timeout, and peak RSS. The locked
evaluator measures behavioral equivalence and staleness. Candidate-reported
hit, miss, eviction, and compute-call counters remain diagnostic.

## Verification strategy

Unit tests are table-driven and cover:

- candidate eligibility and guard failures;
- higher- and lower-is-better primary metrics;
- absolute and relative significance thresholds;
- practical ties and excessive variance;
- incomplete and invalid runs;
- baseline failures;
- conflicting guardrails;
- no eligible candidate;
- deterministic result and recommendation rendering; and
- required negative paths for every decision function.

Schema tests cover every versioned artifact and protocol:

- happy-path decoding;
- unknown fields and enum values;
- unsupported versions;
- malformed units and primitive types;
- duplicate candidates, metrics, guards, and observations;
- trailing JSON or YAML data;
- forbidden YAML dialect features; and
- canonical deterministic output.

Hermetic integration tests use fake evaluators, canned strict JSON, fixture
Git repositories with stable SHAs, and disposable workspaces. They prove:

- every candidate starts from the same base;
- added and binary files are captured;
- protected paths cannot be registered;
- evaluator and workload identities are verified;
- candidates cannot counterfeit harness measurements;
- a registered isolation requirement fails operationally when the platform
  cannot enforce it;
- candidate, evaluator, and operational failures remain distinct;
- interruption resumes only missing observations;
- completed observations cannot be overwritten;
- reruns remain separately visible; and
- cleanup preserves the minimal record while removing rejected bulky
  artifacts.

CLI end-to-end tests drive the built Verdi binary through registration, lock,
execution, recommendation, and ratification. Playwright tests cover the same
behavioral path in the workbench, including the two human review surfaces and
all invalid or inconclusive states. Agent-facing contract tests prove that the
same typed operations and policy decisions are exposed without a privileged
mutation path.

A committed deterministic caching fixture demonstrates that a faster
incorrect candidate loses to a slower correct candidate. No test uses the
network. The full `make verify` and `go test -race ./...` gates remain the
completion authority.

## Rollout

V1 is a narrow vertical slice:

1. Add versioned experiment, capability, observation, result, recommendation,
   and ratification artifacts.
2. Add the closed decision engine and strict artifact seams.
3. Add the generic command evaluator and built-in process observer.
4. Add isolated candidate execution, resume, and retention behavior.
5. Expose the same typed operations through existing CLI, workbench, and
   agent interfaces.
6. Add project policy for paths, commands, resources, and trusted sources.
7. Dogfood the caching comparison or another genuine unresolved Verdi design
   choice and record the process in the chronicle.

The system separates invariant rigor from evidence depth:

- **Standard comparison** uses one representative workload profile, modest
  fixed repetitions, process observation, and a practical-significance
  threshold.
- **Elevated assurance** adds registered workload profiles or environments,
  more repetitions, tighter variability requirements, or an explicit
  reproduction rule.

Both profiles use identical trust, lock, eligibility, recommendation, and
human-ratification semantics. Elevated assurance gathers more evidence; it
does not weaken or replace the standard decision contract.

Reusable experiment templates and evaluator capability discovery reduce the
main expected friction: constructing a fair workload and evaluator. Verdi may
populate mechanical defaults, but a human must still confirm what desired
outcome the experiment is intended to prove.

## Non-goals

This design does not:

- automatically choose or ratify a technical design;
- automatically resolve an open question;
- promote prototype code into production;
- require comparative experiments for every spike;
- evaluate choices whose desired outcome cannot be represented honestly;
- benchmark agents, models, prompts, or engineering harnesses;
- conduct human-subject or UX preference research;
- provide a long-running production experimentation or feature-flag system;
- produce a weighted magic score;
- claim universal superiority beyond registered candidates and conditions;
- add a second Verdi-go integration or competing MCP server;
- recreate Codex, Claude Code, or an embedded chat product; or
- replace ordinary story specs, implementation evidence, review, or PR
  approval.

## Decisions and rejected alternatives

### Accepted decisions

- Use an immutable child experiment under an existing spike.
- Keep free exploration outside the evidence set until a human locks the
  registration.
- Capture every candidate's diff and digest at registration.
- Require a shared behavior contract, workload, primary metric, guardrails,
  execution schedule, and recommendation rule.
- Use one primary metric and bounded secondary guards.
- Recommend only when the evidence proves a materially separated winner.
- Require human ratification for the chosen path.
- Retain all registered patches and normalized evidence, but keep bulky
  artifacts only for the selected candidate's sealed capsule.
- Reimplement the selected design under a normal story spec.
- Extend through strict command protocols and primitive metric types.
- Keep Verdi-bench as the owner of agent and benchmark-grade evaluation.

### Rejected alternatives

**Embed experiment state directly in the spike spec.** This would mix a
mutable measurement lifecycle into the question container, complicate frozen
artifact semantics, and make repeated revisions awkward.

**Store the experiment only in an external benchmark system.** This would
separate the decision evidence from the open question and make Verdi unable to
derive a coherent feature-design journey.

**Automatically select the highest-scoring candidate.** A score cannot prove
that the registered desired outcome is the right product decision. It would
also permit metric weighting and post-hoc interpretation to replace human
ratification.

**Promote the winning prototype.** Prototype code is optimized for learning,
not for satisfying the full story implementation and evidence contract.
Promotion would create a bypass around design, TDD, review, and provenance.

**Adopt benchmark-grade statistics and hidden holdouts in v1.** That would
duplicate Verdi-bench and impose process cost beyond the intended lightweight
technical decision. Comparative spikes instead use fixed protocols,
practical thresholds, scoped proof claims, and honest inconclusive outcomes.

**Create per-tool agent integrations.** Separate Codex, Claude Code,
workbench, and CLI behavior would drift. One typed mutation and experiment
core preserves identical semantics across surfaces.
