# Strict lint yield, true-positive rate, and ratchet mode — spike answer

Resolves `spec/strict-lint-target#oq-1`, `#oq-2`, `#oq-3`, `#oq-4` (via
`spec/strict-lint-target-spike`). Measured at main `5f60c76c` in
`verdi-wt/spike-strict-lint-target`, golangci-lint `2.5.0` (matches the
Makefile's `GOLANGCI_LINT_VERSION ?= v2.5.0` pin — confirmed by both
version strings below).

## Recommendation, up front

Adopt a second, ratchet-gated `lint-strict` target, but not as either side
proposed it going in:

- **Keep and gate on:** `containedctx`, `noctx` (100% TP each, ten-sample
  floor for noctx), `contextcheck`, `errorlint` (70% TP each — see the
  caveat below on what the other 30% is).
- **Do not gate `exhaustive` at its default settings.** In this sample its
  true-positive rate against the written rule is 0/10 — not because the
  linter is wrong, but because this codebase's idiom is an explicit
  `default:` fail-closed clause rather than naming every enum case, and
  `exhaustive`'s default posture does not treat a `default:` as
  exhaustive. Set `settings.exhaustive.default-signifies-exhaustive: true`
  first; re-measure before deciding further (see oq-2).
- **`dupl` cannot enforce CLAUDE.md's cross-package sentence at all —
  "scope it to cross-package pairs" is not an available remedy.**
  golangci-lint dispatches `dupl` per package, so it structurally can
  never compare files across a package boundary; every finding it ever
  produces is same-package by construction. Verified from this lane's own
  data: 0 of 37 findings in the full `./...` run are cross-package
  (`oq2-dupl-package-scope-output.txt`), and 0 of 13,521 remain
  cross-package even at `dupl.threshold: 10` over the retro-witness's
  restricted scope, while cross-*file*, same-package pairs are abundant at
  that threshold. This is also why `dupl` never reaches the feature's own
  motivating case: the two `hasDotDotElement` copies at `1be75d01` are in
  *different* packages (`cmd/verdi` and `internal/readinessload`), so no
  threshold could ever make `dupl` see them as a pair — proven directly by
  a line-range-overlap check against `dupl`'s actual output at every
  threshold from 150 down to 10 (`oq2-dupl-overlap-check-output.txt`), not
  inferred from the absence of a length knob. Consequence for the spec
  seed: ac-3's `dupl` half is unreachable by `dupl` at any setting; ac-1
  must drop `dupl`, keep it report-only for same-package duplicate hygiene
  (a real but CLAUDE.md-silent rule), or name a different tool for the
  cross-package sentence.
- **`gochecknoglobals` cannot be ruled on the same axis as the other six**:
  CLAUDE.md's Go style and Testing rules sections contain no sentence
  banning package-level variables. This is a gap in the parent spec's own
  premise (its problem statement asserts the mapping; the text does not
  exist), recorded here rather than silently patched. Every sampled finding
  is mechanically a real global; two of ten are a genuinely mutable,
  test-reassigned dependency-injection seam (`closeAddPaths`,
  `closeCreateCommit` in `cmd/verdi/close.go`) — the strongest real
  instance of "mutable global state" in the sample, and worth keeping on
  that basis regardless of the missing sentence, but that is this spike's
  judgment call, not a rule application.
- **Ratchet mechanism: prefer a committed baseline over
  `--new-from-rev`.** Both are byte-for-byte deterministic given a fixed
  invocation path. `--new-from-rev` has a reproduced, repeatable false
  negative when invoked from a path reached through a symlink (silently
  reports zero issues, exit 0, when exactly one real new issue exists) — a
  plain baseline-diff does not exhibit this because it never calls
  golangci-lint's own git-diff integration. See oq-3.
- **Coexistence is unconditionally safe**: golangci-lint v2 never
  auto-discovers a file named `.golangci.strict.yml`; the two files cannot
  collide by accident under any invocation this spike tried. See oq-4 for
  the Makefile recipe.

## oq-1 — finding count and runtime per candidate linter

Command (re-runnable as `docs/spikes/strict-lint-target/run-oq1.sh`, one
invocation per linter):

```
golangci-lint run --config docs/spikes/strict-lint-target/golangci.strict.yml \
  --enable-only <linter> --max-issues-per-linter=0 --max-same-issues=0 ./...
```

`--max-issues-per-linter`/`--max-same-issues` are forced to `0` (the CLI
defaults are 50/3) — without this, `gochecknoglobals`' true count would be
silently capped at 50, exactly the "hundreds of findings" case the parent
spec flags as a live risk. Raw output: `oq1-raw/<linter>.{json,txt}`;
figures cross-checked against the JSON issue count (`jq '.Issues|length'`)
and each `.txt` file's own `--show-stats` summary line — all seven agree.

| linter | findings | seconds (heavy load) | seconds (lower load, re-run) |
|---|---|---|---|
| containedctx | 8 | 0.93 | 1.15 |
| noctx | 256 | 1.04 | 0.97 |
| contextcheck | 43 | 0.95 | 0.97 |
| errorlint | 173 | 1.04 | 0.95 |
| exhaustive | 129 | 0.98 | 0.95 |
| dupl | 37 | 0.77 | 0.82 |
| gochecknoglobals | 404 | 1.09 | 1.03 |

**Timing, disclosed as measured under shared load, both runs:** this is a
shared 10-core machine with other spike lanes running concurrently
throughout. First run: `uptime` immediately before/after the batch read
`load averages: 46.94 30.73 17.80` → `47.82 31.19 18.03`
(`oq1-summary-heavyload.txt`). Second run, roughly two hours later,
1-minute load down an order of magnitude but 15-minute still elevated from
the intervening period: `9.01 25.09 38.80` → `8.85 24.79 38.62`
(`oq1-summary.txt`, the one `run-oq1.sh` reproduces by default — this file
is overwritten by design each time the script runs, which is why a second,
explicitly-named copy of the first run was kept). **Findings counts are
byte-identical across both runs** (a useful cross-check in its own right);
per-linter wall clock stays in a 0.77s–1.15s band regardless of the
sixfold difference in 1-minute load average — sub-1.1s-per-linter over 104
packages is cheap enough that this measurement does not appear to be
load-sensitive at this scale, though neither run is a clean single-tenant
measurement and a true idle-machine baseline was not captured.

**This per-linter table cannot fill co-3's budget and was never claimed
to: `make lint-strict` will run one combined invocation of all seven, not
seven separate `--enable-only` passes, and the number that matters is
dominated by cache state, not by linter count or load average.** Measured
directly (`run-oq1-combined.sh` for the cold shape; full transcript
`oq1-combined-summary.txt`), one combined `golangci-lint run --config
golangci.strict.yml ./...` invocation, three cache states:

| cache state | elapsed | issues | load before → after |
|---|---|---|---|
| cold (isolated, empty `GOCACHE`+`GOLANGCI_LINT_CACHE`, the fresh-CI-runner shape) | **32.65s** | 1050 | 40.67/45.04/39.66 → 47.95/46.30/40.33 |
| warm (same isolated dirs, immediately re-run) | 1.29s | 1050 | 23.78/40.08/38.37 → 23.78/40.08/38.37 |
| shared (this machine's ordinary, already-warm caches) | 1.41s | 1048* | 21.89/39.14/38.06 → 20.78/38.62/37.88 |

(*2 fewer than the isolated runs — a cache-state-dependent discrepancy in
two `exhaustive` findings, disclosed but not chased further in
`oq1-combined-summary.txt`; it does not affect the timing conclusion.)

The independent review's own re-derivation (different moment, same
machine) reports 50.19s cold / 5.66s warm / 14.98s shared — different
exact seconds (different ambient load at that moment) but the same
qualitative shape. Cache state sets the shape; load average moves the
cold figure by almost 3x (three cold runs: 18.62s at load ~3.6, 32.65s at
load ~41, 50.19s at load ~32–70), so neither alone explains the number.
**co-3's budget should be stated as a range, roughly 1.3s–50s, with the
top bound being the highest observed cold run (50.19s), the cold-CI-runner
case**, and staying near the bottom in
practice requires caching golangci-lint's own analysis cache directory
across CI runs (`.github/workflows/merge-gate.yml` currently caches only
the golangci-lint *binary*, per its `Cache golangci-lint` step, not its
analysis cache) — a co-3 follow-up to name, not assume already solved.

### Other linters mapping to a written CLAUDE.md ground rule

Full inventory: `golangci-lint help linters` (114 raw lines, 111 actual
linter entries once the two section headers and one blank line are
subtracted — see "Deviations" below — `help-linters.txt`, unreduced, short
enough that no reduction was needed). Every disabled-by-default linter's
one-line description was checked against every sentence
in `/Users/johnyang/code/verdi-system/CLAUDE.md`'s **Go style** and
**Testing rules** sections (the plan's literal scope — not the rest of the
file). Three are confident, direct matches beyond the seven named
candidates:

| linter | one-line description | CLAUDE.md sentence |
|---|---|---|
| `ireturn` | "Accept Interfaces, Return Concrete Types." | "Accept interfaces, return structs; define interfaces at the consumer (the 04 §port pattern)." |
| `wrapcheck` | "Checks that errors returned from external packages are wrapped." | "Errors: wrap with `%w` and context;" |
| `gochecksumtype` | "Run exhaustiveness checks on Go \"sum types\"." | "...unknown enum values fail closed." (a second Go idiom — interface-based sum types — for the same sentence `exhaustive` already covers for `switch`/`iota` enums) |

Two more are real but configuration-dependent, not default-behavior
matches, so kept out of the primary table:

- `iface` ("Detect the incorrect use of interfaces, helping developers
  avoid interface pollution") is a softer, secondary match to the same
  "Accept interfaces, return structs" sentence as `ireturn`.
- `forbidigo` ("Forbids identifiers") *could* enforce "**No network in any
  test**" (Testing rules) if configured with `settings.forbidigo.forbid`
  patterns naming `net.Dial`/`http.Get`/etc. — but forbids nothing by
  default; this is a configuration a future plan would have to write, not
  a linter that already does this out of the box.
- `revive` is configurable enough that some rule combination might reach
  "`-er` names for single-method interfaces," but confirming that needs
  per-rule configuration beyond `help linters`' one-line granularity —
  disclosed as unproven, not chased further inside this timebox.

One explicit non-finding, stated rather than left silent: **`gofmt` never
appears in `help linters` at all.** golangci-lint v2 moved formatters
(`gofmt`, `goimports`) to a separate `formatters:` config section, disjoint
from `linters:`; CLAUDE.md's "`gofmt`-clean" is already gated by `make
fmt-check` (plain `gofmt -l .`), not by anything this exercise touches. "No
cgo" (same sentence) has no linter mapping in the full list; nothing in
`golangci-lint help linters` addresses cgo.

Two rule-class sentences outside the seven candidates and the three matches
above were checked and have **no** linter mapping in the full 114-line
list: "Single responsibility... no god packages," and "Design zero values
to be useful; otherwise provide a constructor." Recorded so the search
reads as exhaustive-and-empty for those, not merely unmentioned.

**oq-1: ANSWERED** — table above; additional-candidate list above, each
with its CLAUDE.md sentence quoted; two explicit no-mapping findings
recorded rather than left silent.

## oq-2 — true-positive rate and the retro-witness

Full ten-sample tables with per-finding rationale for all seven linters:
`samples.md`. Summary:

| linter | TP | FP | rule-not-stated | note |
|---|---|---|---|---|
| containedctx | 8/8 | 0 | 0 | fewer than ten findings exist; all mechanically real struct fields |
| noctx | 10/10 | 0 | 0 | all in `cmd/e2eharness`, none sampled from `cmd/verdi`/`internal/` |
| contextcheck | 7/10 | 0 | 3 | the 3 are "propagate an inherited ctx, don't refabricate `Background()`" — real, but a different rule than "ctx is the first parameter" |
| errorlint | 7/10 | 0 | 3 | the 3 are "use `errors.As`, not a type assertion" — real, but a different rule than "wrap with `%w`" |
| exhaustive | 0/10 | 0 | 10 | every sample either already has a fail-closed `default:` clause, or is a narrow test-fixture switch with no production fail-open risk |
| dupl | 0/10 | 0 | 10 | every sample is same-package (or same-file); the sentence specifically says "across packages" |
| gochecknoglobals | n/a | n/a | 10/10 | no CLAUDE.md sentence exists to be true against (see below); mechanically 10/10 are real globals, 2/10 genuinely mutable at runtime |

`contextcheck` and `errorlint` each bundle two distinct checks under one
linter name — one matching CLAUDE.md's sentence, one a different (real,
sensible) Go idiom the sentence does not state. Both are behind a settings
knob for `errorlint` (`settings.errorlint.asserts: false, comparison:
false` drops its count `173 → 132` over `./...`, verified —
`oq2-errorlint-settings.txt` — removing exactly the type-assertion-safety
class this sample's 3/10 rule-not-stated share names); `contextcheck` has
no equivalent knob found, disclosed as unmeasured rather than assumed
absent. `exhaustive` and `dupl` each score 0% against the literal sentence
in this sample, but for structurally different reasons: `exhaustive` is a
settings fix (below); `dupl` cannot enforce the sentence at all, by
construction — golangci-lint runs `dupl` per package, so it never sees a
cross-package pair regardless of threshold (see the Recommendation
section above for the verified 0/37 and 0/13,521 counts). `gochecknoglobals`
cannot be scored at all: CLAUDE.md's Go style (9 bullets) and Testing
rules (4 bullets) sections were read in full and contain no sentence about
global variables, package-level state, or mutability — the parent
feature spec's problem statement asserts this mapping, but so does its
paraphrase for every one of its other five rows; the real difference is
that those five paraphrases trace to CLAUDE.md sentences that actually
exist, and this one does not. This is recorded as a discrepancy, not
resolved by inventing or paraphrasing a CLAUDE.md sentence that is not
there.

**Supplementary, not one of the four named oqs but load-bearing for the
`exhaustive` verdict above:** re-running with
`settings.exhaustive.default-signifies-exhaustive: true` over
`./cmd/verdi/...` drops the count from 15 to 7 (command and full output:
`oq3-and-exhaustive-extra.txt`) — every `specstate.State`/`Classification`
finding with an explicit `default:` disappears. The 7 that remain split
**4 test / 3 production** (corrected — this README previously miscounted
all 7 as test fixtures; the file:line list was already right in
`oq3-and-exhaustive-extra.txt`, the prose reading it was not). The three
production sites (`cmd/verdi/readiness_snapshot.go:369`,
`cmd/verdi/stubmatch.go:164`, `cmd/verdi/sync_quarantine.go:157`) are
judged individually, not asserted, in `oq2-exhaustive-survivors.txt`:
`stubmatch.go`'s switch is unambiguously a deliberate filter (the
function's own name, `disqualifyingSupersedesOrExempts`, states its
two-type scope directly); `sync_quarantine.go`'s switch is a deliberate,
currently-correct elimination-by-`continue` pattern over a type
(`gitx.Reachability`) whose doc comments describe it as a closed
three-value enum, with a named latent risk if a fourth value is ever
added; `readiness_snapshot.go`'s switch most likely is a deliberate filter
(selecting only the two annotation kinds that read as readiness-blocking)
but its exclusion of `AnnotationDecisionNeeded` specifically is not
self-evidently justified from the code alone and is left as an open
question, not a ruling. The verdict survives: none of the three is argued
to be a fail-open bug today.

### Retro-witness: 1be75d01 vs 410db101

Scratch worktree `verdi-wt/scratch-lint-w1`, `--detach`ed to each commit in
turn, restored to `--detach e963f4d0` afterward (confirmed clean before and
after). Scope: `./internal/readinessload/... ./cmd/verdi/...
./internal/store/...`. Full transcript: `oq2-retro-witness.txt`.

| commit | gochecknoglobals: `serveReadinessLoader`? | dupl: `hasDotDotElement`? |
|---|---|---|
| 1be75d01 | **yes** (`cmd/verdi/serve.go:142:2: serveReadinessLoader is a global variable`) | **no** — silent at every threshold tried: 150 (default), 30, 20, 15, 10, verified by line-range overlap, not by the broken `grep hasDotDotElement` an earlier version of this evidence used (see below) |
| 410db101 | **yes, still** (`cmd/verdi/serve.go:149:2: ...`) | no (the function no longer exists anywhere in the tree at this commit) |

**Both predictions are violated-with-witness**, for two independent
reasons, neither of which is "the underlying feature premise is wrong":

1. `dupl` never reaches the `hasDotDotElement` duplicate
   (`cmd/verdi/context.go:485-494` / `internal/readinessload/conflict.go:232-241`,
   byte-identical 10-line bodies — the file's own comment at 1be75d01 calls
   it "a small, deliberate duplicate") at any tested threshold from 150
   down to 10. The mechanism is structural, not a length limit: the two
   copies are in *different* packages (`cmd/verdi`'s `package main` vs
   `internal/readinessload`'s `package readinessload`), and golangci-lint
   runs `dupl` per package — see the Recommendation section's R-1 fix for
   the 0/37, 0/13,521 counts proving this generally. **Witness
   correction:** the original evidence for "silent at every threshold"
   was `golangci-lint run ... | grep hasDotDotElement`, but `dupl`'s own
   message text never contains a function name (it prints `` N-M lines
   are duplicate of `file:line-line` ``), so that grep could not have
   discriminated the outcome either way — it happened to print nothing
   (exit 1) for the right underlying reason, but the artifact could not
   prove it. Replaced with a line-range-overlap query
   (`oq2-dupl-overlap-check.py`, run against each threshold's JSON output;
   full transcript `oq2-dupl-overlap-check-output.txt`) that checks
   whether any finding's own range or its cited partner's range overlaps
   either function's true extent — confirmed `NO OVERLAP` at every
   threshold, on both commits.
2. `serveReadinessLoader` is still a global at 410db101 because that
   commit's own fix (confirmed via `git diff 1be75d01 410db101 --
   internal/readinessload/conflict.go`) is real but is the *other* half of
   ac-3's example: both copies of `hasDotDotElement` are gone, replaced by
   a new exported `store.HasDotDotElement` in the shared `internal/store`
   package — the textbook-correct remedy. The commit is titled "harden
   co-2 persistence check, add a hermetic conflict-provider seam" and does
   not touch `serve.go`. `410db101` is also not an ancestor of main
   (`git merge-base --is-ancestor 410db101 5f60c76c` fails; it sits on
   `agent/readiness-recovery-wave-1`, which landed on main by a different
   path). At main (`5f60c76c`), `serveReadinessLoader` no longer appears in
   `cmd/verdi/serve.go` at all — the fix is real, it is just not at this
   SHA.

**oq-2: ANSWERED-WITH-CAVEAT** — per-linter TP fractions and rationale
delivered in full (`samples.md`); the retro-witness's two yes/no answers
are delivered as **violated-with-witness** rather than the story's
predicted yes/no, with the exact tool lines and the reason for each
divergence.

## oq-3 — ratchet mechanism: `--new-from-rev` vs a committed baseline

**Two-run determinism, `--new-from-rev` against the merge base with
`origin/main`** (trivial case: this branch has not diverged from `origin/main`
yet, so the merge base is `HEAD` itself and both runs correctly report zero
new issues, byte-identical — `oq3-newfromrev-run1.txt` / `-run2.txt`,
`cmp` exit 0). This proves same-invocation repeatability but, since there
is nothing to find, does not exercise the mechanism's actual job.

**One deliberate new finding, in a `/tmp` clone** (`git clone -q
--no-hardlinks verdi-wt/spike-strict-lint-target /tmp/spike-lint-ratchet`,
never pushed/merged, deleted after this measurement): added one
`gochecknoglobals`-triggering package-level variable to
`internal/lint/spike_ratchet_trial.go`, committed
(`Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`), then ran
`golangci-lint run --config <strict config, absolute path>
--new-from-rev=$(git merge-base HEAD origin/spike/strict-lint-target) ./...`
twice.

This surfaced a real, reproduced defect, found by accident while chasing
byte-for-byte determinism harder than the trivial case above required:

| invocation path | run A | run B | byte-identical? |
|---|---|---|---|
| `/tmp/spike-lint-ratchet` (symlink: macOS `/tmp` → `/private/tmp`) | `0 issues.` exit 0 | `0 issues.` exit 0 | yes — **both wrong** |
| `/private/tmp/spike-lint-ratchet` (the same directory, resolved path) | exactly 1 issue (`spikeRatchetTrialGlobal is a global variable`), exit 1 | same, exit 1 | yes — **both correct** |

Full output: `oq3-onenew-symlinked-run{A,B}.txt`,
`oq3-onenew-physical-run{A,B}.txt`. A plain (non-`--new-from-rev`) run from
the symlinked path correctly finds the same finding among its full output —
so the bug is specific to `--new-from-rev`'s own diff/path-matching
(`golangci-lint run -v` shows its internal `diff` processor stat go from
`32/32` to `32/0`, i.e. it silently filters out every finding, not just the
new one), not to golangci-lint's file scanning in general.

This is **not** "the mechanism is nondeterministic": given a fixed
invocation path, it is perfectly repeatable. It **is** a real correctness
hazard: the exact same repository state, at the exact same commit, invoked
two path-spellings of the same directory apart, silently disagrees on
whether a real new finding exists — and disagrees on the side that looks
like success (`0 issues`, exit 0), the single worst direction for a gate to
be wrong in. Scope of this finding: reproduced on this machine (macOS,
`/tmp` → `/private/tmp`); **not** verified against the `ubuntu-latest`
runner `.github/workflows/merge-gate.yml` actually uses — disclosed as
unproven whether stock GitHub-hosted runners symlink any path golangci-lint
would see, only that the failure mode is real and silent when it triggers.

Separately: `.github/workflows/merge-gate.yml` (the one workflow the
outcome text calls out) already does
```
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
```
so `--new-from-rev`'s merge-base computation would have the full history it
needs in CI, independent of the symlink issue above.

**Committed-baseline draft.** `baseline.draft.jsonl` (1050 lines, one JSON
object per finding: `{linter, file, line, col, text, source}`, generated
from oq-1's own per-linter JSON, sorted by `linter, file, line, col`) is a
draft of the shape, not the deliverable (the Throwaway rule: "the feature's
own plan authors the real file under test"). `source` (the flagged line's
exact text) is included alongside `line` deliberately: a pure line-number
key breaks on any unrelated edit that shifts line numbers without changing
the finding itself (an inserted blank line above a duplicate block would
make every subsequent baseline entry look "new"); keying on `(linter, file,
text, source)` and treating `line`/`col` as advisory survives that class of
false churn. Regeneration protocol (described, not built — building the
comparison tool is real feature-plan work, out of this spike's scope): a
`make lint-strict-baseline-regen` target re-runs the exact `oq1-raw`-style
command, overwrites the committed baseline, and a CI check (or a second,
simple script) asserts the new baseline is a subset of the old one per
linter — "only shrinks" enforced as a set-containment check on commit, not
by trusting the author. **Collation must be pinned, or the whole file
churns on regeneration for no semantic reason:** verified directly (`jq
sort` vs `sort` on `["designimport_test.go","designimport.go"]`) that this
draft's own sort (`jq`'s `sort_by`, used to build `baseline.draft.jsonl`)
is locale-independent — byte order regardless of `LC_ALL` — while a shell-
level `sort` on the same two strings gives byte order under `LC_ALL=C`
but a different order under `LC_ALL=en_US.UTF-8`. The regeneration
protocol should either keep sorting inside `jq` (as this draft did, for
exactly this reason) or, if any shell-level sort is ever used instead,
pin `LC_ALL=C` for it explicitly — not left to whatever the invoking
shell happens to have set.

**Judgment: committed baseline over `--new-from-rev`.** Both are
deterministic in the narrow byte-for-byte sense. `--new-from-rev` has a
demonstrated silent-false-negative mode tied to golangci-lint's own
git-diff integration; a baseline-diff never invokes that code path at all
(it runs golangci-lint plain — shown above to be correct even from the
symlinked path — and does its own set comparison), so it cannot inherit
this specific bug by construction, at the cost of authoring the
regeneration/shrink-check tooling instead of getting it for free. On how a
killed finding is disclosed: `--new-from-rev` never mentions it (a fixed
finding just stops appearing, indistinguishable from "moved out of scope"
or "the tool broke"); a shrink-checked committed baseline makes a killed
finding visible as a diff in the baseline file itself, at review time, for
free.

**oq-3: ANSWERED-WITH-CAVEAT** — mechanism chosen (committed baseline),
two-run byte comparison done for both the trivial and the constructed case,
one-new-finding result recorded; the caveat is the discovered symlink
defect (real, reproduced, scope-limited to this machine, not the actual CI
runner).

## oq-4 — configuration coexistence

`docs/spikes/strict-lint-target/golangci.strict.yml` copied to the worktree
root as `.golangci.strict.yml` for this test only (removed before every
commit — `git status` below and every commit in this branch shows nothing
at the worktree root). Full verbose logs: `oq4-coexistence.txt`.

| invocation | files present | config file golangci-lint used | linters that ran |
|---|---|---|---|
| `golangci-lint run` (plain) | both `.golangci.yml` and `.golangci.strict.yml` | `.golangci.yml` | 5 (parity: `errcheck govet ineffassign staticcheck unused`) |
| `golangci-lint run --config .golangci.strict.yml` | both | `.golangci.strict.yml` | 7 (strict: `containedctx contextcheck dupl errorlint exhaustive gochecknoglobals noctx`) |
| `golangci-lint run` (plain) | **only** `.golangci.strict.yml` (`.golangci.yml` moved aside) | **neither** — no "Used config file" line at all | 5 (golangci-lint's own built-in default, not the strict file) |

The third row is the direct answer to "is `.golangci.strict.yml` ever
auto-discovered": no. With `.golangci.yml` absent, golangci-lint does not
fall back to `.golangci.strict.yml`; it silently uses its built-in default
linter set (which happens to be the same five linters `.golangci.yml`
names, by coincidence of what "standard" means, not because the file was
read — the log's `[lintersdb] Active 5 linters` line is identical, but
there is no `Used config file` line in this run at all, unlike the other
two). Explicit `--config` is therefore required and sufficient; naming
collision with `.golangci.yml`'s auto-discovery is not possible under any
invocation tried.

Makefile recipe (illustrative — this spike does not edit the real
Makefile; the feature's own plan wires this in for real, per the Throwaway
rule):

```make
# lint-strict gates the ground-rule linter set (spec/strict-lint-target)
# beside the parity `lint` target above — same pinned golangci-lint
# version (GOLANGCI_LINT_VERSION), same CI-mandatory/local-warn split, its
# own config file so .golangci.yml's parity with verdi-go stays untouched
# (co-1). --config is required: golangci-lint v2 only ever auto-discovers
# a file literally named .golangci.yml (or .golangci.yaml/.golangci.json/
# .golangci.toml), never .golangci.strict.yml (spike:
# docs/spikes/strict-lint-target/README.md oq-4) — so the two targets
# cannot collide by accident.
lint-strict:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		have=$$(golangci-lint version 2>/dev/null | grep -oE 'version v?[0-9]+\.[0-9]+\.[0-9]+' | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
		if [ -n "$$have" ] && [ "v$${have#v}" != "$(GOLANGCI_LINT_VERSION)" ]; then \
			echo "warning: golangci-lint $$have differs from CI pin $(GOLANGCI_LINT_VERSION); results may diverge from CI" >&2; \
		fi; \
		golangci-lint run --config .golangci.strict.yml; \
	elif [ "$$CI" = "true" ]; then \
		echo "ERROR: golangci-lint not installed but CI=true — lint-strict is mandatory in CI." >&2; \
		exit 1; \
	else \
		echo "WARNING: golangci-lint not installed locally; skipping lint-strict" >&2; \
	fi
```

**oq-4: ANSWERED** — recipe above; each invocation's loaded file stated
and verified by `golangci-lint run -v`'s own `[config_reader] Used config
file ...` line (or its absence, in the third row).

## Deviations from the investigation plan

- The story's step 2 says to run the strict config at 1be75d01/410db101
  "restricted to `./internal/readinessload/ ./cmd/verdi/ ./internal/store/`"
  (space-separated, no `/...` suffix). Run literally, that glob pattern
  matches only files directly in those three directories, not their
  subpackages; this spike used `./internal/readinessload/...
  ./cmd/verdi/... ./internal/store/...` (with `/...`) since all the named
  evidence (`serveReadinessLoader` in `cmd/verdi/serve.go`,
  `hasDotDotElement` in `internal/readinessload/conflict.go`) is directly
  in those three package directories either way, so the substitution does
  not change the result, but is recorded per the discrepancy rule.
- oq-3's investigation plan step names only `--new-from-rev`'s two-run
  comparison and the one-new-finding trial; it does not anticipate a
  symlinked-path invocation. That variant was added because the trivial
  merge-base-equals-HEAD case (true at this branch's current state) proves
  nothing about the mechanism's actual new-issue detection, and the
  standard-path one-new-finding trial alone would have missed the defect
  reported above. Recorded as an addition, not a substitution — the
  plan's literal steps were all still run as specified, first.
- The parent feature spec's problem statement claims golangci-lint offers
  "114 ... linters." Counted precisely from `help-linters.txt` (114 raw
  lines = 2 section headers + 1 blank line + **111** actual linter
  entries: 5 enabled-by-default + 106 disabled-by-default, verified by a
  small parse rather than a raw line count, since the raw line count of
  114 is what produces the wrong number if the two headers and the blank
  line are not subtracted) golangci-lint `2.5.0` offers **111** linters,
  not 114. Recorded as a discrepancy in the parent spec's context material
  rather than silently repeated — it does not change oq-1's answer (the
  seven candidates and the three additional matches are all present and
  correctly counted either way), but the 114 figure should not be
  propagated into the feature's own spec without correction.

## Evidence index (all under this fence)

`golangci.strict.yml` (the drafted strict config, throwaway per the
story's own rule), `run-oq1.sh` (re-runnable), `help-linters.txt`,
`oq1-raw/` (per-linter JSON+text, `--show-stats` cross-checked),
`oq1-summary.txt` (load-annotated), `oq2-samples-raw/` (first-ten sorted
extracts feeding `samples.md`), `samples.md` (full oq-2 tables),
`oq2-retro-witness.txt`, `baseline.draft.jsonl`, `oq3-newfromrev-run{1,2}.txt`,
`oq3-onenew-{symlinked,physical}-run{A,B}.txt`,
`oq3-and-exhaustive-extra.txt`, `oq4-coexistence.txt`,
`oq1-summary-heavyload.txt` (the first oq-1 run; `oq1-summary.txt` itself
holds the second, lower-load run — see oq-1's timing note above).

**Added in fix round 1** (review findings R-1 through R-7):
`run-oq1-combined.sh` (re-runnable, cold-cache shape),
`oq1-combined-cold.txt` + `oq1-combined-summary.txt` (R-2: combined-run
timing across cold/warm/shared caches), `oq2-dupl-package-scope.py` +
`-output.txt` (R-1: the per-package mechanism, verified over all 37
findings), `oq2-dupl-overlap-check.py` + `-output.txt` (R-4: the
line-range-overlap witness replacing the broken `grep`),
`oq2-exhaustive-survivors.txt` (R-3: the 4-test/3-production split, each
production site judged individually), `oq2-errorlint-settings.txt` (R-5:
the errorlint settings remedy, and contextcheck disclosed as unmeasured),
`oq3-baseline-locale-check.txt` (R-7: the collation verification).
