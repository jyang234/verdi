# Lifecycle ceremony audit — specification acceptance

Applies the ratified design's own audit rule (docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md,
"Ceremony audit rule") to Task 7's own scope: retiring `verdi accept`'s
mutation and moving pre-review preparation before review. Every action
below is classified per the design's five-class table (Substantive
judgment / Authorization already expressed by PR review or merge /
Deterministic materialization / Exceptional override / Informational
acknowledgement) and disposed accordingly.

## Disposition table

| Action | Classification | Disposition |
|---|---|---|
| merge specification PR | authoritative specification decision | retain once; merge derives acceptance |
| `verdi accept` | duplicate mechanical acceptance | retire mutation; compatibility notice only |
| obligation scaffold/author | pre-review content preparation | retain as optional idempotent authoring aid; never authority |
| `verdi design start` | reversible workspace/scaffold setup | retain; no lifecycle claim |
| `verdi build start` / `feature start` | reversible implementation workspace setup | retain; requires proven accepted baseline but does not approve it |
| merge implementation PR | authoritative code integration | retain once |
| `verdi close --prepare` and closure PR construction | evidence computation and review preparation | retain before review; closure authority is the reviewed archive merge |
| successor specification merge | authoritative supersession decision | retain once; derive predecessor state without mutation |
| attest, waive, adjudicate, or disposition | distinct human evidence, override, or judgment | retain with identity/provenance checks |
| Codex review | independent technical judgment | retain; never mutates or self-repairs |
| post-merge status edit or completion-ledger commit | deterministic duplicate bookkeeping | prohibit; compute through `verdi spec state` or derived CI data |

## Per-row purpose and test witness

**merge specification PR — authoritative specification decision.** The
merge itself is the one attributable human decision (a solo operator's
authenticated push to the default branch, or a team's required-review
merge); Git reachability from that exact revision is what Verdi treats as
authoritative from then on (internal/specstate.Projector). No second
confirmation is requested.
Witness: `internal/specstate.TestProjector_MergeStrategies_Integration`
(merge/squash/rebase all converge on the same accepted-baseline identity);
`cmd/verdi.TestCmdSpecState_ExactAcceptedPendingBuild` (the read-only
surface reporting the merge's own consequence); `cmd/verdi.
TestRoundFourRitual_FullLoop` (the whole feature-first cascade, merge by
merge); `internal/showcasealign.TestCLIShowcaseCoverage/
design_start_then_merge_signals_acceptance` (proposed before merge,
accepted after, no command in between).

**`verdi accept` — duplicate mechanical acceptance, retired.** The command
used to re-decide what the merge had already decided: flip `status:`,
write a frozen stamp, scaffold obligations, stage, and commit — a second
ceremony for one decision, and (per the design's Problem section) one a
solo operator cannot even self-review on the same PR. It is retired to a
fixed, non-mutating compatibility notice for the transition window.
Witness: `cmd/verdi.TestRunAccept_NonMutation` (byte-identical git status/
HEAD/spec/predecessor/obligation-directory before and after, notice
printed, exit 0); `cmd/verdi.TestRunAccept_AnyValidSpecRefExitsZero`
(informational, never a verdict, regardless of the target's existence or
class); `cmd/verdi.TestCmdAccept_UsageNegative` (the one refusal left —
malformed argument count — is unchanged, exit 2).

**obligation scaffold/author — pre-review content preparation, never
authority.** Evidence obligations mechanically derivable from declared
acceptance criteria are now prepared BEFORE review, inside the same
reviewed pull request, rather than materialized by a post-merge or
accept-time write nobody reviews. `verdi obligation scaffold <story-ref>`
(new, replacing accept's freeze-moment backstop) idempotently creates every
missing declared `(ac, evidence-kind)` obligation and refuses (I-41) once
the target has actually landed — obligation preparation is pre-review
preparation, not a post-merge mutation. `verdi obligation author` (pre-
existing, unchanged) remains the single-pair authoring/regeneration
surface.
Witness: `cmd/verdi.TestRunObligationScaffold_Happy`,
`TestRunObligationScaffold_SecondRunIsIdempotent`,
`TestRunObligationScaffold_NeverOverwritesAnAuthoredFile`,
`TestRunObligationScaffold_AcceptedStoryRefusesMutation` (I-41: resolved
through the specStateResolver seam, not raw status or a merge-base
approximation), `TestRunObligationScaffold_UnprovenRefusesOperationally`;
`cmd/verdi.TestScaffoldMissingObligations` (the creation core, moved
verbatim from the retired accept-time backstop); the pre-existing
`TestRunObligationAuthor_*` suite (unchanged).

**`verdi design start` — reversible workspace/scaffold setup, no
lifecycle claim.** Cutting a design branch and writing a scaffold commits
nothing to the default branch and claims no acceptance; it is purely
reversible preparation. Retained unchanged.
Witness: `cmd/verdi.TestRoundFourRitual_FullLoop` steps 1 and 4 (feature
and story scaffolds); the pre-existing `design_test.go` suite (unchanged).

**`verdi build start` / `feature start` — reversible implementation
workspace setup.** Cutting a build branch requires a PROVEN accepted
baseline (internal/specstate's Git-derived projection) but does not itself
approve anything — it is downstream implementation setup, reversible like
any other branch, gated on a state it never asserts on its own.
Witness: `cmd/verdi.TestRunBuildStart_StatuslessExactDefaultBranch_Starts`
(pre-existing, unchanged); `cmd/verdi.TestRoundFourRitual_FullLoop` step 7
and the rung-4 re-affirmation-gated build start (steps 14-15).

**merge implementation PR — authoritative code integration, retained
once.** Outside this task's own mutation surface (the code build/review
workflow itself); no Verdi-internal test claims to witness a human's own
merge action — Verdi observes only its Git-derived consequence, the same
posture the specification-merge row above takes.

**`verdi close --prepare` and closure PR construction — evidence
computation and review preparation, retained before review.** Out of this
task's scope (the design's own "Initial lifecycle findings" flags closure
for the SAME audit as a follow-up: "status flips and frozen rollups should
be prepared automatically in the reviewed diff or derived from the
merge"). Unchanged by Task 7; existing `cmd/verdi/close_test.go` suite
continues to cover it. Not re-audited here — disclosed, not silently
assumed clean.

**successor specification merge — authoritative supersession decision,
retained once; predecessor state derived without mutation.** A
successor's own reviewed merge — carrying a `links: {type: supersedes}`
edge plus a validated `supersession:` block — is now the sole signal a
predecessor is superseded; nothing writes to the predecessor's own bytes
ever again (supersede.go, the only such writer, is deleted in this task).
I-40 (invention ledger, disclosed open owner question): a story-class spec
can never carry a `supersession:` block, so this derivation exists only
for feature-class pairs; a legacy, already-superseded story predecessor's
EXISTING explicit `status: superseded` field is still read compatibly, but
no NEW story-level supersession can be Git-derived after this task, and no
replacement mechanism is invented to cover that gap.
Witness: `cmd/verdi.TestDerivedSupersession_FeaturePredecessor` (positive:
predecessor bytes unchanged, `specstate.Projector.Resolve` reports
Superseded); `TestDerivedSupersession_ObjectFragmentEdgeDoesNotSupersede`
(negative: a decision-level fragment edge with no `supersession:` block
never derives it); `TestDerivedSupersession_LegacySupersededStoryPreserved`
(compatibility: a legacy explicit `status: superseded` story predecessor
still projects Superseded); `cmd/verdi.TestRoundFourRitual_FullLoop`
rung 3 (the disclosed story-class gap, proven honestly: the predecessor
stays AcceptedPendingBuild after a real successor merge) and rung 4 (the
positive feature-class derivation, exercised through the full CLI/build-
branch workflow); `internal/lint.TestVL010_StatusOnlySupersededFlipRefused`
(VL-010's own status-only-supersession exception is deleted along with its
sole writer — a status-only edit to `superseded` is now an ordinary,
illegal frozen-file modification).

**attest, waive, adjudicate, or disposition — distinct human evidence,
override, or judgment.** Out of this task's scope; each remains an
explicit, attributable action with its own identity/provenance checks,
unchanged by Task 7. Existing `attest_test.go`, `waive_test.go`,
`disposition_test.go` suites continue to cover them.

**Codex review — independent technical judgment, retained; never mutates
or self-repairs.** An external process, not a code path this repository's
test suite instruments; disclosed as outside this audit's own witness
reach rather than silently assumed.

**post-merge status edit or completion-ledger commit — deterministic
duplicate bookkeeping, prohibited.** This is exactly what `verdi accept`'s
retired mutation, and supersede.go's predecessor flip, both were: a
human-invoked (or automatable) commit that only ever restates a fact Git
history already proves. Both are deleted; the replacement is always
computation, never a commit — `verdi spec state` (read-only) or the
Git-derived data internal/specstate/CI already expose.
Witness: `cmd/verdi.TestCmdSpecState_ExactAcceptedPendingBuild` (the
derivation surface, in place of any status-editing command);
`cmd/verdi.TestRunAccept_NonMutation` (the retired command itself proven
to write nothing); `internal/lint.TestVL010_StatusOnlySupersededFlipRefused`
(a hand-authored post-merge status-only edit to a frozen spec is now an
ordinary VL-010 violation, admitted by no exception).

## Concerns and disclosed gaps carried forward

- **I-40 (story-class supersession, open owner question).** Documented
  above and in `cmd/verdi/supersedepredecessor_test.go`'s own file-level
  doc comment and `TestRoundFourRitual_FullLoop`'s rung-3 section. No
  mechanism invented to close it in this task, per the task's own binding
  instruction.
- **R4-I-12 stub-match and the rung-4 blast-radius quorum disclosure**
  (`cmd/verdi/stubmatch.go`, `cmd/verdi/blastradius.go`,
  `cmd/verdi/acceptlint.go`'s D6-23 quartet gate) lost their only
  production caller when `verdi accept`'s ritual was gutted. Their own
  pure-function/table-driven unit tests still pass in isolation, but
  nothing in production calls them anymore. The design's own "Rollout
  sequence" step 8 ("Apply the ceremony audit to closure, supersession,
  build start, evidence synchronization, and the four feature plans before
  their implementation begins") names the pre-merge gate as their next
  home — out of this task's scope, not silently deleted or silently
  re-wired.
- **`verdi close --prepare`, Codex review, and merge-implementation-PR
  rows** are carried forward from the design's own audit table without a
  fresh re-audit in this task (out of scope); flagged so a future focused
  plan does not mistake their presence in the table above for "already
  done."
