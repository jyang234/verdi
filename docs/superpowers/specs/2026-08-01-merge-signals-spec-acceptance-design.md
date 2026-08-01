# Merge-Signaled Specification Acceptance

**Status:** Ratified by the owner on 2026-08-01

**Owner decision:** For a solo operator, merging a reviewed specification pull request into the configured default branch is the single authoritative acceptance ceremony. No separate `verdi accept`, `/accept`, status edit, or confirmation may be required.

## Problem

Verdi currently describes acceptance as the merge of a specification pull request, but requires `verdi accept <spec>` as a separate final action before that merge. The command validates the draft, changes `status: draft` to `accepted-pending-build`, writes a frozen stamp, scaffolds obligations, and commits those mechanical changes.

That creates two ceremonies for one decision. A solo operator can author and merge a pull request but GitHub does not allow the author to submit an approving review on the same pull request. Requiring an additional local command therefore adds a memorable but non-judgmental step without adding independent authority. The current repository rules also do not require the failing `spec-gate` check, so the duplication fails both ergonomically and as a hard safety boundary.

The same failure pattern can occur elsewhere in the lifecycle whenever a pull-request merge authorizes a transition but a person must separately invoke deterministic mechanics to materialize it.

## Decision

A human makes each authoritative decision once. Deterministic consequences are derived or automated.

For a new specification:

1. The specification remains a proposal throughout its pull request.
2. Review comments, requested changes, evidence, and independent agent findings accumulate against exact commits in that pull request.
3. Required pre-merge checks validate that the proposed specification is structurally and semantically eligible for acceptance. Proposal state is not itself a violation.
4. The authorized operator merges the exact reviewed head into the configured default branch.
5. Reachability of that exact specification revision from the default branch makes it accepted. No second human action or content-changing acceptance commit is required.

For a solo repository, the merge action is the approval signal. For a team profile that requires approving reviews, the required reviews authorize merge but do not independently activate the specification; merge remains the point at which repository authority changes.

## Authority and derived state

The committed specification bytes and Git history are authoritative. A mutable forge label, workflow run, comment, local checkout state, or post-merge job is not the source of acceptance truth.

Verdi derives the effective lifecycle state of a specification revision as follows:

- **Proposed:** the specification path or exact candidate revision is not reachable from the configured default branch.
- **Accepted pending build:** the exact active specification revision is reachable from the configured default branch and is not closed or superseded by a later authoritative transition.
- **Superseded or closed:** the existing authoritative supersession and archive records establish that later state.
- **Unproven:** the configured default branch or required Git ancestry cannot be resolved. Verdi discloses the missing witness and cannot claim acceptance.

The accepted baseline identity is deterministic Git data:

- the specification path;
- its blob object ID at the landing revision; and
- the first-parent default-branch commit that landed that blob relative to its parent.

This identity works for merge commits, squash merges, and rebase merges without consulting the forge. Forge PR identity and authenticated merger identity are additional governance witnesses when available; their absence cannot change the Git fact that the revision is on the default branch, though a profile may classify the transition as governance-incomplete.

## Artifact compatibility

Existing accepted artifacts and frozen stamps remain valid. The first implementation is additive:

- Legacy `accepted-pending-build`, `superseded`, and `closed` artifacts continue to decode and preserve their existing meaning.
- The schema permits new active specifications to omit the persisted `status` field. Their proposed-versus-accepted state is entirely Git-derived, so the reviewed bytes remain identical across the merge boundary.
- Before the four feature proposals merge, their legacy `status: draft` lines are removed in a reviewed revision after the compatible decoder lands. This is proposal normalization, not acceptance materialization.
- During migration, a legacy `status: draft` artifact that has already reached the default branch is reported as merge-accepted with a compatibility disclosure rather than misrepresented as an active draft. The migration path removes the stale field through the ratified amendment flow; it never silently edits accepted bytes.
- `frozen.commit` remains recognized for legacy artifacts. New merge-accepted artifacts use the Git-derived baseline identity and do not require a content-changing frozen stamp.

Direct edits to an accepted revision remain forbidden. Lint compares a candidate against the default-branch baseline: a new path is a proposal; a changed path already present on the default branch must follow the existing amendment, supersession, or closure rules.

## Command behavior

`verdi accept` is retired from the ordinary human workflow.

- During a compatibility window, invoking it on a proposal prints a deprecation notice explaining that merging the specification pull request signals acceptance. It performs no status flip, stamp write, obligation scaffolding, staging, or commit.
- Scripts that need to determine eligibility use a read-only acceptance-readiness operation or the common lifecycle projection, not a mutating ritual.
- Obligation scaffolding that is mechanically derivable from declared acceptance criteria moves into proposal validation or an idempotent generation step before review. The required files must be present and reviewed in the same pull request; merge never depends on an unreviewed post-merge write.
- Supersession effects must likewise be represented in the reviewed pull request or derived from its committed successor records. Merge automation does not silently edit predecessor artifacts.

`verdi build start` and every other acceptance consumer resolve effective state through one shared Git-aware lifecycle service. No adapter reimplements reachability or trusts the legacy status field alone.

## Pull-request gates

The repository exposes one always-present required check for every pull request targeting the default branch. It may dispatch internally to cheaper spec/document checks or the full code gate, but path filtering must not make the required check disappear.

For a specification proposal, the gate verifies at least:

- strict artifact decoding and reference integrity;
- acceptance criteria, obligations, stubs, and required sidecars;
- declared and judged conflict readiness;
- absence of unresolved blocking review records available to the forge adapter;
- legal treatment of any already-accepted path;
- deterministic generated artifacts and `spec-align`; and
- exact-head freshness of every required result.

The gate does not report a violation solely because a new specification is proposed and the pull request targets the default branch. The successor to `VL-004` instead refuses an unreviewable or illegal landing shape: a proposal on the default branch that did not pass the required merge gate is prevented by the forge ruleset, while accepted-file immutability remains a repository lint concern.

The default-branch ruleset requires:

- pull requests for all changes;
- the stable merge-gate check;
- resolved review threads when supported by the active profile; and
- dismissal or invalidation of stale approvals after substantive changes for team profiles.

Solo profiles require no approving review because GitHub prohibits self-approval. The authenticated merge by the authorized repository owner is the durable human witness.

## Failure and recovery

Pre-merge validation fails closed. A failing or missing required gate prevents merge and leaves the proposal editable.

Acceptance itself has no post-merge mutation window: the successful merge is the state transition. A failed Pages build, receipt projection, metrics job, or forge synchronization after merge cannot make the accepted Git revision disappear or retroactively become a draft. Such failures are operational or governance-completeness blockers and are reported separately.

If a draft reaches the default branch outside the required pull-request path, Verdi reports a governance violation with the landing commit as witness. It does not ask an operator to repair the event by running `verdi accept`; remediation follows repository history and incident policy.

If default-branch identity or ancestry is unavailable, acceptance is disclosed as unproven. Local convenience must not silently substitute a branch named `main` unless that fallback is an explicitly configured and visible compatibility rule.

## Ceremony audit rule

Every lifecycle plan must inventory its human interactions and classify each one:

| Class | Treatment |
|---|---|
| Substantive judgment | Retain one attributable human decision with the evidence the person judged. |
| Authorization already expressed by PR review or merge | Reuse that forge event; do not request a second confirmation or command. |
| Deterministic materialization | Derive it or automate it before review so its bytes are included in the authorized diff. |
| Exceptional override | Retain an explicit, scoped, attributable action with rationale and witnesses. |
| Informational acknowledgement | Remove it unless it prevents a demonstrated irreversible mistake. |

Initial lifecycle findings:

- Specification acceptance becomes merge-signaled under this design.
- Closure should be reviewed for the same duplication: if merging a closure pull request is closure, status flips and frozen rollups should be prepared automatically in the reviewed diff or derived from the merge.
- Supersession should be expressed by the reviewed successor records and merge, not a separate human status-edit ceremony.
- Build-start branch creation, evidence synchronization, receipt generation, and projection refresh are mechanics, not human governance decisions.
- Deviations, exemptions, waivers, attestations, conflict dispositions, and destructive recovery overrides contain substantive human judgment and remain explicit, attributable actions. Their surrounding scaffolding and validation should be automated.

Each focused feature plan records which ceremonies it removes, retains, or automates and why. A retained human action without a distinct judgment or safety purpose is a plan defect.

## Security and trust boundaries

- Agents may prepare proposal bytes, evidence, and review findings but cannot merge or impersonate the authorized human principal.
- The effective merger is resolved through the shared authenticated-principal and governance-profile seam when forge evidence is available.
- Git reachability proves repository state, not reviewer identity or substantive correctness. Profiles may require additional forge witnesses before an authoritative downstream action while still reporting the repository transition honestly.
- No workflow secret, GitHub App, personal access token, or MCP bridge is required for the solo merge-signaled path.
- CI output and forge metadata are untrusted inputs until strict-decoded and bound to the exact head or landing commit.

## Rollout sequence

1. Ratify this design through the repository revision-note flow without editing frozen binding documents in place.
2. Add the Git-aware effective-state resolver and characterization tests for all supported merge methods and missing-history cases.
3. Route lint, workbench, CLI, MCP, build-start, gate, and lifecycle projections through that resolver.
4. Split proposal readiness from the obsolete draft-on-default verdict and make the stable merge gate always report.
5. Require the stable merge gate in the default-branch ruleset.
6. Deprecate the mutating `verdi accept` path and relocate pre-review obligation and supersession materialization.
7. Exercise the flow with one solo-authored specification pull request: draft during review, required checks green, owner merge, effective state accepted, no follow-up command or commit.
8. Apply the ceremony audit to closure, supersession, build start, evidence synchronization, and the four feature plans before their implementation begins.

The four-feature umbrella pull request remains a design-review vehicle until steps 1–5 land. Afterward, its canonical proposals can merge under the new rule; their merge itself records acceptance.

## Acceptance criteria

1. A new specification pull request remains reviewable and green while its candidate artifact is a proposal.
2. An authorized solo operator can merge their own green specification pull request without running `verdi accept` or any equivalent second ceremony.
3. After merge, every Verdi surface reports the exact landed specification revision as accepted pending build using Git-derived identity.
4. The same revision is reported as proposed before merge and accepted after merge without any content mutation between those observations.
5. A missing or failing stable merge gate blocks the default-branch merge through the live repository ruleset.
6. Direct modification of an already accepted revision remains a named verdict failure with the default-branch baseline as witness.
7. Missing default-branch or ancestry evidence produces an explicit unproven result, never an assumed acceptance.
8. Legacy accepted and frozen artifacts retain their meaning and pass existing gates.
9. Merge, squash, and rebase landing strategies produce deterministic accepted-baseline identities in hermetic Git fixtures.
10. No post-merge workflow, bot identity, GitHub App, personal token, MCP bridge, or manually remembered command is required to make acceptance true.

## Non-goals

- Verdi does not judge whether the human made a substantively correct decision.
- This change does not eliminate reviews, conflict disposition, evidence requirements, or profile-specific separation of duties.
- This change does not allow agents to merge, waive, attest, or author human judgment.
- This change does not silently rewrite legacy frozen artifacts.
- This change does not redesign every lifecycle ceremony in one implementation pull request; it establishes the audit rule and sequences focused follow-ups.
