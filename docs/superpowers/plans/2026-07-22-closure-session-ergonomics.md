# Closure session ergonomics implementation plan

> Execute with the repository's Sonnet implementation / Opus review workflow.
> Every production change follows RED -> GREEN -> REFACTOR. Do not edit frozen
> specs or archived artifacts.

**Goal:** Turn closure cleanup into a derived, resumable CLI workflow that
separates mechanical preparation from human judgment and prevents closure
commits from absorbing unrelated working-tree state.

**Architecture:** Add a structured outcome beneath the existing closure-gate
reporting, add `close --prepare` as an explicit-ref coordinator over the
existing align and preflight engines, and replace close's repository-wide
staging with an empty-index guard plus exact target-spec paths. The evidence
fold, closure conditions, disposition semantics, and exit classes remain
authoritative and unchanged.

**Tech stack:** Go, fixturegit, existing `cmd/verdi` hermetic fakes,
`internal/gitx`, `make verify`, Go race detector.

## Global constraints

- Closure-gate pass/fail semantics are unchanged. A disclosure remains a
  disclosure, not a new failure; the summary must say `READY WITH DISCLOSURES`.
- Preparation may write only the target's living deviation report and only via
  the existing align engine. It never freezes, archives, commits, publishes,
  pushes, opens a PR, or chooses a disposition.
- A current report with undispositioned findings is preserved byte-for-byte;
  preparation prints human action templates and exits 1.
- Story and feature close commit only `.verdi/specs/active/<name>` and
  `.verdi/specs/archive/<name>`. Any pre-existing staged path is refused before
  mutation. Unstaged/untracked unrelated paths survive outside the commit.
- All tests are hermetic and network-free. Every new function has happy and
  negative coverage. Unknown/error states fail closed as operational exit 2.
- Preserve the user's other checkout and its untracked files. Work only in the
  isolated `fix/closure-session-ergonomics` worktree.

## Task 1: Structured closure-gate outcomes and honest summaries

**Files:**

- Modify: `cmd/verdi/closuregate.go`
- Modify: `cmd/verdi/closuregatefeature.go`
- Modify: `cmd/verdi/closepreflight.go`
- Modify: `cmd/verdi/closepreflightfeature.go`
- Test: `cmd/verdi/closuregate_test.go`
- Test: `cmd/verdi/closuregatefeature_test.go`
- Test: `cmd/verdi/closepreflight_test.go`
- Test: `cmd/verdi/closepreflightfeature_test.go`

1. Add failing tests proving a gate outcome counts a condition disclosure and
   per-record disclosure detail without changing `Ready`.
2. Add failing preflight tests for `READY WITH DISCLOSURES` versus `READY`,
   including the local publish-guard disclosure.
3. Run the focused tests and verify each fails for the missing behavior.
4. Introduce the smallest `closureGateOutcome` seam and reuse one reporting
   loop for story and feature conditions.
5. Keep compatibility wrappers where that avoids mechanical churn in existing
   tests and real-close callers.
6. Run focused tests green, then the full `cmd/verdi` package tests.
7. Commit with an imperative subject.

## Task 2: Resumable explicit-ref closure preparation

**Files:**

- Modify: `cmd/verdi/close.go`
- Add: `cmd/verdi/closeprepare.go`
- Add: `cmd/verdi/closeprepare_test.go`
- Modify as needed: `cmd/verdi/dispatch.go`

1. Add failing CLI/parser tests for `--prepare`, mutual exclusivity with
   `--preflight`, missing/extra arguments, and story/feature refs.
2. Add failing behavioral tests for absent/stale report generation, current
   undispositioned preservation, disposition command worklist, mechanically
   blocked preflight, ready-with-disclosures, and ready.
3. Add a negative test proving a second run over a current undispositioned
   report does not invoke the judge or change report bytes.
4. Run each test RED and record the expected failure in the task report.
5. Implement `runPrepare` by composing `storyresolve.Resolve`,
   `loadExistingReport`, `runAlignForSpec(freeze=false)`, and `runPreflight`.
   Reuse close's manifest/judge dependency resolution; do not introduce a
   second align engine or a session file.
6. Print one exact disposition template per finding, leaving the disposition
   and rationale placeholders visibly human-authored.
7. Run focused tests green, then `go test ./cmd/verdi` and race-test the package.
8. Commit with an imperative subject.

## Task 3: Exact closure commit ownership

**Files:**

- Add: `internal/gitx/stagedpaths.go`
- Add: `internal/gitx/stagedpaths_test.go`
- Modify: `cmd/verdi/close.go`
- Modify: `cmd/verdi/closefeature.go`
- Modify: `cmd/verdi/close_test.go`
- Modify: `cmd/verdi/closefeature_test.go`

1. Add failing `gitx` tests for deterministically listing staged add/modify/
   delete paths and for operational errors outside a repository.
2. Add failing story and feature close tests proving pre-existing staged paths
   are named and refused before branch cut/archive mutation.
3. Add failing story and feature tests proving unrelated untracked and modified
   paths survive and are absent from the closure commit.
4. Run each focused test RED.
5. Implement `gitx.StagedPaths` with NUL-delimited git output and deterministic
   sorting.
6. Add one shared close helper that checks the pre-ritual index and one shared
   helper that stages only the active and archive spec directories through
   `gitx.AddPaths`.
7. Replace both `AddAll` call sites; do not duplicate path construction between
   story and feature close.
8. Run focused tests green, then `go test ./internal/gitx ./cmd/verdi` and race
   tests for both packages.
9. Commit with an imperative subject.

## Task 4: Lifecycle documentation and integration coverage

**Files:**

- Modify: `docs/architecture-and-journeys.md`
- Modify: command comments/usage strings adjacent to `cmdClose`
- Test: existing CLI end-to-end tests under `cmd/verdi`

1. Update the living closure journey to show prepare -> human disposition ->
   preflight -> close, including `READY WITH DISCLOSURES` and the exact commit
   boundary. Do not edit frozen specs or archived reports.
2. Add or extend a binary-level test that exercises the user-visible prepare
   resume path if unit-level command dispatch does not already cover it.
3. Verify documented command examples against the built binary's usage.
4. Run formatting, docs-related gates, `go test ./cmd/verdi`, and commit.

## Task 5: First complete verification and task reviews

1. Review every task diff against its brief and the global constraints with a
   fresh Opus reviewer; remediate Critical/Important findings and re-review.
2. Run `gofmt`/format checks, `go vet`, lint, all Go tests, race tests, fixture
   gates, spec-align, and browser e2e through `make verify`.
3. Re-read this plan and the design verification list; record each item as
   proven, violated-with-witness, or disclosed-unproven.

## Task 6: Adversarial sweep round 1 and remediation

Run independent Opus reviews over the complete branch with these lenses:

- governance/spec alignment and three-valued honesty;
- transaction safety, git index/path ownership, and retry behavior;
- Go/API quality, duplication, and test quality.

Adversarially verify each finding against code and tests. Dispatch one principled
fix wave for surviving findings, require focused test evidence, re-review the
fixes, and rerun the relevant gates.

## Task 7: Adversarial sweep round 2 and remediation

After round 1 is clean, create a fresh branch-diff package and run a second set
of independent Opus reviews. Explicitly attack:

- report freshness races and idempotent retries;
- disclosure counting and accidental gate-semantic changes;
- staged, unstaged, untracked, rename, deletion, and failure-window cases;
- tests that could pass while the motivating bug survives.

Adversarially verify and remediate surviving findings, re-review, and rerun the
relevant focused and full gates.

## Task 8: Final verification and pull request

1. Run a final whole-branch Opus review against the merge-base diff.
2. Resolve every material finding through the same fix/re-review loop.
3. Run fresh `make verify` and fresh `go test -race ./...` at the final HEAD.
4. Confirm the original checkout is untouched.
5. Push `fix/closure-session-ergonomics` and open a GitHub PR with the problem,
   design, governance boundaries, changes, adversarial sweeps, and exact test
   evidence.
