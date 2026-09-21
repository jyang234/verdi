# Mutation-ratchet spike — answer

Resolves `spec/mutation-ratchet#oq-1` through `#oq-5` (spike story
`spec/mutation-ratchet-spike`). Investigation ran in two worktrees per the
lane brief: `verdi-wt/spike-mutation-ratchet` (this deliverable, branch
`spike/mutation-ratchet`, base `5f60c76c`) for everything committed, and
`verdi-wt/scratch-mutation-w1` (detached at `e963f4d0`, the wave-1 head)
for every mutation-tool run. Evidence files referenced below live beside
this README under `docs/spikes/mutation-ratchet/`.

## Recommendation (summary)

- **oq-1**: `gremlins@v0.6.0` (`github.com/go-gremlins/gremlins/cmd/gremlins`).
  It is the only one of the three named candidates that installs, runs,
  and leaves the tree clean on Go 1.25 against this module. Both
  go-mutesting forks and ooze are disqualified with witnesses (a runtime
  panic for one; two independent, source-verified false-positive result
  mechanisms for the other two).
- **oq-2**: measured, real per-mutant tables exist for three of the nine
  touched packages (store, readinessload, journey); the rest are
  timing-bounded but not mutant-counted within this session — see that
  section for why (sustained, independently-observed shared-machine load
  in the 1-minute-load-average 40-80 range, an order of magnitude over
  constraint 7's "under 4" target, for most of this spike's oq-2 window).
  cmd/verdi's own coverage baseline alone exceeded Go's 10-minute default
  test timeout and is reported NOT ANSWERED / infeasible as measured.
- **oq-3**: YES, with a caveat — the deciding mutant is hand-seeded, not
  organically produced by any candidate's operator catalog, because
  `internal/readinessload` genuinely has no file-write statement anywhere
  for such an operator to perturb. The seeded mutant survives at 1be75d01
  and is killed at 410db101 by two independent tests.
- **oq-4**: recommend the `testdata/mutation-baseline.json` idiom's
  *structure* (a list of `{file, line, column, operator, reason}` records,
  reusing ac-1's own canonical-report shape plus one field), but reject
  that idiom's usual sibling *digest* file — a mutation baseline's `reason`
  field is meant to be hand-edited (co-4), and a digest ratchet exists
  specifically to forbid hand edits, which would fight the feature's own
  requirement. Regeneration should merge (drop killed entries, keep
  reasons for still-surviving ones, fail closed on any new, unreasoned
  survivor) rather than overwrite.
- **oq-5**: 0 of 30 touched files resolve to exactly one tier by the
  intended mechanical chain; 0 resolve to several by that same chain; 30
  of 30 (100%) resolve to none, because `spec/readiness-recovery` — the
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
checked after every single invocation, dozens of times). Runtime on the
two named packages, mutating every non-test file in each (oq-1's own step
1 scopes to the whole of these two packages, unlike oq-2's touched-files
filtering below — journey in particular has two files, `codec.go` and
`record.go`, that are not part of the wave-1 diff and are excluded from
oq-2's own journey row for exactly that reason): `internal/readinessload`
87.6s (101 tested mutants: 96 killed, 5 lived, plus 14 more
found-but-not-covered; `Mutation testing completed in 1 minute 27
seconds`), `internal/journey` 136s (267 tested mutants: 243 killed, 19
lived, plus 5 not-covered; `2 minutes 16 seconds`).
Neither package's tests exec a built `cmd/verdi` binary (`grep -rln
"exec.Command\|TestMain"` finds only `git`-subprocess helpers in both), so
the story's "handles a package whose tests exec a built binary without
hanging" criterion is not exercised by the two mandated packages —
recorded as a discrepancy below; oq-2's own sweep separately covers
`cmd/e2eharness`, which IS in the Makefile's `CROSS_BINARY_PKGS` list, and
gremlins handled it without hanging (see oq-2).

A bonus finding worth carrying into the real feature's design: gremlins
has a built-in `-D/--diff <ref>` flag that looks purpose-built for dc-2's
"only files the change touched are mutated." This spike could not get it
to select anything but `SKIPPED` for every single mutant in a package,
including mutants on the one file that genuinely changed between
`b810c302` and `e963f4d0` (`internal/store/paths.go`) — full transcript:
`oq2-evidence/gremlins-diff-flag-attempt.log`
(`gremlins unleash --diff b810c302 --output ... ./internal/store`, all
101 mutants across all 9 of the package's files came back `SKIPPED`, 0
killed/lived/not-covered). This spike did not have time to root-cause `--diff`
inside gremlins' own source the way it did for ooze's defect (three
disqualifying findings already existed; a fourth investigation into a
*working* candidate's optional flag was lower priority under the timebox).
The real feature should not assume `--diff` works as documented without
its own, dedicated verification; oq-2's own touched-files scoping in this
spike was done by post-filtering gremlins' per-file JSON report instead,
which is unaffected by whatever `--diff` is doing.

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

Touched non-test `.go` files: `git diff --name-only b810c302 e963f4d0 --
'*.go' | grep -v _test.go` returns 30 files across 9 package directories
(`cmd/e2eharness`, `cmd/verdi`, `internal/journey`, `internal/mcpserve`,
`internal/policyconflict`, `internal/readinessload`, `internal/readinesspilot`,
`internal/readinesspilot/readinesstest`, `internal/store`,
`internal/workbench`). gremlins mutates a whole package at a time (it has
no single-file target); this spike's `run-oq2.sh` runs it once per
touched package and then filters that package's own per-file JSON report
down to just the touched files before tallying, so dc-2's "never mutates
files the change did not touch" is honored by post-filtering, not by
gremlins' `--diff` flag (see oq-1's finding on that flag).

**Load discipline actually followed (constraint 7):** `uptime` was polled
before starting; it read 2.35 / 2.99 / 2.78 at various points early in
this session (1-minute average under 4, as required) and the sweep was
started at one of those windows. It did not stay quiet: over the course
of running the sweep, `uptime`'s 1-minute figure was observed at 4.90,
12.02, 15.53, 46.35, 62.05, 75.19, and 78.84 — other lanes' own gate/review
work on this same machine (confirmed directly: `ps aux` during this
window shows a concurrent `git clone .../spike-self-governance` review
run and several `go test ./...` invocations from a different lane's PID
tree, neither started nor controlled by this spike). Two full sweep
attempts were made; the first (gremlins `--timeout-coefficient 10
--workers 2`) spent over 30 minutes on `internal/store` and
`internal/mcpserve` alone before being interrupted; the second
(`--timeout-coefficient 5 --workers 1`, chosen to reduce gremlins' own
self-contention) is disclosed per-package below. Every number below
carries the load at the time it was actually produced; none is
extrapolated from a quiet-window measurement elsewhere.

### Fully measured (mutant-level kill/survive/timeout counts)

Counts below are filtered to touched files only, per-mutant, from
gremlins' own per-file JSON (`oq2-evidence/*.json`); "mutants" is
killed+survived (gremlins' own convention: a not-covered mutant is found
but never actually run against the test suite, so it is kept as its own
column, not folded into "mutants"). Seconds is the WHOLE PACKAGE's wall
clock (gremlins reports no per-file timing), so for any package with
touched AND untouched files it is an upper bound on the touched-files-only
cost, not an exact figure — flagged per row.

| package | touched files | mutants | killed | survived | not covered | timed out | seconds (package-level; see caveat) | load (1-min) at run |
|---|---|---|---|---|---|---|---|---|
| internal/store | paths.go (1 of 9 files in the package) | 16 | 16 | 0 | 0 | 0 | 29 (whole package; touched file is 16 of the package's 101 total mutants, ~16%, so 29s meaningfully overstates paths.go's own cost) | ~3–12 (second, tuned attempt; see below for the first) |
| internal/readinessload | conflict.go, doc.go, facts.go, load.go, options.go (all 5 non-test files in the package) | 101 | 96 | 5 | 14 | 0 | 88 (whole package = touched-files cost exactly; nothing to filter) | load unrecorded (run predates this spike's load-logging discipline; disclosed as a gap, not hidden) |
| internal/journey | derive.go, eventual.go, facts.go, port.go, project.go, reason.go (6 of 8 files in the package) | 169 | 155 | 14 | 5 | 0 | 136 (whole package; the touched files are 169 of the package's 262 tested mutants, ~65%, so 136s overstates the touched-only cost by roughly a third — the other 93 tested mutants belong to `codec.go`/`record.go`, neither touched by this diff) | load unrecorded, same caveat |
| **measured total** | | **286** | **267** | **19** | **19** | **0** | **253** (upper bound; see per-row caveats) | |

readinessload's touched-file set is its whole package's non-test file
list (nothing to filter out), so its row needs no filtering — the
whole-package run already equals the touched-files run, both in mutant
counts and in wall clock. store's and journey's do not: both packages
have untouched sibling files that gremlins mutated anyway (because it
mutates a whole package at a time), so both rows' mutant/kill/survive/
not-covered counts are correctly filtered down to the touched files only,
but both rows' *seconds* column still reflects the whole package's run
(gremlins gives no finer-grained timing) and is therefore an overstatement
of what a truly file-scoped run would cost — see each row's own note.

internal/store's own before/after: the FIRST attempt, at gremlins'
default timeout calibration under a load average of ~12–17, produced 87
of 101 mutants `TIMED OUT` and 0 killed/lived — a result this spike does
NOT report as real survivor data, because it is a timing artifact, not a
finding about the code (see `oq2-evidence/store-run1-spurious-timeouts.log`).
Raising `--timeout-coefficient` to 10 and lowering `--workers` to 2
(reducing gremlins' own self-contention) under a calmer moment (load
~3) brought it down to 6 residual timeouts out of 101 — still nonzero
under any settings tried, which is itself the finding: **a per-wave gate
step that hard-fails on any survivor cannot safely run gremlins at
default timeout settings on a contended CI runner; the real feature's
co-3 budget decision needs its own timeout-coefficient tuned against real
CI hardware, not this spike's laptop-under-load numbers.**

### Bounded by baseline timing, not mutant-counted within this session

The remaining touched packages' own full-suite baseline pass (the
prerequisite gremlins runs before it can generate a single mutant) was
measured directly (`go test -cover` timing, captured incidentally from
this spike's earlier whole-module `--diff` dry-run attempt before that
attempt aborted on two unrelated pre-existing failures elsewhere in the
tree, one of which is itself a load-sensitivity finding worth carrying
forward — see "Deviations" below):

| package | touched files | baseline pass (coverage-gather only) | mutant-level counts |
|---|---|---|---|
| cmd/e2eharness | readinessallproven.go | 30.9s | not completed this session |
| internal/mcpserve | backend.go, tool_get_document.go | 72.5s | not completed this session (two attempts each ran >15 min without finishing under load averages of 40–80; interrupted rather than let run further) |
| internal/policyconflict | cachejudge.go, service.go | 56.7s | not completed this session |
| internal/readinesspilot | derive.go | 0.45s | not completed this session (package itself is tiny; not reached before this spike's time budget ran out) |
| internal/readinesspilot/readinesstest | readinesstest.go | (not separately measured; package is 72 lines) | not completed this session |
| internal/workbench | boarddocument.go, boardspec.go, branchboard.go, handler.go, readiness.go, readinessrender.go | 269.8s (4.5 min, just for ONE full-suite pass) | not completed this session |

**cmd/verdi (conflictgate.go, context.go, context_conflict.go,
context_constitution.go, mcp.go, readiness_snapshot.go, serve.go,
specdoc.go): NOT ANSWERED / infeasible as measured.** Its own
coverage-gathering baseline pass — the step *before* gremlins can generate
a single mutant — ran for 600.587s and was killed by Go's own default
10-minute per-package test timeout (`FAIL github.com/jyang234/verdi/cmd/verdi
600.587s`), under the same shared-load conditions as everything else in
this table. This is not a mutation-tool finding; it is evidence that
`cmd/verdi`'s own test suite is already at (or past) the edge of what a
single `go test` invocation tolerates on contended hardware, independent
of mutation testing. Full transcript: `oq2-evidence/whole-module-dryrun.log`
(reduced; see the file's own header for how).

**Ratio to the gate baseline (lane specifics, PR #337 @ 01211bd8: test
820s, spec-align 176s, e2e 776s, total 1820s):** using only the fully
measured total (253s across store+readinessload+journey — a strict
under-count, since it excludes 6 of 9 touched packages and all of
cmd/verdi), the ratio is 253/1820 ≈ 0.14 (14%). This ratio is **not**
usable as a ceiling estimate for the real feature: it excludes exactly
the packages (workbench, cmd/verdi) that this spike's own baseline timing
shows are the expensive ones. A defensible upper bound, adding the
baseline-only packages' own coverage-gather time (which is a lower bound
on their real mutation cost, since mutation testing costs at least one
baseline pass plus N mutant re-runs) — 253 + 30.9 + 72.5 + 56.7 + 0.45 +
269.8 ≈ 683s, excluding cmd/verdi entirely — gives a ratio of
683/1820 ≈ 0.38 (38%), still excluding cmd/verdi, still a per-touched-package
cost that must be paid on EVERY wave whose diff touches that package, not
a one-time cost.

**Answered, with caveat.** Three packages have real, decisive mutant
tables proving the technique is fast and effective when it runs (store:
6% residual-timeout rate after tuning; readinessload/journey: 0 timeouts,
sensible kill rates). The other six packages, including the two most
expensive ones this module has (workbench, cmd/verdi), are bounded by real
baseline-timing evidence but not mutant-counted, because this spike's
session ran out of tolerable wall-clock budget under sustained,
independently-confirmed heavy shared-machine load — recorded here as
NOT ANSWERED for those packages' mutant counts specifically, per
constraint 8, rather than guessed or extrapolated.

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

**Answered, with caveat.** YES: the technique reaches PA-017's exact
witness class, demonstrated with the seeded mutant quoted above. Caveat:
this is necessarily a hand-seeded fixture, not an organically
tool-discovered mutant, and that is not a shortfall specific to this
spike's three candidates — it follows from the target file's own content
containing no exploitable write statement for any catalog operator to
perturb. The real feature's ac-3 should be written expecting a hand-seeded
regression fixture (which matches its own wording, "a committed
regression fixture," already) rather than a promise that mutation testing
will spontaneously rediscover this exact defect class on unrelated code.

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
authored so far). It has zero mentions of "readiness" anywhere — not a
package name, not a spec reference, not even in a comment — and zero
mentions of `journey`, `policyconflict`, `mcpserve`, or `e2eharness` as an
owning spec either. `workbench` appears 62 times, but every one of those
62 hits is a comment justifying evidence for a DIFFERENT, pre-existing
spec (workbench-directory, workbench-legibility, draft-boards, and
others) that also happens to touch `internal/workbench` — none of them is
`spec/readiness-recovery`.

**Counts, per the story's own bins, over the 30 touched files:**
- Resolve to exactly one tier via the correct chain: **0**.
- Resolve to several tiers via the correct chain: **0** (nothing to be
  ambiguous between — step 2 of the chain is already empty for every
  file).
- Resolve to none: **30 of 30 (100%)** — the chain breaks at
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

**Answered.** 0 / 0 / 30. The manifest must be hand-authored; ac-4's
"names the tier" is a citation a human writes, not a derivation any
script in this repository could currently produce.

## Deviations from the investigation plan

1. **oq-1's "handles a package whose tests exec a built binary" criterion**
   is not exercised by the two mandated packages (`internal/readinessload`,
   `internal/journey`) — neither execs a built `cmd/verdi` binary (both
   only shell out to `git`). Substitution: relied on oq-2's mandatory
   coverage of `cmd/e2eharness` (a real `CROSS_BINARY_PKGS` member) for
   this specific criterion instead of adding a third package to oq-1's own
   scope.
2. **oq-2's per-mutant seconds could not be fully measured for 6 of 9
   touched packages** (including the two most expensive, `cmd/verdi` and
   `internal/workbench`) within this session's practical wall-clock
   budget, because of sustained, independently-confirmed heavy
   shared-machine load (other lanes' own concurrent gate/review runs; see
   the oq-2 section's load table). Substitution: those packages are
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
   genuinely touched file, came back `SKIPPED`. Substitution: touched-file
   scoping in this spike's own `run-oq2.sh` is done by post-filtering
   gremlins' per-file JSON report to the touched-file set computed
   directly from `git diff --name-only`, not by `--diff`. Root cause not
   investigated (three disqualifying findings already existed among the
   candidates; this is a *working* candidate's optional flag, lower
   priority under the timebox) — recorded as a finding for the real
   feature to verify independently, not silently worked around.
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

## Evidence index

- `oq1-tool-installs.txt` — exact install commands, exit statuses, version
  output, for all four tool/fork candidates.
- `oq1-gomutesting-zimmski-panic.txt` — complete, unedited crash trace.
- `oq1-gomutesting-avito-defect.txt` — reduced run transcript plus the
  exact undefined-symbol compiler errors proving the false-positive
  mechanism.
- `oq1-ooze-defect.txt` — installed-module source excerpts proving the
  vacuous-pass mechanism, plus the timing transcript that first exposed it.
- `oq2-evidence/` — retained raw JSON/log output backing the oq-2 tables:
  per-package gremlins JSON reports (`gremlins-*.json`), the whole-module
  dry-run log this spike's baseline timings and the `cmd/verdi` timeout
  witness are drawn from (`whole-module-dryrun.log`), the spurious-timeout
  log from store's first, untuned attempt
  (`store-run1-spurious-timeouts.log`), and the `--diff` flag transcript
  referenced in oq-1 (`gremlins-diff-flag-attempt.log`).
- `oq3-reach-evidence.txt` — full before/after commands, the exact seeded
  mutant, and both test-run transcripts (survives / killed).
- `oq4-baseline-drafts/` — both idioms, filled with real survivor data,
  plus a real one-survivor-killed diff for each.
- `oq5-tier-mapping-evidence.txt` — the exact `grep`s over
  `verdi.bindings.yaml`, the dc-6 discrepancy, and the full count.
- `run-oq2.sh` — re-runnable: `run-oq2.sh <scratch-worktree-path>
  [<gremlins-bin>]`; `GREMLINS_TIMEOUT_COEFFICIENT`/`GREMLINS_WORKERS`
  env vars tune gremlins' own timeout/concurrency (defaults 10/2; this
  spike's second attempt used 5/1 under heavier load — see oq-2). Its
  end-to-end orchestration (package discovery, touched-file filtering,
  totals) is verified against a stub tool standing in for gremlins (writes
  an instant, minimal JSON instead of actually mutating anything) — real
  gremlins runs under this session's load never completed for every
  touched package in one sweep (see oq-2), so this is the only
  full-script run this spike actually completed. That stub run caught and
  fixed a real bug: the touched-files-per-package filter originally
  matched by path *prefix*, which wrongly folded
  `internal/readinesspilot/readinesstest`'s own touched file into
  `internal/readinesspilot`'s row (a subpackage swallowed by its parent's
  prefix); it now matches by exact `dirname`.

## Throwaway rule

Nothing here is kept by the real feature: no tool, config, baseline, or
Makefile target from this spike is wired into `make verify` or any other
gate. `spec/mutation-ratchet`'s own implementation plan re-creates
whatever it needs, under test, citing this document for the five
decisions above rather than re-deriving them.
