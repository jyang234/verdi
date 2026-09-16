# UAT Round 1 — ac-6: design start bases on the resolved default branch

## Status

PROVEN. `verdi design start` resolves the new branch's base via
`specstate.ResolveDefaultBranch` (never HEAD), fails operationally (exit 2)
when unresolvable, and prints both the resolved base and the checkout
switch on its existing stdout stream. Checkout-switching (dc-2) and
scaffold content are unchanged.

## Risk tier

Tier 2 (dc-5).

## Base..Head

`840c6166..e1c42f8d` on `agent/uat-r1-design-start`, worktree
`/Users/johnyang/code/verdi-system/verdi-wt/uat-r1-design-start`.

## Commits

1. `5b42ac58` — gitx: add CheckoutNewBranchFrom, an explicit-base branch cut
2. `b5b79e65` — design start: base the new branch on the resolved default branch
3. `e1c42f8d` — design start tests: resolve a default branch in the fixtures that need one

## Files changed

- `internal/gitx/branch.go` — new `CheckoutNewBranchFrom(ctx, dir, name, base)`.
- `internal/gitx/branch_test.go` — its happy/negative tests.
- `cmd/verdi/design.go` — the ac-6 fix in `runDesignStart`, a `shortSHA` helper, doc-comment corrections (file header, `runDesignStart`'s own comment).
- `cmd/verdi/design_test.go` — 2 new tests (`TestRunDesignStart_BasesOnDefaultBranch_NotHEAD`, `TestRunDesignStart_UnresolvableDefaultBranch_Exit2`) plus `CI_DEFAULT_BRANCH=main` in 8 existing tests.
- `cmd/verdi/designscaffoldoverride_test.go` — same env fix, 3 tests.
- `cmd/verdi/designstatements_test.go` — same env fix, 6 tests (incl. 2 built-binary subprocess tests).

## Contract implemented

1. **Base resolution.** `runDesignStart` calls
   `specstate.ResolveDefaultBranch(ctx, root)` — the same function/precedence
   build start, gate, and lint already share — as the last preparation step,
   before any Git mutation. Unresolvable → exit 2, reusing
   `unresolvableDefaultBranchMessage`'s exact text (`buildstart.go`, same
   package) so the diagnostic matches build start's for the identical
   failure. `defaultBranch.Ref` is used directly as the checkout base — see
   *Disclosed judgment call*.
2. **Disclosure**, on the stream design start already reports to:
   `design start: base <branch> @ <short sha>` (once resolved, before the
   checkout — `<branch>` is `defaultBranch.Ref` itself, so its own shape,
   bare vs `origin/`-prefixed, discloses which was used) and
   `design start: switched checkout from <old branch or detached sha> to
   design/<name>` (right after the checkout succeeds, unconditionally —
   `CheckoutNewBranchFrom` always lands on a brand-new name, so the switch
   has, by construction, always just happened).
3. **Checkout-switching retained (dc-2).** New primitive
   `gitx.CheckoutNewBranchFrom` replaces the bare `CheckoutNewBranch` call
   at this one site only; `CheckoutNewBranch` and `feature start`'s
   HEAD-based `feature/<name>` cut are untouched.
4. **No new dirty-tree refusal.** `git checkout -b <name> <base>`'s own
   pre-existing dirty-tree refusal is unchanged code; nothing new was
   coded to detect or reject it (see Residual risks).

### Disclosed judgment call — local vs. remote-tracking preference

My dispatch's contract text said to prefer the local ref, falling back to
remote-tracking. That is the **inverse** of `ResolveDefaultBranch`'s actual,
deliberate precedence (`defaultbranch.go`'s `resolveBranchRef`:
remote-tracking wins when both exist — reaffirmed by a dedicated
comment-only fix, `fix/default-branch-ref-precedence`, correcting two
stale docs asserting the opposite). The ratified spec text (`ac-6`) says
only "uses the same `specstate.ResolveDefaultBranch`" — no preference
order of its own. I used `Branch.Ref` as-is (remote-tracking preferred):
it is the litigated, tested, everywhere-else answer to "what is THE
default branch," reusing it verbatim satisfies the spec text most
literally, and inventing a second, contrary precedence for one call site
would itself be an undocumented deviation. Disclosed, not silent — flag
for owner adjudication if local-preferred was actually intended.

## Explicit exclusions

- `designfromstub.go` / `internal/stubinstantiate` (`--from-stub`) — a
  structurally different mechanism (`gitx.UpdateRef` plumbing; never
  switches the checkout), so dc-2 doesn't apply and the UAT reproduction
  was the `--kind/--name` path. Confirmed unaffected:
  `TestRunDesignStartFromStub_*`, `TestRun_DesignStartFromStub_Dispatch`
  still pass untouched.
- Worktree creation, scaffold content, story-ref path semantics,
  workbench/e2e, per the dispatch. No e2e fixture runs `design start` as a
  CLI subprocess (`e2e/tests/fixtures.ts` provisions branches via direct
  git plumbing in `cmd/e2eharness`).
- `internal/specalign`/`internal/showcasealign`'s own `design start` CLI
  probes checked, no change needed: the former runs from a rootless
  tempdir (fails at `store.FindRoot` first); the latter already sets
  `CI_DEFAULT_BRANCH=main` for its own downstream calls.
- `docs/design/uat/uat-findings.md` (dc-6's ledger) does not exist in this
  worktree — not created, outside this lane's write set.

## RED command and observed failure

    go test ./cmd/verdi/... -run 'TestRunDesignStart_BasesOnDefaultBranch_NotHEAD|TestRunDesignStart_UnresolvableDefaultBranch_Exit2' -v

    --- FAIL: TestRunDesignStart_BasesOnDefaultBranch_NotHEAD
        design_test.go:583: design/behind-side-branch's parent = c08ee2d8...,
            want main's tip 406145c5... — based on HEAD/side, not the default branch
    --- FAIL: TestRunDesignStart_UnresolvableDefaultBranch_Exit2
        design_test.go:612: runDesignStart ... = 0, want 2; stdout=design start: created branch design/unresolvable-default ...

(gitx's own RED: `branch_test.go` referenced the not-yet-existing
`CheckoutNewBranchFrom` — `undefined: CheckoutNewBranchFrom`, a build failure.)

## GREEN commands and results

    go build ./...                                                    # clean
    go test -race ./internal/gitx/...                                 # ok  27.9s
    go test -race ./internal/designapp/...                            # ok  18.8s
    go test ./internal/specstate/...                                  # ok   6.2s (confirms untouched)
    go test ./cmd/verdi/... -run 'TestRunDesignStart|TestCmdDesignStart|TestDesignGo|TestRoundFourRitual|TestRun_Design' -v
                                                                       # ok, 0 FAIL
    go test -race -count=1 ./cmd/verdi/...                            # ok 525.070s (full package)
    gofmt -l <6 changed files>                                        # clean (no output)
    go vet ./...                                                      # clean
    golangci-lint run ./cmd/verdi/... ./internal/gitx/...             # 0 issues

Enumerated every `runDesignStart(`/`cmdDesignStart(` call site across the
module (`grep -rn`) to find every test needing `CI_DEFAULT_BRANCH=main`
(fixturegit repos carry no `origin` remote) — 17 existing tests across 3
files, each confirmed necessary by observing it fail, then pass once
patched. `TestRoundFourRitual_FullLoop`/`TestDesignGo_NoOwnersFlag`/the
`specalign`/`showcasealign` probes needed no change (Explicit exclusions).

## Residual risks

- **New dirty-tree exposure.** Before this fix, `checkout -b design/<name>`
  (implicit HEAD base) never touched the working tree. Now that base can
  differ from HEAD, a dirty tree conflicting with the base's tree can make
  git's own (pre-existing, unmodified) checkout safety refuse where it
  previously wouldn't. No new refusal was coded (dispatch constraint 4);
  this is git's own mechanism in a newly-reachable circumstance, wrapped
  through the same error path as every other failure here. Not covered by
  a dedicated test in this lane; flagging as a follow-up candidate.
- Local-vs-remote precedence judgment call above needs owner confirmation.
- `unresolvableDefaultBranchMessage` (`buildstart.go`) now has a second
  caller; its doc comment still names only build start — accurate, not
  misleading, left untouched (outside this lane's write set).

## Integration prerequisites

None identified. No shared/serialized registry (CLI-verb table, MCP-tool
inventory) changed; no spec, PLAN.md, or invention-ledger edit needed — a
code-only conformance fix to an already-ratified acceptance criterion, not
a new interpretation. Safe to merge independently of the other Wave-1
`uat-round-1` lanes (ac-1..ac-5).
