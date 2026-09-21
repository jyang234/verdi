# Ritual write-scope spike — answer

Resolves `spec/ritual-write-scope#oq-1` through `#oq-5` (spike story
`spec/ritual-write-scope-spike`). Base `5f60c76c` (main). Throwaway code
and logs live entirely under this directory (VL-016 fence); the recorder
patch to `internal/gitx` is never committed — see `recorder.patch` and
"Method" below.

## Recommendation, up front

Build the empirical inventory first (done here) before writing ac-1's
registry. Asked what each ritual does with an index entry the operator
staged and the ritual never named, the five split into **four** distinct
shapes, not two: `build_start` never commits at all, so the question is
moot; `design_start` and `commit_to_design` **carry it** into their own
commit (UAT-036, open — reproduced empirically below); `close`
**refuses to begin**, exiting 2 at `requireCleanIndex`, `runClose`'s first
statement (`cmd/verdi/close.go:655`); `policy_adopt` **scopes its commit**
with a `--` pathspec so the entry is simply left in the index. The
seven-field grammar the story proposes needed exactly one change to
express that: its `index_carry_foreign` boolean cannot distinguish the
four, so this spike replaces it with a four-valued `index_carry` enum
(same slot, still seven fields — `refs_move` remains empty for these five
specifically).

The registry's home is a Go literal checked by a gate test — the policy
kernel's payload system technically admits the shape but is a category
mismatch (governance an operator *adopts*, not fixed product behavior an
operator cannot widen). readiness-recovery's ac-9 needs strictly less than
ritual-write-scope's ac-1–ac-3 need from a recorder seam, so
ritual-write-scope should land it, sketched below as a `context.Context`-
scoped `Recorder` (zero gitx signature changes, no new package-level
mutable state).

## Method

`internal/gitx/exec.go`'s `run`, `plumbing.go:28`'s `runStdin`, and
`configvalue.go:75`'s `ConfigValue` were patched (`recorder.patch`, 73
lines) to call a `spikeGitLog(dir, args)` helper that appends
`<who>\t<dir>\t<args>` to the file named by `VERDI_SPIKE_GITLOG` when set,
a no-op otherwise (`who` = `filepath.Base(os.Args[0])` plus the process's
first two `os.Args`, so a `verdi <verb> <sub>` child process labels itself
by verb and an in-process test binary labels itself `<pkg>.test`, per the
lane brief's spec). The patch was reverted from the tree before every
commit (`git status --porcelain` shows nothing outside this directory at
every commit; verified below).

```
VERDI_SPIKE_GITLOG=/tmp/verdi-spike-gitlog-raw.tsv \
  go test ./cmd/verdi/ ./internal/designapp/ ./internal/wtmanager/ \
          ./internal/commitdesign/ ./internal/policyadopt/
```

Exit 0, all five packages `ok` — `test-run-stdout.log`.

This produced 21174 raw lines (`gitlog-raw-sample.tsv` keeps a 300+100
line head/tail sample; the full deduplicated `(who, subcommand, flags)`
reduction — 524 lines, well under the evidence cap — is
`gitlog-dedup-all-verbs.tsv`). Per-ritual reduction is `inventory.tsv`.

That `(who, subcommand, flags)` reduction is **operand-free by
construction**, which drops exactly the tokens `refs_create` and
`stage_paths` are made of. A second, operand-preserving reduction of the
same log — `inventory-operands.tsv`, 37 rows, produced by
`reduce-operands.sh` whose rule it states in full — keeps the complete
argument vector of every mutating invocation of the five rituals, branch
names, bases, pathspecs and `--` separators intact. The whole sweep was
re-run independently in fix round 1 (recorder patch re-applied in a fresh
`/tmp` clone; exit 0, all five packages `ok`, 21173 raw lines against the
first run's 21174), and the reduction over both raw logs is byte-identical
— so every operand cited below reproduced across two independent runs.

**Timing/load disclosure (constraint 7):** this run crossed a session
interruption (an API billing error unrelated to this spike); the
controller's contemporaneous observation put the 1-minute load average
near 47 during the run, consistent with other process-hardening spike
lanes sharing this 10-core machine concurrently. My own `uptime` readings
bracket it loosely: `8.83 14.41 9.61` before the run started, `3.53 2.48
1.98` after work resumed post-interruption. No wall-clock timing claim is
made from this run — only that it completed with exit 0 across all five
packages, which is what makes the resulting log trustworthy as a complete
(not truncated-by-crash) record. The command is re-runnable verbatim
above; it is not itself a timing measurement.

## oq-1: What git commands does each verb actually run today?

**Package substitution (R-PHS-5, recorded again here per constraint 10):**
`internal/closeapp` does not exist at this base; the close ritual lives in
`cmd/verdi/close.go`/`closefeature.go`/`closepreflight.go`/`closeprepare.go`,
exercised via `cmd/verdi`. Ran over `./cmd/verdi/ ./internal/designapp/
./internal/wtmanager/ ./internal/commitdesign/ ./internal/policyadopt/`.

**Attribution.** `cmd/verdi`'s own test suite both calls ritual entry
points in-process (labelled `verdi.test ...`) *and* builds+execs the real
patched binary as each verb (labelled `verdi design start`, `verdi build
start`, `verdi close --force-local`, `verdi policy adopt`, etc. — 55
distinct verb/flag combinations observed, `gitlog-dedup-all-verbs.tsv`).
The direct verb-subprocess entries are unambiguous (the `who` field *is*
the verb argv) and are this table's primary source for `design_start`,
`build_start`, `close`, and `policy_adopt`. `commit_to_design` has no
direct CLI verb (invoked from `board.go`/`forgeboot.go`/workbench
`board.go`); its package-scoped in-process bucket (`commitdesign.test`,
71 lines) is unambiguous the same way, since that whole package *is* the
ritual. `internal/policyadopt`'s own package tests produced **zero**
gitx-recorder lines — its git-touching behavior is exercised only through
`cmd/verdi`'s subprocess-driven `verdi policy adopt` tests (96 lines), a
fact worth carrying into the feature build (the package's own unit tests
don't cover the ritual's actual git footprint).

**Per-ritual command inventory** (full detail, mutating/read-only
classification, and per-line notes: `inventory.tsv`; 69 rows):

| Ritual | Mutating calls | Read-only calls |
|---|---|---|
| `design_start` | `add --` (index, scoped), `checkout -b` (refs+HEAD+worktree), `commit -m` (refs, **not** `--`-scoped) | `ls-tree`, `remote`, `rev-list`, `rev-parse`, `show`, `show-ref`, `symbolic-ref` |
| `build_start` | `checkout -b` (refs+HEAD+worktree) — the *only* mutating call | `config --local --get-all`, `ls-tree`, `remote`, `rev-list`, `rev-parse`, `show`, `show-ref`, `status`, `symbolic-ref` |
| `close` | `add --` (index, scoped, `--force-local` only), `checkout -b` (`close/<name>`, `--force-local` only), `commit -m` (refs, **not** `--`-scoped) | `hash-object` (no `-w`: digest only), `log`, `ls-files`, `ls-tree`, `merge-base`, `remote`, `rev-list`, `rev-parse`, `show`, `show-ref`, `status`, `symbolic-ref` |
| `commit_to_design` | `add --` (index, scoped), `commit -m` (refs, **not** `--`-scoped) | `diff`, `log`, `ls-files`, `merge-base`, `rev-parse`, `status` |
| `policy_adopt` | `add --` (index, scoped), `checkout -b` (refs+HEAD+worktree), `commit -m --` (refs, **`--`-scoped**) | `config --local --get-all`, `rev-parse`, `show-ref`, `symbolic-ref`, `var` |

`internal/gitx/plumbing.go`'s scratch-index primitives (`hash-object -w`,
`read-tree`, `update-index`, `write-tree`, `commit-tree`, `update-ref`)
were **not** observed for any of these five rituals — confirmed both
empirically (no such lines under any of the five buckets above) and by
source: `plumbing.go`'s only non-test callers are `internal/specimport/
publish.go` (`design import`) and `internal/stubinstantiate/
stubinstantiate.go` (the workbench stub-instantiate board action) —
different verbs entirely. All five rituals here go through `exec.go`'s
`run` only, plus `configvalue.go`'s `ConfigValue` for `build_start` and
`policy_adopt`'s identity reads.

**Bonus finding — three of the six UAT findings the parent spec lists are
already fixed at this base, one is confirmed still open, and one (design
start's own case) is confirmed fixed while its sibling ritual is not:**

| Finding | Parent spec's table | This spike's empirical + source finding |
|---|---|---|
| UAT-021 (design start branches from stale HEAD) | fixed, pending merge | **Confirmed fixed.** `design.go`'s `resolveBranchBase`/`gitx.CheckoutNewBranchFrom` resolve the default branch; empirically the observed base was `main`/`origin/<default>` or a disclosed HEAD fallback only when no `origin` remote exists. |
| UAT-023 (build start cuts from HEAD) | open | **Confirmed still open.** `buildstart.go` calls `gitx.CheckoutNewBranch` (no base param — `git checkout -b <name>`, git's implicit-HEAD default), never `resolveBranchBase`/`CheckoutNewBranchFrom` (zero hits, grepped). `gitx/branch.go`'s own doc comment on `CheckoutNewBranchFrom` narrates design start's UAT-021 fix story — build start never received the analogous change. |
| UAT-031 (branch-cutting checks collide against the wrong tree) | open | **Design start's own case is fixed** (`design_test.go`: `TestRunDesignStart_BehindCheckout_NameOnMainRefused`; `designsupersede.go` comments cite the same fix). Build start / close were not traced for an equivalent collision check (grepped: no hits) — **not answered** for those two specifically. |
| UAT-033 (design start commits every untracked file) | open | **Confirmed fixed**, contradicting a literal reading of the parent table (the table reflects spec-authoring time, before this base). `design.go`/`designsupersede.go` call `gitx.AddPaths`, never `gitx.AddAll`; `design_test.go`'s `TestRunDesignStart_ScaffoldCommitStagesOnlySpecDir` asserts `AddAll` is never called. Empirically confirmed: every `add` line for `design_start` is `add -- <one specific spec dir>`, never `-A`. |
| UAT-034 (commit-to-design sweeps the same way) | open | **Confirmed fixed**, same shape: `internal/commitdesign/commitdesign.go` uses `gitx.AddPaths`; `commitdesign_test.go`'s `TestRun_ScaffoldCommitStagesOnlySpecDir` asserts `AddAll` is never called. |
| UAT-036 (ritual commits carry pre-staged index entries) | open | **Confirmed still open for TWO of the five rituals — `design_start` and `commit_to_design`** (this row said three in this README's first issue; see "Correction: how `close` was got wrong" below). Both call `git commit -m "<msg>"` with **no** `--` pathspec and have **no** pre-ritual index guard, so each commits whatever the *whole index* holds; reproduced empirically for `design_start` on an ac-2-seeded fixture (`state-diff/seeded-designstart-commit-files.txt`: the ritual staged one path and its commit carries two, the second being the operator's `spike-foreign-staged.txt`). `close` shares the pathspec-less commit shape and is still **not** exposed: `requireCleanIndex` is `runClose`'s first statement (`cmd/verdi/close.go:655`; guard at `:920-944`) and exits 2 before any mutation — witnessed in `state-diff/seeded-close-stderr.txt`. `policy_adopt` is immune a third way: its commit **is** `--`-scoped (`commit -m ... -- <the 4 paths it added>`). `build_start` never commits. |

This resolves the apparent UAT-033/034 vs. table conflict rather than
silently reconciling it: the parent spec's problem-statement table was
authored against an earlier commit; this base is newer for those two.

**Correction: how `close` was got wrong (fix round 1).** This README's
first issue put `close` in the UAT-036 column and called the finding "the
widest of the six". Two compounding method errors produced that:

1. `index_carry_foreign` was inferred from the *shape of the command log*
   — a `commit -m` with no `--` was read as "carries the whole index" —
   with no check for a guard upstream of the commit. That is this spike's
   own oq-2 ruling turned against itself: a command log over-reports as
   readily as it under-reports, because it records what ran, never what
   refused to run.
2. The absence of a fix was inferred from a token grep (`UAT-036`, no
   source hits). `close`'s guard landed under a different label
   (`D6-33`, ledger `L-N15(2)`, named in `close.go:927-931`), so the grep
   false-negatived.

**The value is now derived from source guards for all five.** The census
is every non-test caller of a pre-ritual index/worktree refusal primitive
(`grep -rn --include='*.go' -E "gitx\.(StagedPaths|StatusDirty|WorktreeChangedPaths)" cmd internal | grep -v _test | grep -v '^internal/gitx/'`,
plus `grep -rn --include='*.go' -E "^func [a-zA-Z]*(require|refuse)[A-Za-z]*\(" cmd internal | grep -v _test`):

| Ritual | Pre-ritual refusal? | Commit pathspec-scoped? | `index_carry` |
|---|---|---|---|
| `design_start` | no — zero guard calls in `design.go`/`designsupersede.go` | no (`gitx.CreateCommit`, design.go:660) | `carried` |
| `build_start` | n/a | never commits (no `AddPaths`/`CreateCommit` in `buildstart.go`) | `no_commit` |
| `close` | **yes** — `requireCleanIndex` (close.go:932) is `runClose`'s first statement (:655) | no (`closeCreateCommit = gitx.CreateCommit`, close.go:162) | `refused` |
| `commit_to_design` | no — zero guard calls in `internal/commitdesign/` | no (`gitx.CreateCommit`, commitdesign.go:222) | `carried` |
| `policy_adopt` | no | **yes** (`policyAdoptCommit = gitx.CreateCommitPaths`, policy.go:57) | `scoped` |

`requireCleanIndex` is the **only** pre-ritual index refusal in the whole
non-test tree; the three other non-test `gitx.StagedPaths` callers
(`internal/journey/port.go`, `internal/repositoryfacts/port.go`,
`internal/constitutionapp/identity.go`) are observation ports, and every
non-test `gitx.StatusDirty` caller belongs to `context`, `gc`, `residue`,
`workbench` or `execworkspace` — none of the five. Both close commit sites
(`close.go:890` spec closure, `closefeature.go:256` feature closure) are
downstream of `runClose`, whose only production caller is `close.go:397`;
`--preflight` takes a separate read-only path that rehearses the same
guard (`closepreflight.go:107`) and never commits.

Two consequences beyond the row itself. **ac-4's pin scope is two rituals,
not three**: a regression witness that "a ritual commit never carries a
pre-staged entry outside its declared paths" has `design_start` and
`commit_to_design` to fix, while `close` needs its existing refusal pinned
instead — a different assertion (exit 2 and no mutation), not the same
one. And under ac-1 a declared `index_carry_foreign: true` for `close`
would have been a *permission*: ac-2's behavioral witness would then pass
a future change that deleted `requireCleanIndex`, which is exactly co-1's
"widening of its declaration without an owner-visible decision".

**29-file cross-reference** (`grep -l github.com/jyang234/verdi/internal/
gitx cmd/verdi/*.go | grep -v _test`, confirmed 29 files): 19 files show
direct, unambiguous evidence in the log (`actorlocal.go`, `align.go`,
`attest.go`, `buildstart.go`, `close.go`, `closepreflight.go`,
`closeprepare.go`, `countersign.go`, `context_execution.go`, `design.go`,
`designsupersede.go`, `experiment_human.go`, `experiment_ports.go`,
`gate.go`, `gc.go`, `obligation.go`, `policy.go`, `sync.go`, `waive.go`);
3 more are present by reasonable inference not directly isolated
(`closefeature.go` — dispatched from `close.go:680`, which is extensively
present; `foldload.go` — its `IsAncestor` helper is the only cmd/verdi
source of the `merge-base --is-ancestor` pattern seen under `verdi close
spec/close-fixture`; `forgeboot.go` — its `loadManifest` is called from
`gate.go`/`sync.go`/`context_conflict.go`, all of which are present); 6
files' own specific behavior was not isolated by this run's test-name
matching (`align_design.go`, `aligndiagramsweep.go`,
`gate_decisionconflict.go`, `gate_threads.go`, `sync_quarantine.go`,
`sync_ancestor.go` — their shared dispatchers, e.g. `cmdAlign`, are
present, but these files' *own* branches were not confirmed reached, and
are marked not-observed rather than guessed present); 1 file
(`acceptdiagram.go`) produced no matching evidence at all in this run.

**ANSWERED**: table above and `inventory.tsv`, backed by the recorder's
log (`gitlog-dedup-all-verbs.tsv`, `gitlog-raw-sample.tsv`).

## oq-2: Non-gitx mutations and the sensor ruling

**Census** (`grep -rn "exec.Command" --include=*.go internal cmd | grep -v
_test | grep -v internal/gitx`, 21 real call sites across 17 files; 2
matches were comment text, excluded):

| Site | Can mutate the operator's repository? | Why |
|---|---|---|
| `internal/align/judge.go:62` (`ExecJudgeRunner.RunJudge`) | **YES — unsandboxed, reachable from `close --prepare`** | `cmd.Dir` is **never set** (inherits verdi's own process cwd — the operator's repo, when reached from a ritual); `cmd.Env` is never set (full ambient environment). Default runner for every `internal/align` judge call site (`decision_report.go`, `decision_judge.go`, `diagram_report.go`, `diagram_judge.go`, `report.go`). `closeprepare.go`'s freeze-in-place branch names `align.judge_cmd` explicitly; this repo's own `verdi.yaml` configures `judge_cmd: [claude, -p, --output-format, json]` — a real agentic CLI. Whether the launched binary itself further mutates the repo is outside this codebase's control (not exercised in this run — no live judge auth). |
| `internal/upstream/runner_real.go:45` (`RealRunner.Run`) | **YES — unsandboxed, reachable from `design_start`, `build_start`, `close`** | `cmd.Dir = r.Dir` (the repository root — confirmed: `design.go`, `buildstart.go`, `close.go` all construct `upstream.RealRunner{..., Dir: root}`); `cmd.Env = os.Environ()` unless overridden. Nothing here prevents the pinned upstream binary (`go run <module>/cmd/<bin>@<commit>`) from writing into `root`; the "read-only, JSON-only" contract (ground rules) is a design intent, never enforced in this code path. **Not triggered in this run's state-diff trial**: `regenerateBaseline` (`baseline.go`) skips the runner entirely when the new spec declares no `impacts:` matching a discovered service — true for a bare `design start --kind feature --name <x>` — so this capability is proven reachable by source but not exercised dynamically here; see the trial below. |
| `internal/execworkspace/isolation.go:235` (`Profile.Command`) | **NO, by construction, for its actual callers** | `Command` itself leaves `cmd.Dir` unset ("the working directory is the consumer's choice"), but both real consumers set it explicitly to an isolated workspace: `internal/sealedexec/claude/adapter.go:246` and `internal/sealedexec/codex/adapter.go:213` both do `command.Dir = launch.Workspace.Path`, never the operator's repo; `cmd.Env` is `p.Env()` (profile-owned, not ambient). Not reachable from any of the five rituals (only `context`, `gc`, `experiment` verbs import `sealedexec`/`execworkspace`). |
| `internal/publicrelease/{execution.go:55,inputs.go:60}` | Capability: yes (fully data-driven, `c.Args[0]`/`args[0]` from caller config) — **reachability: never**, from any `verdi` verb | `internal/publicrelease` is imported only by `cmd/public-execution-contract-release/main.go`, a **separate binary**, not `cmd/verdi`. |
| `internal/publicrelease/sourcecheck.go` (×3: `git show`, `git ls-tree`, `go list -deps`) | No (read-only ops) and same separate-binary reachability gap | — |
| `internal/fixturegit/fixturegit.go:148` | **NO** | Test-scaffolding package; its only non-test importer is `cmd/e2eharness/git.go` (a separate binary); operates only on fixture repos it creates itself. |
| `cmd/e2eharness/*` (7 sites across `git.go`, `main.go`, `provision_showcase_draft.go`, `specimportfixture.go`, `unprovenboard.go`) | Out of scope | `cmd/e2eharness` is the e2e test-fixture/showcase-provisioning binary, never linked into or invoked by `verdi`. |
| `cmd/verdi/actorlocal.go:151` | NO | `git rev-parse --show-toplevel`, read-only. |
| `internal/draftmutation/identity.go:28` | NO | Same, read-only. |
| `internal/filelock/filelock.go:85` | NO | `ps -o lstart=`, not even git. |
| `cmd/verdi/sync_regen.go:42` | Can write build/test artifacts under a discovered service dir (not git-specific); not reachable from the five rituals | `verdi sync --or-regen` only. |
| `internal/experimentevaluator/testdata/evaluator/main.go:55` | NO | Lives under `testdata/` (Go-ignored for builds); a self-contained test fixture program, never linked into the production binary. |

**Sensor trial 1 — pristine fixture (state-diff, `state-diff/sd-*`).** Built
`go build -o /tmp/verdi-rws ./cmd/verdi` from the patched worktree (per
constraint 4, the binary is built in the worktree; the ritual itself runs
in a `/tmp` clone: `git clone -q --no-hardlinks <worktree>
/tmp/verdi-rws-clone`). Snapshotted `git for-each-ref`, `git ls-files -s`,
`git status --porcelain=v2` before and after `VERDI_SPIKE_GITLOG=... \
GOPROXY=off /tmp/verdi-rws design start --kind feature --name spike-trial
--defer-statements` (exit 0; `GOPROXY=off` is a deliberate, disclosed
choice to guarantee this manual trial touches no network regardless of
the `internal/upstream` capability above — matching co-3's spirit, not
just test code).

Result: `diff.for-each-ref.txt` shows exactly one new line (the new
`design/spike-trial` branch); `diff.ls-files-s.txt` shows exactly one new
line (the new spec file, staged); `diff.status.txt` is **empty**. **This
trial proves less than it appears to.** Its `sd-before.status.txt` and
`sd-after.status.txt` are both **0 bytes**: the clone was pristine, so
"the log explains every effect" was true only because there was nothing
for the log to fail to explain. ac-2 demands a fixture "seeded with an
untracked file, a pre-staged unrelated index entry, and a dirty tracked
file"; trial 1 seeded none of the three, and that omission is exactly what
let the `close` row of the UAT-036 table above go wrong. Trial 2 repairs
it.

**Sensor trial 2 — ac-2's seeded fixture (`state-diff/seeded-*`,
`state-diff/seeded-trial.sh`).** A fresh `/tmp` clone seeded with exactly
ac-2's three conditions, one file each — untracked `spike-untracked.txt`,
pre-staged unrelated `spike-foreign-staged.txt` (`git add`-ed and nothing
else), and a dirty tracked `README.md` (appended, never staged) — then two
rituals run against that **same** seeded state, snapshotted the same three
ways before and after each. The script is re-runnable
(`sh seeded-trial.sh <src-worktree> <patched-binary> <out-dir>`).

| | command | exit | what the state diff shows |
|---|---|---|---|
| `close` | `verdi close --force-local spec/ritual-write-scope` | **2** | `diff.seeded-close.{for-each-ref,ls-files-s,status}.txt` all **empty** — the refusal precedes every mutation |
| `design start` | `verdi design start --kind feature --name seeded-trial --defer-statements` | 0 | one new ref, one new index entry, **and the seeded foreign entry gone from `status`** |

`close`'s refusal, verbatim (`seeded-close-stderr.txt`, second line; the
first is the unrelated non-CI escape-hatch notice):

```
close: refusing to run with pre-existing staged paths ["spike-foreign-staged.txt"]; commit or unstage them before running the ritual
```

Its whole command log for that run is four read-only invocations
(`seeded-close-gitlog.tsv`: `remote`, `remote get-url origin`,
`status --porcelain -z --untracked-files=no` — `StagedPaths`' one
invocation — and `rev-parse --show-prefix`). No `checkout -b`, no `add`,
no `commit`. This is F-1's empirical witness: `close` cannot carry a
foreign index entry because it never begins.

`design start` on the identical seeded state **does** reproduce UAT-036.
Its commit's own file list (`seeded-designstart-commit-files.txt`) is:

```
e148ad2f7e8539841413714aae610b718940c2b1
design start: scaffold spec/seeded-trial (feature spec, no tracker ref)

.verdi/specs/active/seeded-trial/spec.md
spike-foreign-staged.txt
```

The ritual staged one path and committed two. The untracked file and the
dirty tracked file both survive untouched (`git status --porcelain` after:
` M README.md`, `?? spike-untracked.txt`), which independently re-confirms
UAT-033 fixed and UAT-019-adjacent scope intact — only the *index* entry
was absorbed.

**A sensor finding the feature build needs (ac-2).** Of the three
snapshots, only `git status --porcelain=v2` showed the sweep, and it showed
it as a line **disappearing** (`diff.seeded.status.txt`: `2d1`, the
`A. ... spike-foreign-staged.txt` row). `git ls-files -s` is blind to it —
the path is in the index before and after, merely with a different
staged-vs-HEAD relation — and `git for-each-ref` names only the new branch.
The command log is blind in the other direction: it records
`commit -m <msg>` with no `--`, which is the *shape* that permits the sweep
but never the *fact* of it. The one artifact that names the effect
directly is the commit's own file list. ac-2's witness therefore needs
three things, not two: the before/after state diff, the command log, and
`git show --name-only` over every commit the ritual created.

**Ruling: log plus state diff, not log alone** — dc-2's design stands.
The strongest ground is not in the `exec.Command` census at all; it is in
the rituals themselves. **Every ritual that produces an artifact writes it
with ordinary Go file I/O, never through git.**
`internal/commitdesign/commitdesign.go:193-206` is the plainest case —
`os.MkdirAll`, `os.WriteFile` for `spec.md`, `boardio.SaveBoardState` for a
frozen `board.json`, `boardio.GraduateStickies` for the annotations — all
before it ever calls `gitx.AddPaths`. `design.go:629-637` and
`internal/policyadopt/write.go` reach the same end through
`internal/atomicfile.Write` (create-temp, fsync, rename-into-place) rather
than `os.WriteFile`, which changes the durability story and nothing about
this one. These are working-tree mutations entirely outside gitx that **no
command log can ever explain**, because no git command caused them. That
alone settles the sensor question: a log-only sensor cannot see the
artifacts a ritual creates, only the `add` that later stages them.

The census adds two further, weaker grounds: two real, unsandboxed,
non-gitx exec sites reachable from three of the five governed rituals
(`upstream.RealRunner` from `design_start`/`build_start`/`close`;
`align.ExecJudgeRunner` from `close --prepare`). Neither fired in either
trial (no declared `impacts:`, no configured live judge auth), so neither
trial can show what a log-only sensor would miss if they did — disclosed,
not counted as proof. A log-only sensor is definitionally blind to a
non-git mutation (e.g. an upstream tool writing a stray file into `root`)
— but `git status --porcelain` (part of the state-diff sensor) would catch
exactly that, since a newly-written file shows as untracked.

Trial 2 adds the sharper reason: the state diff and the log fail in
*opposite* directions. The log **over**-reported `close` (a pathspec-less
`commit -m` that a guard means is never reached with a foreign entry
staged) and **under**-reported `design start` (the same shape, where the
sweep actually happens); only running the ritual against a seeded index
told the two apart. The state-diff sensor is therefore required, and —
per the sensor finding above — must include the created commit's own file
list, not only the three repository snapshots.

**ANSWERED**: census table above (`inventory.tsv`'s ritual coverage cross-
referenced against this table), sensor ruling with reasoning, state-diff
evidence in `state-diff/` for both the pristine (trial 1) and ac-2-seeded
(trial 2) fixtures.

## oq-3: The scope grammar

Five declarations, one YAML shape, `declarations.yaml` (YAML-valid,
verified with `python3 -c "import yaml; yaml.safe_load(...)"` — exit 0).
Summary:

| Ritual | refs_create | refs_move | head_may_switch | stage_paths (scoped?) | index_carry | untracked_may_enter | may_push |
|---|---|---|---|---|---|---|---|
| `design_start` | `design/<name>` | — | true | yes | **`carried` (open defect, UAT-036)** | false | false |
| `build_start` | `feature/<name>` | — | **true (open defect, UAT-023: implicit HEAD base)** | n/a (never stages) | `no_commit` | false (moot) | false |
| `close` | `close/<name>` (`--force-local` only) | unmeasured (plain/CI publish path not traced) | true | yes | **`refused` (requireCleanIndex, close.go:655 — exit 2 before any mutation)** | false | **false (confirmed by source: close.go:915-916 prints a "push it yourself" instruction, never calls `gitx.Push`)** |
| `commit_to_design` | — | — | false (never checks out) | yes | **`carried` (open defect, UAT-036)** | false | false |
| `policy_adopt` | `policy/adopt` (base: resolved default branch; `policy.go:255`, 10/10 observed) | — | true | yes | **`scoped` (`commit -m ... -- <paths>`, policy.go:57)** | false | false |

**Fields the five needed that the shape lacked: one — and this README's
first issue said "none", which is withdrawn.** The story's
`index_carry_foreign` is a boolean, and the five rituals occupy **four**
states, not two: `carried` (an unguarded, pathspec-less commit absorbs the
foreign entry), `refused` (the ritual exits 2 before it begins), `scoped`
(the commit names its own paths, so the entry is left in the index), and
`no_commit` (the ritual creates no commit at all). A boolean collapses the
last three into one `false`, which is wrong in three separate ways that
each matter to this feature:

- **ac-2 cannot write its assertion from a boolean.** The witness must
  assert exit 2 and zero mutation for `refused`, exit 0 with the foreign
  path absent from the commit for `scoped`, and no commit object at all
  for `no_commit`. These are three different tests; `false` names none of
  them.
- **co-1's widening check goes blind.** Under a boolean, deleting
  `requireCleanIndex` and adding a `--` pathspec to `close`'s commit reads
  as `false → false` — no widening — even though the operator-visible
  behaviour flips from "your ritual refuses until you deal with your
  index" to "your ritual proceeds silently". Under the enum it reads
  `refused → scoped` and lands in review.
- **It is how this spike got `close` wrong** (see the correction under
  oq-1): with only `true`/`false` available and a pathspec-less `commit`
  in the log, `true` was the only value that looked like it fit.

**The smallest change that carries it** is to retype the existing slot
rather than add an eighth field: `index_carry_foreign: <bool>` becomes
`index_carry: refused | scoped | carried | no_commit`. The field count
stays at seven, the old boolean is recoverable as `index_carry ==
carried`, and there is no second, derivable copy of the same fact to drift.
`declarations.yaml` is re-issued with it filled for all five, each value
cited to its source guard.

One further near-miss, which is **not** a missing field: `close`'s
branch-naming convention (`close/<name>` vs. `design/<name>`/
`feature/<name>`) is a *value* inside the existing `refs_create` field.

**Fields no ritual used:** `refs_move` is `[]` for all five — none of
them moves an *existing* ref; every `refs_create` is a brand-new branch.
The field is not dead weight for the grammar as a whole: readiness-
recovery's own ac-9 text names "returning to the original branch and
deleting it" as a recovery effect, which does move/delete a ref outside
these five. `may_push` is `false` for all five (four confirmed
dynamically by a clean recorder run; `close` confirmed more strongly by
source — see table). Real field, simply unexercised as `true` by any of
these five specifically; a sixth ritual (the workbench's own push button,
`internal/workbench/boardspecapi.go`, the only other caller of
`gitx.Push` besides an internal gitx helper) is where it would first
read `true`.

**ANSWERED-WITH-CAVEAT**: `declarations.yaml` (five filled declarations)
and the field-list verdict above. The caveat is the grammar change itself:
oq-3's first answer was ANSWERED on a shape that carried one wrong
load-bearing value (`close`), and the field-list verdict of "none missing"
was a consequence of that error rather than an independent finding. Both
are corrected here, but the correction is a *decision* about ac-1's field
list — replacing a story-named field with an enum — not merely a
measurement, so it is flagged rather than presented as a clean measurement
result. `close`'s `refs_move` remains genuinely unmeasured for the
plain/CI `PublishRollup` path.

## oq-4: Where the declaration lives

Grepped `docs/design/specs/03-evidence-model.md` and `04-story-provider.md`
for `gitx|write.scope|scope.grammar|verb.*(mutat|effect)|forbidden.token`
— **zero hits across both files, and zero hits for `gitx` across all of
00–05** (`grep -rniE "gitx|write.scope|scope.grammar"
docs/design/specs/*.md`, exit 1: no matches). Neither spec says anything
about verb git-effects today.

| Candidate | Who reads it | Schema change in `internal/artifact`? | Ratification needed? |
|---|---|---|---|
| **Go registry + gate test** | A static gate test (e.g. an `internal/writescope`-style package with a literal `map[string]ScopeDeclaration`, iterated by a test that fails when a verb calling `gitx` has no entry — the same shape `internal/specalign` already uses for its CLI-verb/MCP-tool inventories, and matching co-2's "data checked by the gate... never a comment") | No — a Go literal in a new internal package needs no `internal/artifact` YAML schema at all | **No.** Specs 03/04 say nothing about verb effects to contradict or extend; this is new product-behavior documentation living in source, the same status as any other internal package. |
| **Policy payload in the store** (`internal/policyartifact`) | Whatever reads adopted policy today (`verdi policy adopt`'s plan/write path, the policy kernel's resolution) | **No, admits it as-is.** `Payload` is an interface (`PayloadKind() string`, `Validate() error`) any feature registers via `RegisterPayloadKind` at `init()` time in its *own* package — exactly the `design_assistance` precedent in `payload.go`. A `write_scope` payload kind implementing that interface would slot in unchanged. | No, same reasoning — but see the mismatch below. |
| **Amendment to design specs 03 or 04** | Whoever the ratification flow routes to | N/A — this candidate *is* the amendment | **Yes, by definition** — this candidate literally proposes changing 03/04's binding text. |

**Recommendation: a Go registry checked by a gate test.** The policy
payload system technically admits the shape (confirmed above), but is a
**category mismatch**: `verdi policy adopt` is how an *operator opts into
a governance choice for their own repository* (the `design_assistance`
precedent — `mode: off|proposal-only|draft-write` is a per-repo decision
someone chooses). A ritual's write scope is not a choice an operator
should be able to widen by adopting a different policy profile — co-1 is
explicit that widening a declaration takes "an owner-visible decision,"
which reads as a decision by the people who ship verdi (a source change,
reviewed, versioned with the binary), not a per-repository adoption
artifact a local operator's `verdi policy adopt` could vary. Putting
`design start`'s write scope in the same typed-payload system as
`design_assistance.mode` would make "how far can this ritual mutate my
repo" a property an operator's policy profile choice could change, which
is exactly backwards from co-1's intent. A Go registry is also the
*already-proven pattern* for exactly this kind of "declared inventory
checked by a gate test" need in this codebase (this repo's own CLAUDE.md:
"The CLI-verb and MCP-tool inventories are serialized shared registries")
— ac-1's write-scope registry is the same shape as those two, not a new
invention.

**Ratification consequence:** none for the recommended home. No spec 03/04
amendment precedes the build; the registry is source, reviewed and shipped
with the binary like any other internal package.

**ANSWERED**: one home recommended (Go registry + gate test), ratification
consequence stated, all three candidates compared with evidence.

## oq-5: Seam and sequencing

**ac-9's text** (`.verdi/specs/active/readiness-recovery/spec.md`):
"...no recovery run's git command log contains reset, restore, clean,
stash, --force, or update-ref." A pure forbidden-token grep over a full
git command log for one `verdi recover` run — strictly less than what
ritual-write-scope's own ac-1/ac-2 need (a **per-verb** log, attributable,
paired with a **state diff**, to populate and check a **registry**, not
just scan for six tokens). **Wave 3 plan: confirmed none exists** (dc-6:
wave 3 = ac-8..ac-10, Tier 3 with an owner risk gate — the highest
scrutiny tier in that spec; the wave-2 plan, read for context at
`verdi-wt/readiness-recovery-w2/docs/superpowers/plans/
2026-09-19-readiness-recovery-wave-2.md`, covers only ac-6/ac-7 —
attestation scaffolding and review symmetry — nothing about a recorder
seam).

**Does a seam built for ac-9 serve ritual-write-scope unchanged? No.** A
minimal ac-9-shaped seam (log every command, grep for six tokens at the
end of a `verdi recover` run) has no per-verb attribution and no state-
diff hook — ritual-write-scope's ac-1 (registry) and ac-2 (behavioral
witness, before/after fixture diffing) would need to widen it. The
reverse holds cleanly: a seam built to ritual-write-scope's fuller spec
(per-call observation via a `Recorder`) trivially serves ac-9 as one
special-case consumer (a `Recorder` that just greps each call's `args`
for the six forbidden tokens and ignores everything else). This asymmetry
is already written into ac-3's own accepted text ("readiness-recovery
ac-9's recovery witness is one consumer of that seam rather than a second
mechanism") — this spike's finding is a confirmation, not a reversal, of
what the spec already commits to.

**Seam sketch — the smallest version, judged against the two named
rules:**

```go
// internal/gitx/recorder.go (sketch, not built by this spike)

// Recorder observes every git invocation gitx makes — dir and the exact
// args — without gitx itself knowing or caring why. One interface serves
// both a forbidden-token scan (ac-9: check args for reset/restore/
// clean/stash/--force/update-ref) and a per-verb log (ritual-write-scope
// ac-1/ac-2: collect lines keyed by the calling ritual).
type Recorder interface {
	RecordGit(dir string, args []string)
}

type recorderKey struct{}

// WithRecorder attaches r to ctx; run/runStdin/ConfigValue check for one
// on every call and no-op when absent (existing behavior, unchanged).
func WithRecorder(ctx context.Context, r Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, r)
}

func recorderFrom(ctx context.Context) (Recorder, bool) {
	r, ok := ctx.Value(recorderKey{}).(Recorder)
	return r, ok
}

// inside run/runStdin/ConfigValue, one line each, before exec:
//   if rec, ok := recorderFrom(ctx); ok { rec.RecordGit(dir, args) }
```

**Why `context.Context`, not a package-level hook or a "gitx runner"
struct** (the brief's two named alternatives): every one of gitx's ~40
exported functions already takes `ctx context.Context` as its first
parameter, so this needs **zero signature changes** anywhere in gitx's
public API — only the three underlying primitives gain one line each,
exactly mirroring this spike's own successful throwaway patch (3 call
sites, 73-line diff). A **package-level hook var** (`var recorder
func(...)`, set once via an exported setter) was this spike's own
throwaway shape (`VERDI_SPIKE_GITLOG`, read via `os.Getenv` — fine for a
single-purpose investigation script) but is the wrong shape for shipped
product code here: `cmd/verdi`'s own test suite runs hundreds of tests,
many via `t.Parallel()`-eligible subtests within one process, and a
shared package-level var would be a `go test -race` data-race risk the
moment two tests concurrently want different (or any vs. no) recording —
exactly what `go test -race ./...` must always catch clean (ground
rules). A **"gitx runner" struct** (wrapping the free functions as
methods with a `Recorder` field) would be the heavier option: gitx has no
receiver type today at all (confirmed: every function in `exec.go`,
`plumbing.go`, `configvalue.go`, and the ~40 other gitx files is a free
function), so this would mean rewriting every one of gitx's call sites
across `cmd/verdi` and `internal/*` from `gitx.Xxx(ctx, ...)` to
`runner.Xxx(ctx, ...)` — a much larger, more invasive change for the same
capability the ctx-value approach gets in three lines. The context-value
sketch above also satisfies "accept interfaces, return structs" (the 04
§port pattern this repo's ground rules name) directly: `Recorder` is a
one-method consumer-defined interface, and nothing about it is a
package-level mutable global.

**On "judged against the gochecknoglobals rule":** disclosed as a
caveat, not silently substituted — this repo's actual `.golangci.yml`
enables golangci-lint v2's **`standard`** set only (`errcheck`, `govet`,
`ineffassign`, `staticcheck`, `unused`); `gochecknoglobals` is not in that
set and is not separately configured anywhere in the file (grepped: zero
hits for the string `gochecknoglobals`). It is therefore **not an active
CI-enforced lint rule** in this repo today — applied here as the design
*principle* the brief asks for (avoid unnecessary package-level mutable
state), not as a literal currently-failing gate. Worth noting for
calibration: the codebase already tolerates the pattern the principle
warns against when justified — `internal/policyartifact/payload.go`'s own
`payloadRegistry` is a package-level `map` guarded by a `sync.RWMutex`,
and `internal/wtmanager/ensure.go`/`gc.go` each keep a package-level `var
worktreeAdd = gitx.WorktreeAdd`-style test-seam hook. The ctx-value sketch
above avoids needing any such exception at all, which is why it is
recommended over the brief's first-named alternative rather than merely
being the one that "passes" an inactive rule.

**Sequencing ruling: ritual-write-scope lands the seam first.**
Readiness-recovery's wave 3 has no plan yet and sits behind waves 1–2 at
the highest risk tier that spec defines (Tier 3, owner risk gate);
ritual-write-scope's own ac-3 text already frames itself as the seam's
owner with ac-9 as "one consumer." Combined with the strict-superset
relationship shown above (ritual-write-scope's needs are a superset of
ac-9's), building the fuller seam once inside ritual-write-scope's own
build and having wave 3 consume it via `WithRecorder`/a token-scanning
`Recorder` implementation avoids a second, redundant mechanism — exactly
what ac-3 already commits to and co-4 requires ("whichever feature lands
first owns the gitx seam and the other consumes it").

**ANSWERED**: seam sketch above, sequencing ruling with reasoning (ac-3's
existing text corroborated, not just asserted).

## Deviations from the investigation plan (constraint 10)

- **`internal/closeapp` substitution (R-PHS-5).** Already named in the
  lane brief; recorded again here for the README's own completeness. Used
  `cmd/verdi` (which contains `close.go`/`closefeature.go`/
  `closepreflight.go`/`closeprepare.go`) plus `internal/commitdesign` and
  `internal/policyadopt` in place of the story's literal `internal/
  closeapp/...`.
- **`docs/design/specs/03-*.md`/`04-*.md` do not exist inside this
  worktree.** The `verdi` repository (this worktree's own git history) has
  no `docs/design/specs/` directory at all — confirmed (`find docs -maxdepth
  2 -type d` lists no such path). Those files live one level up, at the
  outer workspace root (`/Users/johnyang/code/verdi-system/docs/design/
  specs/`), per that root's own `CLAUDE.md` ("workspace root is not [a git
  repository]"). This spike read them there (a plain read of static,
  shared reference documentation, not a git worktree and not a `verdi`
  verb/test/tool invocation) rather than silently treating the grep as
  vacuously "no hits found because the path doesn't exist here." The
  substitution and its outcome (zero hits either way) are recorded in
  oq-4 above.
- **Wave 3 plan: confirmed absent**, exactly as the lane brief's own
  "Wave 3 plan" note anticipated; ac-9's spec text and the wave-2 plan
  (read at the path the brief named, in the separate `readiness-
  recovery-w2` worktree, read-only) were used instead, as instructed.
- **`internal/policyadopt`'s branch name for `refs_create`** was recorded
  as unmeasured in this README's first issue, because the flags-only
  reduction strips non-flag tokens and re-deriving the name was judged not
  worth the script complexity. That judgement was wrong: the gap was
  self-inflicted and one reduction pass closes it. **Now measured**:
  `checkout -b policy/adopt main`, 10 of 10 observed invocations
  (`inventory-operands.tsv`), corroborated by source (`policy.go:255`
  passes the literal `"policy/adopt"`, with `resolveBranchBase` supplying
  the base). The deviation is kept here as a record of the method lesson
  — a reduction that discards operands cannot answer questions about
  operands — not as an open gap.
- **`close`'s plain/CI publish path was not dynamically exercised.**
  `--force-local` is this run's only observed `close` mutation path;
  `refs_move` for the plain, CI-gated `PublishRollup` path is recorded as
  unmeasured (declarations.yaml), not guessed, per constraint 8.

## Evidence index

| File | What it is |
|---|---|
| `recorder.patch` | The reverted-before-every-commit gitx patch (73 lines) |
| `inventory.tsv` | Per-ritual git subcommand/flags/mutation table (69 rows), oq-1's primary artifact |
| `declarations.yaml` | Five hand-written scope declarations, oq-3's primary artifact |
| `gitlog-dedup-all-verbs.tsv` | Deduplicated (who, subcommand, flags) across the whole test sweep, all verbs (524 lines). Operand-free by construction — use `inventory-operands.tsv` for branch names and pathspecs |
| `inventory-operands.tsv` | Every mutating invocation of the five rituals with its **full argument vector**, deduplicated (37 rows): branch names, their bases, pathspecs and `--` separators intact. The evidence `refs_create` and `stage_paths` values are actually checkable against |
| `reduce-operands.sh` | The reduction that produces it, with its rule stated in full and re-runnable over any raw recorder log |
| `gitlog-raw-sample.tsv` | Head(300)+tail(100) of the 21174-line raw recorder log, with the full-file line count and reduction pointer stated inline. The raw log itself is not committed (5.9 MB); an independent re-run in fix round 1 reproduced it at 21173 lines and produced a byte-identical `inventory-operands.tsv` |
| `test-run-stdout.log` | `go test`'s own summary line per package (all `ok`) |
| `state-diff/sd-*`, `state-diff/diff.*` (no `seeded` in the name) | Trial 1, the PRISTINE fixture: before/after `for-each-ref`/`ls-files -s`/`status` snapshots and diffs for the `design start` trial, plus that trial's command log and stdout/stderr. Both `status` snapshots are 0 bytes — which is the trial's limitation, not a clean result |
| `state-diff/seeded-trial.sh` | Trial 2's re-runnable script: seeds ac-2's three conditions in a fresh `/tmp` clone, runs `close` then `design start` against the same seeded state, snapshots three ways around each |
| `state-diff/seeded-*`, `state-diff/diff.seeded*` | Trial 2's output. `seeded-close-stderr.txt` + `seeded-close-exit.txt` + the three empty `diff.seeded-close.*` files are F-1's witness (exit 2, no mutation); `seeded-designstart-commit-files.txt` is UAT-036's (the commit carries `spike-foreign-staged.txt`) |
| `_scratch/` | Reserved for Go evidence under the VL-016 fence; empty — this spike's throwaway logic lived entirely in the patched product files (reverted) and shell/awk reduction, so nothing needed the `_scratch/` carve-out |

## Status summary (three-valued, constraint 8)

- oq-1: **ANSWERED** — `inventory.tsv` (subcommands and flags) plus
  `inventory-operands.tsv` (full argument vectors), backed by a recorder
  run reproduced independently in fix round 1.
- oq-2: **ANSWERED** — census table + two state-diff trials (pristine and
  ac-2-seeded) + sensor ruling (log plus state diff required, and the
  commit's own file list alongside them; log-only refuted by the rituals'
  own non-git artifact writes).
- oq-3: **ANSWERED-WITH-CAVEAT** — `declarations.yaml` re-issued with
  `index_carry` (a four-valued enum) replacing the story's
  `index_carry_foreign` boolean in the same slot. The caveat: the first
  issue of this answer carried one wrong load-bearing value (`close`), and
  its "no fields missing" verdict was a consequence of that error; both are
  corrected here, and the replacement is a *decision* about ac-1's field
  list rather than a measurement. `refs_move` empty for these five.
- oq-4: **ANSWERED** — Go registry + gate test recommended, ratification
  consequence stated (none), all three candidates compared.
- oq-5: **ANSWERED** — seam sketch (ctx-scoped `Recorder`) + sequencing
  ruling (ritual-write-scope lands it first), `gochecknoglobals` status
  disclosed as inactive-but-applied-as-principle.

No open question landed as NOT ANSWERED; oq-3 is the one
ANSWERED-WITH-CAVEAT. Several sub-parts are explicitly UNMEASURED rather
than guessed (`close`'s plain/CI publish `refs_move`; `policy_adopt`'s exact
branch name; `align`/`upstream`'s non-gitx capability confirmed reachable by
source but not dynamically triggered in either trial) — each is named
inline above and in `declarations.yaml`, never silently filled.
