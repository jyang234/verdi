---
id: spec/uat-round-1
kind: spec
title: "UAT Round 1"
owners: [platform-team]
class: feature
problem: { text: "A first real-project UAT pass (importing, populating, accepting, and superseding the verdi-atc F13 gatekeeper feature) surfaced twenty-two product observations: defects in CLI ergonomics, lint presentation, import provenance and mapping, design-start branching, and readiness rules, plus contract gaps that make the spike and supersession models unusable in practice.", anchor: problem }
outcome: { text: "The behaviors that distorted authoring in the UAT pass are corrected on main with regression witnesses: builds identify themselves, help works, lint notices are accurate and quiet, import records name their profile, design start bases on the default branch and can start a superseding revision, spike-claimed open questions no longer block acceptance, and every contract change is ratified before its code lands. The same UAT scenario replays clean against the new build.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "`verdi version` and `verdi --version` print the module version and VCS revision from build info and exit 0; `verdi serve` prints the same string at startup and the workbench renders it in its page footer.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "`help`, `--help`, and `-h` are recognized at top level and after any verb: they print multi-line usage with one line per verb or subverb and exit 0, and never execute the verb. `verdi lint --help` no longer runs a lint.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "When the mutable zone is absent, lint prints the VL-017 disclosure once per run, listing the affected specs, and describes the condition as the mutable zone being absent from this checkout. It does not assert a bare clone. The check's semantics are unchanged.", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "A spec-import record persists the request format and, for a reference profile, the pinned primary digest it was bound to; the record page and `verdi design import record` display them.", evidence: [behavioral, attestation], anchor: ac-4 }
  - { id: ac-5, text: "Policy authority and policy-forbidden error strings carry their package prefix exactly once.", evidence: [behavioral, attestation], anchor: ac-5 }
  - { id: ac-6, text: "`verdi design start`, including `--from-stub`, bases the new design branch on the resolved default branch, not the current HEAD, and prints the base commit and the fact that the primary checkout was switched. When the default branch is unresolvable it follows dc-7: a disclosed HEAD base when no origin remote exists, an operational refusal when one does.", evidence: [behavioral, attestation], anchor: ac-6 }
  - { id: ac-7, text: "The `f13-reference-v1` field map selects whole sentences: no selector ends mid-sentence, begins mid-clause, or joins two claims, and the test-instruction sentence is not selected as an acceptance criterion. The profile test is re-pinned and an import of the pinned F13 bundle yields the corrected criteria.", evidence: [behavioral, attestation], anchor: ac-7 }
  - { id: ac-8, text: "In the workbench import dialog, selecting text in a rendered source creates a source-backed mapping with the byte range computed, and the mapping target is chosen from a picker that offers the problem and outcome statements and all four object kinds.", evidence: [behavioral, attestation], anchor: ac-8 }
  - { id: ac-9, text: "The board's commit dialog prefills a proposal-shaped message naming the spec, keeps it editable, and shows a non-blocking note when the message contains a lifecycle word (accepted, closed, merged, superseded) that the commit itself cannot make true.", evidence: [behavioral, attestation], anchor: ac-9 }
  - { id: ac-10, text: "A declared open question that a spike stub claims through `resolves` is a non-blocking, later-timed readiness concern rather than a blocking one, so a feature can be accepted with such a question open; an unclaimed open question remains blocking.", evidence: [behavioral, attestation], anchor: ac-10 }
  - { id: ac-11, text: "`verdi design start --supersedes spec/<name> --name <new>` scaffolds a successor that carries the predecessor's objects and stubs verbatim, a `supersedes` link, and a `supersession:` block classifying every predecessor object as carried; lint accepts the untouched scaffold under VL-015, and the board of an accepted spec offers a Revise action that invokes the same operation.", evidence: [behavioral, attestation], anchor: ac-11 }
constraints:
  - { id: co-1, text: "No network in any test. CLI paths are exercised through the built binary; browser paths through Playwright under e2e/; forge, git, and provider interactions against hermetic fakes.", anchor: co-1 }
  - { id: co-2, text: "Every verb keeps the exit discipline: 0 clean, 1 verdict, 2 operational. Help and version are clean exits; usage on an unknown verb stays operational.", anchor: co-2 }
  - { id: co-3, text: "No behavior that changes a ratified contract lands before its invention-ledger entry is recorded in PLAN.md §7 and ratified by the owner, and the change is entered in 08-revision-notes. This binds ac-10 and ac-11.", anchor: co-3 }
  - { id: co-4, text: "All workbench and board work (ac-1 footer, ac-8, ac-9, the Revise action of ac-11) is implemented by Fable subagents and carries a Playwright path. Screenshot, trace, video, and screen recording stay disabled.", anchor: co-4 }
  - { id: co-5, text: "Three-valued honesty is preserved in every notice this round touches: a quieter disclosure is still a disclosure, never a vacuous pass.", anchor: co-5 }
decisions:
  - { id: dc-1, text: "Scope is the UAT tracker's defect, data, and two highest-leverage contract items (UAT-002, 003, 004, 005, 006, 007, 017, 019, 020, 021, 022). The remaining contract findings (UAT-001, 008, 009, 010, 011, 018) are deferred to a contract-amendment round and are not open questions of this feature. UAT-012 is excluded until reproduced against two identified builds.", anchor: dc-1 }
  - { id: dc-2, text: "The ac-6 fix keeps design start's checkout-switching behavior and makes it explicit; moving design start onto managed worktrees is out of scope and belongs with the worktree-manager design.", anchor: dc-2 }
  - { id: dc-3, text: "VL-017 keeps its rule and severity. A spec with no declared open questions still receives the disclosure because an uncarried annotation could exist in the absent zone; only presentation changes (ac-3).", anchor: dc-3 }
  - { id: dc-4, text: "The F13 reference profile remains a named, pinned reference profile bound to the exact primary bytes. ac-7 revises its shipped field map and re-pins the test; it does not introduce generic prose splitting.", anchor: dc-4 }
  - { id: dc-5, text: "Risk tiers for orchestration: Tier 1 for ac-2, ac-3, ac-5; Tier 2 for ac-1, ac-4, ac-6, ac-7, ac-8, ac-9; Tier 3 with an owner risk gate for ac-10 (acceptance gating) and ac-11 (provenance and immutable history). Waves: 1 = ac-1..ac-6 in parallel; 2 = ac-7, ac-8, ac-9; 3 = ac-10, ac-11 after their ledger entries are ratified.", anchor: dc-5 }
  - { id: dc-6, text: "The regression witness for the round is a replay of the UAT scenario against the integrated build: import the F13 plan, map constraints and decisions through the UI, accept with a spike stub still claiming an open question, then supersede to add one criterion. Each tracker entry gets a dated status note naming the closing commit.", anchor: dc-6 }
  - { id: dc-7, text: "Unresolvable default branch at design start (I-130): when the repository has no origin remote, the branch is based on the current HEAD and a disclosure line names that substitution; when an origin remote exists but the default branch is unresolvable or ambiguous, design start exits 2 with the shared unresolvable-default-branch message. The ratified text forbids silent substitution and prescribes disclosure, not refusal; a remote-less fresh project has no truth other than HEAD; a configured-but-unresolvable remote is exactly the stale-HEAD hazard UAT-021 reported. Applies to --from-stub identically.", anchor: dc-7 }
stubs:
  - { slug: version-verb, acceptance_criteria: [ac-1] }
  - { slug: cli-help, acceptance_criteria: [ac-2] }
  - { slug: vl017-notice, acceptance_criteria: [ac-3] }
  - { slug: import-record-format, acceptance_criteria: [ac-4] }
  - { slug: policy-error-prefix, acceptance_criteria: [ac-5] }
  - { slug: design-start-base, acceptance_criteria: [ac-6] }
  - { slug: f13-profile-selectors, acceptance_criteria: [ac-7] }
  - { slug: import-mapping-ui, acceptance_criteria: [ac-8] }
  - { slug: commit-dialog-template, acceptance_criteria: [ac-9] }
  - { slug: spike-claimed-questions, acceptance_criteria: [ac-10] }
  - { slug: supersede-revision, acceptance_criteria: [ac-11] }
---
# UAT Round 1

## Problem

A first real-project UAT pass on 2026-09-16 used the 48f8dc7f build to import
the verdi-atc F13 gatekeeper plan, populate its board, accept it, and add one
acceptance criterion through a superseding revision. Twenty-two product
observations are logged in the workspace tracker
`docs/design/uat/uat-findings.md` (UAT-001..022). Four were retracted after
investigation; the rest are defects, shipped-data corrections, or contract
gaps. Two of the contract gaps actively distorted authoring: every declared
open question blocked acceptance even when a spike stub claimed it, so seven
questions were forced into decisions; and no tooling exists to start a
superseding revision, so a one-line change after acceptance was a
copy-and-classify exercise over thirty-one objects.

## Outcome

Builds identify themselves. Help works. Lint notices are accurate and printed
once. Import records name the profile that produced them. Design start bases
on the default branch and can scaffold a superseding revision. Spike-claimed
open questions no longer block acceptance. Every contract change is ratified
before its code lands. The same UAT scenario replays clean against the
integrated build, and each tracker entry carries a dated closing note.

## ac-1

`verdi version` and `verdi --version` print the module version and VCS revision from build info and exit 0; `verdi serve` prints the same string at startup and the workbench renders it in its page footer.

Closes UAT-003. Two builds were live during the UAT and disagreed about the store; nothing identified which one produced a result.

## ac-2

`help`, `--help`, and `-h` are recognized at top level and after any verb: they print multi-line usage with one line per verb or subverb and exit 0, and never execute the verb. `verdi lint --help` no longer runs a lint.

Closes UAT-004.

## ac-3

When the mutable zone is absent, lint prints the VL-017 disclosure once per run, listing the affected specs, and describes the condition as the mutable zone being absent from this checkout. It does not assert a bare clone. The check's semantics are unchanged.

Closes UAT-002. See dc-3 for why a spec with no open questions still receives the disclosure.

## ac-4

A spec-import record persists the request format and, for a reference profile, the pinned primary digest it was bound to; the record page and `verdi design import record` display them.

Closes UAT-005. Adding fields to `verdi.spec-import-record/v1` is additive; existing records without them decode with the fields absent and the page says so.

## ac-5

Policy authority and policy-forbidden error strings carry their package prefix exactly once.

Closes UAT-020.

## ac-6

`verdi design start`, including `--from-stub`, bases the new design branch on the resolved default branch, not the current HEAD, and prints the base commit and the fact that the primary checkout was switched. When the default branch is unresolvable it follows dc-7: a disclosed HEAD base when no origin remote exists, an operational refusal when one does.

Amended 2026-09-16 after the L5 review: the first draft was silent on the unresolvable case and named only the `--kind/--name` path; `--from-stub` is dispatched inside the same verb and had the same defect.

Closes UAT-021. Uses the same `specstate.ResolveDefaultBranch` the rest of the tool uses. See dc-2 for the worktree exclusion.

## ac-7

The `f13-reference-v1` field map selects whole sentences: no selector ends mid-sentence, begins mid-clause, or joins two claims, and the test-instruction sentence is not selected as an acceptance criterion. The profile test is re-pinned and an import of the pinned F13 bundle yields the corrected criteria.

Closes UAT-007. The corrected wording should match the nine criteria accepted on the F13 v2 spec where the source supports it; the owner reviews the selector set before it is pinned.

## ac-8

In the workbench import dialog, selecting text in a rendered source creates a source-backed mapping with the byte range computed, and the mapping target is chosen from a picker that offers the problem and outcome statements and all four object kinds.

Closes UAT-006. This is the mechanism by which three of four sources stayed at zero mapped bytes.

## ac-9

The board's commit dialog prefills a proposal-shaped message naming the spec, keeps it editable, and shows a non-blocking note when the message contains a lifecycle word (accepted, closed, merged, superseded) that the commit itself cannot make true.

Closes UAT-019.

## ac-10

A declared open question that a spike stub claims through `resolves` is a non-blocking, later-timed readiness concern rather than a blocking one, so a feature can be accepted with such a question open; an unclaimed open question remains blocking.

Closes UAT-017. Contract change: 02 §Object model already allows an accepted feature to carry a question a spike later answers; the readiness rule contradicts it. Requires a PLAN.md §7 ledger entry and owner ratification before code (co-3).

## ac-11

`verdi design start --supersedes spec/<name> --name <new>` scaffolds a successor that carries the predecessor's objects and stubs verbatim, a `supersedes` link, and a `supersession:` block classifying every predecessor object as carried; lint accepts the untouched scaffold under VL-015, and the board of an accepted spec offers a Revise action that invokes the same operation.

Closes UAT-022. Extends the 05 §CLI grammar for `design start`; requires a ledger entry and ratification before code (co-3). The Revise action is Fable work (co-4).

## co-1

No network in any test. CLI paths are exercised through the built binary; browser paths through Playwright under e2e/; forge, git, and provider interactions against hermetic fakes.

## co-2

Every verb keeps the exit discipline: 0 clean, 1 verdict, 2 operational. Help and version are clean exits; usage on an unknown verb stays operational.

## co-3

No behavior that changes a ratified contract lands before its invention-ledger entry is recorded in PLAN.md §7 and ratified by the owner, and the change is entered in 08-revision-notes. This binds ac-10 and ac-11.

## co-4

All workbench and board work (ac-1 footer, ac-8, ac-9, the Revise action of ac-11) is implemented by Fable subagents and carries a Playwright path. Screenshot, trace, video, and screen recording stay disabled.

## co-5

Three-valued honesty is preserved in every notice this round touches: a quieter disclosure is still a disclosure, never a vacuous pass.

## dc-1

Scope is the UAT tracker's defect, data, and two highest-leverage contract items (UAT-002, 003, 004, 005, 006, 007, 017, 019, 020, 021, 022). The remaining contract findings (UAT-001, 008, 009, 010, 011, 018) are deferred to a contract-amendment round and are not open questions of this feature. UAT-012 is excluded until reproduced against two identified builds.

Deferring rather than declaring open questions is itself a consequence of UAT-017: on the current build any declared open question would block this feature's acceptance.

## dc-2

The ac-6 fix keeps design start's checkout-switching behavior and makes it explicit; moving design start onto managed worktrees is out of scope and belongs with the worktree-manager design.

## dc-3

VL-017 keeps its rule and severity. A spec with no declared open questions still receives the disclosure because an uncarried annotation could exist in the absent zone; only presentation changes (ac-3).

## dc-4

The F13 reference profile remains a named, pinned reference profile bound to the exact primary bytes. ac-7 revises its shipped field map and re-pins the test; it does not introduce generic prose splitting.

## dc-5

Risk tiers for orchestration: Tier 1 for ac-2, ac-3, ac-5; Tier 2 for ac-1, ac-4, ac-6, ac-7, ac-8, ac-9; Tier 3 with an owner risk gate for ac-10 (acceptance gating) and ac-11 (provenance and immutable history). Waves: 1 = ac-1..ac-6 in parallel; 2 = ac-7, ac-8, ac-9; 3 = ac-10, ac-11 after their ledger entries are ratified.

## dc-7

Unresolvable default branch at design start (I-130): when the repository has no origin remote, the branch is based on the current HEAD and a disclosure line names that substitution; when an origin remote exists but the default branch is unresolvable or ambiguous, design start exits 2 with the shared unresolvable-default-branch message. The ratified text forbids silent substitution and prescribes disclosure, not refusal; a remote-less fresh project has no truth other than HEAD; a configured-but-unresolvable remote is exactly the stale-HEAD hazard UAT-021 reported. Applies to --from-stub identically.

Evidence weighed (L5 review, 2026-09-16): README "Start your own store" flow reaches `design start` with no remote and promises an unproven board, not a refusal, and tells the reader not to invent CI_DEFAULT_BRANCH; docs/local-adoption.md rehearses design start in a separate clean project; cmd/e2eharness/unprovenboard.go models "fresh project, no remote, design start has run" as a supported state. Read-side verbs already degrade to unproven; only verdict verbs (build start, gate) fail closed. Ledger: PLAN.md §7 I-130.

## dc-6

The regression witness for the round is a replay of the UAT scenario against the integrated build: import the F13 plan, map constraints and decisions through the UI, accept with a spike stub still claiming an open question, then supersede to add one criterion. Each tracker entry gets a dated status note naming the closing commit.
