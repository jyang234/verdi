package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/ritualwitness"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// The producer of obligation ritual-effect-witness--ac-2--behavioral
// (spec/ritual-effect-witness ac-2, dc-1, dc-3; ledger SI-341, SI-344,
// SI-347, SI-348, SI-349, SI-351, SI-354). Its table is keyed by registry verb: each case is one
// publication path of that verb's ritual, run through the ritual's real
// entry point (the built binary, the workbench handler on loopback, or
// the MCP server in process) once per seed state, and judged against
// exactly the declaration internal/writescope holds for it.

// Publication paths. A path is a registry verb on one of its dispatch
// branches (SI-341 (4)): the root routes, and beneath /b/ a managed
// worktree's first use, an existing managed worktree, the branch checked
// out at the serving root, and the branch held by another worktree
// (SI-347, SI-354 (5)). dc-3's named paths, which no registry field
// records (SI-341 (7)), are pinned in namedPaths.
const (
	pathRoot      = "root"
	pathBFirstUse = "b-first-use"
	pathBExisting = "b-existing"
	pathBHere     = "b-here"
	// pathBElsewhere is the /b/ branch checked out in another worktree,
	// which every /b/ route refuses before any mutation (ledger SI-347).
	// scaffold_branch's three /b/ actions are declared scoped, so their
	// refusal reads as SI-348 (2)'s one named verdict, which their cases
	// allow (SI-351 (2)).
	pathBElsewhere = "b-elsewhere"
)

// pristineSuffix names a whole-tree guard's extra completion case, run
// over a pristine tree (SI-341 (2)).
const pristineSuffix = "-pristine"

// namedPaths pins, per verb, the publication paths dc-3 and the ledger
// name beyond a verb's dispatch branches (SI-341 (7)): close's four, the
// two design start paths, stub instantiation's CLI path, the propose
// paths, the whole-tree guards' completion cases (SI-341 (2), SI-349
// (2)), the two gc and recover paths, the execution rituals' refusals
// (SI-348 (1), SI-349 (3)). A path below a verb cannot register itself
// (SI-341 (7)), so this list is the disclosed seam where a new one must be
// added.
func namedPaths() map[ws.Verb][]string {
	return map[ws.Verb][]string{
		ws.CLI("close"):                                              {"ci", "force-local", "feature", "unwind"},
		ws.CLI("design start"):                                       {"plain", "supersedes"},
		ws.CLI("design start --from-stub"):                           {"cli"},
		ws.CLI("context constitution propose"):                       {"new-branch", "existing-here", "existing-elsewhere", "existing-elsewhere" + pristineSuffix},
		ws.CLI("context execution"):                                  {"compile-gate", "hand-back" + pristineSuffix},
		ws.CLI("gc"):                                                 {"managed", "reclaim-unmanaged"},
		ws.CLI("recover"):                                            {"unwind", "unwind" + pristineSuffix, "reclaim"},
		ws.CLI("experiment start"):                                   {"input-binding-refusal"},
		ws.CLI("experiment resume"):                                  {"input-binding-refusal"},
		ws.MCP("experiment"):                                         {"input-binding-refusal"},
		ws.CLI("design import apply"):                                {pathRoot, pathRoot + pristineSuffix},
		ws.Workbench("/design/import/apply"):                         {pathRoot, pathRoot + pristineSuffix},
		ws.MCP("import_apply"):                                       {pathRoot, pathRoot + pristineSuffix},
		ws.Workbench("/board/spec/{name}/api/git-switch"):            {pathRoot, pathRoot + pristineSuffix},
		ws.Workbench("/b/{branch}/board/spec/{name}/api/git-switch"): {pathBHere + pristineSuffix},
	}
}

// requiredPaths is every publication path verb must have a case for: its
// dispatch branches (SI-341 (4)), the held-elsewhere one included for
// every /b/ verb (SI-354 (5)), and its named paths.
func requiredPaths(verb ws.Verb) []string {
	var paths []string
	if verb.Surface == ws.SurfaceWorkbench {
		if strings.HasPrefix(verb.Name, "/b/") {
			paths = append(paths, pathBFirstUse, pathBExisting, pathBHere, pathBElsewhere)
		} else {
			paths = append(paths, pathRoot)
		}
	}
	for _, p := range namedPaths()[verb] {
		if !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		paths = []string{pathRoot}
	}
	return paths
}

// ritualRun is what one run of a case must show. A completion (exit 0)
// may have only effects within its declaration. A refusal exits non-zero
// with each of its own words in the driver's error (SI-334 (4)), leaves no
// git mutation and creates no commit object. scopedRefusal is SI-348 (2)'s
// allowance, named per case: a whole-tree guard refusing under a scoped
// declaration reads as exactly one outside verdict, the index carry's.
// unprovenCompletion, when set, names the gap a refusal stands in for: the
// ritual is declared to complete, and SI-348 (1) or SI-349 (1) has its row
// assert this refusal instead, so the producer abstains while it runs
// (SI-354 (3)).
type ritualRun struct {
	exit               int
	words              []string
	scopedRefusal      bool
	unprovenCompletion string
}

// completes is the expectation of a ritual run to completion.
func completes() ritualRun { return ritualRun{} }

// refuses is the expectation of a refusal with exit and every one of
// words.
func refuses(exit int, words ...string) ritualRun { return ritualRun{exit: exit, words: words} }

// allowingScopedRefusal is r with SI-348 (2)'s named allowance.
func allowingScopedRefusal(r ritualRun) ritualRun {
	r.scopedRefusal = true
	return r
}

// standingInForCompletion is the refusal r asserted where the ritual is
// declared to complete, naming the gap (SI-348 (1), SI-349 (1), SI-354
// (3)).
func standingInForCompletion(r ritualRun, gap string) ritualRun {
	r.unprovenCompletion = gap
	return r
}

// ritualCase is one publication path of a registry verb. base is the
// store the fixture's seed commit carries; seed adds post-build state with
// plain git under the fixture's isolated configuration; driver builds the
// run's driver once the fixture is seeded; want gives the expectation per
// state. pristine runs the case once, over a pristine tree (the SeedClean
// fixture with the operator's work removed): a whole-tree guard's
// completion (SI-341 (2)). fixture, when set, replaces BuildWith for a
// ritual whose fixture is not a plain store (context execution).
type ritualCase struct {
	path     string
	ritual   string
	base     map[string]string
	pristine bool
	fixture  func(t *testing.T, ctx context.Context, state ritualwitness.SeedState) *ritualwitness.Fixture
	seed     func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture)
	driver   func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) ritualwitness.Driver
	want     func(state ritualwitness.SeedState) ritualRun
	// disclosure, when set, is logged with every run: what the case does
	// not prove (SI-348 (1)).
	disclosure string
}

// states is the seed states a case runs in.
func (c ritualCase) states() []ritualwitness.SeedState {
	if c.pristine {
		return []ritualwitness.SeedState{ritualwitness.SeedClean}
	}
	return []ritualwitness.SeedState{ritualwitness.SeedFull, ritualwitness.SeedClean}
}

// TestRitualEffects_EveryDeclaredRitual is the producer of obligation
// ritual-effect-witness--ac-2--behavioral: every ritual in
// internal/writescope's registry, run through its real entry point on
// each publication path in both seeded states, has every observed effect
// within its declaration, and completes in the clean-index state when
// declared to (spec/ritual-effect-witness ac-2).
//
// The sequential top level requires the awaiting-fix list to be empty;
// every (declaration, verb) of the registry to have a case naming that
// declaration's ritual; every table key to be a registry verb; and every
// verb's dispatch branches and named paths to have a case (dc-3; SI-341
// (4), (7); SI-354 (5)). The table's runs are shared with
// TestForbiddenTokens_EveryVerbCommandLog (forEachRitualEffectsRun; ledger
// SI-359 (8)): whichever test runs first runs them, pinning for the whole
// table ambient git configuration isolation, the CI environment (PinCIEnv:
// an in-process driver reads the test process's own environment, SI-344
// (2)), and a process-wide VERDI_GITLOG, so the cases can run in parallel
// without touching the process environment.
//
// Every run carries its git command log (spec/gitx-recorder-seam ac-2):
// the built binary's from the VERDI_GITLOG file it appends to, and the
// workbench's and MCP server's from the gitx.Observer the in-process
// drivers attach through the servers' contexts (ledger SI-359 (1)). Each
// in-process run's log is compared with the process-wide VERDI_GITLOG
// record for record, so no context root dropped a call (SI-359 (7)), and
// any difference fails the run.
//
// Each run fails on any outside verdict, except the one SI-348 (2) names
// for a whole-tree guard's refusal under a scoped declaration; requires
// the exit and, for a refusal, the refusal's own words, with no git
// mutation remaining and no commit created; and fails on any
// unattributable verdict when its log was supplied, which is every run
// (SI-341 (1) as SI-359 (9) reads it: the no-log tolerance has ended).
// Every verdict is logged (ritualRunViolations is the law), and so are the
// per-driver counts of runs, logged calls, and verdicts.
//
// The whole-tree guards (SI-341 (2), SI-349 (2)) — board switch,
// constitution propose onto a branch HEAD is elsewhere from, context
// execution (at its compile gate, SI-348 (5)), spec import on every
// surface, and recover's unwind — refuse in both seeded states with
// nothing remaining, and complete in a case over a pristine tree driven
// through the same surface. A scoped guard's exit-2 refusal carries SI-348
// (2)'s allowance, named in its case. Spec import's CLI refusal exits 1
// where its workbench and MCP refusals exit 2 for the same refusal
// (backlog BL-156), and each case asserts its own surface's exit.
//
// Disclosed, not driven (SI-341 (4)): the sealed origin-only /b/ refusal,
// and close's non-unwinding failure paths. Disclosed, not judged (backlog
// BL-171): a worktree-own ref created in a worktree the ritual added. Each
// of the three is a named gap of the abstain decision (undrivenGaps, ledger
// SI-367 (4)). Disclosed as unproven, each row
// asserting its refusal in its own words with nothing remaining and never
// reading as completed: close's completion and so its failure unwind,
// since every hermetic close refuses at the closure gate's countersign
// condition (SI-349 (1), backlog BL-44); and the execution rituals'
// completion (SI-348 (1), SI-349 (3), SI-351 (1), backlog BL-155), whose
// rows assert a mismatched input binding's refusal in the binding slot's
// own words, the earliest refusal before any effect common to every
// platform. Disclosed beside the log (spec/gitx-recorder-seam): gitx
// records a call before it runs, so a failed call can still be credited
// with an effect made outside gitx (SI-325 (8)); a call that names no
// paths is credited per worktree (SI-329 (8′)); and git a child program
// runs itself, or a verdi process the binary starts, is outside the log
// (SI-359 (4b), (15)).
//
// So the producer abstains (SI-354 (3)). It runs and asserts every case,
// and any failure fails it; then, while the run itself shows a disclosed
// gap — a refusal standing in for a completion the ritual is declared to
// make, or a run whose driver failed to supply its command log — or a path
// the table does not drive or the witness does not judge stays open
// (SI-367 (4)), it ends with t.Skip naming each gap, which the go-test
// producer reads as abstain (testproducer.go's verdictForOutcome), never
// pass.
func TestRitualEffects_EveryDeclaredRitual(t *testing.T) {
	if awaiting := ws.AwaitingFixes(); len(awaiting) != 0 {
		t.Fatalf("the registry's awaiting-fix list holds %d path(s), want none: %+v", len(awaiting), awaiting)
	}
	bin := buildVerdiBinary(t)
	table := ritualEffectsTable(t, bin)
	if err := checkRitualCoverage(ws.Registry(), table); err != nil {
		t.Fatal(err)
	}

	var gaps ritualEffectsGaps
	var counts ritualEffectsCounts
	forEachRitualEffectsRun(t, bin, table, func(t *testing.T, c ritualCase, decl ws.Declaration, state ritualwitness.SeedState, run ritualEffectsRun) {
		if c.disclosure != "" {
			t.Logf("disclosed: %s", c.disclosure)
		}
		want := c.want(state)
		res := run.result()
		gaps.record(want, res)
		counts.record(run)
		logVerdicts(t, res)
		judgeRitualRun(t, decl, want, res)
		for _, g := range run.LogGaps {
			t.Error(g)
		}
	})
	for _, line := range counts.lines() {
		t.Log(line)
	}
	if open := gaps.open(); len(open) > 0 {
		t.Skipf("ac-2 abstains while its disclosed gaps are open (ledger SI-354 (3), SI-367 (4)); every case ran and was asserted, and none of the gaps is a pass: %s", strings.Join(open, "; "))
	}
}

// ritualEffectsCounts tallies the table's runs per driver kind: runs, runs
// whose log was supplied, logged calls, in-process records compared with
// the process-wide VERDI_GITLOG, and verdicts by status. Its zero value is
// ready; the parallel cases record into it concurrently.
type ritualEffectsCounts struct {
	mu       sync.Mutex
	byDriver map[string]*driverCounts
}

// driverCounts is one driver kind's tally.
type driverCounts struct {
	runs, logged, calls, compared int
	verdicts                      map[ritualwitness.Status]int
}

// record notes one run.
func (c *ritualEffectsCounts) record(run ritualEffectsRun) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byDriver == nil {
		c.byDriver = map[string]*driverCounts{}
	}
	d := c.byDriver[run.Driver]
	if d == nil {
		d = &driverCounts{verdicts: map[ritualwitness.Status]int{}}
		c.byDriver[run.Driver] = d
	}
	d.runs++
	if run.Log.OK {
		d.logged++
	}
	d.calls += len(run.Log.Calls)
	d.compared += run.Compared
	for _, v := range run.Verdicts {
		d.verdicts[v.Status]++
	}
}

// lines is the tally, one line per driver kind, sorted, then the total.
func (c *ritualEffectsCounts) lines() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var kinds []string
	for k := range c.byDriver {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	var out []string
	total := driverCounts{verdicts: map[ritualwitness.Status]int{}}
	line := func(kind string, d *driverCounts) string {
		return fmt.Sprintf("driver %s: %d run(s), %d with a command log, %d logged call(s), %d in-process record(s) compared with the process-wide VERDI_GITLOG; verdicts: %d within, %d unattributable, %d outside",
			kind, d.runs, d.logged, d.calls, d.compared, d.verdicts[ritualwitness.Within], d.verdicts[ritualwitness.Unattributable], d.verdicts[ritualwitness.Outside])
	}
	for _, k := range kinds {
		d := c.byDriver[k]
		out = append(out, line(k, d))
		total.runs += d.runs
		total.logged += d.logged
		total.calls += d.calls
		total.compared += d.compared
		for s, n := range d.verdicts {
			total.verdicts[s] += n
		}
	}
	return append(out, line("(all)", &total))
}

// The gaps the producer's run can show (ledger SI-354 (3)).
const (
	// noCommandLogGap is a run whose driver failed to supply its command
	// log (SI-341 (1), SI-354 (3)), or no run at all.
	noCommandLogGap = "a run without its command log: an effect made and undone within that run is unobserved, and no effect it made is attributed (ledger SI-341 (1), SI-354 (3); spec/gitx-recorder-seam ac-2)"
	// closeCompletionGap is close's completion (SI-349 (1)).
	closeCompletionGap = "close's completion: every hermetic close refuses at the closure gate's countersign condition (ledger SI-349 (1); backlog BL-44)"
	// executionCompletionGap is the execution rituals' completion
	// (SI-348 (1), SI-351 (1)).
	executionCompletionGap = "the execution rituals' completion: their rows assert a mismatched input binding's refusal (ledger SI-348 (1), SI-351 (1); backlog BL-155)"
)

// The gaps no run can close, because the table does not drive or the
// witness does not judge the path (ledger SI-367 (4)): each stays open
// until a row drives it or the witness judges it, so the producer cannot
// pass once BL-44 and BL-155 lift while any remains (story dc-3: an
// undriven path never passes).
const (
	// sealedOriginOnlyGap is the /b/ dispatch's sealed refusal for a
	// branch that resolves only to a remote-tracking ref (SI-341 (4)).
	sealedOriginOnlyGap = "the sealed origin-only /b/ refusal: every route but the page and fragment of a branch that resolves only to a remote-tracking ref refuses with 403 (internal/workbench/branchboard.go, serveSealed), and no row drives it, so its effects are unobserved (ledger SI-341 (4))"
	// closeNonUnwindingGap is close's failure paths after the freeze,
	// designed without rollback (SI-341 (4)).
	closeNonUnwindingGap = "close's non-unwinding failure paths: a failure after the freeze, in writeRollup, flipSpecStatusToClosed or store.ArchiveMove, leaves its in-place, uncommitted state by design and does not unwind (cmd/verdi/close.go), and no row drives one, so their effects are unobserved (ledger SI-341 (4))"
	// worktreeOwnRefsGap is a worktree-own ref created in a worktree the
	// ritual added, which gets no verdict (BL-171).
	worktreeOwnRefsGap = "a worktree-own ref (refs/worktree/*, refs/bisect/*) created in a worktree the ritual added: the witness judges only that the worktree was added (internal/ritualwitness/evaluate_worktrees.go, worktrees), so such a change gets no verdict (backlog BL-171; ledger SI-359 (16))"
)

// undrivenGaps is every gap no run can close, in a fixed order. A fixed
// table, so a function.
func undrivenGaps() []string {
	return []string{sealedOriginOnlyGap, closeNonUnwindingGap, worktreeOwnRefsGap}
}

// ritualEffectsGaps gathers, from every run of the producer, the disclosed
// gaps that run shows (ledger SI-354 (3)). Its zero value is ready; the
// parallel cases record into it concurrently.
type ritualEffectsGaps struct {
	mu       sync.Mutex
	runs     int
	unlogged int
	unproven map[string]bool
}

// record notes one run: whether its driver supplied the command log, and
// the gap its expectation's refusal stands in for, if any.
func (g *ritualEffectsGaps) record(want ritualRun, res ritualwitness.Result) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.runs++
	if !res.Log.OK {
		g.unlogged++
	}
	if want.unprovenCompletion != "" {
		if g.unproven == nil {
			g.unproven = map[string]bool{}
		}
		g.unproven[want.unprovenCompletion] = true
	}
}

// open is the abstain decision: every gap the recorded runs show, the
// missing command log first and then each completion a refusal stood in
// for, sorted, and then every gap no run can close (undrivenGaps, ledger
// SI-367 (4)), in its fixed order. The command-log gap stays open while any
// run (or, with no run at all, every run) lacked the log: an unlogged run's
// effects are unobserved whatever another run supplied. None open means
// the producer may pass, which the undriven gaps rule out until a row
// drives each path or the witness judges it.
func (g *ritualEffectsGaps) open() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []string
	if g.runs == 0 || g.unlogged > 0 {
		out = append(out, noCommandLogGap)
	}
	var unproven []string
	for gap := range g.unproven {
		unproven = append(unproven, gap)
	}
	sort.Strings(unproven)
	return append(append(out, unproven...), undrivenGaps()...)
}

// sortedVerbs returns table's keys in a stable order.
func sortedVerbs(table map[ws.Verb][]ritualCase) []ws.Verb {
	verbs := make([]ws.Verb, 0, len(table))
	for v := range table {
		verbs = append(verbs, v)
	}
	sort.Slice(verbs, func(i, j int) bool { return verbs[i].String() < verbs[j].String() })
	return verbs
}

// checkRitualCoverage is the table's coverage law: every registry
// (declaration, verb) has a case naming the declaration's ritual, every
// key is a registry verb whose every case names its declaration, no path
// repeats under a verb, and every required path (requiredPaths) has a
// case.
func checkRitualCoverage(decls []ws.Declaration, table map[ws.Verb][]ritualCase) error {
	owner := map[ws.Verb]string{}
	for _, d := range decls {
		for _, v := range d.Verbs {
			owner[v] = d.Ritual
		}
	}
	var problems []string
	for _, v := range sortedVerbs(table) {
		ritual, ok := owner[v]
		if !ok {
			problems = append(problems, fmt.Sprintf("table key %s is no registry verb (a stale case)", v))
			continue
		}
		seen := map[string]bool{}
		for _, c := range table[v] {
			if c.ritual != ritual {
				problems = append(problems, fmt.Sprintf("%s's case %q names ritual %s, but the registry declares %s for it", v, c.path, c.ritual, ritual))
			}
			if seen[c.path] {
				problems = append(problems, fmt.Sprintf("%s has two cases for path %q", v, c.path))
			}
			seen[c.path] = true
		}
	}
	for _, d := range decls {
		for _, v := range d.Verbs {
			cases := table[v]
			if len(cases) == 0 {
				problems = append(problems, fmt.Sprintf("registry verb %s (ritual %s) has no case", v, d.Ritual))
				continue
			}
			have := map[string]bool{}
			for _, c := range cases {
				have[c.path] = true
			}
			for _, p := range requiredPaths(v) {
				if !have[p] {
					problems = append(problems, fmt.Sprintf("registry verb %s (ritual %s) has no case for publication path %q", v, d.Ritual, p))
				}
			}
		}
	}
	for v := range namedPaths() {
		if _, ok := owner[v]; !ok {
			problems = append(problems, fmt.Sprintf("named paths are pinned for %s, which is no registry verb", v))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("ritual effect coverage (spec/ritual-effect-witness dc-3; ledger SI-341 (4), (7)):\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

// makePristine removes the operator's work every seeded state carries
// (the dirty tracked file, the untracked file, and SeedFull's staged
// foreign entry), leaving a pristine tree for a whole-tree guard's
// completion case (SI-341 (2)).
func makePristine(t *testing.T, fx *ritualwitness.Fixture) {
	t.Helper()
	if fx.State == ritualwitness.SeedFull {
		gitTestOutput(t, fx.Dir, "rm", "-q", "--cached", "--", ritualwitness.ForeignFile)
		if err := os.Remove(filepath.Join(fx.Dir, ritualwitness.ForeignFile)); err != nil {
			t.Fatal(err)
		}
	}
	gitTestOutput(t, fx.Dir, "checkout", "-q", "--", ritualwitness.TrackedFile)
	if err := os.Remove(filepath.Join(fx.Dir, ritualwitness.UntrackedFile)); err != nil {
		t.Fatal(err)
	}
	if out := gitTestOutput(t, fx.Dir, "status", "--porcelain", "--untracked-files=all"); out != "" {
		t.Fatalf("the pristine tree is not clean:\n%s", out)
	}
}

// scopedRefusalVerdict is the one verdict SI-348 (2) allows a named
// whole-tree guard's refusal under a scoped declaration (SI-329 (3′)).
func scopedRefusalVerdict() ritualwitness.Verdict {
	return ritualwitness.Verdict{Field: "index_carry", Status: ritualwitness.Outside, Detail: "declares scoped; observed refused"}
}

// noCommandLogDisclosure is logged with every run whose driver failed to
// supply its command log (SI-341 (1), SI-354 (4)), with the reason.
const noCommandLogDisclosure = "this run's driver supplied no command log, so its observation is unproven: an effect made and undone within the run is unobserved, and no effect it made is attributed (ledger SI-341 (1))"

// judgeRitualRun logs res's verdict counts, the disclosure a run without a
// command log carries, and the ritual's answer, and fails t with each of
// ritualRunViolations.
func judgeRitualRun(t *testing.T, decl ws.Declaration, want ritualRun, res ritualwitness.Result) {
	t.Helper()
	counts := verdictCounts(res)
	t.Logf("verdict counts: %d within, %d unattributable, %d outside; command log supplied: %t (%d call(s))",
		counts[ritualwitness.Within], counts[ritualwitness.Unattributable], counts[ritualwitness.Outside], res.Log.OK, len(res.Log.Calls))
	if !res.Log.OK {
		t.Logf("disclosed: %s: %s", noCommandLogDisclosure, res.Log.Reason)
	}
	if res.Err != nil {
		t.Logf("the ritual answered: %v", res.Err)
	}
	for _, v := range ritualRunViolations(decl, want, res) {
		t.Error(v)
	}
}

// verdictCounts is res's verdicts counted by status.
func verdictCounts(res ritualwitness.Result) map[ritualwitness.Status]int {
	counts := map[ritualwitness.Status]int{}
	for _, v := range res.Verdicts {
		counts[v.Status]++
	}
	return counts
}

// ritualRunViolations is the per-run law (the test's doc) applied to res:
// every way the run breaks it, none when it conforms. A wrong exit, a
// completion that answered an error, a refusal missing one of its words,
// and a misnamed allowance each end the reading, as nothing after them is
// meaningful.
func ritualRunViolations(decl ws.Declaration, want ritualRun, res ritualwitness.Result) []string {
	if res.Exit != want.exit {
		return []string{fmt.Sprintf("%s exited %d, want %d: %v", decl.Ritual, res.Exit, want.exit, res.Err)}
	}
	var out []string
	if want.exit == 0 {
		if res.Err != nil {
			return []string{fmt.Sprintf("%s completed with an error: %v", decl.Ritual, res.Err)}
		}
		if want.unprovenCompletion != "" {
			return []string{fmt.Sprintf("the case names gap %q, but only a refusal stands in for a completion (SI-354 (3))", want.unprovenCompletion)}
		}
	} else {
		if len(want.words) == 0 {
			return []string{fmt.Sprintf("the case expects a refusal (exit %d) but names none of its words (SI-334 (4))", want.exit)}
		}
		for _, w := range want.words {
			if w == "" || res.Err == nil || !strings.Contains(res.Err.Error(), w) {
				return []string{fmt.Sprintf("%s's refusal = %v, want its own words %q", decl.Ritual, res.Err, w)}
			}
		}
		out = append(out, nothingRemainsViolations(res)...)
	}

	var outside []ritualwitness.Verdict
	for _, v := range res.Verdicts {
		if v.Status == ritualwitness.Outside {
			outside = append(outside, v)
		}
	}
	switch {
	case want.scopedRefusal:
		if decl.IndexCarry != ws.CarryScoped || want.exit != 2 {
			return append(out, fmt.Sprintf("SI-348 (2)'s allowance applies only to a scoped declaration's exit-2 refusal; %s declares %s and the case expects exit %d", decl.Ritual, decl.IndexCarry, want.exit))
		}
		if len(outside) != 1 || outside[0] != scopedRefusalVerdict() {
			out = append(out, fmt.Sprintf("outside verdicts = %v, want exactly %v (SI-348 (2))", outside, scopedRefusalVerdict()))
		}
	case len(outside) > 0:
		out = append(out, fmt.Sprintf("%d effect(s) outside %s's declaration: %v", len(outside), decl.Ritual, outside))
	}

	if n := verdictCounts(res)[ritualwitness.Unattributable]; n > 0 && res.Log.OK {
		out = append(out, fmt.Sprintf("%d unattributable verdict(s) with a command log supplied: the no-log tolerance ends once the log exists (SI-341 (1), SI-359 (9))", n))
	}
	return out
}

// nothingRemainsViolations is every git mutation the run left and every
// commit object it created: a refusal is a guard before any mutation
// (SI-325 (3), SI-329 (3′), SI-348 (2)), so it leaves none.
func nothingRemainsViolations(res ritualwitness.Result) []string {
	var out []string
	for _, c := range []struct {
		what          string
		before, after any
	}{
		{"refs", res.Before.Refs, res.After.Refs},
		{"the remote's refs", res.Before.RemoteRefs, res.After.RemoteRefs},
		{"the remote's HEAD", res.Before.RemoteHead, res.After.RemoteHead},
		{"HEAD", res.Before.Head, res.After.Head},
		{"the index", res.Before.Index, res.After.Index},
		{"linked worktrees", res.Before.Worktrees, res.After.Worktrees},
		{"the configuration", res.Before.Config, res.After.Config},
		{"hooks and info", res.Before.GitFiles, res.After.GitFiles},
	} {
		if !reflect.DeepEqual(c.before, c.after) {
			out = append(out, fmt.Sprintf("the refusal left %s changed:\nbefore %+v\nafter  %+v", c.what, c.before, c.after))
		}
	}
	if created := createdCommits(res); len(created) != 0 {
		out = append(out, fmt.Sprintf("the refusal created commits %v", created))
	}
	return out
}

// binaryOutput runs the built binary in dir for a case's seeding (a
// read-only preview or recovery listing), returning its stdout and
// failing the test unless it exits wantExit. It inherits the test
// process's pinned environment.
func binaryOutput(t *testing.T, ctx context.Context, bin, dir string, wantExit int, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("verdi %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	if code != wantExit {
		t.Fatalf("verdi %v exited %d, want %d\nstdout: %s\nstderr: %s", args, code, wantExit, stdout.String(), stderr.String())
	}
	return stdout.String()
}
