# UAT Round 1 — ac-6: design start bases on the resolved default branch

## Status

PROVEN, after one review-fix pass. `verdi design start` — both the
`--kind/--name` path and `--from-stub` — resolves the new branch's base
via `specstate.ResolveDefaultBranch` (never the calling checkout's HEAD),
follows dc-7's scoped rule when unresolvable (disclosed HEAD fallback with
no "origin" remote; operational refusal when "origin" exists but the
default branch still doesn't resolve), and prints the base/disclosure
lines on its existing stdout stream. Checkout-switching (dc-2, `--kind/
--name` only) and scaffold content are unchanged. L5 review returned
REVISE (2 Important, 3 Minor); all five findings are addressed below.

## Risk tier

Tier 2 (dc-5).

## Base..Head

`840c6166..5a2ffb41` on `agent/uat-r1-design-start`, worktree
`/Users/johnyang/code/verdi-system/verdi-wt/uat-r1-design-start`. Original
submission was `840c6166..bd28e5d5`; the review-fix range is
`bd28e5d5..5a2ffb41` (see below).

## Commits

Original submission (unchanged, still on branch):
1. `5b42ac58` — gitx: add CheckoutNewBranchFrom, an explicit-base branch cut
2. `b5b79e65` — design start: base the new branch on the resolved default branch
3. `e1c42f8d` — design start tests: resolve a default branch in the fixtures that need one
4. `bd28e5d5` — Report ac-6 design-start default-branch fix

Review-fix pass (new, this round):
5. `5709cf39` — specstate: share UnresolvedDefaultBranchMessage with non-cmd/verdi callers
6. `01546a1d` — gitx: prove CheckoutNewBranchFrom refuses a conflicting dirty tree
7. `d6a148fe` — design start: scope dc-7's unresolvable-default-branch rule; hoist above the interview
8. `5a6f4454` — design start tests: exercise the disclosed-HEAD fallback by default
9. `5a2ffb41` — design start --from-stub: apply dc-7 in the shared stub-instantiate core

## Files changed (cumulative)

- `internal/gitx/branch.go` / `branch_test.go` — `CheckoutNewBranchFrom`, its happy/negative tests, and F4's dirty-tree-conflict negative subtest.
- `internal/specstate/unresolvedmessage.go` / `_test.go` — new `UnresolvedDefaultBranchMessage`, the one shared diagnostic text every consumer now reuses.
- `internal/stubinstantiate/stubinstantiate.go` / `_test.go` — new `ResolvedBase`, `resolveDesignBranchBase` (dc-7), `CommitScaffoldBranch`'s new `(ResolvedBase, error)` signature, `Result.Base`.
- `internal/workbench/boardspecapi.go` — one-line call-site fix for the new `CommitScaffoldBranch` signature (`actionCreate`); no behavior/UI change.
- `cmd/verdi/buildstart.go` — `unresolvableDefaultBranchMessage` now delegates to `specstate.UnresolvedDefaultBranchMessage`.
- `cmd/verdi/design.go` / `design_test.go` — dc-7 scoping, hoisted above the interview, `shortSHA` helper, doc-comment corrections, new/rewritten tests.
- `cmd/verdi/designfromstub.go` / `_test.go` — prints the base/disclosure lines; new tests.
- `cmd/verdi/designscaffoldoverride_test.go`, `designstatements_test.go` — env-var revisit (see Review-fix range).

## Contract implemented (current, post-amendment)

1. **Base resolution**, hoisted above statement sourcing (F3): `runDesignStart`
   calls `specstate.ResolveDefaultBranch` as the FIRST thing after
   class/template resolution, before the TTY interview can ever collect
   (and then discard) an operator's answers.
2. **dc-7's scoped unresolvable rule** (F2, I-130): no "origin" remote at
   all (`gitx.RemoteURL` → `gitx.ErrNoSuchRemote`) → base on HEAD, print
   `design start: default branch unresolved (no origin remote); basing on
   current HEAD <sha> — disclosed, not a default-branch base`, exit 0.
   "origin" configured but still unresolvable/ambiguous → exit 2 with
   `specstate.UnresolvedDefaultBranchMessage`'s text. A real remote-read
   failure (neither case) also exits 2, never guessed.
3. **`--from-stub` gets the identical rule** (F1, Important): implemented
   once, in `internal/stubinstantiate.CommitScaffoldBranch` — the shared
   core both `--from-stub` and the board's stub-instantiate/creation-form
   actions call — so all three surfaces are fixed together, never a
   second CLI-side copy. The CLI path alone prints the disclosure lines;
   the board actions discard the returned `ResolvedBase` (no UI change —
   base SELECTION is corrected for them too, but neither surfaced this
   text before, and none does now).
4. **Disclosure and checkout-switching** (unchanged from the original
   submission): `design start: base <branch> @ <sha>` when resolved,
   `design start: switched checkout from <old> to design/<name>`
   unconditionally after the `--kind/--name` path's checkout succeeds.
   `--from-stub` never prints a switch line — it is pure `gitx.UpdateRef`
   plumbing and never touches the calling checkout (dc-2 doesn't apply).
5. **F4: dirty-tree refusal proven, not re-implemented.** No new guard was
   added; `TestCheckoutNewBranchFrom_Negative`'s new subtest proves git's
   own pre-existing checkout safety still refuses a conflicting dirty tree.
6. **F5: no change.** The `design start: created branch %s` line (both
   paths) is untouched; existing assertions reference it.

### Local vs. remote-tracking precedence — confirmed correct

The coordinator confirmed `Branch.Ref` used as-is (remote-tracking
preferred, per `specstate.ResolveDefaultBranch`'s own established
precedence) is correct, and that the original dispatch's "prefer local"
wording was the error, not my implementation. No code change was needed
for this; the disclosed judgment call in the original submission stands
as the record of that resolution.

## Review-fix range: `bd28e5d5..5a2ffb41`

Addresses the L5 review's REVISE verdict against the amended spec
(`spec/uat-round-1` ac-6 amended, dc-7 added; PLAN.md §7 I-130; branch
`design/uat-round-1` @ `12ebd008`).

- **F1 (Important) — `--from-stub` had the identical defect.** ac-6 was
  amended to name it explicitly. Fixed at the shared-core level
  (`internal/stubinstantiate.CommitScaffoldBranch`), not just the CLI —
  see commit `5a2ffb41`.
- **F2 (Important) — the unconditional exit-2 was itself wrong.** dc-7
  scopes it: no-origin → disclosed HEAD base, exit 0; origin-exists →
  exit 2. See commit `d6a148fe`.
- **F3 (Minor) — base resolution ran after the TTY interview.** Hoisted
  above statement sourcing entirely; a refusal there can no longer follow
  a collected-then-discarded operator answer. Same commit.
- **F4 (Minor) — dirty-tree refusal unproven.** New subtest, no code
  change (commit `01546a1d`).
- **F5 (Minor) — redundant "created branch" line.** No change, per the
  ruling (existing assertions reference it).

**Count correction:** my original report claimed 8 pre-existing
`design_test.go` tests needed `CI_DEFAULT_BRANCH=main`; the actual count
was 10 (I had omitted `TestCmdDesignStart_NameFlagOrdering` and
`TestRunDesignStart_ScaffoldUsesAtomicWrite`), making 19 total across the
3 files, not 17 as the coordinator's message (quoting my own miscount)
said. All 19 were revisited. **None needed to keep `CI_DEFAULT_BRANCH=
"main"`** — every one of them tests something orthogonal to default-branch
resolution (provider degrade, flag parsing, template override, class
mismatch, statement sourcing, atomic write, branch-collision retry) and
now exercises dc-7's disclosed-HEAD-fallback path by running against a
plain, remote-less fixturegit repo with the variable explicitly cleared
(`t.Setenv("CI_DEFAULT_BRANCH", "")`, not merely omitted, for
hermeticity — matching `buildstart_test.go`'s own established negative-test
convention). The one test that DOES need a resolved, HEAD-differing
default branch — `TestRunDesignStart_BasesOnDefaultBranch_NotHEAD` — kept
`"main"`, unchanged, since proving that exact scenario is its whole point.

`--from-stub`'s own fixtures (`buildFromStubRepo` and two ad hoc repos in
`designfromstub_test.go`) keep `CI_DEFAULT_BRANCH=main` for an unrelated,
pre-existing reason: the accepted-pending-build gate
(`SealedFeatureWallGuard`, fed by `specstate.Resolve`) independently
requires a resolvable default branch to prove the FEATURE's own status —
`specstate.Projector.ResolveMany`'s very first step is `ResolveDefaultBranch`,
unconditionally, for every candidate. This means a genuinely remote-less
fixture can never reach `Instantiate`'s call into `CommitScaffoldBranch`
at all through that gate — it refuses earlier, for a reason unrelated to
dc-7. So the "remote-less → disclosed HEAD" scenario for `--from-stub` is
proven directly against `CommitScaffoldBranch` in
`internal/stubinstantiate/stubinstantiate_test.go` (which carries no such
precondition), not through the full CLI flow; the "behind main" scenario
(proving the base is main's tip, not HEAD/side) IS proven through the
full CLI flow, since that fixture's default branch already resolves.

## Explicit exclusions (unchanged)

- Worktree creation, scaffold content, story-ref path semantics,
  workbench/e2e HTML/JS, per the dispatch. `internal/workbench/
  boardspecapi.go`'s one-line fix is a call-site compile fix for the new
  `CommitScaffoldBranch` signature, not a UI change.
- `docs/design/uat/uat-findings.md` (dc-6's ledger) still does not exist
  in this worktree — not created, outside this lane's write set.

## RED command and observed failure (this round)

Changing `runDesignStart`'s unconditional exit-2 to dc-7's scoped rule
turned the original submission's own
`TestRunDesignStart_UnresolvableDefaultBranch_Exit2` red (expected — it
encoded the now-superseded contract):

    go test ./cmd/verdi/... -run TestRunDesignStart_UnresolvableDefaultBranch_Exit2 -v
    --- FAIL: TestRunDesignStart_UnresolvableDefaultBranch_Exit2
        design_test.go:622: runDesignStart with an unresolvable default branch = 0, want 2

Replaced by `TestRunDesignStart_NoOriginRemote_DisclosedHeadBase` (exit 0,
disclosure asserted) and `TestRunDesignStart_OriginExistsButUnresolvable_
Exit2` (exit 2, origin configured via `git remote add`). Both pass; see
GREEN below.

## GREEN commands and results (this round, all passing)

    go build ./...                                                              # clean
    go test -race -count=1 ./internal/gitx/... ./internal/stubinstantiate/... ./internal/specstate/...
                                                                                 # ok 23.5s / 2.9s / 7.1s
    go test -count=1 ./cmd/verdi/... -run 'TestRunDesignStart|TestCmdDesignStart|TestRun_Design|FromStub' -v
                                                                                 # ok, 37/37 subtests PASS, 0 FAIL, 12.0s
    go test ./internal/specalign/ -run TestVocabProseWitness -count=1           # ok 1.2s
    go test ./internal/workbench/...                                           # ok 108.9s (actionCreate/actionStubInstantiate unaffected)
    go test -race -count=1 ./cmd/verdi/...                                     # ok 487.2s (full package, final safety net)
    gofmt -l <13 changed files>                                                 # clean (no output)
    go vet ./cmd/verdi/... ./internal/gitx/... ./internal/specstate/... ./internal/stubinstantiate/... ./internal/workbench/...
                                                                                 # clean
    golangci-lint run ./cmd/verdi/... ./internal/gitx/... ./internal/specstate/... ./internal/stubinstantiate/... ./internal/workbench/...
                                                                                 # 0 issues

## Residual risks

- **New dirty-tree exposure** (carried from the original report, now with
  F4's proof that the underlying git mechanism itself still behaves
  correctly): a dirty tree conflicting with a non-HEAD base can now
  surface a checkout refusal where the pre-fix code never touched the
  working tree at all. Disclosed, not newly coded; no dedicated
  end-to-end regression test beyond `TestCheckoutNewBranchFrom_Negative`'s
  own subtest at the `gitx` level.
- `unresolvableDefaultBranchMessage` in `buildstart.go` is now a one-line
  delegate to `specstate.UnresolvedDefaultBranchMessage`; its own doc
  comment was tightened but still doesn't name every consumer by file —
  accurate, not misleading.
- Local-vs-remote precedence: resolved, see above — no longer a residual
  risk.

## Integration prerequisites

None identified. No shared/serialized registry (CLI-verb table, MCP-tool
inventory) changed. No further spec/ledger edit needed for this lane's
own scope — the amendment and I-130 entry the coordinator named are
already ratified on `design/uat-round-1` @ `12ebd008`. Touches
`internal/workbench` (one call site) and `internal/stubinstantiate`
(signature change) beyond the original `cmd/verdi`/`internal/gitx`
footprint; both are covered by the full `internal/workbench` and
`internal/stubinstantiate` suites above and require no coordination with
other Wave-1 lanes (ac-1..ac-5), which do not touch these packages.
