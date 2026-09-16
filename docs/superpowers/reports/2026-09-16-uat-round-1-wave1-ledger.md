# UAT Round 1 — Wave 1 controller ledger

Controller: FABLE (Claude Fable 5.1). Authority: spec/uat-round-1 (design/uat-round-1 @ 54b90176, proposed), PLAN.md §7 I-128/I-129 (drafted, awaiting ratification; bind ac-10/ac-11 only). Base for every lane: main 840c6166 (PR #322 merge). Integration branch: agent/uat-round-1.

## Lanes

| Lane | AC | Tier | Branch | Head | Gate | Review | Ruling |
|---|---|---|---|---|---|---|---|
| L1 cli | ac-1 (CLI), ac-2 | 2 | agent/uat-r1-cli | 21ec37e0 (code f9470eeb) | passed | ACCEPT-WITH-MINOR (6 minors) | minors 1–5 closed at cc14149f..42e45a17 (controller-verified: specalign witnesses ok, help/version/registry tests ok; full cmd/verdi race ok 483.8s at f9470eeb); INTEGRATED at 3d46f12c, 0 drift; minor 4 ruled conforming (R-13); R-11 → dc-8 |
| L2 vl017 | ac-3 | 1→3 | agent/uat-r1-vl017 | c56c3733 | passed | REVISE (F1 Critical: vocab witness regression) | Tier 3 fix range c56c3733..7698dc76 (fixer distinct from reviewer): F1 marker relocated, F2 gate = rule+severity+exact message, F3 comment, exit-1 test added; fresh re-review ACCEPT (F1–F3 CLOSED by mutation probes; exit-1 test has teeth); INTEGRATED, 0 drift; Tier 3 owner risk gate owed at wave close |
| L3 import-record | ac-4 (backend, CLI) | 2 | agent/uat-r1-import-record | 586ec7c2 | passed | ACCEPT-WITH-MINOR | minors 2,3,5 closed at 9bbb9272 (controller-verified: specimport race ok 26.1s); INTEGRATED at edd948de, 0 lines drift vs reviewed head |
| L4 policy-prefix | ac-5 | 1 | agent/uat-r1-policy-prefix | 4e1dc62a | passed | ACCEPT-WITH-MINOR | minor 2 closed at b6010cd7 (controller-verified: designapp race ok, vocab witness ok); minor 1 → Fable lane (R-5); INTEGRATED, 0 drift |
| L5 design-start | ac-6 | 2 | agent/uat-r1-design-start | 44c14802 | passed | REVISE (F1 --from-stub HEAD-based; F2 exit-2 invention unrecorded; F3–F5 minor) | fix range bd28e5d5..44c14802 (F1–F4; F5 no-change per ruling); controller gate: 14 files, Go-only workbench call-site change, specalign witnesses ok, gitx/stubinstantiate/specstate race ok, focused cmd ok; original reviewer CLOSED (F1–F4 verified; dc-7 implemented identically on both paths; stubinstantiate-level proof for remote-less --from-stub accepted because the CLI wall guard refuses earlier); INTEGRATED at b01ba359 — design.go auto-merged with L1, verified as L1 hunks + L5 hunks only, 0 drift elsewhere |

## Pre-review gate evidence (controller-run)

- L3: ancestry ok; tree clean; 10 files all in write set; no workbench/js/html/e2e; `go test -race -count=1 ./internal/specimport/...` ok 28.5s; `go test -race -count=1 ./cmd/verdi/...` ok 519.1s (exit 0).
- L4: ancestry ok; tree clean; 8 files in write set; `go test -race -count=1 ./internal/policyauthority/... ./internal/designapp/...` ok (3.1s, 21.3s). Full internal/workbench suite taken from lane manifest (139.8s ok), to be re-run at wave gate.
- L2: ancestry ok; tree clean; 5 files in write set; no "bare clone" in printed text; `go test -race -count=1 ./internal/lint/...` ok 125.0s; `go test ./cmd/verdi -run 'TestRunLintVerb|TestDisclosureSeam'` ok.
- L1: ancestry ok; tree clean; 11 files in write set; verbPhase and compact usage untouched; specalign vocab + CLI-verb witnesses ok 31.5s; `go test -race -count=1 ./internal/buildinfo/...` ok; focused cmd help/version/serve tests ok; smoke: `version` exit 0, `lint --help` prints usage exit 0 without linting, unknown verb exit 2. Full cmd/verdi race run in background.
- L5: ancestry ok; tree clean; 7 files in write set; `go test -race -count=1 ./internal/gitx/...` ok 24.4s; `go test ./cmd/verdi -run 'TestRunDesignStart|TestCmdDesignStart|TestRun_Design'` ok.

## Rulings

- R-1 (L3 review minor 1): the record page half of ac-4 is not closed by L3; owed by the wave-1b Fable lane (two fact rows on renderSpecImportRecord + Playwright path). ac-4 stays open until then.
- R-2 (L3 review note 6): the spec-import contract doc's record-schema sentence (docs/superpowers/specs/2026-09-14-spec-import-contract.md:337-340) must name `format` and `profile_primary_digest`; authority text, authored by the controller as a separate commit at L3 integration, not by the lane. DONE at 2cbfcb5f.
- R-3 (L5, RULED 2026-09-16 → spec/uat-round-1 dc-7, PLAN.md §7 I-130): scoped rule — no origin remote: base on HEAD with an explicit disclosure line; origin present but unresolvable/ambiguous: exit 2. Evidence: README quickstart, docs/local-adoption.md rehearsal, e2eharness unproven-board fixture all model the remote-less fresh project as a supported disclosed-unproven state; only verdict verbs fail closed. Original note: after ac-6, `design start` in a repository with no remote and no CI_DEFAULT_BRANCH exits 2 because specstate.ResolveDefaultBranch is unresolved there by design (no silent substitution of a local `main`). Options: (a) keep exit 2; (b) disclosed fallback to HEAD with an explicit line. Evidence requested from the L5 reviewer; decision to be recorded as a decision on spec/uat-round-1 before integration.
- R-4 (L5 precedence, CONFIRMED by reviewer: defaultbranch_test.go:107 pins remote-tracking-first): the lane used ResolveDefaultBranch's established remote-tracking-first ref rather than the dispatch's "local first" elaboration. Spec text ac-6 names only the shared resolver, so the lane's reading is the conforming one; the dispatch wording was the error. Pending reviewer confirmation.
- R-5 (L4 review minor 1): the policy setup guide now quotes the bare detail with no code (boardshellrender.go:290, :303). Information loss introduced by the lane; fix is workbench markup → Fable wave-1b lane (add PolicyCode to asdShell, render code + ": " + detail; keep PolicyDetail bare for the discriminant at boardspecasd.go:299).
- R-6 (L2 process): RED outputs were reconstructed after implementation rather than captured first. Disclosed by the lane; the reverted-line runs are genuine. Accepted for Tier 1; noted so it is not mistaken for RED-first evidence.

- R-7 (L2 review F1): vl017.go:64 moved the "wave close" literal out from under its `vocab:identity` marker; `go test ./internal/specalign -run TestVocabProseWitness` fails on the lane head and passes on the integration branch (controller-reproduced). Gate-breaking in `make verify`; Critical per reviewer, so the lane escalates to Tier 3 (fresh Opus fixer + fresh re-reviewer). Process correction: the pre-review gate for every remaining lane now includes the specalign vocab witness, since lanes only run their own packages.
- R-8 (L2 review F2): CollapseVL017Disclosures must gate on the absent-zone message, not severity alone, so a restated mutable-zone-present VL-017 finding can never be replaced by a false "absent" line. Unreachable today; fixed in the same Tier 3 pass.

- R-9 (L5 review F1): `--from-stub` is dispatched inside `design start` and still based on HEAD; ac-6 amended to name it explicitly; fix in internal/stubinstantiate under the same dc-7 rule; workbench stub-instantiate shares the seam (base selection only, no markup).
- R-10 (L5 review note 6): `build start` cuts `feature/<name>` from HEAD after resolving the default branch — same shape as UAT-021, out of this round's scope; logged as UAT-023 (plausible, not reproduced).

- R-11 (L1, RULED → spec/uat-round-1 dc-8): candidates build from a real .git checkout; no ldflags stamping this round. Reviewer reproduced: worktree build has zero vcs.* settings even with -buildvcs=true (silent); fresh-repo copy stamps correctly including `modified`. Original note: Go embeds no vcs.* settings when building from a linked worktree, so `verdi version` prints the honest placeholder there; all candidate binaries in this workspace are worktree builds. Reviewer asked to reproduce and cost a Makefile ldflags fallback; scope decision pending.

- R-12 (L2 fixer disclosure): the disclosure sentence still ends "(adjudicated at W2 wave close)", build-process trivia in user-facing output. Predates the lane; ac-3 does not ask for it; left as-is this round. Candidate copy cleanup for the tracker, not a defect.

- R-13 (L1 review minor 4): ac-2's "multi-line usage with one line per verb or subverb" is measured per form; a single-form verb printing one line conforms. No change.
- R-14 (L1 review minor 3): `verdi design board --help` exits 1 treating `--help` as a spec ref (pre-existing, two-level help is outside ac-2). To be logged in the UAT tracker as a follow-up, not fixed this round.

- R-15 (L5 closure note, CORRECTED by whole-wave review): all three callers of the shared scaffold core (`--from-stub`, board stub-instantiate, board creation form) sit behind the same accepted-pending-build wall guard, which refuses first in a remote-less store; the disclosed-HEAD fallback is therefore reachable only via the CLI `--kind/--name` path today. The guard is also what makes R-16's base change safe: the parent feature is on the default branch by construction.
- R-16 (L5 closure residual): board stub-instantiate and creation now base on the resolved default branch instead of the serving checkout's HEAD; e2e specs 31-board-stub-instantiate and 48-board-creation-form assert base-insensitive facts. Playwright is owed at the wave gate (browser behavior changed this wave).

- R-17 (Fable 1b review minor 1): `.site-foot` was undefined. Ruling: widen the Fable lane's write set by one file, internal/dex/assets/style.css, for a single minimal `footer.site-foot` rule; dex tests must pass. Manifest sentence corrected.
- R-18 (Fable 1b review): the wave gate must run e2e spec 50-design-workbench (axe wcag2a/aa scan covers the new footer); the integration worktree's npm install is checked for @axe-core/playwright before the gate.

## Fable wave-1b lane (agent/uat-r1-fable-1b, base 3d46f12c, head f604fe49)

Gate (controller): ancestry ok; tree clean; 15 files all inside write set (workbench Go + tests, e2e/tests 73–75 + fixtures.ts, report); no assets/CSS/dex touched; no recording artifacts; specalign vocab witness ok; `go test -race -count=1 ./internal/workbench/...` ok 120.5s. Lane ran focused Playwright 73–75 serially (5 passed, trace off) and the recording-artifact scan (0 files). Opus review ACCEPT-WITH-MINOR (reviewer reproduced 5/5 focused specs from a real-.git clone with a stamped footer line, plus specs 72 and 27: 26 passed; spec 50 could not run there for want of @axe-core/playwright). Minors 1–3 closed at 02544fc7..e44b7c3e (controller-verified: dex race ok, focused workbench ok); minor 4 accepted as disclosed; INTEGRATED at c2a56b3f, 0 drift. R-1 and R-5 CLOSED. Items:

1. Workbench footer renders internal/buildinfo string (ac-1 UI).
2. Record page renders Format and ProfilePrimaryDigest (ac-4 UI; R-1).
3. Policy guide quotes code + detail (R-5).
Each with a Playwright path under e2e/.

## Wave gate

Integrated head c2a56b3f (six lanes: L3 edd948de, L4 e7b700a2, L2 57f1bd55, L1 3d46f12c, L5 b01ba359, Fable 1b c2a56b3f). Whole-wave Opus review dispatched over 840c6166..c2a56b3f. `make verify` started on the integration worktree (log: scratchpad/wave1-verify.log).

Whole-wave Opus review: ACCEPT-WITH-MINOR, conditional on the gate. Independently verified: design.go merge = `git merge-file` of L1 and L5 byte-for-byte; per-lane blob comparison 0 drift; dc-7 three-way outcome probed live on the integrated binary; co-3 holds (no ac-10/ac-11 code). Minors: (1) stale comment in context_project_test.go → UAT-027; (2) dirty-tree refusal after interview → UAT-026; (3) footer shell set has no drift-guard test → wave-2 obligation for the Fable lane (mirror TestVerbUsageRegistry_CoversEveryVerb); (4) R-15 corrected above; (5) UAT-024 body extended to name `design start --help`; (6) fixtures.ts mirrors f13PrimarySHA256 by hand — pre-existing convention, note only.

## Wave-2 obligations carried
- Footer shell drift-guard test (whole-wave minor 3).
- UAT-026, UAT-027 (low) — candidates for a wave-2 cleanup lane alongside UAT-024/025.

### Wave gate run 1 — FAILED at `make test` (18:16–18:30)

`make verify` on 710b4bf7: build, fmt-check, vet passed; `make test` failed in ONE package — `internal/showcasealign` `TestShowcaseCoverage_EnumerationIsComplete` — everything else ok (cmd/verdi 506.4s ok, lint 174.7s ok, workbench 160.7s ok, sealedexec 164.5s ok). Later stages (fixture, lint-store, spec-align, lint-showcase, showcase-coverage, e2e) did not run.

- R-19 (gate failure, L1): the enumeration-completeness witness default-denies any pre-phase reference to `verb` in `run()` other than `verb := args[0]` and the blessed `if verb == "lint"` arm (whose body must not reference verb), because a verb dispatched before the `verbPhase` lookup escapes the showcase-coverage enumeration. L1's help-token and version intercepts are two new pre-phase arms, and its lint arm body now references `verb`. Same class as R-7 (a cross-binary witness the lane did not run) → L1 escalated to Tier 3: fresh Opus fixer on agent/uat-r1-cli, fresh re-review, re-merge, and a full re-run of `make verify`. Ruling on shape: the gate must stay default-deny; the honest fix blesses exactly the new arms and enumerates help/version in cliVerbs (satisfying whatever coverage that demands); evading the witness by hiding the `verb` reference is rejected. Process correction: the pre-review gate for every remaining lane and wave now runs `go test ./internal/showcasealign/... ./internal/specalign/...` in full, not just the vocab witness.
