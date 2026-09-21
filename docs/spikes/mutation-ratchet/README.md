# Mutation-ratchet spike — answer

Resolves `spec/mutation-ratchet#oq-1` through `#oq-5` (spike story
`spec/mutation-ratchet-spike`). Everything committed lives in
`verdi-wt/spike-mutation-ratchet` (this deliverable, branch
`spike/mutation-ratchet`, base `5f60c76c`). Every mutation-tool run
happened at the wave-1 head `e963f4d0`, outside any deliverable worktree:
the first round ran in the scratch worktree `verdi-wt/scratch-mutation-w1`
(detached at `e963f4d0`, left clean and detached there), and **fix round 1
ran exclusively in a throwaway local clone of it** — `git clone -q
--no-hardlinks <scratch> /tmp/fix-mr && git -C /tmp/fix-mr checkout
--detach e963f4d0` — because deviation 8 below records a candidate that
mutates sources in place, and no such tool should run anywhere refs are
shared. Evidence files referenced below live beside this README under
`docs/spikes/mutation-ratchet/`.

**FIX ROUND 1 (this revision).** An independent review found one Critical
and eleven further defects in the first version of this document. The
Critical one is now root-caused: gremlins v0.6.0 reports a FALSE 100% kill
rate for every `package main` directory in this module, because it derives
the wrong `go test` target. Full mechanism, instrumented trace, and
counterfactual: `oq1-gremlins-mainpkg-defect.txt`. Every number, count and
disposition below has been re-derived or restated accordingly; the
per-finding map is in the lane's fix report. Where the first version was
wrong, this one says so in place rather than quietly overwriting it.

## Recommendation (summary)

- **oq-1**: `gremlins@v0.6.0` (`github.com/go-gremlins/gremlins/cmd/gremlins`)
  **fits only with two named, mandatory mitigations**, not as shipped. It
  is the only one of the three named candidates that installs, runs, and
  leaves the tree clean on Go 1.25 against this module; both go-mutesting
  forks and ooze are disqualified with witnesses (a runtime panic for one;
  two independent, source-verified false-positive result mechanisms for
  the other two). But gremlins has the SAME class of defect as the two it
  beat, in two places: (1) it computes the package under test from the Go
  package-clause identifier, so every `package main` directory — here
  `cmd/verdi`, `cmd/e2eharness`, `cmd/public-execution-contract-release` —
  is tested by running `go test github.com/jyang234/verdi`, a path with no
  package, which fails `[setup failed]` in ~80 ms and is scored KILLED;
  (2) it expects exit code 2 to mean "mutant does not compile", which Go
  1.25 never returns, so non-viable mutants are also scored KILLED. Both
  produce 100.00% efficacy indistinguishable from a real perfect score.
  The mitigations: exclude the three `package main` directories from the
  mutation set (costing 9 of the 33 touched files), and make every
  package's report prove itself with a canary mutant that MUST be reported
  LIVED. co-2's pin is therefore `gremlins@v0.6.0` **plus** an explicit
  operator set, **plus** that exclusion list, **plus** the canary — a
  version string alone does not reproduce a baseline.
- **oq-2**: the touched set is **33 non-test `.go` files across 10 package
  directories** (one of the 33 is a deletion). Real per-mutant tables
  exist for three of the ten (store, readinessload, journey), re-measured
  in this round with the operator set and the file scoping both pinned
  explicitly. `cmd/verdi` and `cmd/e2eharness` are not measurable by this
  tool at all (the defect above). The remaining five are bounded by real
  baseline-timing evidence but not mutant-counted, under shared-machine
  load. `cmd/verdi`'s coverage pass, re-probed in this round with
  `-timeout 30m`, **passes in 431.731 s** — the earlier 600.587 s FAIL was
  Go's unconfigurable 10-minute default being blown by load, not a suite
  that cannot finish.
- **oq-3**: the *technique* can kill a write-path mutant when one exists —
  proven with a hand-seeded fixture that survives at 1be75d01 and is
  killed at 410db101. But the parent's ac-3 as written, a mutant surviving
  the pre-410db101 test **organically**, is NOT reproduced, and cannot be:
  `internal/readinessload` has no write statement for any catalogue
  operator to perturb. The parent's own oq-3 rule ("If the technique
  cannot reproduce the known witness the feature has no evidence it
  addresses PA-017 and should not be built") therefore lands on the side
  that needs owner adjudication, not on a YES. See that section.
- **oq-4**: recommend the `testdata/mutation-baseline.json` idiom's
  *structure* (a list of `{file, line, column, operator, reason}` records,
  reusing ac-1's own canonical-report shape plus one field), but reject
  that idiom's usual sibling *digest* file — a mutation baseline's `reason`
  field is meant to be hand-edited (co-4), and a digest ratchet exists
  specifically to forbid hand edits, which would fight the feature's own
  requirement. Regeneration should merge (drop killed entries, keep
  reasons for still-surviving ones, fail closed on any new, unreasoned
  survivor) rather than overwrite.
- **oq-5**: 0 of **33** touched files resolve to exactly one tier by the
  intended mechanical chain; 0 resolve to several by that same chain; **33
  of 33** (100%) resolve to none, because `spec/readiness-recovery` — the
  owning spec of every touched file — has no `verdi.bindings.yaml` entry
  at all yet. The manifest must be hand-authored.

## oq-1: which tool fits

Full command transcripts, exit statuses, and version output:
`oq1-tool-installs.txt`.

Three candidates were installed and run against `./internal/readinessload/...`
and `./internal/journey/...`, exactly as the story's step 1 specifies, all
inside the scratch worktree (detached at `e963f4d0`).

**gremlins v0.6.0** — installs cleanly (`go install
github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`), runs with no
network after install, and leaves the tree byte-identical after every run
in this spike (`git status --porcelain` / `git diff --stat` both empty,
checked after every single invocation, dozens of times; it mutates only
inside its own `wd-*` copies under `TMPDIR`). Runtime on the two named
packages, mutating every non-test file in each (oq-1's own step 1 scopes
to the whole of these two packages, unlike oq-2's touched-files scoping
below — journey in particular has two files, `codec.go` and `record.go`,
that are not part of the wave-1 diff and are excluded from oq-2's own
journey row for exactly that reason): `internal/readinessload` 87.6s (115
mutants found; 101 tested, of which 96 killed and 5 lived; plus 14
found-but-not-covered; `Mutation testing completed in 1 minute 27
seconds`), `internal/journey` 136s (**267 mutants found; 262 tested, of
which 243 killed and 19 lived; plus 5 not-covered** — the first version of
this document wrote "267 tested", which is the total, not the tested
count; `2 minutes 16 seconds`). Both figures used gremlins' DEFAULT
operator set; see "Operator catalogue" below, and oq-2 for the re-measured
figures with the set pinned.

### The criterion the story singles out: gremlins fails it (fix round 1)

The story's oq-1 asks specifically whether the tool "handles a package
whose tests exec a built binary without hanging"; the parent spec names
that cluster `CROSS_BINARY_PKGS`. The first version of this document
claimed oq-2's sweep had witnessed `cmd/e2eharness` passing that test.
**That claim was false twice over, and the truth is worse than a gap.**

First, `cmd/e2eharness`'s TESTS do not exec a built binary at all.
`buildBinary` (`cmd/e2eharness/main.go:326`) is called only from
`main.go:91`, `unprovenboard.go:146` and `specimportfixture.go:291`, all
production paths driven by the harness binary; `grep -rn "func TestMain"
cmd/e2eharness/` is empty. The genuine cross-binary witness in this module
is a package like `internal/designapp`, whose `conformance_test.go:54`
runs `exec.Command("go", "build", "-o", bin, ".../cmd/verdi")` from a
test.

Second, and decisively: **gremlins cannot measure `cmd/e2eharness` at
all.** Reproducing the reviewer's run in a throwaway clone at `e963f4d0`:

```
$ /tmp/fix-mr-bin/gremlins unleash --timeout-coefficient 10 --workers 2 \
    --output /tmp/fix-mr-e2eharness.json ./cmd/e2eharness
Mutation testing completed in 2 seconds 906 milliseconds
Killed: 354, Lived: 0, Not covered: 233
Timed out: 0, Not viable: 0, Skipped: 0
Test efficacy: 100.00%
```

354 mutants in 2.9 s, for a package whose own suite costs 13.747 s per
run. Root cause, with the instrumented `go`-wrapper trace, gremlins' own
source, and a like-for-like counterfactual, in
`oq1-gremlins-mainpkg-defect.txt`; in one paragraph:
`internal/engine/engine.go:160` derives the package under test by walking
the file's directory upward for a component that ends with the file's Go
**package-clause identifier**. For `cmd/e2eharness/*.go`, whose clause is
`package main`, no component ends with "main", so the loop falls through
to `pkg = mu.module.Name` and every per-mutant command becomes

```
CWD=.../gremlins-2094269129/wd-3585363107/cmd/e2eharness
ARGV=test -timeout 5.37614084s -failfast github.com/jyang234/verdi
RC=1
# github.com/jyang234/verdi
no required module provides package github.com/jyang234/verdi; to add it:
	go get github.com/jyang234/verdi
FAIL	github.com/jyang234/verdi [setup failed]
```

— a `[setup failed]` in ~80 ms that never compiles the code under test.
`internal/engine/executor.go:257` maps `go test` exit code 1 to KILLED, so
all 354 "kills" are that error. The same mapping expects exit code 2 to
mean "does not compile" (NOT VIABLE); Go 1.25.5 returns 1 for build
failures, test failures and missing packages alike (measured), so NOT
VIABLE is unreachable on this toolchain and non-viable mutants are scored
as kills too. Every gremlins report in this spike says "Not viable: 0";
that zero is a property of the tool, not of the mutants.

Same 11 mutants (the one touched file of that package), scoped with
`-E`, `--workers 1`, as shipped versus with a wrapper that corrects only
the final `go test` argument:

| run | wall clock | per mutant | what actually ran |
|---|---|---|---|
| as shipped | 904 ms | 82 ms | 11/11 `[setup failed]` |
| path corrected | 46.057 s | 4.2 s | 5 compile failures + 6 genuine test failures |

Both print `Killed: 11, Lived: 0 ... Test efficacy: 100.00%`. Even the
corrected run's honest kill rate is 6 of 11, not 11 of 11. The control —
`internal/designapp`, the genuine cross-binary package, whose directory
basename does equal its clause name — gets the correct target
(`ARGV=test ... github.com/jyang234/verdi/internal/designapp`), real
failures (`--- FAIL: TestGetBoard ... board_test.go:19`), and 21.457 s for
11 mutants. So gremlins DOES handle a package whose tests build and exec
`cmd/verdi` (it copies the whole module root per worker,
`workdir.NewCachedDealer(workDir, mod.Root)`), and it does NOT handle a
`package main` directory.

Blast radius here: 3 of this module's 105 packages are `package main`
(`cmd/verdi`, `cmd/e2eharness`, `cmd/public-execution-contract-release`);
two of them hold 9 of the 33 touched files. No other package in the module
has a clause name differing from its directory basename, so nothing else
in the touched set is affected — but the general rule is "the directory
path must end with the package-clause identifier", not "must not be
`main`".

**Re-ruling on oq-1 (this is the answer, replacing the first version's
unqualified ANSWERED): gremlins v0.6.0 FITS WITH NAMED MITIGATIONS.** The
mitigations, and why the alternatives were rejected, are in
`oq1-gremlins-mainpkg-defect.txt` §6:

1. **Exclude the three `package main` directories** from the mutation set.
   This is the only correct-and-affordable configuration. It costs
   coverage of 9 of the 33 touched files, `cmd/verdi`'s 8 among them.
2. **Require a canary** per package: seed one mutant that must survive and
   fail the gate unless the run reports it LIVED. Under this defect a
   package reports 100.00% efficacy with every mutant a setup failure;
   a correctly targeted package can also legitimately report 100.00%
   (`internal/store`, 17/17 killed), so the seeded canary, not the
   efficacy figure, is the discriminator. The canary itself is proposed
   here and UNMEASURED: no canary was built or run in this spike; its
   premise (a correctly targeted run reports LIVED mutants) is witnessed
   by readinessload's 22 LIVED. ac-1's byte-identical report cannot be
   trusted without it.
3. Rejected: `-i/--integration` genuinely avoids the defect
   (`executor.go:199-201,234-237` replace both cwd and package argument
   with `rootDir` and `./...`), but runs the whole module suite per
   mutant — ~40 hours for `cmd/e2eharness` alone at this module's 820 s
   `make test`. Rejected: patching gremlins, which would break co-2's
   "pinned like golangci-lint" property unless the fork is itself pinned
   and published. There is no flag for the package argument; the complete
   `unleash` flag list is quoted in that evidence file.

### Operator catalogue (fix round 1)

Every measurement in the first version of this document used gremlins'
DEFAULT operator set, and did not say so. 6 of gremlins' 11 operators are
off by default: `INVERT_ASSIGNMENTS`, `INVERT_BITWISE`, `INVERT_BWASSIGN`,
`INVERT_LOGICAL`, `INVERT_LOOPCTRL`, `REMOVE_SELF_ASSIGNMENTS`. The 5
default-on ones are `ARITHMETIC_BASE`, `CONDITIONALS_BOUNDARY`,
`CONDITIONALS_NEGATION`, `INCREMENT_DECREMENT`, `INVERT_NEGATIVES`, and
those are the only five appearing in any committed report. So the first
version's yield was a partial-catalogue yield. A pinned version string
alone therefore does NOT reproduce a baseline, which is exactly what ac-1
(byte-identical report) and ac-2 (ratchet) require: **co-2's pin must name
the tool version, the operator set, the exclusion list and the canary.**
oq-2's re-measured table below states its operator set explicitly on the
command line, and `run-oq2.sh` does the same.

A finding worth carrying into the real feature's design: gremlins has a
built-in `-D/--diff <ref>` flag that looks purpose-built for dc-2's "only
files the change touched are mutated." This spike could not get it to
select anything but `SKIPPED` for every single mutant in a package,
including mutants on the one file that genuinely changed between
`b810c302` and `e963f4d0` (`internal/store/paths.go`) — full transcript:
`oq2-evidence/gremlins-diff-flag-attempt.log`
(`gremlins unleash --diff b810c302 --output ... ./internal/store`, all
101 mutants across the 9 files gremlins' own report names came back
`SKIPPED`, 0 killed/lived/not-covered). `--diff`'s root cause is not
investigated: it is a working candidate's optional flag, and fix round 1
found the flag that DOES work. The real feature should not assume
`--diff` works as documented without its own, dedicated verification.
**Touched-file scoping in this spike is now done with
`-E/--exclude-files`, natively, before mutant generation** — see oq-2's
dc-2 section and `oq2-evidence/exclude-files-scoping.txt`. (The first
version of this document post-filtered gremlins' per-file JSON report
instead and described that as honoring dc-2; it does not, and it is no
longer what `run-oq2.sh` does.)

**go-mutesting, zimmski fork (`github.com/zimmski/go-mutesting`,
`v0.0.0-20210610104036-6d9217011a00`, the only version that resolves — no
tagged releases, last commit 2021-06-10)** — installs cleanly, then
segfaults inside `go/types` on first real use against this module, before
generating a single mutant. Full trace: `oq1-gomutesting-zimmski-panic.txt`.
DISQUALIFIED: does not run on Go 1.25 against this module.

**go-mutesting, avito-tech fork (`github.com/avito-tech/go-mutesting`,
`v0.0.0-20251226130216-48d0401f00fb`, also untagged but actively
maintained)** — installs cleanly and DOES run to completion (239 mutants,
"mutation score 1.000000"), but its own report is internally
self-contradictory (`mutationCodeCoverage: 0` alongside `msi: 1`, i.e. 0%
coverage and a perfect kill rate claimed simultaneously) and its own
stdout shows why: at least 161 of the 239 mutants (conflict.go's 67 and
load.go's 94) are built inside a per-mutant temp copy that is missing
sibling-file declarations (`JudgeMode`, `JudgeRun`, `ActorsResolver`,
`PredecodedRequest`, `Options` — all in this package's own `options.go`),
so those mutants' builds fail to compile for a reason unrelated to the
specific mutation, and the tool counts that build failure as a "kill"
indistinguishably from a real one. Full transcript and root-cause:
`oq1-gomutesting-avito-defect.txt`. DISQUALIFIED: results are not
trustworthy for this module's package layout (any package with more than
one non-test file is at risk).

It is also DISQUALIFIED a second, independent way, which the first
version of this document failed to record: **this fork mutates sources in
place in the working tree.** The first version recorded the clean-tree
check for gremlins and, vacuously, for zimmski (which crashed before
mutating anything), but never for the one fork that actually generated
mutants; its evidence file attributed a dirty tree to an interrupted run.
Witnessed directly in fix round 1, DURING a normal run in a throwaway
clone:

```
$ git -C /tmp/fix-mr status --porcelain
 M internal/readinessload/facts.go
?? internal/readinessload/facts.go.tmp
```

The story's step 1 makes the clean-tree check a disqualifier in its own
right, so this fork fails step 1 twice.

**ooze v0.2.0 (`github.com/gtramontina/ooze`)** — not a standalone CLI at
all; it is a library you `go get` and drive from your own
`//go:build mutation`-tagged `_test.go` file via `go test -tags=mutation`
— a structurally different integration model from "pinned like
golangci-lint" (co-2) even before any defect. It reports a 100%
kill rate for `internal/readinessload` at an average of 0.01s per mutant
— implausibly fast next to that same package's own non-mutated tests
taking up to 1.5s each in the same `-v` run. Reading the installed
module's own source (not just running it harder — even `-v`/`-ooze.v`
print nothing about *why* a mutant was "killed") shows the mechanism:
`ooze.Release`'s default `WithRepositoryRoot(".")` resolves to the Go test
binary's own working directory, which Go always sets to the *package's*
source directory, not the module root; ooze's `LinkAllToTemporaryRepository`
then symlinks only that one directory's own files into an
`os.MkdirTemp("", ...)` location with no `go.mod` anywhere in its
ancestry; the default `go test -count=1 ./...` run there fails
immediately for lack of a module, and `cmdtestrunner.Test` counts *any*
non-zero exit — a genuine test failure or a bare Go-toolchain error alike
— as a kill. Full source excerpts and the exact reasoning chain:
`oq1-ooze-defect.txt`. DISQUALIFIED: default configuration is vacuous for
any module with more than one internal package, which this module very
much is (105 packages).

**Answered.** `gremlins@v0.6.0` is the recommendation; both other named
candidates are disqualified with a concrete, reproducible witness each.

## oq-2: yield and cost over the wave-1 diff

**The touched set is 33 non-test `.go` files across 10 package
directories** (the first version of this document said 30 across 9, while
its own per-package lists summed to 33 and named 10). Recomputed in fix
round 1:

```
$ git diff --name-only b810c302 e963f4d0 -- '*.go' | grep -v _test.go | wc -l
33
$ git diff --name-only b810c302 e963f4d0 -- '*.go' | grep -v _test.go \
    | xargs -n1 dirname | sort -u | wc -l
10
$ git diff --name-status b810c302 e963f4d0 -- '*.go' | grep -v _test.go \
    | awk '{print $1}' | sort | uniq -c
   7 A
   1 D
  25 M
```

The 10 directories are `cmd/e2eharness`, `cmd/verdi`, `internal/journey`,
`internal/mcpserve`, `internal/policyconflict`, `internal/readinessload`,
`internal/readinesspilot`, `internal/readinesspilot/readinesstest`,
`internal/store`, `internal/workbench`. One of the 33 is a DELETION
(`cmd/verdi/readiness_snapshot.go`): it is in the denominator the story
asks for and has no content at `e963f4d0` to mutate.

**Load discipline (constraint 7), disclosed once for this whole section.**
Every timing below was measured on a machine shared with other lanes of
this build, and is reported as such. `uptime`'s 1-minute figure was
observed across this spike's two sessions at 2.35, 2.78, 2.99, 4.90,
7.21, 8.06, 8.47, 12.02, 13.19, 15.53, 29.53, 46.35, 62.05, 75.19 and
78.84 — the high end being other lanes' own concurrent gate and review
runs, confirmed at the time with `ps aux` (a concurrent
`git clone …/spike-self-governance` review run and several `go test
./...` invocations from a different lane's PID tree, neither started nor
controlled by this spike). Configuration B's table carries a before →
after reading on every row; configuration A's does not, and is marked
accordingly. No figure anywhere in this section is extrapolated from a
quiet-window measurement taken elsewhere, and the measuring command is
left re-runnable as `run-oq2.sh`.

### How files are scoped: natively, with `-E` (dc-2)

The first version said dc-2's "never mutates files the change did not
touch" was "honored by post-filtering," and that gremlins "has no
single-file target". **Both are withdrawn.** Post-filtering mutates every
file in the package and drops rows from the report afterwards — for
`internal/store` it generated 101 mutants and kept 16 — and dc-2
constrains what RUNS, not what is reported. gremlins does have a
file-scoping mechanism, `-E/--exclude-files`, which the first version
never tried; `internal/engine/engine.go`'s `WalkDir` consults the
exclusion rules BEFORE `runOnFile`, so an excluded file is never parsed
and never produces a mutant. Proof, semantics, and the three limitations
that matter (the regexps match paths relative to the target package dir;
it is an exclude-list so the keep-set is expressed by enumerating the
others; and it does NOT scope gremlins' whole-package coverage pass):
`oq2-evidence/exclude-files-scoping.txt`. `run-oq2.sh` now builds that
exclusion list per package from `git diff --name-only` and asserts
afterwards that the report names no untouched file.

**Ruling for dc-2's first clause: "only touched files are mutated" is
achievable natively.** Not with `-D/--diff`, which returned `SKIPPED` for
every mutant including touched ones (see oq-1), but with `-E`. dc-2's
second clause, "only the tests covering them run", is NOT addressed by
this spike: every per-mutant invocation is `go test -failfast <pkg>` with
no `-run` filter, so the whole package suite runs per mutant.

### Configuration A — the first version's figures (gremlins defaults, whole package, post-filtered)

Settings: gremlins' DEFAULT operator set (5 of 11 — see "Operator
catalogue" in oq-1); whole-package runs; counts post-filtered to the
touched files; `--timeout-coefficient 10 --workers 2` for store, oq-1's
defaults for the other two. Counts are per-mutant from
`oq2-evidence/gremlins-*.json`; "mutants" is killed+survived, with
not-covered kept as its own column.

| package | touched files | mutants | killed | survived | not covered | timed out | seconds (whole package) | load (1-min) before → after |
|---|---|---|---|---|---|---|---|---|
| internal/store | paths.go (1 of **10** non-test files) | 16 | 16 | 0 | 0 | 0 | 29 | ~3 → unrecorded |
| internal/readinessload | conflict.go, doc.go, facts.go, load.go, options.go (all 5) | 101 | 96 | 5 | 14 | 0 | 88 | **unrecorded → unrecorded** |
| internal/journey | derive.go, eventual.go, facts.go, port.go, project.go, reason.go (6 of 8) | 169 | 155 | 14 | 5 | 0 | 136 | **unrecorded → unrecorded** |
| **total** | | **286** | **267** | **19** | **19** | **0** | **253** | |

Two of those three rows carry no load reading at all, which constraint 7
does not admit; and the rows come from two different tool configurations,
which the first version's table never stated. The counts reproduce
exactly (an independent reviewer re-tallied every row and the total from
the committed JSON). **The seconds column does not.** Re-running the
lane's own script at its own defaults, an independent reviewer measured
`internal/journey` at **397 s** against this table's 136 s — identical
mutant counts, 2.9x the wall clock, at a 1-minute load that walked from
3.21 to 66.4 during the sweep. Both figures are real; neither is the
number a budget can be set from. (The first version's "1 of 9 files" for
store was gremlins' own report file count, not the package's file count:
`internal/store` has 10 non-test `.go` files.)

### Configuration B — fix round 1 (operator set pinned, files scoped natively)

Settings, stated once for the whole table and printed by the script that
produced it: `gremlins@v0.6.0`; **all eleven operators explicitly
enabled**; `-E` exclusion of every untouched non-test file in each
package, so the mutants ARE the touched files' mutants and the seconds
are the touched-files cost, not a whole-package upper bound;
`--timeout-coefficient 10 --workers 2`; 10 cores. Verbatim sweep:
`oq2-evidence/run-oq2-operators-all.log`.

| package | touched files | mutants | killed | survived | not covered | timed out | seconds | load (1-min) before → after |
|---|---|---|---|---|---|---|---|---|
| internal/journey | 6 of 8 (codec.go, record.go excluded) | 220 | 192 | 28 | 12 | 2 | 587 | 8.47 → 13.19 |
| internal/readinessload | all 5 (nothing excluded) | 133 | 111 | 22 | 21 | 0 | 454 | 13.19 → 8.06 |
| internal/store | paths.go (9 excluded) | 17 | 17 | 0 | 0 | 0 | 7 | 8.06 → 7.90 |
| **total** | **12 of 33 touched files** | **370** | **320** | **50** | **33** | **2** | **1048** | sweep wall clock 1050 s |

What changes, and why it matters to the spec:

- **Survivors nearly triple, 19 → 50.** The six default-off operators
  find 31 more surviving mutants in the same three packages. ac-2's
  ratchet baseline would have been seeded from a partial catalogue.
- **Native scoping is cheaper where it bites.** `internal/store` falls
  from 29 s to 7 s: 17 mutants generated instead of 101 generated-then-
  discarded. Where the touched set IS the package (readinessload), there
  is nothing to save.
- **Two TIMED OUT appear in journey** where configuration A had none.
  (Configuration A ran at gremlins' defaults, `--timeout-coefficient 3`
  and `--workers 0` = NumCPU = 10; configuration B ran `--workers 2
  --timeout-coefficient 10`; the two are not the same setting, corrected
  at the closure check.) Together with store's first,
  untuned attempt (below), that is three independent observations that
  gremlins' timeout calibration is load-sensitive — on a contended
  runner a timeout is indistinguishable from a survivor, which a
  hard-failing gate cannot tolerate.
- **The seconds are still load-bearing, not settled.** readinessload
  reads 454 s here against configuration A's 88 s for 1.32x the mutants.
  Most of that spread is the configuration, not the machine: A ran ten
  workers, B two (the closure check's quiet-machine re-run of B gives
  458 s at load 3–8, within 1% of the 454 s measured at load 13). co-3's
  58% lower bound is therefore a two-worker figure; a ten-worker run of
  the same catalogue is unmeasured.

### Packages the pinned tool cannot measure at all

`cmd/verdi` (8 touched files) and `cmd/e2eharness` (1) are `package main`
directories, which gremlins v0.6.0 scores 100% KILLED without compiling
or running anything — root cause, trace and counterfactual in
`oq1-gremlins-mainpkg-defect.txt`, summarised in oq-1. **9 of the 33
touched files are therefore out of reach of the recommended tool as
pinned**, and no measurement of them is reported here. `run-oq2.sh`
refuses them by default and labels the row `DEFECT`.

### Packages not reached: bounded by baseline timing only

The remaining five touched packages' own full-suite baseline pass — the
prerequisite gremlins runs before it can generate a single mutant — was
measured directly (`go test -cover` timing, captured from this spike's
earlier whole-module dry-run attempt before that attempt aborted on two
unrelated pre-existing failures elsewhere in the tree; see "Deviations"):

| package | touched files | baseline pass (coverage-gather only) | mutant-level counts |
|---|---|---|---|
| internal/mcpserve | backend.go, tool_get_document.go | 72.5s | not completed this session (two attempts each ran >15 min without finishing under load averages of 40–80; interrupted rather than let run further) |
| internal/policyconflict | cachejudge.go, service.go | 56.7s | not completed this session |
| internal/readinesspilot | derive.go | 0.45s | not completed this session (package itself is tiny; not reached before this spike's time budget ran out) |
| internal/readinesspilot/readinesstest | readinesstest.go | **0.240s** (`ok … readinesstest 0.240s`, `whole-module-dryrun.log:106` — the first version wrote "not separately measured" although the log it cited has it) | not completed this session |
| internal/workbench | boarddocument.go, boardspec.go, branchboard.go, handler.go, readiness.go, readinessrender.go | 269.8s (4.5 min for ONE full-suite pass) | not completed this session |

### cmd/verdi's timeout: probed, not assumed

The first version reported `cmd/verdi` infeasible on the strength of a
600.587 s FAIL. 600 s is Go's DEFAULT per-package test timeout, and
gremlins passes **no `-timeout` flag at all** on its coverage pass — the
instrumented argv is `test -cover -coverprofile <f> ./<pkg>/...`, quoted
verbatim in `oq1-gremlins-mainpkg-defect.txt`. `gremlins unleash --help`
exposes `--timeout-coefficient`, but that scales only the PER-MUTANT
timeout (`executor.go:227`); there is no knob for the coverage pass.

So the probe was run once, directly:

```
$ go test -timeout 30m -cover -coverprofile /tmp/fix-mr-cmdverdi.cov ./cmd/verdi/...
ok  	github.com/jyang234/verdi/cmd/verdi	431.731s	coverage: 61.3% of statements
exit status: 0            uptime before 29.53  after 3.96
```

**cmd/verdi's suite is not out of reach: it passes in 431.731 s.** The
earlier FAIL was the unconfigurable 10-minute wall being blown under
load, at 1.39x of headroom — reproducible whenever the machine is ~40%
busier. But that is a secondary risk: the binding blocker for `cmd/verdi`
is the `package main` defect, and if it were fixed, gremlins' per-mutant
timeout at this spike's settings would be `2s + 431.731s × 10` ≈ 72
minutes per mutant, each run paying up to one full suite. Full reasoning:
`oq2-evidence/cmd-verdi-timeout-probe.log`.

### Spurious timeouts under load (unchanged finding, restated)

`internal/store`'s FIRST attempt, at gremlins' default timeout
calibration under a load average of ~12–17, produced 87 of 101 mutants
`TIMED OUT` and 0 killed/lived — a timing artifact, not a finding about
the code (`oq2-evidence/store-run1-spurious-timeouts.log`). Raising
`--timeout-coefficient` to 10 and lowering `--workers` to 2 brought it to
6 residual timeouts out of 101; configuration B's native scoping brought
store to 0 of 17, but journey to 2 of 234. Still nonzero under every
setting tried, which is the finding: **a per-wave gate step that
hard-fails on any survivor cannot run gremlins at default timeout
settings on a contended runner; co-3's budget needs its own coefficient
tuned against real CI hardware, not this spike's laptop-under-load
numbers.**

### Ratio to the gate baseline, as a range with an honest lower bound

Gate baseline (lane specifics, PR #337 @ 01211bd8): test 820 s,
spec-align 176 s, e2e 776 s, **total 1820 s**.

| basis | seconds | ratio | what it covers |
|---|---|---|---|
| configuration A total | 253 | **14%** | 3 of 10 packages, default operators, one row's load unrecorded, journey's 136 s not reproducible |
| configuration A with the reviewer's journey re-run (397 s) substituted | 514 | **28%** | same coverage, same counts, a different machine-hour |
| **configuration B total (this round's best evidence)** | **1048** | **58%** | 3 of 10 packages, 12 of 33 files, full operator catalogue, native file scoping, load recorded on every row |

**58% is the honest LOWER bound**, and it is a lower bound three times
over: it excludes 7 of the 10 touched packages, it excludes the 9 touched
files the tool cannot measure at all, and it is a per-wave cost paid on
every wave whose diff touches those packages, not once. The first
version's "14%–38%" range was optimistic, not conservative: 14% came from
a partial operator catalogue over whole-package runs with two unrecorded
loads, and 38% added only the missing packages' coverage-gather time,
which is a floor on their mutation cost, not an estimate of it.

**There is no defensible UPPER bound from this spike's evidence.** The
two largest touched packages by suite cost (`internal/workbench` at
269.8 s per pass, `cmd/verdi` at 431.731 s) were never mutated, and
gremlins' cost model is one baseline pass plus up to one full suite per
mutant.

**Disposition: ANSWERED-WITH-CAVEAT.** Three packages have real, decisive
mutant tables under a fully stated configuration. Two are unmeasurable by
the pinned tool. Five are bounded by real baseline-timing evidence but
not mutant-counted, because this spike's session ran out of tolerable
wall-clock budget under sustained, independently-confirmed shared-machine
load — recorded as NOT ANSWERED for those packages' mutant counts
specifically, per constraint 8, rather than guessed or extrapolated.

## oq-3: does the technique reach the PA-017 witness

Full transcript: `oq3-reach-evidence.txt`.

First, a necessary preliminary: `internal/readinessload` has no
file-write call anywhere in its production source, at either commit —
checked exhaustively (`grep -niE "os\.(WriteFile|Create|Mkdir|OpenFile|
Remove)|ioutil\."` across every `.go` file in the package at `1be75d01`,
zero matches). This is the package working as designed (co-2's whole
point), but it means no catalog mutation operator — not gremlins',
not either go-mutesting fork's, not ooze's — can produce "a mutant that
writes an arbitrary file" from this file's actual content: every operator
in every candidate perturbs an *existing* expression or statement; none
synthesizes a new I/O call from nothing. So this spike answers oq-3 the
way the parent spec's own ac-3 is worded — "a committed regression
fixture," a hand-seeded fault standing in for what a real implementation
would also have to hand-author, not an organic tool discovery.

The seeded mutant (identical text, identical location, at both commits):
an unconditional `os.WriteFile` of `.verdi/data/scratch-cache.tmp`,
inserted as the first statement inside `func (l loader) load(...)` in
`internal/readinessload/load.go`, guarded by `os.MkdirAll` and a `panic`
on either call's failure (diagnostic scaffolding for this spike only, to
prove the write genuinely lands rather than silently no-op'ing against a
fixture directory that might not pre-exist — an earlier attempt that
discarded the write's error DID silently no-op at both commits, which
would have been a false NOT-ANSWERED; this is recorded as a deviation
below).

- At `1be75d01` (pre-fix): the seeded mutant SURVIVES. `go test
  ./internal/readinessload/... -count=1` is green (18.1s, no panic fired,
  meaning the write genuinely happened and nothing noticed) — the
  pre-410db101 `assertNoPersistence` only greps written file **names** for
  the substring "readiness"; `scratch-cache.tmp` does not contain it.
- At `410db101` (fixed): the seeded mutant is KILLED. Two independent
  tests fail with an exact before/after file-tree diff:
  `TestLoad_WithRequestCacheMissNeverRunsJudge` and
  `TestLoad_PersistenceBoundary` both report
  `.verdi/data changed during a derivation that must persist nothing:
  before=map[] after=map[.verdi/data/scratch-cache.tmp:1]`.
  (`TestLoad_WithRequestCacheHit` does not call the persistence assertion
  at all — it checks judge-call counts — so its continued pass is
  expected, not a gap.)

### Restated for fix round 1: what is proven, and which side of the parent's own rule this lands on

The first version of this section answered "YES: the technique reaches
PA-017's exact witness class." That overstates the proof and skips a
decision the parent spec reserves. Restated precisely:

**Proven.** (a) The pre-410db101 `assertNoPersistence` misses an
arbitrary-file write: it greps written file *names* for the substring
"readiness". (b) The fixed test catches it, with an exact before/after
file-tree diff. (c) Therefore the *technique* — a committed fixture that
performs an unexpected write — can kill a write-path mutant **when one
exists**. That is a property of the TESTS, demonstrated with a
hand-written statement insertion.

**Not proven, and not provable here.** The parent's ac-3 as written asks
for a mutant that survives the pre-410db101 test **organically** — one the
pinned tool generates. That is not reproduced. gremlins' catalogue is 11
expression/statement perturbations (see "Operator catalogue" above); none
synthesises an I/O call, and `internal/readinessload` contains no write
statement anywhere at either commit for one to perturb (`grep -niE
"os\.(WriteFile|Create|Mkdir|OpenFile|Remove)|ioutil\."`, zero matches).
So "the technique reaches the witness" is the wrong verb: the technique
*detects* the witness class once a fixture supplies it; the *tool* cannot
*produce* it here.

**The rule this triggers.** Parent §oq-3: "If the technique cannot
reproduce the known witness the feature has no evidence it addresses
PA-017 and should not be built." The Spec seed makes oq-3 binary —
"confirms ac-3 verbatim or closes the feature as not-doing." The evidence
above does NOT confirm ac-3 verbatim. The first version took a silent
third path (keep the feature, rewrite ac-3's expectation) without naming
it as a departure or citing the rule it departed from. Named now:

- The "not-doing" branch is live and **needs owner adjudication.** This
  spike does not have the authority to take it or to refuse it.
- If the owner keeps the feature, ac-3 must be rewritten to say what the
  fixture actually tests — that the persistence assertion is strong enough
  to catch an unexpected write — and must NOT promise that mutation
  testing rediscovers PA-017's defect class on unrelated code. The
  fixture would then be a regression test for the *test*, valuable but a
  different deliverable from a mutation ratchet.
- Weighing on that decision, from oq-1: the pinned tool also cannot
  measure `cmd/verdi` or `cmd/e2eharness` at all, and scores non-viable
  mutants as kills everywhere. PA-017's own witness lived in
  `internal/readinessload`, which the tool does handle.

**Disposition: ANSWERED-WITH-CAVEAT on the narrow question (the fixture
kills the seeded mutant, evidence above), NOT ANSWERED on ac-3 as
written** — recorded as a spec-seed decision for the owner, not as a
softened yes.

## oq-4: baseline record shape

Full drafts: `oq4-baseline-drafts/` (both idioms, filled with the 5 real
survivors from `internal/readinessload`'s gremlins run: `conflict.go:50:41`,
`conflict.go:186:13`, `facts.go:79:20`, `facts.go:83:28`, `load.go:295:25`,
all `CONDITIONALS_NEGATION`).

**Idiom A** (`testdata/mutation-baseline.json` + sibling digest, the
`testdata/svcfix-canned/digests.json` pattern): a canonical JSON list of
`{file, line, column, operator, reason}` records, plus a
`verdi.fixture-digests/v1`-shaped `.digest` file hashing it.

**Idiom B** (the `internal/policyartifact/testdata/golden-digests.json`
pattern — verified at that exact path): a single flat map from a
deterministic mutant-identity string (`file:line:column:operator`) to a
value — adapted here to the *reason*, since (unlike policyartifact's
digests of decoded policy markdown) a mutation record has no separate
"real content" elsewhere to digest; the record's own fields are the
primitive data.

**Diff readability when one survivor is killed** (both drafts, killing
`conflict.go:186:13`, canonicalized with `ensure_ascii=False`/sorted keys
to match this repo's own Go `encoding/json` convention rather than
Python's default escaping, which would otherwise inject spurious
`\uXXXX` diff noise unrelated to either idiom): idiom A's diff removes a
7-line struct entry; idiom B's removes exactly 1 line. Idiom B is
mechanically the smaller diff.

**How an equivalent mutant carries its one-line reason**: idiom A carries
it as a named `reason` field beside explicit `file`/`line`/`column`/
`operator` fields — directly greppable and filterable by any one of those
without parsing a composite key. Idiom B packs `file`, `line`, `column`,
and `operator` into one opaque string key; extracting any single field
for tooling (e.g. "group survivors by operator") requires parsing that
key back apart, which idiom A's callers never need to do. This is the
same shape ac-1 already specifies for the survivor *report* ("file, line,
operator, status") — idiom A is that shape plus one field; idiom B
diverges from it for a diff-size win only.

**How a regeneration command discloses itself, and why this spike does
NOT recommend copying either precedent's digest-sibling verbatim**: both
precedents (`svcfix-canned/digests.json`, `policyartifact/golden-digests.json`)
exist to make a file's content **only ever come from a real tool re-run**,
never a hand edit — that is precisely what a digest ratchet is for, and
precisely why grabbing that idiom whole for a mutation baseline would be
wrong: co-4 requires "a mutant that cannot be killed is baselined with a
one-line reason," and that reason is supposed to be **hand-written**
prose from whoever triaged the survivor, not toolchain output. A digest
that locks the whole file would forbid the exact hand edit the feature
needs. The regeneration command this spike recommends instead is a merge,
not an overwrite: re-run the pinned tool, drop baseline entries for
mutants that are now killed, keep the existing `reason` for every mutant
that still survives unchanged, and fail closed (refuse to write) if the
new run finds a survivor with no existing baseline entry and therefore no
reason — forcing a human to add one, which is itself the disclosure (a
diff that ADDS a new entry with a human-authored reason is exactly the
"diff a reviewer reads" ac-2 already asks for; a diff that silently drops
an entry with no explanation is not).

**Recommendation: idiom A's record shape (reusing ac-1's own report shape
plus a `reason` field), with a merge-style regeneration command and NO
whole-file digest sibling.**

## oq-5: can tier be derived mechanically

Full evidence and exact commands: `oq5-tier-mapping-evidence.txt`.

The chain the story specifies is file → package → test files in that
package → `verdi.bindings.yaml` producer → the producer's bound
acceptance criteria → `dc-6` tier of the criteria's owning spec. There is
exactly one real `verdi.bindings.yaml` in this repository (store root;
2235 lines, one `spec:`/`bindings:` block per spec that has had evidence
authored so far).

The decisive grep, re-run in fix round 1 in a throwaway clone at
`e963f4d0`: `grep -in "readiness" verdi.bindings.yaml | wc -l` → **0**.
Not a package name, not a spec reference, not a comment.

Two other greps quoted in the first version of this section, and in
`oq5-tier-mapping-evidence.txt`, **did not reproduce**; both are corrected
in place there with fresh transcripts. `grep -in
"journey\|policyconflict\|mcpserve\|e2eharness" verdi.bindings.yaml` was
recorded as zero matches; it prints **9** lines (49, 90, 211, 404, 442,
645, 799, 1876, 1938), every one a `#` comment inside another spec's
block. `grep -in "workbench" … | wc -l` was recorded as 62; it prints
**56** (57 raw occurrences). Worse for the first version's reasoning, 8 of
those 56 are NOT comments but real bound criteria —
`spec/workbench-directory#ac-2..ac-6` and
`spec/workbench-legibility#ac-1..ac-3`. They belong to pre-existing specs
that also touch `internal/workbench`; none belongs to
`spec/readiness-recovery`. The ruling is unchanged and now rests on the
`readiness` zero, which does reproduce — but the hazard below is sharper
than first written, because a naive by-package-name script would find real
criteria references, not just prose, and believe it had succeeded.

**Counts, per the story's own bins, over the 33 touched files:**
- Resolve to exactly one tier via the correct chain: **0**.
- Resolve to several tiers via the correct chain: **0** (nothing to be
  ambiguous between — step 2 of the chain is already empty for every
  file).
- Resolve to none: **33 of 33 (100%)** — the chain breaks at
  "package → producer," before `dc-6` is ever consulted, because
  `spec/readiness-recovery` (the owning spec of every touched file) has
  authored no `verdi.bindings.yaml` entry at all as of this diff.

A hazard worth naming separately from the count: a manifest-generation
script naive enough to match on package name alone (ignoring which
spec's block a hit belongs to) would find `internal/workbench`'s 6
touched files "resolving" to several tiers borrowed from unrelated
specs' bindings entries — a wrong answer produced silently. This spike
did not exercise that naive matching; it is recorded so the real
feature's implementation plan does not reach for it as a shortcut.

**Discrepancy recorded (constraint 10):** `spec/readiness-recovery` does
not exist anywhere in the scratch worktree at `e963f4d0` (neither
`.verdi/specs/active/` nor `archive/`) — the wave-1 implementation branch
never merged the spec describing the feature it implements; the spec
reached `main` separately and exists at this lane's deliverable worktree
base (`5f60c76c`). Substitution made: `dc-6` was read from the spec at
`main` rather than declaring oq-5 entirely unanswerable on a technicality
— disclosed here and in the evidence file rather than made silently. This
does not change the count above (the count is about `verdi.bindings.yaml`,
which is identical prose at both points — no bindings block for this spec
exists at either commit).

**Answered.** 0 / 0 / 33. The manifest must be hand-authored; ac-4's
"names the tier" is a citation a human writes, not a derivation any
script in this repository could currently produce.

## Deviations from the investigation plan

- **Evidence anchored on an unpushed local branch (whole-wave review
  W-3).** The commits b810c302, 1be75d01, 410db101 and e963f4d0 this
  spike measures against live only on the local branch
  `agent/readiness-recovery-wave-1`; origin carries wave 2, not wave 1,
  and no remote ref or tag contained them at wave close. The controller
  tagged all four locally as `spike-evidence/readiness-recovery-wave-1/
  <sha>` so worktree reclamation cannot lose them; pushing those tags is
  an owner action.
1. **oq-1's "handles a package whose tests exec a built binary" criterion
   is now measured, and gremlins FAILS it for `package main` directories.**
   (Superseded, fix round 1. The first version of this deviation said the
   criterion was "not exercised by the two mandated packages" and leaned on
   oq-2's supposed coverage of `cmd/e2eharness`; that coverage did not
   exist, and when run, it is a false 100%-kill result.) What is measured
   now: `cmd/e2eharness` — whose own tests do NOT in fact exec a built
   binary — is unmeasurable by gremlins because it is `package main`;
   `internal/designapp`, whose tests DO `go build ./cmd/verdi` and exec it,
   is measured and gremlins handles it correctly. Both in
   `oq1-gremlins-mainpkg-defect.txt`. Neither mandated package
   (`internal/readinessload`, `internal/journey`) execs a built binary, so
   the substitution the story's step 1 needed was a third package; the
   third package is `internal/designapp`, not `cmd/e2eharness`.
2. **oq-2's per-mutant seconds could not be fully measured for 7 of 10
   touched packages** — 2 of those 7 (`cmd/verdi`, `cmd/e2eharness`) are
   not measurable by the pinned tool at all (deviation 1), and 5
   (including the most expensive one still reachable,
   `internal/workbench`) were not reached within this session's practical
   wall-clock budget, because of sustained, independently-confirmed heavy
   shared-machine load (other lanes' own concurrent gate/review runs; see
   the oq-2 section's load readings). Substitution: those packages are
   reported with real baseline-timing evidence (a genuine lower bound on
   their mutation cost) and an explicit NOT ANSWERED for their mutant-level
   counts, rather than an extrapolated or invented number. That baseline
   timing comes from a whole-module coverage-gathering attempt
   (`oq2-evidence/whole-module-dryrun.log`) which itself hit two
   unrelated, pre-existing failures in this wave-1 branch's own tree
   before it could finish: `internal/align` (a hardcoded 1-second
   subprocess-response deadline that this session's shared load blew — a
   genuine load-sensitivity gap in the product's own suite, independent
   of anything mutation-related) and `internal/sealedexec` ("stale
   witness source cmd/verdi/context.go" / "...internal/policyconflict/
   service.go" — consistent with the wave-1 diff having edited both files
   without regenerating whatever witness fixture tracks them). Neither is
   this spike's to fix; both are named here rather than silently worked
   around, since the second is a real gap in the very branch this spike
   was asked to measure.
3. **gremlins' `--diff` flag does not select what its own `--help` text
   describes** — every mutant in every test of it, including mutants on a
   genuinely touched file, came back `SKIPPED`. Substitution (revised, fix
   round 1): touched-file scoping is now done with gremlins' own
   `-E/--exclude-files`, which works, is applied before mutant generation,
   and therefore satisfies dc-2 natively. The first version used a
   post-filter over whole-package runs and wrongly described that as
   honoring dc-2; see the oq-2 section. `--diff`'s root cause is still not
   investigated (it is a working candidate's optional flag, and `-E` makes
   it unnecessary) — recorded as a finding for the real feature to verify
   independently if it wants `--diff`, not silently worked around.
4. **`.verdi/data/gate/timings.tsv`** (the file the story's step 2 names)
   does not exist at either worktree's base. Substitution: used the exact
   numbers the lane brief supplies directly (PR #337 @ 01211bd8: test
   820s, spec-align 176s, e2e 776s, total 1820s) rather than reading the
   file.
5. **`spec/readiness-recovery` does not exist in the scratch worktree**
   (detached at `e963f4d0`) that the investigation plan says to work in —
   see oq-5's own discrepancy note for the full explanation and the
   substitution made (read `dc-6` from the deliverable worktree's base,
   `main` @ `5f60c76c`, instead).
6. **An early attempt at oq-3's seeded mutant silently no-op'd** (the
   `.verdi/data` directory did not exist in the test fixture, and a
   discarded `os.WriteFile` error hid the failure), which would have
   produced a false "survives at both commits" result. Caught by adding
   `os.MkdirAll` and a `panic` on either call's failure before trusting
   either result; recorded here because it is exactly the kind of
   silent-gap failure PA-017 itself is about, and this spike is not
   exempt from it.
7. **This session was interrupted once by an infrastructure (API billing)
   fault**, unrelated to any tool under test, mid-way through oq-1's ooze
   investigation. The scratch worktree was found dirty on resume (a
   partially-applied mutation from the interrupted run); it was restored
   with `git checkout -- . && git clean -fd` and reconfirmed clean and
   detached at `e963f4d0` before any further tool ran, per the brief's own
   scratch-worktree discipline.
8. **The avito go-mutesting fork mutates sources IN PLACE in the working
   tree, which is a step-1 disqualifier in its own right** (fix round 1).
   The first version recorded the clean-tree check for gremlins and,
   vacuously, for zimmski (which crashed before mutating anything), but
   never for the one fork that actually generated mutants; its evidence
   file said the fork "DID restore the working tree correctly on a normal
   exit," attributing the dirty tree to an interrupted run. Witnessed
   directly this round, DURING a normal run in a throwaway clone:

   ```
   $ git -C /tmp/fix-mr status --porcelain
    M internal/readinessload/facts.go
   ?? internal/readinessload/facts.go.tmp
   ```

   So the fork dirties the tree for the duration of every mutant, not only
   when interrupted. Step 1 of the story's plan makes the clean-tree check
   a disqualifier on its own; this fork fails it independently of the
   compile-failure-as-kill defect already recorded against it.
9. **This fix round ran every mutation tool in a throwaway `/tmp` clone of
   the scratch worktree, not in the scratch worktree itself** (`git clone
   -q --no-hardlinks <scratch> /tmp/fix-mr && git -C /tmp/fix-mr checkout
   --detach e963f4d0`). Deviation 8 is why that matters: a tool that
   mutates in place cannot be allowed near a worktree whose refs are
   shared. `run-oq2.sh` now refuses a path whose `.git` is a file (a
   worktree) rather than a directory (a clone).

## Evidence index

- `oq1-gremlins-mainpkg-defect.txt` — **fix round 1, the Critical
  finding's root cause**: the instrumented `go`-wrapper trace, gremlins'
  own `pkgName`/`getTestFailedStatus` source, the measured Go 1.25 exit
  codes, the like-for-like counterfactual (same 11 mutants, 904 ms broken
  vs 46.057 s corrected), the `internal/designapp` control, the blast
  radius, the complete `unleash` flag list, and the mitigations tried.
- `oq1-tool-installs.txt` — exact install commands, exit statuses, version
  output, for all four tool/fork candidates.
- `oq1-gomutesting-zimmski-panic.txt` — complete, unedited crash trace.
- `oq1-gomutesting-avito-defect.txt` — reduced run transcript plus the
  exact undefined-symbol compiler errors proving the false-positive
  mechanism.
- `oq1-ooze-defect.txt` — installed-module source excerpts proving the
  vacuous-pass mechanism, plus the timing transcript that first exposed it.
- `oq2-evidence/` — retained raw JSON/log output backing the oq-2 tables.
  From the first version: per-package gremlins JSON reports for
  configuration A (`gremlins-*.json`), the whole-module dry-run log
  (`whole-module-dryrun.log`), the spurious-timeout log from store's
  first, untuned attempt (`store-run1-spurious-timeouts.log`), and the
  `--diff` flag transcript referenced in oq-1
  (`gremlins-diff-flag-attempt.log`). Added in fix round 1:
  `exclude-files-scoping.txt` (dc-2's native file scoping, proven, with
  the `-E` semantics read out of gremlins' source),
  `run-oq2-operators-all.log` (the configuration-B sweep, verbatim, the
  source of every configuration-B figure), and
  `cmd-verdi-timeout-probe.log` (the `-timeout 30m` cover probe).
- `oq3-reach-evidence.txt` — full before/after commands, the exact seeded
  mutant, and both test-run transcripts (survives / killed).
- `oq4-baseline-drafts/` — both idioms, filled with real survivor data,
  plus a real one-survivor-killed diff for each.
- `oq5-tier-mapping-evidence.txt` — the exact `grep`s over
  `verdi.bindings.yaml`, the dc-6 discrepancy, and the full count.
- `run-oq2.sh` — re-runnable, and in fix round 1 it is the script that
  produced configuration B's figures, not a stub: `run-oq2.sh
  <clone-path> [<gremlins-bin>]`, where `<clone-path>` is a throwaway
  `/tmp` clone detached at `e963f4d0` (the script refuses a worktree).
  It now pins the operator set explicitly (`OPERATORS=all|default`),
  scopes files with gremlins' own `-E` instead of post-filtering, asserts
  the report contains no untouched file, records `uptime` before and after
  every package, and refuses `package main` directories with the defect
  named (`MEASURE_MAIN_PKGS=1` to see the false result).
  `GREMLINS_TIMEOUT_COEFFICIENT`/`GREMLINS_WORKERS` still tune
  timeout/concurrency (defaults 10/2); `ONLY_PKGS` restricts the sweep.
  Retained from the first version: its touched-files-per-package grouping
  matches by exact `dirname`, not by path prefix, after a stub-tool run
  caught a real bug where `internal/readinesspilot/readinesstest`'s own
  touched file was folded into `internal/readinesspilot`'s row.

## Throwaway rule

Nothing here is kept by the real feature: no tool, config, baseline, or
Makefile target from this spike is wired into `make verify` or any other
gate. `spec/mutation-ratchet`'s own implementation plan re-creates
whatever it needs, under test, citing this document for the five
decisions above rather than re-deriving them.
