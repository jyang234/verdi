# oq-2 samples — first ten findings per linter, path order, judged

Source data: `oq1-raw/<linter>.json` (golangci-lint v2.5.0, `--config
golangci.strict.yml --enable-only <linter> --max-issues-per-linter=0
--max-same-issues=0 ./...`, HEAD 5f60c76c). Sorted by `.Pos.Filename` (the
`../../../` worktree-relative-path prefix golangci-lint emits here is
stripped for readability — every path below is relative to the module
root) then `.Pos.Line`, first ten taken. Judgment is against the CLAUDE.md
ground rule quoted under each linter's heading: **TP** (true positive: a
real instance of what the sentence forbids/requires), **FP** (false
positive: the linter is factually wrong that this is an instance), or
**rule-not-stated** (the finding is a real instance of *something*, but not
of what the quoted sentence says — a scope mismatch between the linter's
own rule and CLAUDE.md's text).

## containedctx — 8 of 8 (fewer than ten exist)

> CLAUDE.md: "`context.Context` is the first parameter of anything that
> blocks or does I/O; **never stored in a struct**."

| # | file:line | finding | judged |
|---|---|---|---|
| 1 | internal/draftmutation/transaction.go:71 | struct field `ctx context.Context` | TP |
| 2 | internal/evidence/fold.go:19 | struct field `Context context.Context` | TP |
| 3 | internal/experimentevaluator/adapter_test.go:37 | struct field `runCtx context.Context` (test) | TP |
| 4 | internal/lint/engine.go:17 | struct field `Ctx context.Context` | TP |
| 5 | internal/sealedexec/claude/mcp_test.go:172 | struct field `ctx context.Context` (test) | TP |
| 6 | internal/sealedexec/codex/adapter_test.go:454 | struct field `ctx context.Context` (test) | TP |
| 7 | internal/sealedexec/mcp_test.go:955 | embedded `context.Context` field (test) | TP |
| 8 | internal/sealedexec/scoped_mcp_test.go:314 | struct field `ctx context.Context` (test) | TP |

**8/8 TP (100%).** Verified by direct terminal run (not just the JSON): every
line golangci-lint underlines is a real struct-field declaration of type
`context.Context`. Caveat: 6/8 are in `_test.go` files (test-double structs
holding a context for later use); CLAUDE.md's sentence carries no test
exception, so the letter of the rule is still violated, but a real strict
target may want an explicit ruling on test doubles before gating on this.

## noctx — 10 of 256

> CLAUDE.md: "`context.Context` **is the first parameter of anything that
> blocks or does I/O**; never stored in a struct."

| # | file:line | finding | judged |
|---|---|---|---|
| 1 | cmd/e2eharness/draftboards_test.go:96 | `exec.Command(...).Output()`, no ctx (test) | TP |
| 2 | cmd/e2eharness/emptyglance.go:82 | `net.Listen("tcp", ...)`, no ctx | TP |
| 3 | cmd/e2eharness/emptyglance_test.go:33 | `http.Get(url)`, no ctx (test) | TP |
| 4 | cmd/e2eharness/familyboardlinks_test.go:36 | `exec.Command(...)`, no ctx (test) | TP |
| 5 | cmd/e2eharness/git_test.go:31 | `exec.Command(...).CombinedOutput()`, no ctx (test) | TP |
| 6 | cmd/e2eharness/main.go:174 | `net.Listen(...)`, no ctx | TP |
| 7 | cmd/e2eharness/main.go:201 | `net.Listen(...)`, no ctx | TP |
| 8 | cmd/e2eharness/main.go:216 | `net.Listen(...)`, no ctx | TP |
| 9 | cmd/e2eharness/provision_vocab.go:230 | `net.Listen(...)`, no ctx | TP |
| 10 | cmd/e2eharness/provision_vocab_test.go:32 | `http.Get(url)`, no ctx (test) | TP |

**10/10 TP (100%).** Every finding is a genuinely blocking I/O call
(`net.Listen`, `exec.Command`, `http.Get`) with a real context-aware
alternative (`(*net.ListenConfig).Listen`, `exec.CommandContext`,
`http.NewRequestWithContext`) that the call site does not use. Caveat: all
ten cluster in `cmd/e2eharness` (a test-harness support binary), none in
`cmd/verdi` production code or `internal/`; the true positive rate may not
generalize evenly across the tree (unmeasured beyond this sample).

## contextcheck — 10 of 43

> CLAUDE.md: "`context.Context` is the first parameter of anything that
> blocks or does I/O; never stored in a struct."

| # | file:line | finding | judged | why |
|---|---|---|---|---|
| 1 | cmd/e2eharness/specimportfixture.go:303 | "Non-inherited new context, use function like `context.WithXXX` instead" | rule-not-stated | Flags `context.WithCancel(context.Background())` inside a function that already has an incoming `ctx` — a real, sensible idiom (propagate, don't refabricate), but a **different** rule than "ctx is the first parameter / never in a struct." The surrounding comment documents this as deliberate: the subprocess must outlive the request. |
| 2 | cmd/e2eharness/unprovenboard.go:163 | same message | rule-not-stated | Same pattern, same documented deliberate design (comment: "the subprocess outlives this request"). |
| 3 | cmd/verdi/context_execution.go:454 | "Function `assembleSealedRuntime$1` should pass the context parameter" | rule-not-stated | The flagged closure has signature `func() error` — fixed by the `terminals.bind(...)` callback contract, structurally cannot take a ctx parameter. Comment: "Amendment 003 gives both adapters a parent-hosted context surface" — a deliberate interrupt-path `context.Background()`. Same propagation-idiom mismatch as #1/#2. |
| 4 | cmd/verdi/foldload.go:85 | "Function `Fold` should pass the context parameter" | TP | `evidence.Fold(in Input) (StoryResult, error)` takes no ctx parameter; it unwraps `ctx := in.Context` from inside the `Input` **struct** and threads it to `foldAC`. This is exactly "never stored in a struct" from the callee's own definition, seen at the call site. |
| 5 | cmd/verdi/gc_reclaim_cli_test.go:71 | "Function `gcCLIFixture` should pass the context parameter" (test) | TP | `gcCLIFixture(t *testing.T)` does real I/O (`gitx.CheckoutNewBranch`, `os.WriteFile`, `gitx.Checkout`) via an internally fabricated `context.Background()`, with no ctx parameter at all. |
| 6 | cmd/verdi/rollup.go:144 | "Function `Fold` should pass the context parameter" | TP | Same `evidence.Fold(in)` pattern as #4, a second call site. |
| 7 | internal/experimentapp/journey_test.go:97 | "Function `authenticatedHuman` should pass the context parameter" (test) | TP | `authenticatedHuman(t)` takes no ctx, yet calls `resolver.Resolve(context.Background(), ...)` — the caller (journey_test.go) already has a live `ctx` in scope it cannot pass in. |
| 8 | internal/experimentapp/journey_test.go:210 | "...`mintRatificationVerification->signRatificationChallenge` should pass the context parameter" (test) | TP | `signRatificationChallenge` takes no ctx, calls `experimenthuman.Verify(context.Background(), ...)`. |
| 9 | internal/experimentapp/journey_test.go:245 | same chain (test) | TP | Second call site, same helper. |
| 10 | internal/experimentapp/journey_test.go:246 | same chain (test) | TP | Third call site, same helper. |

**7/10 TP, 3/10 rule-not-stated, 0 FP.** contextcheck bundles two distinct
checks under one linter name: "function doesn't expose ctx as a parameter at
all" (#4–#10, which matches CLAUDE.md's sentence directly) and "function
creates a fresh root context instead of propagating an available one"
(#1–#3, a real and sensible Go idiom, but not the sentence CLAUDE.md
actually wrote).

## errorlint — 10 of 173

> CLAUDE.md: "Errors: **wrap with `%w`** and context; distinguish verdict
> failures from operational errors..."

| # | file:line | finding | judged | why |
|---|---|---|---|---|
| 1 | cmd/verdi/align_test.go:1010 | non-wrapping `%v` for `splitErr` in `fmt.Errorf` (test) | TP | `splitErr` is an error value formatted with `%v`, not `%w`. |
| 2 | cmd/verdi/closepreflight_quarantine_test.go:70 | "type assertion on error will fail on wrapped errors. Use errors.As" (test) | rule-not-stated | Consumer-side inspection safety (`err.(*exec.ExitError)` vs `errors.As`), not the producer-side "wrap with %w" CLAUDE.md states. |
| 3 | cmd/verdi/context_execution.go:369 | non-wrapping `%v` for `cause` | TP | `sealedTerminalOutcome(cause error, ...)` — `cause` is statically `error`; the call already uses `%w` for `sealedexec.ErrOperational` and `%v` for `cause`. Go 1.25 (this module's pin) supports multiple `%w` verbs per `Errorf` since 1.20, so wrapping both is possible and the finding is real. |
| 4 | cmd/verdi/context_execution.go:503 | non-wrapping `%v` for `err` | TP | `err` from `sealedexec.PreservedExecutionForBytes(...)`, a real error value formatted with `%v` alongside a `%w`. |
| 5 | cmd/verdi/context_execution.go:518 | non-wrapping `%v` for `err` | TP | Same pattern, `sealedexec.EncodeExecutionPartial(...)`'s error. |
| 6 | cmd/verdi/context_execution.go:522 | non-wrapping `%v` for `err` | TP | Same pattern. |
| 7 | cmd/verdi/context_execution.go:540 | non-wrapping `%v` for `err` | TP | Same pattern. |
| 8 | cmd/verdi/context_execution.go:544 | non-wrapping `%v` for `err` | TP | Same pattern. |
| 9 | cmd/verdi/context_execution_contract_test.go:2962 | type assertion on error (test) | rule-not-stated | Same class as #2. |
| 10 | cmd/verdi/designappcli_test.go:32 | type assertion on error (test) | rule-not-stated | Same class as #2. |

**7/10 TP, 3/10 rule-not-stated, 0 FP.** errorlint also bundles two checks:
the `%w`-verb check (CLAUDE.md's actual sentence) and a type-assertion
safety check (a real Go 1.13+ idiom, but a different rule).

## exhaustive — 10 of 129

> CLAUDE.md: "...unknown enum values **fail closed**."

| # | file:line | finding | judged | why |
|---|---|---|---|---|
| 1 | cmd/verdi/buildstart.go:201 | missing `specstate.Unproven` case | rule-not-stated | `switch result.State` has `default: // specstate.Unproven` that already returns exit 2 with a "cannot be proven" message — the code already fails closed; exhaustive wants every case *named*, which is a stricter, different rule. |
| 2 | cmd/verdi/claude_execution_e2e_test.go:1119 | missing ~20 of 21 `sealedexec.ControllerOperation` cases (test) | rule-not-stated | A test-fixture switch that selectively enriches two known operations; every other operation is left unmodified (a no-op), which is the correct fixture behavior, not a production fail-open path. |
| 3 | cmd/verdi/close.go:587 | missing `specstate.Unproven` case | rule-not-stated | Same shape as #1: `default: // specstate.Unproven` already returns exit 2, fail-closed. |
| 4 | cmd/verdi/closeexperiment.go:147 | missing `experimentapp.ClassificationOperational` case | rule-not-stated | `default:` branch's own comment: "a **defensive fail-closed** line rather than a silent skip if it ever does." Already exactly what CLAUDE.md asks for. |
| 5 | cmd/verdi/closeexperiment.go:214 | same enum, second switch | rule-not-stated | `default: ...; return 2` — fail-closed. |
| 6 | cmd/verdi/context_execution_contract_test.go:1693 | missing 2 of 4 `sealedexec.InspectionKind` cases (test) | rule-not-stated | Narrow test-fixture switch building one specific scenario; no default, no production reach. |
| 7 | cmd/verdi/context_execution_contract_test.go:2614 | missing 2 `sealedexec.ControllerOperation` cases (test) | rule-not-stated | Test-fixture switch; disappears entirely when `default-signifies-exhaustive: true` is set (see below) — it does have a default. |
| 8 | cmd/verdi/context_execution_contract_test.go:2794 | missing `sealedexec.PreservedState` case (test) | rule-not-stated | Narrow test-fixture switch, no default, no production reach. |
| 9 | cmd/verdi/gate.go:342 | missing 3 `specstate.State` cases | rule-not-stated | `default: // Proposed, Superseded, Closed` already returns a non-OK `gateCondition` — fail-closed. |
| 10 | cmd/verdi/obligation.go:446 | missing 3 `specstate.State` cases | rule-not-stated | `default: // AcceptedPendingBuild, Superseded, Closed` already refuses (exit 1) — fail-closed. |

**0/10 TP, 10/10 rule-not-stated.** Every sampled finding is either (a) a
switch that already has an explicit `default:` fail-closed clause — meeting
CLAUDE.md's actual (behavioral) rule while failing exhaustive's stricter
(syntactic, every-case-named) one — or (b) a narrow test-fixture switch with
no production fail-open risk. Confirmed structurally: re-running with
`settings.exhaustive.default-signifies-exhaustive: true` over `./cmd/verdi/...`
drops the count from 15 to 7 (command + counts in `oq3-and-exhaustive-extra.txt`);
the 7 that remain are exactly the switches with **no** default clause
(including #2, #6, #8 above), none of which are production fail-open bugs in
this sample. **This is the single most consequential number in this spike
for ac-1's linter-list decision**: at default settings exhaustive's TP rate
against the written rule is 0% in this sample; the fix is a one-line setting,
not dropping the linter.

**Fix-round-1 correction to the 7 survivors' split** (an earlier version of
the README's oq-2 section mischaracterized all 7 as test fixtures; this
section's own per-row labels were already correct, but the aggregate split
was never stated plainly): the 7 split **4 test / 3 production**. Test:
`claude_execution_e2e_test.go:1119`, `context_execution_contract_test.go:1693`,
`context_execution_contract_test.go:2794`, `selfevidence_test.go:120`.
Production: `readiness_snapshot.go:369`, `stubmatch.go:164`,
`sync_quarantine.go:157` — each judged individually in
`oq2-exhaustive-survivors.txt` (not merely asserted): `stubmatch.go`'s
switch is unambiguous (the function's own name states its two-type scope);
`sync_quarantine.go`'s is a deliberate, currently-correct
elimination-by-`continue` pattern over a doc-commented closed three-value
enum, with a named latent risk if a fourth value is ever added;
`readiness_snapshot.go`'s most likely is deliberate but its exclusion of
`AnnotationDecisionNeeded` specifically is left an open question, not a
ruling this sample has the context to make. None of the three is argued to
be a fail-open bug today — the sentence above ("none of which are
production fail-open bugs") already held under the corrected split, it was
just never spelled out.

## dupl — 10 of 37

> CLAUDE.md: "Anything used by two or more packages lives in a shared
> `internal/` package...; **never copy-paste across packages**."

| # | file:line | duplicate of | same package? | judged |
|---|---|---|---|---|
| 1 | cmd/e2eharness/specimportfixture.go:279 | cmd/e2eharness/unprovenboard.go:134 | yes — both `package main` in `cmd/e2eharness` | rule-not-stated |
| 2 | cmd/e2eharness/unprovenboard.go:134 | cmd/e2eharness/specimportfixture.go:279 | yes (reverse direction of #1) | rule-not-stated |
| 3 | cmd/verdi/claude_execution_e2e_test.go:236 | cmd/verdi/context_execution_contract_test.go:3097 | yes — both `package main` in `cmd/verdi` | rule-not-stated |
| 4 | cmd/verdi/claude_execution_e2e_test.go:279 | cmd/verdi/context_execution_contract_test.go:3145 | yes | rule-not-stated |
| 5 | cmd/verdi/claude_execution_e2e_test.go:417 | cmd/verdi/context_execution_contract_test.go:734 | yes | rule-not-stated |
| 6 | cmd/verdi/context_execution_contract_test.go:734 | cmd/verdi/claude_execution_e2e_test.go:417 | yes (reverse of #5) | rule-not-stated |
| 7 | cmd/verdi/context_execution_contract_test.go:3097 | cmd/verdi/claude_execution_e2e_test.go:236 | yes (reverse of #3) | rule-not-stated |
| 8 | cmd/verdi/context_execution_contract_test.go:3145 | cmd/verdi/claude_execution_e2e_test.go:279 | yes (reverse of #4) | rule-not-stated |
| 9 | internal/contextcompile/semantic_dependencies_test.go:303 | internal/contextcompile/semantic_dependencies_test.go:326 | yes — **same file** | rule-not-stated |
| 10 | internal/contextcompile/semantic_dependencies_test.go:326 | internal/contextcompile/semantic_dependencies_test.go:350 | yes — same file | rule-not-stated |

**0/10 TP, 10/10 rule-not-stated.** Package membership verified directly
(`package` clause of every file involved — all five files are `package
main` or `package contextcompile`, and every pairing is within one
directory, two pairs within one file). CLAUDE.md's sentence specifically
prohibits **cross-package** copy-paste, and this is not a sampling
artifact: golangci-lint dispatches `dupl` per package, so it structurally
cannot report a cross-package pair at all, ever — verified over the full
`./...` run, not just this ten-sample draw: 0 of all 37 dupl findings are
cross-package (`oq2-dupl-package-scope.py`, output in
`oq2-dupl-package-scope-output.txt`), and 0 of 13,521 remain cross-package
even at `dupl.threshold: 10` over the retro-witness's restricted scope
(`oq2-dupl-overlap-check-output.txt`), while cross-file same-package pairs
are abundant at that threshold. None of the ten sampled findings are true
positives against the sentence as written, and none *can* be, at any
setting; several are legitimate same-file/same-package "extract a helper"
opportunities under ordinary Go hygiene (a rule CLAUDE.md does not
separately state). See oq-2's retro-witness below for the corrected,
line-range-overlap-verified second data point on dupl's reach.

## gochecknoglobals — 10 of 404

> **No CLAUDE.md sentence in the Go style or Testing rules sections states
> this rule.** The parent feature spec's own problem statement maps
> gochecknoglobals to "no package-level mutable state," but that phrase (or
> an equivalent) does not appear in `/Users/johnyang/code/verdi-system/CLAUDE.md`
> — confirmed by reading the file in full (Go style: 9 bullets; Testing
> rules: 4 bullets; neither mentions globals, package-level state, or
> mutability). This is a discrepancy in the parent spec's own premise, not
> something this spike can resolve by writing a rule CLAUDE.md does not
> contain — recorded here, not silently patched. (The parent spec's table
> does not quote CLAUDE.md for *any* of its six rows; the other five differ
> from this one only in that their paraphrases trace to sentences that
> actually exist.)

Because there is no sentence to hold a finding against, all ten are formally
**rule-not-stated** by construction. What follows is the mechanical-accuracy
check instead (is the finding at least a real package-level variable):

| # | file:line | finding | mechanically real? | note |
|---|---|---|---|---|
| 1 | cmd/e2eharness/git.go:31 | `deterministicGitEnv` | yes | assigned once, never reassigned |
| 2 | cmd/e2eharness/main.go:266 | `ciEnvVars` | yes | assigned once, never reassigned |
| 3 | cmd/e2eharness/specimportfixture.go:87 | `specImportParentSpecRel` | yes | `filepath.Join` result — logically a constant, `var` only because Go `const` cannot hold a function call |
| 4 | cmd/e2eharness/unprovenboard.go:51 | `serveInjectionEnvVars` | yes | assigned once, never reassigned |
| 5 | cmd/e2eharness/unprovenboard.go:90 | `unprovenBoardSpecRel` | yes | same as #3 |
| 6 | cmd/verdi/aligndiagramsweepstatic_test.go:15 | `forbiddenDiagramSweepReferences` (test) | yes | assigned once |
| 7 | cmd/verdi/audit_test.go:18 | `auditTestNow` (test) | yes | fixed time literal, effectively constant |
| 8 | cmd/verdi/close.go:161 | `closeAddPaths` | yes | **genuinely mutable**: reassigned by `close_test.go`/`closefeature_test.go` (`closeAddPaths = func(...){...}`, restored via `defer`) as a fault-injection seam — a documented, deliberate DI pattern |
| 9 | cmd/verdi/close.go:162 | `closeCreateCommit` | yes | same DI-seam pattern as #8, sibling declaration |
| 10 | cmd/verdi/closeexperiment_test.go:66 | `closeExperimentWaiverSlug` (test) | yes | assigned once |

**10/10 mechanically real package-level variables; 2/10 (#8, #9) genuinely
reassigned at runtime** (the class a "no mutable global state" rule would
most want to catch); the other 8/10 are assigned exactly once and never
reassigned again (closer to "a named constant Go's `const` cannot express").
No finding is a linter mistake; the open question is entirely about whether
CLAUDE.md should gain the sentence the parent spec assumes it already has.

## oq-2 retro-witness (1be75d01 vs 410db101)

Scratch worktree `verdi-wt/scratch-lint-w1`, checked out `--detach` to each
commit in turn and restored to `--detach e963f4d0` afterward (confirmed
clean both before and after — see `oq2-retro-witness.txt`). Scope:
`./internal/readinessload/... ./cmd/verdi/... ./internal/store/...`.

| commit | gochecknoglobals reports `serveReadinessLoader`? | dupl reports `hasDotDotElement`? |
|---|---|---|
| 1be75d01 (first head) | **yes** — `cmd/verdi/serve.go:142:2: serveReadinessLoader is a global variable (gochecknoglobals)` | **no**, at any threshold tried (150 default, 30, 20, 15, 10 — see below) |
| 410db101 (named "fixed" head) | **yes, still** — `cmd/verdi/serve.go:149:2: serveReadinessLoader is a global variable (gochecknoglobals)` | no (the function no longer exists anywhere in the tree at this commit) |

Both story predictions are **violated-with-witness**, for two different
reasons:

1. **dupl never reports `hasDotDotElement` at 1be75d01**, not even at
   `dupl.threshold: 10` (golangci-lint's default is 150; the sweep 150/30/
   20/15/10 produced 6/944/3765/6919/13521 total findings respectively —
   confirming the config is live and threshold-sensitive — but the specific
   9-line duplicate pair `cmd/verdi/context.go:485` /
   `internal/readinessload/conflict.go:232` never appears at any of those
   thresholds). The duplicate is real: byte-identical 9-line function
   bodies, one file's own comment at 1be75d01 calls it "a small, deliberate
   duplicate of cmd/verdi/context.go's own helper." dupl's clone detector
   does not reach a function this short regardless of threshold tuning —
   disclosed-as-unproven exactly *why* (not established beyond "not a pure
   threshold effect"), but the non-detection itself is proven and
   reproducible.
2. **`serveReadinessLoader` is still a global at 410db101** — the commit
   named as "the fixed one" is not where that particular fix landed. The
   diff between the two commits (`git diff 1be75d01 410db101 --
   internal/readinessload/conflict.go`) shows 410db101 *does* fix the dupl
   half of ac-3's example (both copies of `hasDotDotElement` are gone,
   replaced by a call to a new exported `store.HasDotDotElement` in the
   shared `internal/store` package — the textbook-correct remedy CLAUDE.md's
   sentence prescribes) but is titled "harden co-2 persistence check, add a
   hermetic conflict-provider seam" and does not touch `serve.go`'s globals
   at all. `410db101` is also not an ancestor of main (`git merge-base
   --is-ancestor 410db101 5f60c76c` fails); it is a commit on
   `agent/readiness-recovery-wave-1`, a branch whose net changes landed on
   main by a different path. At main (5f60c76c), `serveReadinessLoader` no
   longer appears in `cmd/verdi/serve.go` at all — the global-variable fix
   is real, it just is not at 410db101.

Net: the retro-witness's dupl half is unreachable at any tested threshold
(a tool-capability gap, not a config mistake); its gochecknoglobals half
picked a SHA that predates that particular fix. Neither is a reason to
distrust the underlying feature premise — both underlying problems (the
cross-package duplicate, the global loader variable) were real and were
both eventually fixed on this branch — but ac-3's own two example commits,
read literally, do not both hold.
