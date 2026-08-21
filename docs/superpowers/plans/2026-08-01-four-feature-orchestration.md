# Four-Feature Orchestration Plan

> **For agentic workers:** This file is the sequencing index, not a license to
> implement directly. Before changing runtime code, use a focused
> implementation plan only when the unit's complexity materially benefits
> from one. Execute an approved multi-step plan with the smallest useful
> workflow. Spec-only authority work follows the direct-authoring and review
> rules below; do not create ceremonial plans or handoffs for mechanical
> edits. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ratify and deliver four related Verdi features without duplicating authority schemas, creating competing workflow state, or allowing implementation and review contexts to contaminate one another.

**Architecture:** Git and GitHub pull requests are the durable coordination
layer. Implementation-heavy work uses isolated Claude Code lanes from explicit
base commits: FABLE owns frontend production, while Sonnet and Opus perform
delegated non-frontend implementation and repair. The main agent adjudicates
their work and the human owner alone performs authoritative merge and lifecycle
actions. Spec-only authority is authored and repaired directly by the main
agent; substantial authoring receives one read-only cross-model exact-head
challenge. Shared contracts land before feature-owned consumers, backend
record shapes land before adapters, and all workbench presentation work is
serialized after the underlying records stabilize.

**Tech Stack:** Go, filesystem-backed Verdi artifacts, Git worktrees, GitHub
pull requests, Claude Code implementation subagents, cross-model independent
review, hermetic Go tests, Playwright, `make verify`, and `go test -race ./...`.

## Global Constraints

- The binding system semantics remain `docs/design/specs/00..05-*.md`; `docs/design/specs/08-revision-notes.md` remains the ratification history.
- The owner's merge of this pull request ratifies the four documents indexed here: the two canonical proposals become accepted pending build under the merge-signaled acceptance lifecycle, and the two design documents become ratified design authority that is not yet canonical lifecycle authority.
- Never edit frozen artifacts or binding system specs directly. Ratified semantic changes follow the existing amendment flow.
- Never import `verdi-go` packages. Execute its pinned CLIs and strict-decode their JSON outputs.
- Preserve three-valued honesty: proven, violated with a witness, or explicitly disclosed as unproven. Missing context, identity, forge state, evidence, or review is never a pass.
- Preserve the exit contract: `0` clean, `1` verdict failure, `2` operational failure.
- Strict-decode YAML through `internal/artifact`; strict-decode JSON with unknown-field and trailing-data rejection; fail closed on unknown enum values.
- Canonical outputs are deterministically sorted, digest-bound, and free of incidental wall-clock or random values except declared provenance stamps.
- Every behavior change follows TDD. Every function receives happy and negative unit coverage; integrations use hermetic fakes; browser-facing paths receive Playwright coverage.
- No test uses the network. Every delivery unit closes with fresh `make verify` and `go test -race ./...` evidence.
- Frontend implementation and fixes are assigned to a FABLE subagent.
  Implementation-heavy non-frontend work is assigned to Sonnet, with Opus
  repairing accepted implementation findings and gate failures. These
  production assignments do not apply to spec-only authority work.
- In every implementation-heavy Claude Code execution session, FABLE remains
  the chief architect and final producer-side judge. FABLE dispatches the
  repository-defined implementation role, personally adjudicates every
  returned diff and evidence packet, and requests an independent Opus
  verification when a proof claim, authority boundary, or gate result is not
  adequately grounded.
- Claude Code never accepts a specification, approves a pull request, merges a branch, authors a human attestation or waiver, or dispositions a judgment-bearing finding.
- Spec-only authority work is authored and repaired directly by the main
  agent. Mechanical specification or documentation edits may remain
  single-agent. Substantial spec-only authoring receives exactly one
  independent read-only cross-model review of the consolidated exact head:
  Claude reviews Codex-authored work, and Codex reviews Claude-authored work.
  The authoring agent adjudicates every finding and authors every accepted
  correction; the same reviewer performs one closure check after at most one
  correction pass. No automatic third round or reviewer/fixer chain follows.
  Claude's read-only role is a narrow reviewer exception, not permission to
  produce or repair specification authority.
- A blocking spec-only finding must cite binding authority, exhibit a reachable
  conforming state, identify its concrete incorrect result, and stay inside
  the declared threat model. Wording preferences, out-of-model interference,
  and optional hardening are non-blocking.
- Every canonical promotion includes a source-coverage/losslessness witness
  mapping all source authority to its destination, naming transformations or
  intentional omissions, and reporting the coverage total.
- The main agent retains responsibility for authority interpretation, finding
  adjudication, verification, and final handoff for every lane.
- Only the human owner may approve the final merge.
- One authoritative human decision occurs once: for a solo operator, merging a reviewed specification pull request is acceptance. No separate `verdi accept`, `/accept`, status edit, or confirmation may repeat that decision. Deterministic sealing and lifecycle projection follow the [merge-signaled acceptance design](../specs/2026-08-01-merge-signals-spec-acceptance-design.md).
- Every focused plan inventories its human ceremonies and removes or automates any action that does not carry distinct substantive judgment, an exceptional override, or demonstrated irreversible-risk protection.
- Until a successor build contract explicitly relocates it, unresolved semantic ambiguities are recorded in `PLAN.md` section 7 and block implementation rather than being silently resolved.
- This specification-index pull request does not override the workspace's current v0-only build instructions. Wave 0 must make the successor scope and invention-ledger location repository-visible before any runtime implementation begins.

---

## Contents

1. [How to use this index](#how-to-use-this-index)
2. [Scope boundaries](#scope-boundaries)
3. [Specification index](#specification-index)
4. [Shared ownership boundaries](#shared-ownership-boundaries)
5. [Execution surfaces and security](#execution-surfaces-and-security)
6. [Ten-hour acceleration window](#ten-hour-acceleration-window)
7. [Delivery waves](#delivery-waves)
8. [Wave orchestration and review cadence](#wave-orchestration-and-review-cadence)
9. [Per-unit pull request protocol](#per-unit-pull-request-protocol)
10. [Producer-to-reviewer handoff contract](#producer-to-reviewer-handoff-contract)
11. [Review and merge gates](#review-and-merge-gates)
12. [Stop conditions](#stop-conditions)
13. [Completion ledger](#completion-ledger)

## How to use this index

This document answers which contract lands before another contract, which work may run concurrently, and which evidence permits the next transition. It deliberately does not prescribe package names, Go types, or test bodies before the corresponding feature plan has assessed the current codebase at its starting commit.

For each delivery unit:

1. Start from the current default branch after every listed dependency has
   merged, then create one isolated Git worktree and one branch.
2. Classify the unit before selecting a workflow. An already-decided
   mechanical specification or documentation edit may remain single-agent and
   needs no design document, implementation plan, or independent review chain.
3. For substantial spec-only authority, the main agent authors and repairs the
   work directly. A canonical promotion also carries the required
   source-coverage/losslessness witness and its coverage total.
4. For implementation-heavy work, write a focused plan only when it materially
   improves correctness or reduces risk, then let the repository-defined
   Claude Code role execute it. Preserve FABLE ownership of frontend work.
5. Treat the delivery unit, not each internal task or commit, as the independent
   review boundary. Open one draft pull request for that unit and attach the
   applicable handoff evidence below.
6. Substantial spec-only work receives exactly one independent cross-model
   exact-head review, at most one author correction pass, and one closure check.
   Implementation-heavy work receives one consolidated independent Codex
   exact-head review after its implementation and gates are complete.
7. The main agent adjudicates every finding, verifies the exact head, and
   produces the final handoff.
8. Let the human owner merge only after required CI and governance checks pass.

Concurrency is phase-specific rather than one global number. The table below is
the ceiling, never a target; a lane may start only when its prerequisites are
merged, its semantic and changed-file ownership is disjoint, and it does not
share a schema registry, `Makefile`, workbench asset, or Playwright path with
another active lane.

| Phase | Maximum active implementation lanes | Additional constraint |
|---|---:|---|
| Wave 0 | 2 | Spec-only promotion lanes only; shared registries and owner decisions remain serialized. |
| Wave 1 | 1 | The shared authority kernel and its first consumers define types used downstream. |
| Wave 2 | 2 | Use the existing disjoint Track A/Track B ownership. |
| Wave 3A | 1 | Context compilation precedes the policy-conflict gate in one CI-owned sequence. |
| Wave 3.5 | 2 | After one serialized readiness contract and shared board-link constructor export land, one backend snapshot lane and one FABLE cockpit lane may proceed over disjoint files; integration is serialized. |
| Wave 3B | 2 | Only a focused CSE plan proving disjoint evaluator/observer and scheduling/resume ownership may use both; otherwise use one. |
| Wave 4 | 1 | Shared identity, authority, receipt, and human-record schemas remain serialized. |
| Wave 5 | 3 | Each lane must pass a package/file ownership check; any shared adapter or registry reduces the ceiling. |
| Wave 6 | 1 | All workbench presentation is FABLE-owned and fully serialized. |
| Wave 7 | 3 | Up to three independent dogfood journeys may run concurrently; whole-program verification, review, and approval remain serialized. |

Read-only reconnaissance does not consume an implementation lane and cannot
grant Gate P or Gate I. When a ceiling or ownership condition is uncertain,
serialize rather than infer permission.

## Scope boundaries

This pull request indexes and sequences the four specifications. It does not:

- perform any acceptance action in its own bytes: no acceptance command, status field, or frozen stamp is written; the owner's merge is what accepts the two canonical proposals;
- promote ASD or CSE into canonical Verdi artifacts;
- change runtime code, schemas, CLI or MCP inventories, workbench assets, tests, dependencies, or CI;
- choose package names, Go interfaces, migrations, configuration keys, performance budgets, or rollout defaults for later implementation plans;
- replace GitHub reviews with an agent-to-agent MCP message bridge;
- alter the current v0 build instructions or silently authorize out-of-contract implementation; or
- perform any human-only governance or merge action.

The branch is independently reversible by reverting its documentation commit. Later units remain separately reversible because each owns one focused pull request and must state its compatibility and rollback posture before review.

Keep this umbrella pull request as a design-review vehicle until the [merge-signaled acceptance design](../specs/2026-08-01-merge-signals-spec-acceptance-design.md) is ratified and its stable required merge gate is live. After that prerequisite lands, merging the reviewed canonical proposals is their acceptance; no preparatory acceptance command or status-only commit is permitted.

## Specification index

| Key | Specification | Current authority posture | Delivery units named by the specification | Entry gate |
|---|---|---|---|---|
| GLG | [Guided Lifecycle and Governance](../../../.verdi/specs/active/guided-lifecycle-governance/spec.md) | Statusless canonical Verdi feature artifact whose lifecycle state `verdi spec state` derives: proposed while this pull request is open, accepted pending build once the owner's merge lands on the default branch | `journey-projection`, `obligation-quality`, `lifecycle-governance`, `accountable-human-records`, `committed-authority`, `continuous-readiness`, `lifecycle-recovery`, `journey-metrics` | Profile-required exact-head review and the owner's merge of this pull request |
| CI | [Context Integrity and Constitutional Execution](../../../.verdi/specs/active/context-integrity/spec.md) | Statusless canonical Verdi feature artifact whose lifecycle state `verdi spec state` derives: proposed while this pull request is open, accepted pending build once the owner's merge lands on the default branch | `policy-authority`, `context-compiler`, `policy-conflict-gate`, `sealed-codex-execution`, `context-receipts-review`, `constitution-workbench` | Profile-required exact-head review and the owner's merge of this pull request |
| ASD | [AI-assisted spec design](../specs/2026-07-30-ai-assisted-spec-design.md) | Design document that the owner's merge of this pull request ratifies as design authority; not a canonical Verdi feature artifact until its sequenced canonical-promotion unit merges | Draft-mutation core, structured CLI, workbench migration, bounded context and capabilities, proposal-only dogfood, draft-write, review/provenance views, Codex/Claude conformance | Profile-required exact-head review and the owner's merge of this pull request; canonical promotion is later scoped work accepted by the merge of its own reviewed pull request |
| CSE | [Comparative spike experiments](../specs/2026-07-30-comparative-spike-experiments-design.md) | Design document that the owner's merge of this pull request ratifies as design authority; not a canonical Verdi feature artifact until its sequenced canonical-promotion unit merges | Experiment schemas, decision engine, evaluator/observer, isolated execution and resume, CLI/workbench/agent adapters, policy, dogfood comparison | Profile-required exact-head review and the owner's merge of this pull request; canonical promotion is later scoped work accepted by the merge of its own reviewed pull request |

No implementation plan may treat the two design documents as accepted lifecycle authority. Their promotion must preserve their reviewed decisions, constraints, non-goals, and rollout ordering in canonical artifact form.

## Shared ownership boundaries

### Governance profile and authenticated principals

CI and GLG explicitly require one schema and one implementation seam. Factor a prerequisite delivery unit named `governance-principal-kernel` before either feature defines consumers.

- GLG retains ownership of the lifecycle-wide governance contract.
- CI records and enforces the resolved profile and principals for policy, context, execution, and receipts.
- Neither feature may introduce a second profile enum, actor type, trust-source resolver, or authorization interpretation.

### Human-artifact scaffolds and templates

CI `policy-authority` owns the immutable identity, authority, scope, lifecycle, ownership, and provenance kernel plus the shared renderer for configurable human-authored artifacts. ASD must consume this seam for agent-assisted creation and may add typed draft operations, but it may not create a competing template or policy model.

### Context facts and journey facts

CI owns context compilation, authority classification, conflict verdicts, sealed execution, and context receipts. GLG may consume those canonical results as journey operands but may not recompile context, rejudge conflicts, or upgrade an advisory result to authoritative. CI must not depend on the journey projection to compute its verdicts.

### Worktree, isolation, and capability mechanics

CI sealed execution and CSE candidate execution both need isolated workspaces and capability enforcement. Their focused plans must name a common low-level boundary before either adds feature-specific behavior:

- CI owns the authority claim that an agent run was project-sealed and the vendor base remained opaque.
- CSE owns common-base candidate materialization, protected comparison inputs, evaluator capabilities, and experiment environment fingerprints.
- Shared Git/worktree primitives may be reused; context receipts and experiment recommendations remain separate proof types.

### Provenance and build context

ASD design provenance is committed, non-authoritative, and excluded from normal build context. The context compiler must classify that sidecar explicitly rather than silently include it as authority or silently omit it from the candidate universe. The ASD core plan must publish the sidecar identity and exclusion contract before the context compiler finalizes its input classifier.

### Application core, transports, and presentation

Each feature establishes one typed application core before CLI, MCP, or workbench adapters. Adapters never independently derive semantic state. All UI implementation waits until the relevant record schemas and core operations are approved, then runs serially under FABLE ownership.

## Execution surfaces and security

### Local lane

Use one local Claude Code session for work that depends on uncommitted files, a local worktree, workspace-root instructions, or local-only agent configuration. The current Task 8 merge-gate work remains in this lane until its branch is committed, pushed, independently reviewed, and merged. Do not run a second session against the same worktree.

The local lane may continue unattended for bounded repository work, but this plan does not use Claude Code Remote Control. No Claude process exposes a remotely controlled local shell, and no cloud session receives access to local GitHub credentials, SSH agents, unrelated MCP servers, sibling worktrees, or workspace-private files.

### Claude Code web lanes

Use Claude Code web for repository-contained units whose full authority, plan, and starting state are committed and reachable from GitHub. Each web session receives one branch, one delivery unit, and one explicit base SHA. A web session must not rely on:

- uncommitted local changes;
- the workspace-root `PLAN.md` unless the applicable entries are copied into a committed, repository-visible authority packet;
- local `.claude/agents/` files or remembered model behavior;
- credentials, MCP servers, fixtures, or tools absent from the repository; or
- another Claude session's transcript or unpushed branch.

Cloud branches may be prepared concurrently, but they become merge-eligible only after rebasing onto every merged prerequisite and rerunning their complete gates. A cloud session stops instead of inventing missing successor authority or silently substituting its own orchestration model.

### Owner-only control plane

The human owner alone performs actions whose result changes authority or repository enforcement:

- merge a pull request or accept a specification by merge;
- activate, weaken, remove, or repair a GitHub ruleset or required check;
- approve a residual disclosure, waiver, attestation, or finding disposition;
- authorize a release or integrated rollout; and
- opt into any future remote-control mechanism.

Claude Code may prepare exact commands, payloads, diffs, rollback instructions, and read-only verification evidence for these actions. It may not execute the mutation or interpret owner silence as approval.

### Self-contained cloud dispatch packet

Every cloud session starts with a packet containing all of the following:

| Field | Required value |
|---|---|
| Role | `FABLE chief architect and final producer-side judge`; named Sonnet implementation and Opus verification/fix responsibilities |
| Authority | Exact accepted spec paths, proposal or landing commits, applicable decisions, and repository-visible invention-ledger entries |
| Git topology | Repository, base SHA, branch name, delivery-unit slug, and allowed worktree scope |
| Ownership | Files and semantic seams the unit owns; overlapping files and authority actions it must not touch |
| Plan | Focused plan path and commit, or a planning-only brief that explicitly forbids runtime implementation |
| Gates | Exact targeted red/green commands, `make verify`, `go test -race ./... -count=1`, and required browser or CLI end-to-end commands |
| Handoff | Exact head SHA, changed-file list, commit-to-task map, requirement coverage, observed command output, and three-valued disclosures |
| Stop rules | Missing authority, stale base, shared-file collision, ambiguous semantics, unavailable evidence, failing gate, or requested owner-only action |

No dispatch packet says "continue from prior context" or assumes access to a local conversation. If the packet cannot name a committed source for a requirement, the session is planning-only and must report the missing authority.

## Ten-hour acceleration window

The acceleration window spent Claude capacity on independent, durable outputs
without violating dependency gates. Phases A and B below preserve the
historical assignments that produced the current authority; they do not
override the live global routing rules for future work. Relative phase times
were targets rather than authority deadlines.

### Phase A — protect the critical path and prepare successors

Run these lanes concurrently:

| Lane | Surface | Assignment | Output permitted before Task 8 lands |
|---|---|---|---|
| L0 | Local | Complete Task 8's workflow implementation and evidence under the existing FABLE-led workflow | One focused enforcement PR; no ruleset mutation |
| W1 | Web | Read-only Task 9 readiness audit against PR #258 and the accepted merge-signaled design | Rebase/diff checklist and stop-condition report; no branch mutation based on an unmerged prerequisite |
| W2 | Web | Prepare ASD canonical-promotion mapping | Planning packet mapping every reviewed design decision into one canonical proposal artifact; no runtime code |
| W3 | Web | Prepare CSE canonical-promotion mapping | Planning packet mapping every reviewed design decision into one canonical proposal artifact; no runtime code |
| W4 | Web, only if capacity remains | Cross-feature authority audit for `governance-principal-kernel`, shared isolation, and ASD provenance classification | Contradiction/ownership matrix only; no implementation plan approval or runtime code |

Planning lanes may run in parallel because they own separate reports and make no lifecycle mutation. They must not edit this orchestration index or a shared registry. Keep no more than four planning-only cloud sessions active at once, reduce that count if subscription contention slows the critical L0 lane, and close a session as soon as its durable packet is pushed.

### Phase B — land enforcement and normalize the four proposals

1. Codex independently reviews Task 8's exact head.
2. Claude Code repairs accepted findings and obtains exact-head re-review.
3. The owner merges the Task 8 workflow PR.
4. The owner reads the live ruleset, performs the narrowly reviewed mutation, proves the required `merge-gate` in both green and red directions, and records rollback evidence.
5. Dispatch Task 9 to one web lane from the resulting `main`: rebase PR #258, remove only the two legacy `status: draft` fields, run all gates, and assemble its handoff.
6. Codex reviews PR #258's exact head; Claude Code repairs findings; the owner merges once. That merge accepts the two canonical proposals and ratifies the two design authorities without a second acceptance ceremony.

Task 9 may run in Claude Code web because its starting point, branch, authority, and expected mutations are GitHub-visible after Task 8. Its session must receive the local agent-role split explicitly in its dispatch packet.

### Phase C — convert prepared work into mergeable Wave 0 units

After Task 9 merges, revalidate W2, W3, and W4 against the actual landing commit. Then:

1. Run ASD and CSE canonical-promotion units concurrently only if their changed-file inventories are disjoint and neither edits a shared index or schema registry.
2. Give each unit its own pull request and owner merge. The main agent directly
   authors each substantial canonical promotion, includes its complete
   source-coverage/losslessness witness, and obtains one independent
   cross-model exact-head review under the global spec-only rule.
3. Incorporate the W4 ownership matrix into repository-visible successor authority and resolve every contradiction before approving the focused `governance-principal-kernel` plan.
4. Draft the kernel plan from the fully accepted four-spec base. Do not begin kernel implementation until Wave 0's exit gate is satisfied.

### Phase D — spend remaining capacity without outrunning authority

If time remains after Wave 0:

- start `governance-principal-kernel` implementation as the only shared-authority unit;
- use spare web capacity for read-only AC-to-test matrices and file-ownership reconnaissance for the next eligible units;
- do not begin `policy-authority`, ASD runtime, GLG runtime, CSE runtime, shared isolation, context compilation, or workbench code ahead of their merged prerequisites; and
- end every active session with a pushed branch or committed planning packet, exact base/head SHAs, a clean-worktree statement, and explicit next gate. Unpushed chat conclusions are not progress.

The phase-specific concurrency table in [How to use this index](#how-to-use-this-index)
supersedes the former global ceiling of two. During the shared-kernel phase the
ceiling remains one because the kernel controls types and semantics consumed by
both CI and GLG. Read-only analysis may use spare sessions, but it cannot grant
Gate P or Gate I.

## Delivery waves

### Wave 0 — Ratify authority and planning boundaries

- [ ] Review all four documents together for contradictions, missing cross-links, and scope overlap.
- [ ] Ratify merge-signaled specification acceptance: proposal during review, merge as the solo operator's single acceptance ceremony, and Git-derived effective state afterward.
- [ ] Deliver and require one stable merge gate before any canonical proposal lands; retire `VL-004`'s rejection of a proposal merely because its pull request targets the default branch.
- [ ] Inventory every human ceremony in the successor lifecycle and classify it as substantive judgment, existing forge authorization, deterministic materialization, exceptional override, or removable acknowledgement.
- [ ] Ratify this index as the successor orchestration authority for the four features and update the workspace/repository instructions to name it.
- [ ] Establish a repository-visible successor invention ledger or explicitly retain `PLAN.md` section 7 with a portable access path for every implementation worktree.
- [ ] Promote ASD and CSE into canonical feature proposal artifacts without losing reviewed decisions or adding new semantics.
- [ ] Merge all four reviewed canonical feature specifications through the ratified merge-signaled lifecycle; their merges are acceptance and require no second human action.
- [ ] Ratify `governance-principal-kernel` as shared prerequisite work.
- [ ] Ratify the reusable worktree/isolation boundary between CI and CSE.
- [ ] Ratify the ASD provenance-sidecar classification consumed by the CI context compiler.
- [ ] Record any new invention in `PLAN.md` section 7 with its spec citation and smallest reversible choice.

**Exit gate:** Merge-signaled acceptance and its required gate are live; four canonical feature specifications are accepted by reviewed merge; all retained human ceremonies carry distinct judgment or safety purpose; cross-feature conflicts and shared schema ownership are resolved.

### Wave 1 — Shared authority foundation, solo

- [ ] Deliver `governance-principal-kernel`.
- [ ] Deliver CI `policy-authority`, including the human-artifact kernel, shared renderer, typed constraint catalogs, projection rules, and prospective adoption behavior.
- [ ] Prove legacy stores retain existing behavior when the constitution capability is not adopted.

**Exit gate:** One strict schema and resolver for profiles and principals; one shared human-artifact/template seam; deterministic policy artifacts and projection checks; full gates green.

### Wave 2 — First domain cores, maximum two concurrent units

Track A:

- [ ] Deliver ASD's atomic draft-mutation core and structured CLI adapter.
- [ ] Establish the committed provenance-sidecar schema, transaction recovery contract, policy modes, semantic diff, and build-context exclusion identifier.

Track B:

- [ ] Deliver GLG `journey-projection` as a read-only projection over existing lifecycle sources.
- [ ] Deliver GLG `obligation-quality`, keeping unresolved scaffolds visible and blocking at authoritative build.

After either track merges, the next eligible unit may start:

- [ ] Deliver the CSE versioned artifact schemas and closed deterministic recommendation engine without execution or UI adapters.

**Exit gate:** ASD mutations are atomic and provenance-bound; journey records and obligation blockers are deterministic; CSE can evaluate complete canned observations into byte-identical results; no adapter owns independent semantics.

### Wave 3A — Compilation and conflict boundaries

Track A, sequential:

- [ ] Deliver CI `context-compiler`, including explicit ASD provenance exclusion and complete included/excluded/opaque ledgers.
- [ ] Deliver CI `policy-conflict-gate`, including mechanical proof, semantic candidates, dispositions, exemptions, staleness, and one shared verdict.


**Exit gate:** Context compilation is byte-deterministic; conflict unknowns
block; every lifecycle consumer uses the one shared conflict verdict.

### Wave 3.5 — Bounded UX readiness pilot

- [x] Ratify the sequencing overlay and lossless carve-out matrix in the
  [Wave 3.5 UX readiness pilot plan](2026-08-18-wave-3-5-ux-readiness-pilot.md).
- [x] Deliver a pure read-only readiness projection and an explicit startup
  snapshot through `verdi serve --context-request <path>`.
- [x] Deliver the FABLE-owned hybrid cockpit: a four-area linear process rail,
  prioritized attention queue, complete concern list, board deep links, and
  exact CLI fallbacks.
- [x] Run the solo-author pilot and record comprehension, navigation,
  terminology, unnecessary-process, stale-state, and missing-corrective-seam
  findings without treating telemetry as lifecycle truth.
- [x] Adjudicate every accepted finding to Wave 3.5, its original downstream
  wave, or an explicit non-goal.

The pilot adds no workflow engine, persisted readiness artifact, cockpit
mutation, controlled execution, or favorable interpretation of unknown facts.
It is not a substitute for GLG `continuous-readiness`, GLG `journey-metrics`,
the complete Wave 6 workbench, or any Wave 7 dogfood journey.

**Exit gate:** The immutable snapshot is exact and stale-by-design; the cockpit
never hides an unresolved fact; the solo-author evaluation is recorded and its
findings are mapped; the owner has accepted the Wave 3B re-entry decision.

### Wave 3B — CSE evaluator and isolated execution boundaries

Eligible only after the Wave 3.5 exit gate:

- [ ] Ratify the prerequisite evaluator, observation/result V2, multi-run, and
  durable-receipt authority fixed by the focused
  [Wave 3B CSE implementation plan](2026-08-20-cse-wave-3b-evaluator-isolated-execution.md).
- [ ] Deliver CSE evaluator capability discovery, generic command evaluator,
  process observer, candidate materialization, deterministic scheduling,
  interruption resume, and retention behavior according to that plan.

**Exit gate:** Experiments distinguish candidate verdicts from operational
failures; changed inputs refuse resume; unsupported isolation remains an
operational refusal rather than ambient execution; exact-head Linux CI proves
the real default-deny journey while Darwin/other platform tests prove refusal
without skipping.

### Wave 4 — Authoritative execution and accountable lifecycle governance

Serialize changes that touch shared identity, authority, receipts, or human-record schemas:

- [ ] Deliver CI `sealed-codex-execution`.
- [ ] Deliver CI `context-receipts-review` with a fresh independent review capsule and authenticated managed-runner trust.
- [ ] Deliver GLG `lifecycle-governance` consumers over the shared kernel.
- [ ] Deliver GLG `accountable-human-records`.
- [ ] Deliver GLG `committed-authority`.

**Exit gate:** No authoritative run launches without proven isolation; no authoritative transition consumes uncommitted human bytes; builder and reviewer receipts remain separate and fresh; experimental posture cannot mint authoritative proof.

### Wave 5 — Adapters and remaining non-UI lifecycle behavior

Up to three units may run in parallel only after a file/package ownership
check. Wave 3.5's provisional readiness and pilot-observation slices do not
remove the full `continuous-readiness` or `journey-metrics` obligations here:

- [ ] Deliver ASD capability/context reads, MCP adapter, proposal-only dogfood, draft-write, semantic review, and provenance read paths.
- [ ] Deliver CSE CLI and agent adapters, registration including any explicit
  reproduction rule, ratification record, spike-closure integration, cleanup,
  and selected-capsule retention.
- [ ] Deliver GLG `continuous-readiness` and feature-attestation scaffolding without agent-authored human claims.
- [ ] Deliver GLG `lifecycle-recovery` as diagnosis-first, read-only-by-default projection over observable state.
- [ ] Deliver GLG `journey-metrics` only after stable action, blocker, and outcome-event identifiers exist.

**Exit gate:** Every adapter passes conformance against its application core; human-only actions are absent or refused through every agent surface; recovery never guesses; metric events never drive lifecycle truth.

### Wave 6 — Workbench presentation, fully serialized

Each UI unit receives its own FABLE-owned delivery unit and pull request. Use a
focused implementation plan only when the unit's complexity materially
benefits from one:

- [ ] ASD board synchronization, unsaved-edit protection, semantic review, and on-demand provenance.
- [ ] CI Constitution rule ledger, derivation trail, impact review, and Git-backed proposal workflow.
- [ ] GLG journey, readiness, attestation, and recovery projections.
- [ ] CSE registration lock, execution state, result explanation, and ratification surfaces.

The Wave 3.5 cockpit shell and solo-author navigation language are promoted
inputs to this wave, not a claim that the GLG workbench unit is complete. Live
refresh, broader roles, authoritative actions, recovery UX, accessibility,
responsive behavior, performance, and complete lifecycle presentation remain
here.

**Exit gate:** All browser paths consume the same records as CLI and MCP, expose repository and authority posture, have keyboard-accessible Playwright coverage, and create no UI-only authority or shadow database.

### Wave 7 — Dogfood and whole-program approval

The ASD, CSE, and CI dogfood journeys may run concurrently when their fixtures,
ports, and output paths are disjoint. The GLG end-to-end journey begins after
their facts are available; final gates, whole-program review, and human
authorization are serialized.

- [ ] Run one ASD journey through both Claude Code and Codex using the same typed mutation contract.
- [ ] Run one genuine CSE comparison and preserve a scoped recommendation or
  honest inconclusive result; call it reproduced only when Wave 5 registered
  and the journey satisfied an exact reproduction rule.
- [ ] Run one CI sealed build and fresh Codex review, disclosing the opaque vendor boundary.
- [ ] Run the GLG journey across design, acceptance, build, evidence, review, recovery, and closure states.
- [ ] Run `make verify` and `go test -race ./...` from the final integrated tree.
- [ ] Start a fresh Codex whole-branch review from the accepted specifications, final diff, evidence bundles, and implementation receipts without builder conversations.
- [ ] Obtain human authorization before merge or release.

**Exit gate:** Every feature AC has fresh evidence, every disclosure is explicit, no important Codex finding remains, and the human owner approves the integrated result.

## Wave orchestration and review cadence

A delivery wave is one orchestration campaign, not one branch or pull request.
The main agent carries the wave from its entry conditions through its exit
gate and keeps the dependency order, concurrency ceiling, shared ownership,
and accumulated evidence visible across the campaign.

Each named delivery unit remains a coherent branch and pull request. Internal
tasks and commits receive author verification, TDD where applicable, and the
unit's required gates, but they do not trigger automatic independent reviews.
When the complete delivery unit is ready, one independent reviewer challenges
its consolidated exact head. If the author accepts findings and changes that
head, the same reviewer performs one closure check. Do not create a
task-review-task-review loop, an automatic third review round, or a separate
review event merely because an implementation plan had multiple steps.

After every delivery unit in a wave has merged, the main agent performs one
integration review against the resulting default-branch head. That review is
limited to cross-unit contracts, regressions introduced by composition, and
the wave's stated exit gate; it does not repeat each closed pull-request review
line by line. A concrete integration defect becomes a focused correction unit.
Wave 7 retains its fresh whole-program review and human authorization boundary.

## Per-unit pull request protocol

Every delivery unit uses this Git topology:

```text
current main after dependencies
    └── feature/delivery-unit-slug
          └── draft PR to main
```

Do not build all four features on long-lived branches and merge them at the end. Merge approved prerequisites serially so later plans assess the real repository state. Rebase or recreate an unstarted branch after each dependency merge; never resolve semantic conflicts by choosing whichever branch version applies cleanly.

The pull request remains draft until its author has completed the applicable
producer-side review or direct-author verification and attached fresh gate
evidence.
Independent review is requested against an exact head commit. A pushed
correction invalidates the initial review and permits the one closure check
defined above; it does not begin a repeated review chain.

The branch must contain only its declared delivery unit. Generated data under `.verdi/data/`, exploratory prototypes, local profiles, `.DS_Store`, transcripts, and reviewer scratch files never enter the commit.

## Producer-to-reviewer handoff contract

Every reviewed delivery-unit pull request body includes these sections and
concrete values from the actual branch:

| Section | Required content |
|---|---|
| Authority | Accepted feature ref and accepted baseline identity — spec path, blob OID, and first-parent landing commit; legacy frozen commit only when applicable; delivery-unit slug; approved plan path and commit; full base and head SHAs |
| Requirement coverage | One row per applicable AC, decision, and constraint, naming its implementation seam and test or evidence artifact |
| Verification | Commands and observed red/green results; complete `make verify` result; complete `go test -race ./...` result |
| Disclosures | Proven claims with witnesses; violations with witnesses; unproven facts and authority effect; applicable `PLAN.md` section 7 entries or the literal value `none` |
| Review scope | Complete changed-file list; explicit exclusions; revert boundary or backward-compatibility posture |

The authoring agent fills every applicable field before requesting review.
Missing rows or evidence remain unproven and keep the pull request in draft.

The independent reviewer receives only the accepted specifications, any
applicable approved plan, PR diff, handoff record, canonical evidence, and
repository instructions. The reviewer does not receive the author's transcript
or hidden reasoning. For a canonical promotion, Requirement coverage includes
the source-coverage/losslessness witness, its transformations and omissions,
and the reported total.

## Review and merge gates

### Gate D — Design ready

- The feature exists as a canonical accepted Verdi artifact.
- Acceptance criteria, decisions, constraints, stubs, links, and non-goals are internally consistent.
- Shared ownership is explicit and no duplicate schema is permitted.

### Gate P — Plan approved

- This gate applies only when a focused plan materially benefits the unit; a
  mechanical specification or documentation edit does not require one.
- A focused plan names exact files and interfaces after assessing the current base commit.
- Every spec requirement maps to a task and test.
- The plan contains no unresolved placeholders, speculative abstractions, or undeclared dependencies.
- The main agent adjudicates the plan against binding authority before
  implementation begins. Gate P does not create a separate automatic
  cross-model review; the implemented delivery unit receives the consolidated
  Gate C review.

### Gate I — Producer complete

- This gate applies to implementation-heavy work, not spec-only authority.
- The producer has completed the coherent delivery unit, its self-review, and
  its required targeted and full gates. Internal task completion does not
  require an independent reviewer loop.
- The branch contains small, intentional commits and no unrelated changes.
- The handoff packet contains fresh command evidence and three-valued disclosures.

### Gate C — Independent review approved

- Implementation-heavy work receives a fresh read-only Codex review of the
  consolidated exact head; its Claude Code producer repairs accepted findings.
  If that repair changes the head, the same reviewer performs one closure
  check. No task-level review chain or automatic third round follows.
- Substantial spec-only work receives exactly one read-only cross-model review
  of the exact head: Claude reviews Codex-authored work, and Codex reviews
  Claude-authored work. The author adjudicates and repairs accepted findings,
  and the same reviewer performs one closure check after at most one correction
  pass. No automatic third round follows.
- The independent reviewer verifies spec fidelity, regression risk, strict
  decoding, determinism, authority boundaries, failure classification, tests,
  provenance, and—when applicable—the canonical-promotion losslessness
  witness.
- No Critical or Important finding remains. Minor findings are fixed or explicitly dispositioned by the human owner.

### Gate H — Human merge authorized

- Required CI checks pass on the approved head.
- The applicable independent review remains fresh for that head.
- The human owner accepts any residual disclosed uncertainty and performs or authorizes the merge.

GitHub review is the durable conversation and audit trail. MCP surfaces may expose typed Verdi context and domain operations, but they do not replace commits, review threads, CI status, or human merge authority.

## Stop conditions

An agent stops and reports a blocking witness when any of these conditions holds:

- A feature remains a design document or draft rather than accepted canonical authority.
- The active agent instructions still restrict implementation to the completed v0 build contract or cannot resolve the successor invention ledger.
- A required dependency has not merged or its approval applies to another commit.
- Two features claim ownership of the same schema, resolver, state transition, proof type, or application operation.
- A spec ambiguity changes proof meaning, authority, lifecycle state, or public interface and lacks a `PLAN.md` section 7 decision.
- The working tree contains unrelated user changes or generated/private data that cannot be safely separated.
- A test requires network access, unverifiable identity, missing managed-runner trust, or unavailable isolation and the active posture does not permit an honest advisory result.
- `make verify` or `go test -race ./...` fails.
- Required independent-review evidence is stale against the current head.
- The requested action would accept, approve, attest, waive, disposition, or merge on behalf of the human owner.

## Completion ledger

This table enumerates the waves and their delivery units. It carries no status
columns and records no progress. Completion is derived at read time from Git
and GitHub facts: the merged pull requests that name a delivery unit, the
applicable exact-head independent-review evidence on those heads, their
required check runs, and `verdi spec state` for the specifications involved.
Never hand-edit this ledger to record plan review, implementation, review, or
merge progress; a hand-maintained copy of that state would duplicate
bookkeeping, contradict the derived facts, and collide with concurrent work on
one shared file.

| Wave | Delivery unit |
|---|---|
| 0 | Four-spec ratification and cross-feature decisions |
| 1 | `governance-principal-kernel` |
| 1 | CI `policy-authority` |
| 2 | ASD draft-mutation core and CLI |
| 2 | GLG `journey-projection` |
| 2 | GLG `obligation-quality` |
| 2 | CSE schemas and decision engine |
| 3A | CI `context-compiler` |
| 3A | CI `policy-conflict-gate` |
| 3.5 | UX readiness projection, cockpit, and solo-author pilot |
| 3B | CSE evaluator and isolated execution |
| 4 | CI `sealed-codex-execution` |
| 4 | CI `context-receipts-review` |
| 4 | GLG `lifecycle-governance` |
| 4 | GLG `accountable-human-records` |
| 4 | GLG `committed-authority` |
| 5 | ASD adapters and review/provenance paths |
| 5 | CSE adapters, ratification, and retention |
| 5 | GLG `continuous-readiness` |
| 5 | GLG `lifecycle-recovery` |
| 5 | GLG `journey-metrics` |
| 6 | ASD workbench |
| 6 | CI `constitution-workbench` |
| 6 | GLG workbench journeys |
| 6 | CSE workbench |
| 7 | Integrated dogfood and whole-branch approval |
