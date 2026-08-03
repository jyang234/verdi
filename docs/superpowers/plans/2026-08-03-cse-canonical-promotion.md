# CSE Canonical-Promotion Implementation Plan

> **For agentic workers:** This is a **planning-only packet** for the Wave 0
> delivery unit "Promote CSE into a canonical feature proposal artifact." It
> authors no runtime code, no schemas, and no canonical spec bytes itself. The
> later implementation unit that executes §9's tasks MUST use
> `superpowers:subagent-driven-development` task by task, after this plan has
> received its read-only Codex plan review. Steps use checkbox (`- [ ]`)
> syntax for tracking.

**Goal:** Map every reviewed decision of the Comparative Spike Experiments
(CSE) design into one canonical Verdi feature proposal artifact —
losslessly, without adding new semantics — and specify exactly how the
future implementation unit authors, validates, and lands that artifact under
the merge-signaled acceptance lifecycle.

**Architecture:** The CSE design document
(`docs/superpowers/specs/2026-07-30-comparative-spike-experiments-design.md`,
ratified design authority) is transformed into a statusless
`.verdi/specs/active/comparative-spike-experiments/spec.md` feature artifact
following the exact structural precedent of `spec/context-integrity` and
`spec/guided-lifecycle-governance`: YAML frontmatter objects (problem,
outcome, acceptance criteria, stubs, links, decisions, constraints, open
questions) plus a body whose sections carry the design's full normative
detail. Every source statement maps to a named destination through the
losslessness table in §5; ambiguities are carried as declared open questions
or disclosures, never silently resolved.

**Tech Stack:** Verdi filesystem store (`.verdi/specs/active/`), strict YAML
frontmatter under `internal/artifact` (`verdi.artifact/v1` conventions), Git
worktrees, GitHub draft PRs with the stable `merge-gate` check,
`verdi lint` / `make lint-store` / `internal/specalign` as validation gates.
The stable `merge-gate` check runs on every PR and is an active
ruleset-required status check (verified live against ruleset `19021982`;
§10 G-6).

## Global Constraints

- Binding semantics remain `docs/design/specs/00..05-*.md`;
  `docs/design/specs/08-revision-notes.md` is the ratification history.
- The CSE design is ratified design authority, **not** canonical lifecycle
  authority until this unit's promotion artifact merges (design header, L4;
  orchestration index §Specification index).
- Promotion must "preserve their reviewed decisions, constraints, non-goals,
  and rollout ordering in canonical artifact form" and add **no new
  semantics** (orchestration index, Specification index note and Wave 0).
- The canonical artifact is statusless: proposed-versus-accepted state is
  Git-derived under merge-signaled acceptance; no `status:` field, no
  `frozen:` stamp, no acceptance command (merge-signals design §Artifact
  compatibility; precedent commit `ae183b15`).
- Never edit frozen artifacts, binding specs, the orchestration index, shared
  registries, workflows, or `.verdi` store contents in the planning lane.
- Three-valued honesty: every claim below is proven with a witness, violated
  with a witness, or disclosed as unproven. Silence is never a pass.
- Exit contract `0` clean / `1` verdict / `2` operational is preserved
  verbatim in the promoted text.
- Only the human owner merges; Claude Code never accepts, approves, or marks
  ready. Codex plan review precedes the implementation unit (Gate P).

---

## Contents

1. [Authority and provenance](#authority-and-provenance)
2. [Source inventory method](#source-inventory-method)
3. [Canonical artifact path and identity](#canonical-artifact-path-and-identity)
4. [Proposed canonical structure](#proposed-canonical-structure)
5. [Losslessness table](#losslessness-table)
6. [Verdict and failure-state separation](#verdict-and-failure-state-separation)
7. [Shared worktree, isolation, and capability boundary](#shared-worktree-isolation-and-capability-boundary)
8. [Human-ceremony inventory](#human-ceremony-inventory)
9. [Implementation tasks for the promotion unit](#implementation-tasks-for-the-promotion-unit)
10. [Three-valued disclosures and blocking authority gaps](#three-valued-disclosures-and-blocking-authority-gaps)
11. [Experimental evidence and CI authoritative proof](#experimental-evidence-and-ci-authoritative-proof)

## Authority and provenance

Accepted landing commit (origin/main at plan time):
`6d71fd7d33beaf8128fa675833ee12595205481d` — the merge of PR #258, which
ratified the CSE design as design authority and accepted the two canonical
proposals under merge-signaled acceptance.

Verified authority blobs at that commit (witness: `git rev-parse
HEAD:<path>` from the plan worktree):

| Document | Path | Blob OID |
|---|---|---|
| CSE design (the promotion source) | `docs/superpowers/specs/2026-07-30-comparative-spike-experiments-design.md` | `c28bdf43d948aafa186568bedade3da51930bb78` |
| Context Integrity (structural exemplar + boundary counterpart) | `.verdi/specs/active/context-integrity/spec.md` | `18a2b92b7c92b5ba807336a13cb38c9e0d9a406c` |
| Guided Lifecycle (structural exemplar) | `.verdi/specs/active/guided-lifecycle-governance/spec.md` | `c347668014d26f987d38fbd9dca0082228238694` |

Additional authority read in full for this plan: the four-feature
orchestration index (`docs/superpowers/plans/2026-08-01-four-feature-orchestration.md`),
the merge-signaled acceptance design
(`docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md`),
binding specs `00..05` and `08-revision-notes.md` (workspace
`docs/design/specs/`), the workspace `AGENTS.md`, and the repository
`CLAUDE.md`. The repository itself tracks **no** `AGENTS.md` (disclosure in
§10).

Line numbers throughout this plan cite the CSE design blob above ("L*n*").

## Source inventory method

An exhaustive extraction pass over the CSE design produced a numbered
inventory of **174 semantically distinct items**, each anchored to its
section heading and line range and classified as one of: decision,
acceptance-criterion, constraint, schema-element, lifecycle-rule,
evaluator-rule, observer-rule, recommendation-rule, rollout-step, non-goal,
adapter-behavior, proof-boundary, open-question, dependency, or ceremony.
FABLE independently read the full source and adjudicated the inventory.
Seven ambiguities were flagged (AMB-1..AMB-7, dispositioned in §10); none is
resolved silently in the proposed canonical text.

The losslessness table in §5 is the durable form of that inventory: it maps
every source block (covering all 174 items; coverage 174/174) to a canonical
destination with its transformation named. The item-numbered inventory
itself is session working evidence — this unit deliberately creates exactly
one file, so it is not committed; the committed, auditable witness is §5's
line-anchored table, independently re-walked by Opus verification and
mechanically re-verified by the implementation unit (§9 Task 3). Any future
edit to the promotion content must keep that table exhaustive.

## Canonical artifact path and identity

**Proposed path:** `.verdi/specs/active/comparative-spike-experiments/spec.md`

**Proposed identity (frontmatter kernel):**

```yaml
id: spec/comparative-spike-experiments
kind: spec
title: "Comparative Spike Experiments"
owners: [platform-team]
class: feature
```

Justification against repository conventions:

- **Directory-per-spec, single `spec.md`.** Every artifact in
  `.verdi/specs/active/` is one directory holding one `spec.md`
  (store-layout spec §Directory layout; observed for all ten existing spec
  directories).
- **Kebab-case name = directory name = `name` half of the ref**
  (`internal/artifact` `simpleNameRe`; artifact-contract §Identity and
  references). `comparative-spike-experiments` mirrors the design document's
  own title, satisfies the name grammar, and collides with nothing in the
  store (no CSE artifact exists yet — verified by directory listing).
- **`class: feature`** matches the exemplars: CSE, like CI and GLG, is a
  feature-class proposal decomposed into stubs that later become story
  specs. The design's experiment artifacts themselves (`experiment.yaml`
  etc.) are *runtime store content of the future feature*, not spec-kind
  artifacts, so no new kind is introduced.
- **`owners: [platform-team]`** matches both exemplars; ownership of the
  CSE lane per the orchestration index rests with the same platform team.
- **Statusless.** No `status:` field and no `frozen:` stamp: merge-signaled
  acceptance derives proposed/accepted from Git reachability
  (merge-signals design §Authority and derived state). Precedent: commit
  `ae183b15` ("Normalize four feature proposals for merge acceptance")
  removed exactly `status: draft` from the two exemplars, adding nothing.
- **Accepted-baseline identity** after the promotion PR merges will be the
  deterministic Git triple (spec path, blob OID at landing, first-parent
  landing commit) — no content-changing acceptance stamp.

## Proposed canonical structure

The complete proposed frontmatter follows. Object `text` values are the
promotion's condensed normative statements; each is anchored to a body
section that carries the source's full detail (the losslessness table binds
each to its source lines). The implementation unit authors exactly this
structure; wording-level polish during authoring is permitted only where the
losslessness table's transformation column already names the source it must
preserve.

```yaml
---
id: spec/comparative-spike-experiments
kind: spec
title: "Comparative Spike Experiments"
owners: [platform-team]
class: feature
problem:
  text: "Verdi can freeze specs and judge evidence, but when several viable technical approaches could implement one feature it has no immutable, executable way to compare them: informal local benchmarks are unregistered and unauditable, prototype results can be cherry-picked or silently overfitted, correctness and performance claims blur together, and nothing prevents prototype code or untrusted self-reported numbers from steering the accepted design without a human-ratified, evidence-bounded decision record."
  anchor: problem
outcome:
  text: "an existing question-resolving spike can carry immutable child experiments that freeze a fair comparison before measurements count, evaluate every candidate against one behavioral contract under registered workloads and guards, distinguish candidate verdicts from operational failures, deterministically recommend a materially separated winner or disclose an honest inconclusive result, and preserve the evidence and human ratification that justify the spike's answer — while prototype code stays disposable and the selected design is reimplemented afresh under a normal story spec."
  anchor: outcome
acceptance_criteria:
  - id: ac-1
    text: "a spike owner can explore candidate prototypes freely in disposable workspaces, then register an immutable child experiment — common base commit, canonical candidate patches with digests, behavioral contract, evaluator identity and capabilities, workload and fixture identities, guards, one primary metric with thresholds, execution schedule, environment policy, recommendation-algorithm version, and retention policy — that only a human can lock; after lock no registered input can change, and any change becomes a new child experiment revision"
    evidence: [static, behavioral, attestation]
    anchor: ac-1
  - id: ac-2
    text: "given one locked definition and complete valid observations, the closed recommendation engine deterministically emits byte-identical canonical results expressing exactly one of proven-winner, violated-with-witness, or disclosed-unproven in the registered evaluation order, with no weighted score, dynamic metric selection, post-run threshold editing, or automatic tie-breaker, and the experiment verb follows Verdi's exit contract of 0 proven, 1 unproven verdict, 2 operational failure"
    evidence: [static, behavioral, attestation]
    anchor: ac-2
  - id: ac-3
    text: "project evaluators participate only through versioned strict protocols — a capability describe handshake, argument-vector invocation with no shell strings, and strict observation records — and every measurement carries exactly one trust classification in which only harness-measured and evaluator-measured values may determine eligibility or the recommendation, while candidate-reported values remain diagnostic unless a registered independent observer corroborates them, the effect of that corroboration being unresolved (oq-2)"
    evidence: [static, behavioral, attestation]
    anchor: ac-3
  - id: ac-4
    text: "every candidate runs in a disposable isolated workspace derived from the registered base commit under the registered environment policy and deterministic schedule with network disabled by default; an interrupted run resumes only missing observations against unchanged inputs; reruns keep separate, always-visible identities; and after ratification the selected candidate retains one sealed capsule manifest while rejected candidates' bulky artifacts are cleaned up without ever removing the minimal durable record"
    evidence: [static, behavioral, attestation]
    anchor: ac-4
  - id: ac-5
    text: "CLI, workbench, and agent surfaces expose the same typed experiment operations over one application core with no privileged mutation path; a human ratification record against the immutable result is the only path to a chosen design; and the ratified answer flows into the existing spike acceptance and closure lifecycle without any new spec kind, edge vocabulary, evidence kind, or closure path"
    evidence: [static, behavioral, attestation]
    anchor: ac-5
  - id: ac-6
    text: "project and organization policy constrain experiment paths, permitted evaluators and protocol versions, capabilities, resources, observation sizes, and trusted measurement sources, with a lower layer unable to weaken a higher one; candidates cannot modify protected comparison inputs; and mutation provenance records actor — an attribution record, never an authority decision — operation, prior digest, resulting digest, and policy decision across every experiment mutation surface: CLI, workbench, agent, and the direct Git-edit draft path"
    evidence: [static, behavioral, attestation]
    anchor: ac-6
  - id: ac-7
    text: "one genuine unresolved Verdi design choice completes the full journey — exploration, registration, human lock, deterministic execution, a scoped recommendation or honest inconclusive result, human ratification, and existing spike closure — with the chronicle able to identify the human effort, failures, and decision confidence without relying on an informal transcript"
    evidence: [behavioral, attestation]
    anchor: ac-7
stubs:
  - { slug: experiment-schemas, acceptance_criteria: [ac-1] }
  - { slug: decision-engine, acceptance_criteria: [ac-2] }
  - { slug: evaluator-observer, acceptance_criteria: [ac-3] }
  - { slug: isolated-execution, acceptance_criteria: [ac-4] }
  - { slug: experiment-adapters, acceptance_criteria: [ac-5] }
  - { slug: experiment-workbench, acceptance_criteria: [ac-5] }
  - { slug: experiment-policy, acceptance_criteria: [ac-6] }
  - { slug: dogfood-comparison, acceptance_criteria: [ac-7] }
links:
  - { type: depends-on, ref: spec/verdi-artifact-contract }
  - { type: depends-on, ref: spec/verdi-evidence-model }
  - { type: depends-on, ref: spec/worktree-manager }
decisions:
  - id: dc-1
    text: "each locked comparison is an immutable child experiment under an existing spike — subordinate evidence used to choose and justify the spike's answer, never a new spec kind, edge vocabulary, evidence kind, or alternate closure path"
    anchor: dc-1
  - id: dc-2
    text: "experiment state is derived from the presence and validity of artifacts — exploratory, registered, measured, recommended, inconclusive, ratified — with no second lifecycle status; a locked revision is never edited, and any changed candidate, metric, threshold, workload, evaluator, or environment becomes a new child experiment"
    anchor: dc-2
  - id: dc-3
    text: "work before registration is explicitly exploratory and can never be copied into the experiment as decision evidence; evidence counts only after a human locks the registered question, candidates, workload, metrics, constraints, environment, and decision rule"
    anchor: dc-3
  - id: dc-4
    text: "one preregistered primary metric determines the ranking; secondary metrics are bounded guardrails that can never compensate for a worse primary outcome, and no weighted or composite score exists"
    anchor: dc-4
  - id: dc-5
    text: "correctness precedes optimization: a candidate that violates the common behavioral or safety contract is ineligible regardless of its performance, and a strong primary metric cannot compensate for a failed correctness guard"
    anchor: dc-5
  - id: dc-6
    text: "Verdi makes the execution schedule, aggregation, thresholds, and recommendation mechanical over disclosed noise; it does not claim raw performance observations are mathematically repeatable, and high trust comes from fixed protocol plus honest inconclusive outcomes"
    anchor: dc-6
  - id: dc-7
    text: "Verdi recommends and never decides: locking a registration and ratifying a result are human-only acts, and no result without human ratification can close the linked question"
    anchor: dc-7
  - id: dc-8
    text: "prototype code is disposable and reasoning is durable: registered candidate diffs, normalized observations, guard witnesses, identities, the recommendation, and the human decision are retained; rejected candidates' bulky artifacts are not, and the selected candidate's complete reproduction set is sealed in one capsule manifest"
    anchor: dc-8
  - id: dc-9
    text: "a ratified selection authorizes a future story spec to implement the chosen design afresh under the ordinary build, verification, and pull-request process; it never authorizes copying, cherry-picking, or promoting the prototype patch"
    anchor: dc-9
  - id: dc-10
    text: "the architecture is a closed deterministic kernel with open typed adapters: provenance, trust, and decision invariants are not configurable, extension happens only through independently versioned strict command protocols, and there are no in-process plugins, imported evaluator packages, untyped hooks, or shell command strings"
    anchor: dc-10
  - id: dc-11
    text: "there is no privileged agent path: workbench, CLI, and agent surfaces invoke one typed experiment application core, and direct Markdown/Git edits pass the same strict validators and policy checks before they can change lifecycle state"
    anchor: dc-11
  - id: dc-12
    text: "every measurement carries one trust classification — harness-measured, evaluator-measured, or candidate-reported — and only the first two may determine eligibility or the recommendation; candidate-reported values are diagnostic unless a registered independent observer corroborates them, and what corroboration changes is deliberately left open (oq-2)"
    anchor: dc-12
  - id: dc-13
    text: "the execution schedule is deterministic and declared before execution — fixed rotation by default, warmups excluded from the measured set — and the environment fingerprint records at least OS and architecture, runtime and evaluator versions, CPU and memory allocation where controllable, relevant environment variables, workload and fixture digests, network policy, and the Verdi and recommendation-engine versions"
    anchor: dc-13
  - id: dc-14
    text: "failures retain their meaning: a correctness or safety failure is a valid candidate verdict with a witness; a candidate crash or timeout is a candidate result when the harness and workload stayed healthy; an evaluator crash, malformed response, protected-input mismatch, missing round, environment mismatch, or unavailable required isolation control invalidates the run as an operational error; excessive variance, ties, conflicting bounds, or no eligible candidate yield disclosed-unproven"
    anchor: dc-14
  - id: dc-15
    text: "an interrupted execution resumes only missing observations against unchanged registered inputs; existing observations are immutable; reruns have separate identities that are all shown, never silently filtered to the most favorable; and a result may be called reproduced only under an explicit registered reproduction rule"
    anchor: dc-15
  - id: dc-16
    text: "ratification is a human response to one immutable result — select the recommendation, select another candidate with reason, reject all, declare the framing wrong, or request a new revision — authenticated by an adapter, never self-declared by a payload, with the ratification actor resolving to an authenticated principal through the shared governance-principal kernel and unproven authentication blocking ratification, and incorporated into the normal spike closure review rather than a redundant approval"
    anchor: dc-16
  - id: dc-17
    text: "experiment artifacts and captured patches live inside the spike's own spec directory as VL-016 already permits; a registered patch may describe ephemeral product-source changes applied only inside an isolated candidate workspace, and those changes are never committed on the spike branch, so VL-016 continues to fence the durable spike diff"
    anchor: dc-17
  - id: dc-18
    text: "Verdi-bench remains the owner of agent, model, prompt, and harness benchmarking with hidden corpora and anti-tamper machinery; comparative experiments compare technical implementations of the same feature behavior and claim high-trust decision evidence, never adversarially secure or statistically universal proof"
    anchor: dc-18
  - id: dc-19
    text: "standard comparison and elevated assurance are evidence-depth profiles over identical trust, lock, eligibility, recommendation, and human-ratification semantics; elevated assurance gathers more evidence and can never weaken or replace the standard decision contract"
    anchor: dc-19
constraints:
  - id: co-1
    text: "every experiment result preserves three-valued honesty — proven with witnesses, violated with a preserved witness, or explicitly disclosed as unproven; missing or ambiguous evidence is never a pass, and an operational failure is never converted into a decision verdict"
    anchor: co-1
  - id: co-2
    text: "every experiment artifact and protocol strict-decodes through versioned schemas in which unknown fields, enum values, metric types, aggregations, trust classes, and versions fail closed; schema elaboration may add provenance fields following established Verdi conventions but may not rename, reinterpret, or weaken the ratified fields"
    anchor: co-2
  - id: co-3
    text: "canonical experiment outputs are deterministic over declared inputs — sorted, canonical-JSON encoded, digest-bound, append-only where observational — and the recommendation engine emits byte-identical output and the same decision for the same locked definition and complete normalized observations"
    anchor: co-3
  - id: co-4
    text: "the experiment verb exits 0 for a completed comparison with a proven winner, 1 for a completed comparison whose verdict is unproven, and 2 for an operational failure, preserving Verdi's existing exit contract"
    anchor: co-4
  - id: co-5
    text: "a proven result claims only its registered boundary — best demonstrated path among the registered candidates for the registered outcome, workload, environment, and comparison revision — and never universal superiority over unregistered designs or unrepresented production conditions"
    anchor: co-5
  - id: co-6
    text: "network is disabled by default during execution; an experiment declaring network access runs only where policy permits and must disclose its weaker isolation in the result; a registered isolation requirement fails operationally when the platform cannot enforce it"
    anchor: co-6
  - id: co-7
    text: "every delivery story carries table-driven happy and negative decision-function coverage, strict-decode and canonical-output schema tests for every versioned artifact, hermetic integration tests with fake evaluators and fixture Git repositories, built-binary CLI end-to-end coverage, Playwright coverage for the two human review surfaces and invalid or inconclusive states, agent-facing contract tests proving no privileged mutation path, a committed deterministic caching fixture in which a faster incorrect candidate loses to a slower correct one, no network in any test, and clean make verify plus go test -race evidence"
    anchor: co-7
open_questions:
  - id: oq-1
    text: "the extensibility section versions a verdi.experiment-evaluator/v1 protocol distinct from the capabilities and observation protocols, but no section of the ratified design defines its scope; the schemas story must propose its content (or its removal) through the ratified amendment flow before freezing the protocol set"
    anchor: oq-1
  - id: oq-2
    text: "the observation protocol says candidate-reported values are diagnostic unless a registered independent observer corroborates them, while the trust model names no corroborated class and forbids adapters promoting measurements to a higher trust class; whether corroboration ever makes candidate-reported values decision-eligible, and under what recorded identity, is unresolved"
    anchor: oq-2
---
```

**Proposed body outline** (each heading resolves an anchor above; content
per the losslessness table in §5):

| Body section | Carries |
|---|---|
| `# Comparative Spike Experiments` | title |
| `## Problem` | Purpose L30-51 + Current-state grounding L53-78 (spike semantics preserved, Verdi-bench/chronicle boundaries) |
| `## Outcome` | Scope and success criteria L80-106 (supported alternative classes, nine-step successful journey, v1 success test) |
| `## Experiment journey` | the nine-stage actor-annotated journey diagram L493-519, carried as mermaid |
| `## Decision-proof model` | L140-193: the immutable decision contract's seven elements, the decision sequence, the three-valued result vocabulary, the operational-failure exclusion, the exit contract, the scoped-claim boundary statement |
| `## AC-1` … `## AC-7` | full normative detail per AC: AC-1 carries L195-341 (the experiment directory tree, the derived-state table, the VL-016 residence posture, exploration rule, the 12-element registration capture list, protected paths, the semantic review packet and lock, the complete `experiment.yaml` v1 example with its field-stability rule); AC-2 carries L404-434 (the eight-step evaluation order, no-tiebreaker rules, noise posture); AC-3 carries L343-402 plus the metric-primitive and aggregation vocabulary L647-660 (describe handshake declaration list, observation record example, three trust classes, append-only keying; the primitive/aggregation vocabulary shared with `## DC-10`); AC-4 carries L436-480 + L589-621 (workspace derivation, schedule, environment fingerprint list, network default, failure taxonomy, resume, rerun identity, retention lists, capsule manifest, cleanup ordering); AC-5 carries L482-558 + L560-587 (one typed core, agent may/may-not lists, normal agent context rule, the two human moments, ratification record semantics, spike-closure integration, no promotion); AC-6 carries L667-670 + L681-711 (project/organization policy layering, policy constraint list, abuse-resistance mechanics, overfit disclosure, Verdi-bench boundary); AC-7 carries L104-106 + L837-838 (dogfood success test) |
| `## Caching example` | L713-768 carried verbatim as the illustrative worked example (candidates, workload mix, decision contract, normalized-result table, witness preservation, measurement attribution) |
| `## Delivery sequence` | Rollout steps 1-7 (L825-856) mapped onto the stubs in the design's own step order, with the step-5 surface split into its CLI/agent and workbench halves: experiment-schemas → decision-engine → evaluator-observer → isolated-execution → experiment-adapters → experiment-workbench (its UI unit runs serialized in orchestration Wave 6) → experiment-policy → dogfood-comparison; plus the standard/elevated assurance profiles and the explicit v1 boundary (generic command evaluator and built-in process observer only, L662-679) |
| `## Relationship to Context Integrity` | the shared worktree/isolation/capability boundary dependency, stated per §7 of this plan — named, not ratified; plus the packet-consumed actor rule and experiment-surface scope pin (§5 row 69) and the CX-11 context-boundary restatement (§5 row 41), each carried only as owner-adjudicated |
| `## Relationship to Verdi-bench and the chronicle` | L48-51, L74-78, L708-711: ownership boundaries |
| `## DC-1` … `## DC-19` | rationale prose per decision, carrying the source's own reasoning including the six rejected alternatives (L896-922), each attached to the DC it motivates (dc-1 ← embed-in-spike and external-system rejections; dc-4/dc-7 ← auto-select rejection; dc-9 ← promote-prototype rejection; dc-18 ← benchmark-grade rejection; dc-11 ← per-tool-integration rejection) |
| `## CO-1` … `## CO-7` | constraint elaborations, with CO-7 carrying the full verification strategy L770-823 |
| `## OQ-1`, `## OQ-2` | the two carried ambiguities with their source anchors |
| `## Non-goals` | the 13 explicit non-goals L858-875, carried verbatim as a list |

## Losslessness table

Legend for the Transformation column — **verbatim**: text carried
byte-near-identically (lists, examples, diagrams); **condensed**: normative
content restated in frontmatter object text with full detail retained in the
named body section; **restructured**: same content, relocated to the
canonical section shape; **cross-ref**: satisfied by citing existing binding
authority rather than restating it (no new bytes of semantics);
**oq-carry**: preserved as a declared open question, unresolved;
**plan-only**: consumed by this plan, not carried into the artifact (with
reason). Preservation evidence cites the source inventory item numbers
(1-174) plus the verification the implementation unit runs (§9 Task 3).

| # | Source anchor (section, lines) | Content | Canonical destination | Transformation | Preservation evidence (inventory items) |
|---|---|---|---|---|---|
| 1 | Header, L1-4 | title, date, dual-authority status line | artifact `title`; status line superseded by promotion itself | plan-only (the status line describes the pre-promotion state; merge of this unit's PR ends it) | 1; AMB-2 disposition §10 |
| 2 | Purpose, L30-41 | the feature-design question; immutable executable comparison; preregistered outcome; scoped recommendation or honest inconclusive | `problem`/`outcome` attributes; `## Problem`, `## Outcome` | condensed | 2-3 |
| 3 | Purpose, L43-46 | Verdi recommends, human ratifies; disposable prototypes; fresh reimplementation | `dc-7`, `dc-9`; `## Problem` | condensed | 4 |
| 4 | Purpose, L48-51 | lightweight high-trust mechanism; not Verdi-bench-grade | `dc-18`; `## Relationship to Verdi-bench and the chronicle` | condensed | 5 |
| 5 | Current-state grounding, L53-69 | parent spike lifecycle facts: `spike: true`, `resolves` edges, no `implements`, draft-or-accepted attachment, evidence-model exemption, VL-016 fence, answer-as-deliverable, frozen-feature `resolved-by` backlink | `## Problem` (grounding paragraph) | cross-ref (binding authority: artifact-contract §Kind registry, evidence-model §Ceremony pricing, VL-016) | 6-13 |
| 6 | Current-state grounding, L70-72 | experiments are subordinate evidence, not a new kind/edge/evidence/closure path | `dc-1`; `ac-5` | condensed | 14 |
| 7 | Current-state grounding, L74-78 | Verdi-bench owns agent-stack comparison; chronicle reports ergonomics, never decides winners | `dc-18`; `## Relationship to Verdi-bench and the chronicle` | condensed | 15 |
| 8 | Scope, L82-90 | v1 scope constraint — alternatives exercisable against one common behavioral contract, compared using machine-produced observations (L82-84) — plus the supported alternative classes (caching, storage/indexing/invalidation, algorithms, protocol/serialization) | `## Outcome` (scope constraint carried verbatim alongside the class list) | verbatim (list + scope sentence) | 16-17 |
| 9 | Scope, L92-102 | the nine-step successful journey | `## Outcome` | verbatim (list) | 18-26 |
| 10 | Scope, L104-106 | v1 success test: dogfood one genuine choice; chronicle identifies effort/failures/confidence without informal transcript | `ac-7`; `## AC-7` | condensed | 27; AMB-5 disposition §10 |
| 11 | Design principles 1-10, L110-138 | outcome-first; explore-then-lock; correctness-first; one primary metric; deterministic interpretation of disclosed noise; recommend-never-decide; disposable code durable reasoning; closed kernel open adapters; no privileged agent path; three-valued honesty | `dc-3..dc-7, dc-8, dc-10, dc-11`, `co-1`; each restated in its `## DC-n`/`## CO-1` body | condensed | 28-37 |
| 12 | Decision-proof model, L140-161 | "best" is a scoped claim; the seven-element immutable decision contract; the four-step decision sequence | `## Decision-proof model` | verbatim (contract list + sequence block) | 38-39 |
| 13 | Decision-proof model, L163-178 | three-valued result vocabulary (proven winner / violated with witness — correctness, safety, or resource guard / disclosed unproven); operational failure is not a fourth verdict, records an error, produces no recommendation, must be resumed or rerun | `co-1`, `dc-14`; `## Decision-proof model`; §6 of this plan | condensed | 40-43 |
| 14 | Decision-proof model, L180-182 | exit contract 0/1/2 | `co-4`; `## Decision-proof model` | condensed | 44 |
| 15 | Decision-proof model, L184-193 | prominent boundary statement; no universal-superiority claim; human registration review is the faithfulness judge; harness makes comparison mechanical | `co-5`; `## Decision-proof model` | verbatim (boundary quote) | 45-46 |
| 16 | Architecture, L195-218 | spike remains container; experiment directory tree (`experiment.yaml`, `candidates/*.patch`, `observations.jsonl`, `result.json`, `recommendation.md`, `ratification.yaml`, `selected/capsule-manifest.json`); terminal shape appears as lifecycle reaches states; directory follows spike active→archive | `## AC-1` body (tree carried verbatim); `dc-1`, `dc-2` | verbatim (tree) | 47-49 |
| 17 | Architecture, L219-235 | derived-state table (exploratory/registered/measured/recommended/inconclusive/ratified); no schema mutation, no second status; multiple revisions; never edit after lock | `dc-2`; `## AC-1` body (table carried) | verbatim (table) | 50-52 |
| 18 | Architecture, L237-242 | artifacts under spike spec dir per VL-016; durable support files only under `spike_paths`; patches may describe ephemeral product changes applied only in isolated workspaces, never committed on the spike branch | `dc-17`; `## AC-1` body | condensed | 53; AMB-6 disposition §10 |
| 19 | Explore/register/lock, L244-249 | pre-registration exploration disposable; results cannot be copied in as evidence | `dc-3`; `## AC-1` body | condensed | 54 |
| 20 | Explore/register/lock, L251-264 | the 12-element registration capture list | `ac-1`; `## AC-1` body | verbatim (list) | 55 |
| 21 | Explore/register/lock, L266-268 | all candidates apply to one base; protected paths (definition, evaluator, workload, fixtures, policy); protected-path-touching candidate cannot register | `ac-1`, `ac-6`; `## AC-1` body | condensed | 56 |
| 22 | Explore/register/lock, L270-277 | derived semantic review packet; human lock assigns definition digest; the only new pre-execution checkpoint, replacing line-by-line ceremony | `ac-1`, `dc-7`; `## AC-1` body; §8 rows C1/C6 | condensed | 57-58 |
| 23 | Experiment definition, L279-341 | `verdi.experiment/v1` strict schema; unknown fields/enums/types/aggregations/versions fail closed; full YAML example (id, spike, question, base_commit, candidates+digests, evaluator argv/digest/capabilities_digest, workload, decision block: primary_metric/baseline_improvement/candidate_separation/guards, execution block: warmups/rounds/order/timeout/environment_policy); field-stability rule (elaboration may add provenance fields, may not rename/reinterpret/weaken) | `co-2`; `## AC-1` body (example carried verbatim) | verbatim (YAML) | 59-61 |
| 24 | Evaluator/observation, L343-361 | evaluator is argv command, never shell; registration records identity/config/capabilities; `describe` returns strict `verdi.experiment-evaluator-capabilities/v1` with 7-item declaration list | `ac-3`; `## AC-3` body | verbatim (list) | 62-64 |
| 25 | Evaluator/observation, L363-385 | strict `verdi.experiment-observation/v1` records; full JSON example (candidate, round, guards with verdict+witness, measurements with source, disclosures) | `ac-3`, `co-2`; `## AC-3` body (example carried verbatim) | verbatim (JSON) | 65 |
| 26 | Evaluator/observation, L387-398 | three trust classes defined; only harness-/evaluator-measured determine eligibility/recommendation; candidate-reported diagnostic unless registered independent observer corroborates | `dc-12`; `## AC-3` body; `oq-2` | condensed + oq-carry (corroboration effect) | 66-67; AMB-7 |
| 27 | Evaluator/observation, L400-402 | observations append-only, keyed by experiment digest/run/candidate/round; normalized values preserve units and witnesses | `co-3`; `## AC-3` body | condensed | 68 |
| 28 | Deterministic recommendation, L404-408 | engine in closed kernel; byte-identical canonical JSON and same decision for same inputs | `co-3`, `ac-2` | condensed + cross-ref (the sorted/digest-bound canonical-JSON byte form is the repo-wide convention, artifact-contract §Generated artifacts and digests) | 69 |
| 29 | Deterministic recommendation, L410-423 | the eight-step evaluation order | `ac-2`; `## AC-2` body | verbatim (ordered list) | 70-77 |
| 30 | Deterministic recommendation, L425-428 | no weighted score, dynamic metric selection, post-run threshold editing, automatic tie-breaker; guardrails cannot compensate primary; primary cannot compensate correctness | `ac-2`, `dc-4`, `dc-5` | condensed | 78 |
| 31 | Deterministic recommendation, L430-434 | noise honesty: fixed schedule, registered aggregation/variability, practical thresholds, honest inconclusive | `dc-6`; `## AC-2` body | condensed | 79 |
| 32 | Execution/isolation/recovery, L436-441 | disposable workspace from registered base; patch applied, identity verified; common evaluator+workload under registered environment policy | `ac-4`; `## AC-4` body | condensed | 80 |
| 33 | Execution/isolation/recovery, L443-446 | deterministic declared schedule; fixed rotation default; warmups never measured | `dc-13`; `## AC-4` body | condensed | 81 |
| 34 | Execution/isolation/recovery, L447-456 | environment fingerprint minimum 7-item list | `dc-13`; `## AC-4` body | verbatim (list) | 82 |
| 35 | Execution/isolation/recovery, L457-459 | network disabled by default; declared network access needs policy and discloses weaker isolation | `co-6`; `## AC-4` body | condensed | 83 |
| 36 | Execution/isolation/recovery, L461-471 | four-way failure taxonomy (candidate verdict with witness; crash/timeout as candidate result; evaluator/harness faults as operational invalidation; variance/tie/conflict/no-eligible as disclosed-unproven) | `dc-14`; `## AC-4` body; §6 of this plan | verbatim (list) | 84-87 |
| 37 | Execution/isolation/recovery, L473-480 | resume only missing observations; observations immutable; resume refused on changed inputs; reruns separate identities, all shown; reproduction only under registered rule | `dc-15`; `## AC-4` body | condensed | 88-89 |
| 38 | Human/agent interaction, L484-489 | one typed application core; direct Markdown/Git editing first-class but same validation and policy, no privileged path | `dc-11`, `ac-5`; `## AC-5` body | condensed | 90 |
| 39 | Human/agent interaction, L491-519 | nine-stage journey diagram with actor annotations, two HUMAN REQUIRED stages | `## Experiment journey` | restructured (ASCII journey converted to the exemplars' mermaid form with stage-order and actor-annotation fidelity) | 91 |
| 40 | Human/agent interaction, L521-540 | agent-may list (8 items); agent-may-not list (7 items) | `## AC-5` body | verbatim (both lists) | 92-93 |
| 41 | Human/agent interaction, L542-546 | normal agent context contents; exploratory transcript excluded; excerpts non-authoritative and outside build context | `## AC-5` body; framing restated in `## Relationship to Context Integrity` | verbatim + restated as CI-compiler classification requirements per the audit's CX-11/R-7 clause 3 (the enumeration binds what the CI context compiler must include and exclude for experiment phases, never a freestanding competing context definition) | 94; AMB-5 |
| 42 | Human/agent interaction, L548-553 | the two required human moments are consequential, not ceremonial (lock = fairness; ratification = path decision) | `dc-7`, `dc-16`; §8 rows C1-C2 | condensed | 95-96 |
| 43 | Human/agent interaction, L555-558 | PR review + owner merge remain implementation governance; no competing approval ceremony; no recreated Codex/Claude Code/Verdi-go integrations | `## AC-5` body; §8 row C4 | condensed | 97 |
| 44 | Ratification/closure, L562-572 | `ratification.yaml` five-option vocabulary; names result digest, actor identity, disposition, reason; adapter authenticates actor, payload cannot self-declare human authority | `dc-16`; `## AC-5` body | verbatim (option list); the actor clause is additionally strengthened by the packet-sourced principal rule (row 69) | 98-99 |
| 45 | Ratification/closure, L574-583 | no direct feature edit, no new resolution edge; ratified decision becomes the spike answer through existing acceptance/closure; `resolves` edge sole connection; `resolved-by` backlink on frozen features; incorporated into normal closure review, no redundant approval; no auto-close without human ratification | `dc-16`, `ac-5`; `## AC-5` body; §8 row C3 | condensed | 100-101 |
| 46 | Ratification/closure, L585-587 | selection authorizes fresh implementation, never patch promotion | `dc-9` | condensed | 102 |
| 47 | Retention, L591-609 | retained-for-all list (6 items); not-retained-for-rejected list (6 items) | `dc-8`; `## AC-4` body | verbatim (both lists) | 103-104 |
| 48 | Retention, L611-615 | sealed capsule manifest for selected candidate; contains or content-addresses reproduction set; evidence, not a delivery vehicle | `dc-8`, `ac-4`; `## AC-4` body | condensed | 105 |
| 49 | Retention, L617-621 | cleanup only after durable human decision; cleanup failure operational and disclosed; retention cannot remove minimal record or keep product-ready prototypes | `ac-4`; `## AC-4` body | condensed | 106 |
| 50 | Extensibility, L625-638 | closed kernel / open adapters; the 10 kernel-owned invariants extensions cannot replace | `dc-10`; `## DC-10` body | verbatim (invariant list) | 107-108 |
| 51 | Extensibility, L640-645 | four independently versioned protocols, including the undefined `verdi.experiment-evaluator/v1` | `co-2`; `## DC-10` body; `oq-1` | verbatim + oq-carry | 109; AMB-1 |
| 52 | Extensibility, L647-660 | capability-handshake extension surface (guards, workload drivers, metrics, external observers, fingerprint providers, presentation/retention adapters); core metric primitives (duration, bytes, count, ratio, scalar, boolean); closed aggregation vocabulary (p50, p95, maximum, mean, rate); new primitive/aggregation/trust class requires protocol revision | `## AC-3` and `## DC-10` bodies | verbatim (lists) | 110-111 |
| 53 | Extensibility, L662-665 | future adapters (Go bench, JMH, pytest, telemetry) are extension-boundary examples, not v1 commitments | `## DC-10` body; `## Delivery sequence` (v1 scope note) | verbatim | 112; AMB-3 disposition §10 |
| 54 | Extensibility, L667-670 | project config selects permitted evaluators/capabilities; org policy narrows project; lower cannot weaken higher; capability inspection exposed | `ac-6`; `## AC-6` body | condensed | 113 |
| 55 | Extensibility, L672-679 | no in-process plugins/imports/hooks/shell; adapters cannot ratify, resolve, change recommendation semantics, or self-promote trust; v1 ships only generic command evaluator + built-in process observer | `dc-10`; `## Delivery sequence` | condensed | 114-115 |
| 56 | Policy/abuse, L683-696 | six-item policy constraint surface; argv execution, size limits, strict decode, undeclared-capability refusal, protected inputs; mutation provenance five-tuple across every surface | `ac-6`; `## AC-6` body | verbatim (lists) + scoped: the source's ambiguous "across every surface" is pinned to CSE's own experiment mutation surfaces per the authority audit (its CX-9/OD-4/R-7 — the audit's challenge review confirmed the narrow reading as the likely intent and asked the promotion to pin it) | 116-117 |
| 57 | Policy/abuse, L698-706 | decision-gaming counters; overfit-to-visible-workload boundary; candidate diffs reviewable; multiple workload profiles | `## AC-6` body; `co-5` | condensed | 118-119 |
| 58 | Policy/abuse, L708-711 | Verdi-bench owns adversarial machinery; high-trust decision evidence, not adversarially secure or universal proof | `dc-18`; `co-5` | condensed | 120 |
| 59 | Caching example, L713-768 | worked example end to end (request path, three candidates, workload mix, six-clause decision contract, normalized-result table, witness preservation, fresh reimplementation, measurement attribution) | `## Caching example` | verbatim | 121-128 |
| 60 | Verification strategy, L770-823 | unit (10 items), schema (8 items), hermetic integration (11 items), CLI e2e, Playwright, agent contract tests; committed deterministic caching fixture; no network; `make verify` + `go test -race` completion authority | `co-7`; `## CO-7` body | verbatim (lists) | 129-133; AMB-4 disposition §10 and §11 |
| 61 | Rollout steps 1-7, L827-838 | seven ordered vertical-slice steps | `## Delivery sequence`; `stubs` order | restructured (steps → stubs in the design's step order; step 5's surface splits into adapter and workbench stubs, with the workbench unit scheduled in orchestration Wave 6 per the completion ledger — a scheduling note, not a rollout-order change) | 134-140 |
| 62 | Rollout, L840-851 | standard comparison vs elevated assurance; identical semantics, more evidence | `dc-19`; `## Delivery sequence` | condensed | 141-143 |
| 63 | Rollout, L853-856 | templates + capability discovery reduce friction; Verdi may populate mechanical defaults, human must confirm desired outcome | `## Delivery sequence`; §8 row C6 | condensed | 144 |
| 64 | Non-goals, L860-875 | 13 explicit non-goals | `## Non-goals` | verbatim (list) | 145-157 |
| 65 | Accepted decisions, L881-894 | 11 accepted decisions | `dc-1, dc-3, dc-4, dc-7, dc-8, dc-9, dc-10, dc-18` and `ac-1/ac-2` texts (each decision named in the DC that carries it) | condensed | 158-168 |
| 66 | Rejected alternatives, L896-922 | 6 rejected alternatives with rationale | `## DC-1`, `## DC-4`, `## DC-7`, `## DC-9`, `## DC-11`, `## DC-18` bodies (rationale prose) | verbatim (rationale carried into the owning DC section) | 169-174 |
| 67 | Contents, L6-28 | the source's own table of contents | none | plan-only (navigation, non-normative; the artifact's navigation is its frontmatter anchors and body headings) | — |
| 68 | Orchestration index §Worktree/isolation/capability mechanics (non-CSE source, by declared exception) | the three CI/CSE boundary-ownership bullets | `## Relationship to Context Integrity` | verbatim (three bullets, per §7 — named, never ratified) | §7; independent Opus verification (word-by-word check against the index) |
| 69 | Cross-feature authority audit packet §2 item 2 / OD-4 / R-7 (PR #264; non-CSE source, by declared exception; consumed only as owner-adjudicated) | the two-class actor rule: mutation-provenance actors are attribution records; `ratification.yaml` actors require authenticated kernel principals, unproven authentication blocks ratification; plus the experiment-surface scope pin (its CX-9). The packet's companion embed rule (kernel principal ID or explicit `unauthenticated` marker, never a bare string) is a disclosed deferral to the kernel's landing (§7) | `dc-16`, `ac-6`; `## Relationship to Context Integrity` | conditional consumption — final wording follows the owner-merged adjudication and authority PRs, never the packet alone; contingent on R-5/OD-4 sequencing per the merged R-7, with OD-2 as R-5's own vehicle/custody prerequisite (§7); §9 Task 1 Step 4 blocks authoring until adjudicated | §7 actor-rule and deferral paragraphs; packet CX-4/OD-4/§12 C1 |

Coverage: rows 1-66 jointly cover inventory items 1-174 with no item
unmapped and no item double-assigned; rows 67-69 account for the only three
blocks outside that inventory (the source's navigation TOC; the
orchestration-index boundary bullets; and the authority-audit actor rule and
scope pin consumed as owner-adjudicated cross-feature authority). The
implementation unit re-verifies this claim mechanically (§9 Task 3, step
"losslessness audit").

## Verdict and failure-state separation

The promoted artifact keeps these five state families structurally
separate. The artifact itself carries the design's own taxonomy text —
verbatim for the failure-taxonomy list (§5 row 36) and condensed into
`co-1`/`dc-14` with full detail in `## Decision-proof model` (§5 row 13);
the table below is this plan's cross-walk of that text. One presentation note, disclosed rather than silent: the design
defines a **single** operational category (L467-469) covering evaluator
faults and harness/environment faults alike — the two middle rows below
partition that one category by fault origin because this unit's mandate
names them separately; both rows share the same semantics (run invalidated,
no recommendation, exit 2), so the partition adds no meaning.

| State family | Members | Recorded where | Exit code | Who acts next |
|---|---|---|---|---|
| Candidate verdicts | guard pass; correctness, safety, or resource guard failure **with preserved witness**; candidate crash or timeout while harness and workload stayed healthy; ineligible | `observations.jsonl` guard records; `result.json` eligibility | contributes to 0 or 1 | nobody — the verdict stands as evidence |
| Evaluator failures | evaluator crash; malformed response; protected-input mismatch (L467-469) | operational error record; run invalidated; **no recommendation produced** | 2 | operator repairs and reruns/resumes |
| Operational failures | missing round; environment mismatch; unavailable required isolation control (L467-469); cleanup failure after ratification — disclosed, decision unchanged, and the design assigns it no exit code (L617-618) | operational error record | 2 for the run-invalidating members | operator; a cleanup failure never rewrites the decision |
| Recommendation state | proven winner; disclosed-unproven (no eligible candidate, noise, tie, conflicting constraints) | `result.json` + `recommendation.md`, byte-deterministic | 0 (proven) / 1 (unproven) | a human reviews the decision packet |
| Owner ratification | select recommended; select other with reason; reject all; declare misframed; request new revision | `ratification.yaml`, adapter-authenticated actor | n/a (a human record, not a verb verdict) | existing spike acceptance/closure flow consumes the answer |

Invariants across the families: an operational failure is never a fourth
decision verdict and is never converted into evidence; missing or ambiguous
evidence is never a pass; no recommendation, however proven, closes the
question without the human ratification record; and ratification itself
never edits the parent feature or mints a new edge.

## Shared worktree, isolation, and capability boundary

CSE candidate execution (ac-4) and Context Integrity sealed execution both
need isolated workspaces and capability enforcement. The orchestration index
requires their focused plans to "name a common low-level boundary before
either adds feature-specific behavior," and Wave 0 carries a distinct item —
"Ratify the reusable worktree/isolation boundary between CI and CSE" — that
is **not this unit's to decide**.

This plan therefore records the dependency exactly as the index divides it,
and no further:

- **CI owns** the authority claim that an agent run was project-sealed and
  the vendor base remained opaque.
- **CSE owns** common-base candidate materialization, protected comparison
  inputs, evaluator capabilities, and experiment environment fingerprints.
- **Shared Git/worktree primitives may be reused**; context receipts and
  experiment recommendations remain separate proof types.

The proposed canonical artifact names this boundary in its
`## Relationship to Context Integrity` section using only the three bullets
above and links `depends-on: spec/worktree-manager` for the existing
worktree primitives — an edge both exemplars already declare, asserting
reuse of an existing archived primitive spec
(`.verdi/specs/archive/worktree-manager/spec.md`), not pre-committing the
Wave 0 shared-seam ratification. It does **not** select shared isolation
semantics,
define the shared seam's schema, or assign implementation ownership — all of
that is the separate Wave 0 ratification, which remains a blocking
prerequisite for the Wave 3 unit `CSE evaluator and isolated execution`
(§10, gap G-2).

The Wave 0 inventory is the cross-feature authority audit packet (PR #264,
`docs/superpowers/plans/2026-08-03-cross-feature-authority-audit.md`,
merged at `c99acbf3`). Three distinct events matter, and the packet's merge
is only the first: (1) the **packet merged** — a non-authoritative
inventory that ratifies nothing; (2) the **owner adjudicates** the rulings
it names, in repository-visible, owner-merged form; (3) the **resulting
authoritative contracts land** as their own owner-merged authority PRs
(the R-5 kernel ownership contract; the OD-6/OD-12 store-layout amendment).
This promotion **consumes** the outcomes of (2) and (3) and decides none of
them. The promotion unit's preflight (§9 Task 1 Step 4) blocks until the
owner adjudication exists for the rulings this artifact's text depends on:

- **OD-6 — isolation-boundary vehicle** (extend the `spec/worktree-manager`
  lineage via a ratified `verdi-store-layout` amendment plus a story, or a
  new shared component spec);
- **OD-12 — store-layout amendment ownership**: the experiment directory
  tree the artifact carries from the design (dc-17) describes future store
  content whose admission into `verdi-store-layout`'s committed-zone
  enumeration is a separate ratified component-spec amendment — a named
  dependency of the Wave 2+ implementation units, neither performed nor
  preempted by the promotion;
- **OD-7 — capability terminology**: CSE "capabilities" are execution
  grants drawn from the shared grant vocabulary (distinct from ASD's
  adapter-surface discovery), recorded in the promoted text once ratified;
- **OD-4 — ratification-principal treatment and mutation-provenance
  scope** (see the actor rule below);
- **OD-11 — ratification transport** (merge-witnessed judgment, no separate
  post-merge ratify ceremony), which shapes §8's C2/C3 rows;
- **R-7 clause 3 — context-boundary restatement (its CX-11)**: the design's
  "normal agent context" enumeration (L542-546) is carried not as a
  freestanding context definition but as **requirements on the CI context
  compiler's classification** for experiment phases — what it must include
  (refined spike, locked experiment, applicable decisions and policy,
  evaluator capabilities, resulting evidence) and exclude (the complete
  exploratory transcript; supporting excerpts as non-authoritative) — so the
  enumeration cannot drift once CI's compiler owns classification.

**Actor rule consumed from the packet (§2 item 2, per its CX-4/OD-4/R-7):**
CSE experiment **mutation-provenance actors are attribution records** —
never authority decisions — while **`ratification.yaml` actors must resolve
to authenticated principals through the shared governance-principal kernel,
and unproven authentication blocks ratification**. The proposed `dc-16` and
`ac-6` texts in §4 carry this rule; §5 row 69 records the packet as its
source. The packet proposes this wording; only the owner's adjudication
and the resulting authority PRs make it binding — if they land with
different wording, the artifact text follows the owner-merged authority,
not the packet, updated in the promotion PR before review, never after
acceptance.

Two disclosed deferrals ride with the actor rule, neither silently dropped:
the packet's companion **embed rule** (every attribution record embeds a
kernel canonical principal ID or an explicit `unauthenticated` marker —
never a bare string presented as identity) is deferred to the
governance-principal kernel's landing, since it depends on the kernel's
not-yet-ratified ID representation; and the rule's final wording is
**contingent on R-5/OD-4 sequencing, exactly as the merged packet's R-7
states it** ("OD-3 governs only ASD's actor-upgrade timing"). OD-2 — the
vehicle and custody decision for R-5's kernel ownership contract — may be
an additional prerequisite for R-5's landing, but it does not replace
CSE's OD-4. Until the R-5 authority PR lands, `dc-16` names the kernel
seam on the strength of the already-merged orchestration authority (one
shared governance-profile/principal schema; neither feature may introduce
a second actor type), and the packet rule rides as conditional wording.

## Human-ceremony inventory

Classification follows the merge-signals design's ceremony audit rule
(substantive judgment / forge authorization reuse / deterministic
materialization / exceptional override / removable acknowledgement).

| # | Ceremony | Class | Treatment |
|---|---|---|---|
| C1 | Semantic registration review + lock (design L506, L548-551) | substantive judgment | **Retain** — the one pre-execution human checkpoint; it replaces line-by-line registration ceremony with one review of the consequential decision contract. Verdi derives the review packet (deterministic materialization of the packet itself). |
| C2 | Decision-packet review + ratification (L512, L552-553) | substantive judgment | **Retain** — the only path from recommendation to chosen design; five-option vocabulary; adapter-authenticated. |
| C3 | Spike answer acceptance and closure (L515, L581-583) | forge authorization reuse | **Reuse, deduplicated** — the ratification record is incorporated into the existing spike closure review; the design forbids a redundant approval and this plan retains that elimination. No second acknowledgement of the same decision. |
| C4 | Implementation governance for the later story (L555-558) | forge authorization reuse | **Reuse** — profile-required PR review and the owner's merge remain the gate; CSE adds no competing approval ceremony. |
| C5 | Canonical-promotion acceptance (this unit's own lifecycle) | forge authorization reuse | **Reuse** — the owner's merge of the promotion PR *is* acceptance under merge-signaled lifecycle; no `verdi accept`, no status flip, no post-merge mutation, no confirmation ritual. |
| C6 | Confirming the desired outcome when templates prefill defaults (L853-856) | substantive judgment | **Retain** — Verdi may populate mechanical defaults, but a human confirms what the experiment is intended to prove. |
| C7 | Registration-packet derivation, execution, recommendation rendering, capsule sealing, cleanup (L270-274, L509, L611-621) | deterministic materialization | **Automate** — no human shepherding; cleanup runs only after the ratification record is durable. |
| C8 | Exploratory workspace create/discard (L244-249) | removable acknowledgement | **Removed by design** — exploration is unceremonied and non-evidentiary; nothing to acknowledge. |
| C9 | Codex plan review of this packet; Codex exact-head review of the promotion PR | not a human ceremony — an agent review required by orchestration Gates P/C, listed for completeness and deliberately outside the five-class human taxonomy | **Retain** — read-only, fresh-context, exact-head; findings return to Claude Code, never self-repaired by Codex. |

Duplicated acknowledgements eliminated (net): no second acceptance ceremony
for the canonical artifact (C5), no re-approval of an already-ratified
experiment at spike closure (C3), no per-candidate or per-file registration
sign-off (collapsed into C1), no competing implementation approval (C4).
Every retained ceremony carries distinct substantive judgment or is an
existing forge authorization — satisfying the orchestration index's
per-plan ceremony requirement.

## Implementation tasks for the promotion unit

These tasks execute **after** this plan passes read-only Codex review (Gate
P) and only from a fresh branch off the then-current `main` with every Wave
0 prerequisite below satisfied. Planning-lane scope ends here; nothing in
this section runs in this lane.

**Branch topology:** `main` (post-plan-merge) → `feature/cse-canonical-promotion`
→ draft PR to `main`. Rollback posture is phase-dependent, because the
owner's merge **is acceptance** under the merge-signaled lifecycle:
- **Before merge:** trivial — close the draft PR and delete the branch;
  nothing has entered authority.
- **After merge:** the artifact is an accepted canonical proposal. It is
  not casually removable: reversal is a governed lifecycle action —
  supersession or closure through the ratified amendment flow, decided and
  authorized by the owner — never a plain `git revert` of accepted
  authority. What the single-file shape does guarantee is containment: no
  runtime coupling, no data migration, and no other artifact depends on it
  until later delivery units land, so a governed reversal remains cheap.

**Fixtures:** the unit adds no runtime code and therefore authors no new
hermetic fixtures; its test surface is the store's own gates (`verdi lint`,
`make lint-store`, `internal/specalign`) plus the unchanged existing fixture
gates, which must remain green under the full `make verify` in Task 4.
Hermetic fixtures for experiment behavior (fake evaluators, canned strict
JSON, fixture Git repositories) belong to the later runtime stories and are
specified by the promoted artifact's `co-7`.

### Task 1: Preflight and authority freshness

**Files:** none (read-only checks)

**Interfaces:** Produces the go/no-go witness the remaining tasks require.

- [ ] **Step 1:** `git fetch origin && git rev-parse origin/main` — record
  the base SHA. Verify this plan's commit is reachable from it (`git
  merge-base --is-ancestor <plan-commit> origin/main`). Expected: yes;
  otherwise STOP (stale authority).
- [ ] **Step 2:** Re-verify the source blob:
  `git rev-parse origin/main:docs/superpowers/specs/2026-07-30-comparative-spike-experiments-design.md`.
  Expected: `c28bdf43d948aafa186568bedade3da51930bb78`. A different OID
  means the design was amended after this plan — STOP and re-map before
  authoring.
- [ ] **Step 3:** Re-verify link targets resolve in the committed store:
  `ls .verdi/specs/active/verdi-artifact-contract/spec.md
  .verdi/specs/active/verdi-evidence-model/spec.md
  .verdi/specs/archive/worktree-manager/spec.md`. Expected: all three
  present (verified during planning: `spec/worktree-manager` lives in the
  **archive** zone; VL-003 resolves refs against the whole committed zone,
  and both exemplars declare the identical edge).
- [ ] **Step 4:** Verify the Wave 0 authority prerequisites — three
  distinct events, verified separately; each missing one is a STOP, not a
  proceed-with-disclosure:
  (a) the repository names a successor invention-ledger location (G-1,
  packet OD-9/OD-10);
  (b) **packet merged** — the cross-feature authority audit packet landed
  (done: PR #264 merged at `c99acbf3`); this is a non-authoritative
  inventory and by itself ratifies nothing;
  (c) **owner adjudication recorded** — repository-visible, owner-merged
  records of the owner's decisions on the rulings this artifact consumes:
  OD-4 (ratification-principal treatment and mutation-provenance scope),
  OD-6 (isolation-boundary vehicle), OD-7 (capability terminology), OD-11
  (ratification transport), OD-12 (store-layout amendment ownership);
  (d) **authoritative contracts landed where the consumed wording depends
  on them** — the R-5 kernel ownership contract (sequencing per the merged
  R-7: R-5/OD-4, with OD-2 as R-5's own vehicle/custody decision) as its
  own owner-merged authority PR; where an authority PR has not yet landed,
  the artifact carries only wording grounded in already-merged authority
  and §7's deferral paragraph governs. Then revalidate §7's consumed
  wording — the actor rule, the surface-scope pin, the boundary bullets,
  §8's C2/C3 transport posture — against those owner-merged authority
  texts (never against the packet alone) and amend this plan first if they
  diverge. The full shared-seam *implementation* (the OD-6/OD-12
  store-layout amendment itself) may still be pending — the artifact
  names, never selects — but the owner rulings on wording the artifact
  carries must exist before authoring.
- [ ] **Step 5:** Re-confirm the stable `merge-gate` check remains an
  **active required status check** on the default-branch ruleset
  (read-only: `gh api repos/{owner}/{repo}/rulesets/19021982`). Verified
  live during planning (§10 G-6): enforcement `active`, required status
  check context `merge-gate`, strict policy. Expected: unchanged; if the
  rule has been weakened or removed, STOP — the orchestration index
  requires the gate live before any canonical proposal lands.
- [ ] **Step 6:** `git worktree add` a fresh worktree on
  `feature/cse-canonical-promotion` from origin/main; `git status
  --porcelain` empty. Expected: clean.

### Task 2: Author the canonical artifact

**Files:**
- Create: `.verdi/specs/active/comparative-spike-experiments/spec.md`

**Interfaces:** Produces the artifact whose frontmatter is §4's block
verbatim (modulo any Codex-plan-review conditions) and whose body follows
§4's outline, with every §5 destination populated.

- [ ] **Step 1:** Write the frontmatter exactly as §4 specifies: 7 ACs
  (each with `evidence` including `attestation` — the outcome floor VL-006
  requires), 8 stubs, 19 DCs, 7 COs, 2 OQs, `problem`/`outcome`
  attributes, links per §4 (re-verified at Task 1 Step 3). No `status:`,
  no `frozen:`.
- [ ] **Step 2:** Write the body per §4's outline, carrying every
  `verbatim` row of §5 byte-near-identically (the directory tree, derived-
  state table, `experiment.yaml` example, observation JSON example, the
  eight-step evaluation order, environment fingerprint list, failure
  taxonomy, agent may/may-not lists, ratification options, retention lists,
  kernel invariants, metric primitives and aggregations, policy surface,
  caching example, verification lists, the 13 non-goals, the six
  rejected-alternative rationales) and every heading resolving its
  frontmatter anchor.
- [ ] **Step 3:** Confirm no new semantics beyond ratified authority: the
  artifact must contain no normative statement without a §5 source row —
  the two declared exceptions being `## Relationship to Context Integrity`
  (the orchestration index's boundary bullets, §5 row 68, carried verbatim)
  and the packet-sourced actor rule and surface-scope pin (§5 row 69,
  carried only as owner-adjudicated). The two OQs carry AMB-1 and AMB-7
  verbatim as open questions; no other ambiguity is resolved in the
  artifact text.

### Task 3: Validate against the store gates

**Files:**
- Verify: the new `spec.md` only; no other file changes in the unit.

- [ ] **Step 1:** `go build -o .build/verdi ./cmd/verdi && ./.build/verdi
  lint`. Expected: exit 0 — strict decode passes, anchors resolve
  (slug-symmetric heading match), link refs resolve, VL-006 satisfied
  (every AC declares evidence kinds incl. `attestation`).
- [ ] **Step 2:** `make lint-store`. Expected: pass (includes `verdi model
  check`).
- [ ] **Step 3:** `go test -count=1 ./internal/specalign/...`. Expected:
  pass — the six component-spec fidelity checks are unaffected (feature
  specs are explicitly out of the fidelity scope); the store may legally
  grow.
- [ ] **Step 4:** Losslessness audit: for each §5 row marked `verbatim`,
  diff the carried block against the source lines
  (`git show origin/main:docs/superpowers/specs/2026-07-30-comparative-spike-experiments-design.md`)
  and record any intentional divergence with its reason; for `condensed`
  rows, confirm the named destination exists and names every source clause.
  Record the result (174/174 or the exact shortfall) in the PR body's
  requirement-coverage table.
- [ ] **Step 5:** `git diff --check`. Expected: no whitespace errors.
- [ ] **Step 6:** `git add .verdi/specs/active/comparative-spike-experiments/spec.md
  && git commit -m "Add comparative-spike-experiments canonical feature proposal"`.

### Task 4: Full gates, push, and draft PR

- [ ] **Step 1:** `make verify` and `go test -race ./... -count=1` from the
  unit worktree. Expected: clean (the unit adds no code, but the
  orchestration's per-unit protocol requires fresh full-gate evidence; e2e
  requires node/npm present — a missing toolchain HARD-FAILS and is an
  environment defect to fix, not skip).
- [ ] **Step 2:** Push `feature/cse-canonical-promotion`; open a **draft**
  PR to `main` whose body carries the Claude-to-Codex handoff contract:
  Authority (design blob OID, landing commit, this plan's path and commit,
  base/head SHAs), Requirement coverage (one row per AC/DC/CO/OQ naming its
  §5 source rows), Verification (observed command output), Disclosures
  (§10 carried forward, updated), Review scope (the single created file;
  revert boundary).
- [ ] **Step 3:** Wait for the `merge-gate` check on the exact head
  (it runs unconditionally on every PR and is ruleset-required). Expected:
  green. Re-confirm Task 1 Step 5's active-ruleset reading still holds
  before the PR can be merge-eligible. Do not mark ready; request Codex
  exact-head review; the owner alone merges — and that merge **is**
  acceptance.

## Three-valued disclosures and blocking authority gaps

**Proven (with witnesses):**

- origin/main at plan time is exactly
  `6d71fd7d33beaf8128fa675833ee12595205481d`; branch
  `agent/cse-canonical-promotion-plan` was created fresh from it; worktree
  clean at creation (witness: command outputs recorded in the session and
  the PR body).
- The three authority blobs match the mandated OIDs (witness: `git
  rev-parse` outputs in §Authority and provenance).
- Every normative line of the source maps to a §5 row: the mapped
  complement contains only headings, blank lines, and the navigation
  Contents block (§5 row 67). Witness: the table's own line anchors,
  independently re-walked by Opus verification (18 anchor ranges and every
  declared item count re-read against the source); the numbered 174-item
  session inventory is disclosed working evidence, not a committed
  artifact, and the claim is mechanically re-verified at §9 Task 3.
- `spec/worktree-manager` resolves in the committed store at
  `.verdi/specs/archive/worktree-manager/spec.md`; VL-003 resolves refs
  against the whole committed zone, and both exemplars declare the same
  `depends-on` edge (witness: grep + exemplar frontmatter).
- No CSE artifact currently exists under `.verdi/specs/` (witness: store
  directory listing).
- Default-branch ruleset `19021982` is active and requires the
  `merge-gate` status check with strict policy (witness: the Codex
  reviewer's own verification, independently re-confirmed by this FABLE
  planning lane's read-only `gh api repos/{owner}/{repo}/rulesets/19021982`
  query through the owner-authenticated `gh` CLI during Codex round 1; see
  G-6).

**Violated:** none observed in this lane.

**Disclosed as unproven, and authority gaps (G-*) that block later
transitions:**

- **G-1 (blocking for the implementation unit, not this plan):** the
  successor invention-ledger location is a Wave 0 item not yet visibly
  resolved in the repository. The orchestration stop conditions forbid
  implementation while the ledger is unresolvable. AMB dispositions below
  that require ledger entries inherit this gap.
- **G-2 (blocking):** the reusable CI/CSE worktree/isolation/capability
  boundary is unratified. The cross-feature authority audit packet
  **has merged** (PR #264, landing commit `c99acbf3`) — but that landing
  is only the non-authoritative inventory; the owner decisions it names —
  OD-4 (ratification principals, mutation-provenance scope), OD-6
  (isolation-boundary vehicle), OD-7 (capability terminology), OD-11
  (ratification transport), OD-12 (store-layout amendment ownership) —
  and the authority PRs that would result from them (the R-5 kernel
  ownership contract; the store-layout amendment) **remain pending**. The
  full boundary implementation blocks Wave 3; the **owner rulings on
  wording this artifact carries block the promotion unit's authoring**
  (§9 Task 1 Step 4). This plan and the proposed artifact name the
  boundary (§7) and select nothing.
- **G-3 (resolved during verification — retained for the record):** the
  `spec/worktree-manager` link target is proven to resolve (see the Proven
  list above); the material fact the implementation unit carries is that
  the target sits in the **archive** zone, not `active/`. §9 Task 1 Step 3
  re-verifies at implementation time.
- **G-4 (disclosure):** the repository tracks no `AGENTS.md`; the
  workspace-root `AGENTS.md` (outside this Git repository) and the
  repository `CLAUDE.md` were read as the applicable committed
  instructions. A web/cloud implementation session cannot see the
  workspace file; everything it needs is carried in this plan.
- **G-5 (unproven in this lane):** full `make verify` was not run here by
  explicit dispatch instruction (planning-only; avoid competing with the
  resolver lane). Validation in this lane is `git diff --check` plus the
  narrow `internal/specalign` suite; the implementation unit owes the full
  gates (§9 Task 4).
- **G-6 (proven-live; retained for the record):** the `merge-gate`
  required-status rule **is active**. Witness: the Codex reviewer's own
  verification, independently re-confirmed by this FABLE planning lane's
  read-only API query of default-branch ruleset `19021982` through the
  owner-authenticated `gh` CLI during Codex round 1 —
  `enforcement: "active"`, rule `required_status_checks` with context
  `merge-gate` and `strict_required_status_checks_policy: true`, plus
  pull-request-required and review-thread-resolution rules. Repository
  prose at the base commit (`merge-gate.yml`'s header; commit `f3bcee02`)
  predates the owner's activation and describes the pre-activation state —
  stale relative to the live forge, not evidence against it. The
  orchestration prerequisite ("gate live before any canonical proposal
  lands") is satisfied today; §9 Task 1 Step 5 remains a read-only
  re-confirmation at execution time, STOPping only on regression.

**Ambiguity dispositions (no silent resolution):**

| ID | Ambiguity (source) | Disposition |
|---|---|---|
| AMB-1 | `verdi.experiment-evaluator/v1` listed (L644) but never defined | **oq-carry** → artifact `oq-1`; resolution belongs to the `experiment-schemas` story through the ratified amendment flow |
| AMB-2 | dual "design authority vs canonical lifecycle authority" senses (L4) | **resolved by the unit itself** — promotion + owner merge ends the split; no artifact text needed |
| AMB-3 | Extensibility's not-v1 disclaimer absent from Rollout | **editorial cross-reference** — `## Delivery sequence` restates the v1 boundary (generic evaluator + process observer only), citing L662-679; no semantic change |
| AMB-4 | no explicit CI-venue statement for the completion gates | **cross-ref to existing binding authority** — evidence-model §Provenance classes (`source: ci` authoritative, local advisory) already governs; §11 below states the consequence as this plan's own boundary statement, and the artifact carries only design-sourced text plus the cross-reference; no new artifact semantics |
| AMB-5 | chronicle audit fidelity vs excluded exploratory transcript | **disclosure carried in `## AC-7` body** — the v1 success test stands as written; how chronicle captures exploration effort is measured at dogfood, not predetermined here |
| AMB-6 | patch *content* under `spike_paths` vs VL-016 intent | **carried as dc-17 exactly as the design asserts it** (branch-diff fencing is what VL-016 governs; patch bytes are store artifacts) — flagged for the Codex plan review to challenge; if review finds it substantive, it becomes a third OQ, not a silent fix |
| AMB-7 | observer corroboration vs closed trust classes | **oq-carry** → artifact `oq-2` |

## Experimental evidence and CI authoritative proof

Stated explicitly as this unit's own boundary. In the artifact itself the
line is carried by design-sourced text alone — `co-5`'s scoped-claim
constraint, `co-7`'s completion-authority sentence (`make verify` and
`go test -race ./...` remain the completion authority), and the
`## Decision-proof model` boundary statement — plus a cross-reference to
the binding evidence model's provenance classes. The elaboration below is
this plan's restatement of existing binding authority, not new artifact
semantics:

**Experimental evidence cannot mint CI authoritative execution proof.**
Experiment observations, results, recommendations, and even a ratified
selection are *decision evidence* scoped to the registered contract,
candidates, workload, and environment. They are never implementation
evidence: they cannot satisfy a story's acceptance criteria, cannot feed the
merge or closure gates as authoritative records, and cannot substitute for
`make verify`, `go test -race ./...`, or CI-provenance (`source: ci`)
evidence — which the binding evidence model alone defines as authoritative,
with local runs advisory. The orchestration index's Wave 4 exit gate
("experimental posture cannot mint authoritative proof") and Context
Integrity dc-19 (an experimental profile cannot mint authoritative receipts)
bound the same line from the other side. The selected design earns its
implementation proof only through the ordinary story lifecycle: fresh
implementation, TDD, review, CI evidence, and the owner's merge.

---

*Prepared by the FABLE-led planning lane for delivery unit
`cse-canonical-promotion` (orchestration Phase A lane W3, converted per
Phase C against landing commit `6d71fd7d`). Retained for Codex plan-review
rounds; the owner alone merges.*
