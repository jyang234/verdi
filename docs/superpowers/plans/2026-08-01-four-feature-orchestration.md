# Four-Feature Orchestration Plan

> **For agentic workers:** This file is the sequencing index, not a license to implement directly. Before changing runtime code, create and obtain review of a focused implementation plan for the named delivery unit. Use `superpowers:subagent-driven-development` to execute approved plans task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ratify and deliver four related Verdi features without duplicating authority schemas, creating competing workflow state, or allowing implementation and review contexts to contaminate one another.

**Architecture:** Git and GitHub pull requests are the durable coordination layer. Claude Code produces implementation branches and evidence; a fresh Codex context reviews each plan, story diff, and whole-feature result; the human owner alone accepts specifications and authorizes merges. Shared contracts land before feature-owned consumers, backend record shapes land before adapters, and all workbench presentation work is serialized after the underlying records stabilize.

**Tech Stack:** Go, filesystem-backed Verdi artifacts, Git worktrees, GitHub pull requests, Claude Code implementation subagents, Codex independent review, hermetic Go tests, Playwright, `make verify`, and `go test -race ./...`.

## Global Constraints

- The binding system semantics remain `docs/design/specs/00..05-*.md`; `docs/design/specs/08-revision-notes.md` remains the ratification history.
- The four documents indexed here are draft design authority until the human owner completes their applicable document-review and Verdi acceptance rituals.
- Never edit frozen artifacts or binding system specs directly. Ratified semantic changes follow the existing amendment flow.
- Never import `verdi-go` packages. Execute its pinned CLIs and strict-decode their JSON outputs.
- Preserve three-valued honesty: proven, violated with a witness, or explicitly disclosed as unproven. Missing context, identity, forge state, evidence, or review is never a pass.
- Preserve the exit contract: `0` clean, `1` verdict failure, `2` operational failure.
- Strict-decode YAML through `internal/artifact`; strict-decode JSON with unknown-field and trailing-data rejection; fail closed on unknown enum values.
- Canonical outputs are deterministically sorted, digest-bound, and free of incidental wall-clock or random values except declared provenance stamps.
- Every behavior change follows TDD. Every function receives happy and negative unit coverage; integrations use hermetic fakes; browser-facing paths receive Playwright coverage.
- No test uses the network. Every delivery unit closes with fresh `make verify` and `go test -race ./...` evidence.
- Frontend implementation and fixes are assigned to a FABLE subagent. Non-frontend implementation is assigned to the Sonnet implementation subagent. Review findings and gate failures are repaired by the Opus fixer.
- Claude Code never accepts a specification, approves a pull request, merges a branch, authors a human attestation or waiver, or dispositions a judgment-bearing finding.
- Codex reviews are read-only. Codex does not repair its own findings; accepted findings return to Claude Code and receive a fresh Codex re-review.
- Only the human owner may approve the final merge.
- Until a successor build contract explicitly relocates it, unresolved semantic ambiguities are recorded in `PLAN.md` section 7 and block implementation rather than being silently resolved.
- This specification-index pull request does not override the workspace's current v0-only build instructions. Wave 0 must make the successor scope and invention-ledger location repository-visible before any runtime implementation begins.

---

## Contents

1. [How to use this index](#how-to-use-this-index)
2. [Scope boundaries](#scope-boundaries)
3. [Specification index](#specification-index)
4. [Shared ownership boundaries](#shared-ownership-boundaries)
5. [Delivery waves](#delivery-waves)
6. [Per-unit pull request protocol](#per-unit-pull-request-protocol)
7. [Claude-to-Codex handoff contract](#claude-to-codex-handoff-contract)
8. [Review and merge gates](#review-and-merge-gates)
9. [Stop conditions](#stop-conditions)
10. [Completion ledger](#completion-ledger)

## How to use this index

This document answers which contract lands before another contract, which work may run concurrently, and which evidence permits the next transition. It deliberately does not prescribe package names, Go types, or test bodies before the corresponding feature plan has assessed the current codebase at its starting commit.

For each delivery unit:

1. Start from the current default branch after every listed dependency has merged.
2. Create one isolated Git worktree and one branch for that unit.
3. Write a focused plan under `docs/superpowers/plans/` with exact files, interfaces, failing tests, commands, and commits.
4. Obtain a read-only Codex plan review before implementation.
5. Let Claude Code execute the approved plan with the repository-defined model split.
6. Open a draft pull request and attach the handoff evidence defined below.
7. Obtain a fresh Codex review, return findings to Claude Code, and repeat until the exact head commit is approved.
8. Let the human owner merge only after required CI and governance checks pass.

At most two implementation units may be active concurrently. Parallel units must have merged prerequisites, disjoint semantic ownership, and no planned edits to the same shared package, schema registry, `Makefile`, workbench assets, or Playwright path. When any of those conditions is false, serialize the units.

## Scope boundaries

This pull request indexes and sequences the four specifications. It does not:

- accept or freeze any specification;
- promote ASD or CSE into canonical Verdi artifacts;
- change runtime code, schemas, CLI or MCP inventories, workbench assets, tests, dependencies, or CI;
- choose package names, Go interfaces, migrations, configuration keys, performance budgets, or rollout defaults for later implementation plans;
- replace GitHub reviews with an agent-to-agent MCP message bridge;
- alter the current v0 build instructions or silently authorize out-of-contract implementation; or
- perform any human-only governance or merge action.

The branch is independently reversible by reverting its documentation commit. Later units remain separately reversible because each owns one focused pull request and must state its compatibility and rollback posture before review.

Because this umbrella review contains active artifacts with `status: draft`, it is not mergeable as submitted. Keep it as a draft design-review pull request. Wave 0 must choose and execute a lifecycle-compliant landing path—separate acceptance pull requests or an explicitly ratified combined ritual—before any draft artifact reaches the default branch. The orchestration index may land with that ratification or in its own documentation pull request.

## Specification index

| Key | Specification | Current authority posture | Delivery units named by the specification | Entry gate |
|---|---|---|---|---|
| GLG | [Guided Lifecycle and Governance](../../../.verdi/specs/active/guided-lifecycle-governance/spec.md) | Canonical Verdi feature artifact, `status: draft` | `journey-projection`, `obligation-quality`, `lifecycle-governance`, `accountable-human-records`, `committed-authority`, `continuous-readiness`, `lifecycle-recovery`, `journey-metrics` | Human review and Verdi acceptance of the draft |
| CI | [Context Integrity and Constitutional Execution](../../../.verdi/specs/active/context-integrity/spec.md) | Canonical Verdi feature artifact, `status: draft` | `policy-authority`, `context-compiler`, `policy-conflict-gate`, `sealed-codex-execution`, `context-receipts-review`, `constitution-workbench` | Human review and Verdi acceptance of the draft |
| ASD | [AI-assisted spec design](../specs/2026-07-30-ai-assisted-spec-design.md) | Design document approved in session and awaiting document review; not yet a canonical Verdi feature artifact | Draft-mutation core, structured CLI, workbench migration, bounded context and capabilities, proposal-only dogfood, draft-write, review/provenance views, Codex/Claude conformance | Document review, promotion to a canonical feature artifact, and Verdi acceptance |
| CSE | [Comparative spike experiments](../specs/2026-07-30-comparative-spike-experiments-design.md) | Design document approved in session and awaiting document review; not yet a canonical Verdi feature artifact | Experiment schemas, decision engine, evaluator/observer, isolated execution and resume, CLI/workbench/agent adapters, policy, dogfood comparison | Document review, promotion to a canonical feature artifact, and Verdi acceptance |

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

## Delivery waves

### Wave 0 — Ratify authority and planning boundaries

- [ ] Review all four documents together for contradictions, missing cross-links, and scope overlap.
- [ ] Ratify this index as the successor orchestration authority for the four features and update the workspace/repository instructions to name it.
- [ ] Establish a repository-visible successor invention ledger or explicitly retain `PLAN.md` section 7 with a portable access path for every implementation worktree.
- [ ] Promote ASD and CSE into canonical draft feature artifacts without losing reviewed decisions or adding new semantics.
- [ ] Accept all four feature specifications through the existing human-governed lifecycle.
- [ ] Ratify `governance-principal-kernel` as shared prerequisite work.
- [ ] Ratify the reusable worktree/isolation boundary between CI and CSE.
- [ ] Ratify the ASD provenance-sidecar classification consumed by the CI context compiler.
- [ ] Record any new invention in `PLAN.md` section 7 with its spec citation and smallest reversible choice.

**Exit gate:** Four accepted canonical feature specifications, resolved cross-feature conflicts, accepted stubs, and no unowned shared schema.

### Wave 1 — Shared authority foundation, solo

- [ ] Plan and deliver `governance-principal-kernel`.
- [ ] Plan and deliver CI `policy-authority`, including the human-artifact kernel, shared renderer, typed constraint catalogs, projection rules, and prospective adoption behavior.
- [ ] Prove legacy stores retain existing behavior when the constitution capability is not adopted.

**Exit gate:** One strict schema and resolver for profiles and principals; one shared human-artifact/template seam; deterministic policy artifacts and projection checks; full gates green.

### Wave 2 — First domain cores, maximum two concurrent units

Track A:

- [ ] Plan and deliver ASD's atomic draft-mutation core and structured CLI adapter.
- [ ] Establish the committed provenance-sidecar schema, transaction recovery contract, policy modes, semantic diff, and build-context exclusion identifier.

Track B:

- [ ] Plan and deliver GLG `journey-projection` as a read-only projection over existing lifecycle sources.
- [ ] Plan and deliver GLG `obligation-quality`, keeping unresolved scaffolds visible and blocking at authoritative build.

After either track merges, the next eligible unit may start:

- [ ] Plan and deliver the CSE versioned artifact schemas and closed deterministic recommendation engine without execution or UI adapters.

**Exit gate:** ASD mutations are atomic and provenance-bound; journey records and obligation blockers are deterministic; CSE can evaluate complete canned observations into byte-identical results; no adapter owns independent semantics.

### Wave 3 — Compilation, conflict, and isolated execution boundaries

Track A, sequential:

- [ ] Plan and deliver CI `context-compiler`, including explicit ASD provenance exclusion and complete included/excluded/opaque ledgers.
- [ ] Plan and deliver CI `policy-conflict-gate`, including mechanical proof, semantic candidates, dispositions, exemptions, staleness, and one shared verdict.

Track B, eligible after the shared isolation boundary is approved:

- [ ] Plan and deliver CSE evaluator capability discovery, generic command evaluator, process observer, candidate materialization, deterministic scheduling, interruption resume, and retention behavior.

**Exit gate:** Context compilation is byte-deterministic; conflict unknowns block; experiments distinguish candidate verdicts from operational failures; changed inputs refuse resume.

### Wave 4 — Authoritative execution and accountable lifecycle governance

Serialize changes that touch shared identity, authority, receipts, or human-record schemas:

- [ ] Plan and deliver CI `sealed-codex-execution`.
- [ ] Plan and deliver CI `context-receipts-review` with a fresh independent review capsule and authenticated managed-runner trust.
- [ ] Plan and deliver GLG `lifecycle-governance` consumers over the shared kernel.
- [ ] Plan and deliver GLG `accountable-human-records`.
- [ ] Plan and deliver GLG `committed-authority`.

**Exit gate:** No authoritative run launches without proven isolation; no authoritative transition consumes uncommitted human bytes; builder and reviewer receipts remain separate and fresh; experimental posture cannot mint authoritative proof.

### Wave 5 — Adapters and remaining non-UI lifecycle behavior

Units may run in parallel only after a file/package ownership check:

- [ ] Plan and deliver ASD capability/context reads, MCP adapter, proposal-only dogfood, draft-write, semantic review, and provenance read paths.
- [ ] Plan and deliver CSE CLI and agent adapters, ratification record, spike-closure integration, cleanup, and selected-capsule retention.
- [ ] Plan and deliver GLG `continuous-readiness` and feature-attestation scaffolding without agent-authored human claims.
- [ ] Plan and deliver GLG `lifecycle-recovery` as diagnosis-first, read-only-by-default projection over observable state.
- [ ] Plan and deliver GLG `journey-metrics` only after stable action, blocker, and outcome-event identifiers exist.

**Exit gate:** Every adapter passes conformance against its application core; human-only actions are absent or refused through every agent surface; recovery never guesses; metric events never drive lifecycle truth.

### Wave 6 — Workbench presentation, fully serialized

Each UI unit receives its own FABLE-owned plan and pull request:

- [ ] ASD board synchronization, unsaved-edit protection, semantic review, and on-demand provenance.
- [ ] CI Constitution rule ledger, derivation trail, impact review, and Git-backed proposal workflow.
- [ ] GLG journey, readiness, attestation, and recovery projections.
- [ ] CSE registration lock, execution state, result explanation, and ratification surfaces.

**Exit gate:** All browser paths consume the same records as CLI and MCP, expose repository and authority posture, have keyboard-accessible Playwright coverage, and create no UI-only authority or shadow database.

### Wave 7 — Dogfood and whole-program approval

- [ ] Run one ASD journey through both Claude Code and Codex using the same typed mutation contract.
- [ ] Run one genuine CSE comparison and preserve a scoped recommendation or honest inconclusive result.
- [ ] Run one CI sealed build and fresh Codex review, disclosing the opaque vendor boundary.
- [ ] Run the GLG journey across design, acceptance, build, evidence, review, recovery, and closure states.
- [ ] Run `make verify` and `go test -race ./...` from the final integrated tree.
- [ ] Start a fresh Codex whole-branch review from the accepted specifications, final diff, evidence bundles, and implementation receipts without builder conversations.
- [ ] Obtain human authorization before merge or release.

**Exit gate:** Every feature AC has fresh evidence, every disclosure is explicit, no important Codex finding remains, and the human owner approves the integrated result.

## Per-unit pull request protocol

Every delivery unit uses this Git topology:

```text
current main after dependencies
    └── feature/delivery-unit-slug
          └── draft PR to main
```

Do not build all four features on long-lived branches and merge them at the end. Merge approved prerequisites serially so later plans assess the real repository state. Rebase or recreate an unstarted branch after each dependency merge; never resolve semantic conflicts by choosing whichever branch version applies cleanly.

The pull request remains draft until Claude Code has completed producer-side review and attached fresh gate evidence. Codex review is requested against an exact head commit. Any pushed fix invalidates the prior approval and requires another review of the new head.

The branch must contain only its declared delivery unit. Generated data under `.verdi/data/`, exploratory prototypes, local profiles, `.DS_Store`, transcripts, and reviewer scratch files never enter the commit.

## Claude-to-Codex handoff contract

Every implementation pull request body includes these sections and concrete values from the actual branch:

| Section | Required content |
|---|---|
| Authority | Accepted feature ref and frozen commit; delivery-unit slug; approved plan path and commit; full base and head SHAs |
| Requirement coverage | One row per applicable AC, decision, and constraint, naming its implementation seam and test or evidence artifact |
| Verification | Commands and observed red/green results; complete `make verify` result; complete `go test -race ./...` result |
| Disclosures | Proven claims with witnesses; violations with witnesses; unproven facts and authority effect; applicable `PLAN.md` section 7 entries or the literal value `none` |
| Review scope | Complete changed-file list; explicit exclusions; revert boundary or backward-compatibility posture |

Claude Code fills every field before requesting review. Missing rows or evidence remain unproven and keep the pull request in draft.

Codex receives only the accepted specifications, approved plan, PR diff, handoff record, canonical evidence, and applicable repository instructions. It does not receive the Claude Code implementation transcript or hidden reasoning.

## Review and merge gates

### Gate D — Design ready

- The feature exists as a canonical accepted Verdi artifact.
- Acceptance criteria, decisions, constraints, stubs, links, and non-goals are internally consistent.
- Shared ownership is explicit and no duplicate schema is permitted.

### Gate P — Plan approved

- A focused plan names exact files and interfaces after assessing the current base commit.
- Every spec requirement maps to a task and test.
- The plan contains no unresolved placeholders, speculative abstractions, or undeclared dependencies.
- Codex issues `APPROVED` or all conditions are incorporated and re-reviewed.

### Gate I — Producer complete

- Claude Code's implementer and internal reviewer have completed their loop.
- The branch contains small, intentional commits and no unrelated changes.
- The handoff packet contains fresh command evidence and three-valued disclosures.

### Gate C — Independent review approved

- A fresh Codex context reviews the exact head read-only.
- Codex verifies spec fidelity, regression risk, strict decoding, determinism, authority boundaries, failure classification, tests, and provenance.
- Claude Code fixes accepted findings; Codex re-reviews the new head.
- No Critical or Important finding remains. Minor findings are fixed or explicitly dispositioned by the human owner.

### Gate H — Human merge authorized

- Required CI checks pass on the approved head.
- Codex approval remains fresh for that head.
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
- Codex review evidence is stale against the current head.
- The requested action would accept, approve, attest, waive, disposition, or merge on behalf of the human owner.

## Completion ledger

Update this table only with links to merged pull requests and fresh evidence. A blank result means not started, not passed.

| Wave | Delivery unit | Plan review | Implementation PR | Codex review | Human merge |
|---|---|---|---|---|---|
| 0 | Four-spec ratification and cross-feature decisions |  |  |  |  |
| 1 | `governance-principal-kernel` |  |  |  |  |
| 1 | CI `policy-authority` |  |  |  |  |
| 2 | ASD draft-mutation core and CLI |  |  |  |  |
| 2 | GLG `journey-projection` |  |  |  |  |
| 2 | GLG `obligation-quality` |  |  |  |  |
| 2 | CSE schemas and decision engine |  |  |  |  |
| 3 | CI `context-compiler` |  |  |  |  |
| 3 | CI `policy-conflict-gate` |  |  |  |  |
| 3 | CSE evaluator and isolated execution |  |  |  |  |
| 4 | CI `sealed-codex-execution` |  |  |  |  |
| 4 | CI `context-receipts-review` |  |  |  |  |
| 4 | GLG `lifecycle-governance` |  |  |  |  |
| 4 | GLG `accountable-human-records` |  |  |  |  |
| 4 | GLG `committed-authority` |  |  |  |  |
| 5 | ASD adapters and review/provenance paths |  |  |  |  |
| 5 | CSE adapters, ratification, and retention |  |  |  |  |
| 5 | GLG `continuous-readiness` |  |  |  |  |
| 5 | GLG `lifecycle-recovery` |  |  |  |  |
| 5 | GLG `journey-metrics` |  |  |  |  |
| 6 | ASD workbench |  |  |  |  |
| 6 | CI `constitution-workbench` |  |  |  |  |
| 6 | GLG workbench journeys |  |  |  |  |
| 6 | CSE workbench |  |  |  |  |
| 7 | Integrated dogfood and whole-branch approval |  |  |  |  |
