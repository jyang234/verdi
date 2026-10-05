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
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/ritualwitness"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// The producer of obligation ritual-effect-witness--ac-2--behavioral
// (spec/ritual-effect-witness ac-2, dc-1, dc-3; ledger SI-341, SI-344,
// SI-347, SI-348). Its table is keyed by registry verb: each case is one
// publication path of that verb's ritual, run through the ritual's real
// entry point (the built binary, the workbench handler on loopback, or
// the MCP server in process) once per seed state, and judged against
// exactly the declaration internal/writescope holds for it.

// Publication paths. A path is a registry verb on one of its dispatch
// branches (SI-341 (4)): the root routes, and beneath /b/ a managed
// worktree's first use, an existing managed worktree, and the branch
// checked out at the serving root. dc-3's named paths, which no registry
// field records (SI-341 (7)), are pinned in namedPaths.
const (
	pathRoot      = "root"
	pathBFirstUse = "b-first-use"
	pathBExisting = "b-existing"
	pathBHere     = "b-here"
)

// pristineSuffix names a whole-tree guard's extra completion case, run
// over a pristine tree (SI-341 (2)).
const pristineSuffix = "-pristine"

// namedPaths pins, per verb, the publication paths dc-3 and the ledger
// name beyond a verb's dispatch branches (SI-341 (7)): close's four, the
// two design start paths, stub instantiation's CLI path, the propose
// paths, the whole-tree guards' completion cases (SI-341 (2)), the two
// gc and recover paths, and the execution rituals' refusals (SI-348 (1)).
// A path below a verb cannot register itself (SI-341 (7)), so this list is
// the disclosed seam where a new one must be added.
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
// dispatch branches (SI-341 (4)) and its named paths.
func requiredPaths(verb ws.Verb) []string {
	var paths []string
	if verb.Surface == ws.SurfaceWorkbench {
		if strings.HasPrefix(verb.Name, "/b/") {
			paths = append(paths, pathBFirstUse, pathBExisting, pathBHere)
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
type ritualRun struct {
	exit          int
	words         []string
	scopedRefusal bool
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
// (4), (7)). It then pins, for the whole run, ambient git configuration
// isolation and the CI environment (PinCIEnv: an in-process driver reads
// the test process's own environment, SI-344 (2)), so the cases can run
// in parallel without touching the process environment.
//
// Each run fails on any outside verdict, except the one SI-348 (2) names
// for a whole-tree guard's refusal under a scoped declaration; requires
// the exit and, for a refusal, the refusal's own words, with no git
// mutation remaining and no commit created; and tolerates an
// unattributable verdict only while the driver supplies no command log
// (SI-341 (1)), logging the counts, so the test tightens on its own once
// spec/gitx-recorder-seam lands the log. Every verdict is logged.
//
// The whole-tree guards (SI-341 (2), SI-349 (2)) — board switch,
// constitution propose onto a branch HEAD is elsewhere from, context
// execution (at its compile gate, SI-348 (5)), spec import on every
// surface, and recover's unwind — refuse in both seeded states with
// nothing remaining, and complete in a case over a pristine tree driven
// through the same surface. A scoped guard's exit-2 refusal carries SI-348
// (2)'s allowance, named in its case.
//
// Disclosed, not driven (SI-341 (4)): the sealed origin-only /b/ refusal,
// and close's non-unwinding failure paths. Disclosed as unproven, each row
// asserting its refusal in its own words with nothing remaining and never
// reading as completed: close's completion and so its failure unwind,
// since every hermetic close refuses at the closure gate's countersign
// condition (SI-349 (1), backlog BL-44); and the execution rituals'
// completion (SI-348 (1), SI-349 (3), backlog BL-155), whose rows assert a
// mismatched input binding's refusal, the earliest refusal before any
// effect common to every platform. Every effect's attribution is
// disclosed as unproven until the command log lands (SI-341 (1)).
func TestRitualEffects_EveryDeclaredRitual(t *testing.T) {
	if awaiting := ws.AwaitingFixes(); len(awaiting) != 0 {
		t.Fatalf("the registry's awaiting-fix list holds %d path(s), want none: %+v", len(awaiting), awaiting)
	}
	bin := buildVerdiBinary(t)
	table := ritualEffectsTable(t, bin)
	if err := checkRitualCoverage(ws.Registry(), table); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ritualwitness.PinCIEnv(t, ritualwitness.CIEnv{})

	decls := map[string]ws.Declaration{}
	for _, d := range ws.Registry() {
		decls[d.Ritual] = d
	}
	for _, verb := range sortedVerbs(table) {
		for _, c := range table[verb] {
			decl := decls[c.ritual]
			t.Run(c.ritual+" "+c.path+" "+verb.String(), func(t *testing.T) {
				t.Parallel()
				for _, state := range c.states() {
					t.Run(state.String(), func(t *testing.T) {
						runRitualCase(t, c, decl, state)
					})
				}
			})
		}
	}
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

// ritualCaseTimeout bounds one run, fixture to judgment.
const ritualCaseTimeout = 120 * time.Second

// runRitualCase builds, seeds, and runs one case in one state, and judges
// the run (judgeRitualRun).
func runRitualCase(t *testing.T, c ritualCase, decl ws.Declaration, state ritualwitness.SeedState) {
	ctx, cancel := context.WithTimeout(context.Background(), ritualCaseTimeout)
	defer cancel()
	var fx *ritualwitness.Fixture
	if c.fixture != nil {
		fx = c.fixture(t, ctx, state)
	} else {
		fx = ritualwitness.BuildWith(t, ctx, state, c.base)
	}
	if c.pristine {
		makePristine(t, fx)
	}
	if c.seed != nil {
		c.seed(t, ctx, fx)
	}
	if c.disclosure != "" {
		t.Logf("disclosed: %s", c.disclosure)
	}
	res := ritualwitness.RunOn(t, ctx, fx, c.driver(t, ctx, fx), decl)
	logVerdicts(t, res)
	judgeRitualRun(t, decl, c.want(state), res)
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

// judgeRitualRun applies the per-run law to res (the test's doc).
func judgeRitualRun(t *testing.T, decl ws.Declaration, want ritualRun, res ritualwitness.Result) {
	t.Helper()
	counts := map[ritualwitness.Status]int{}
	var outside []ritualwitness.Verdict
	for _, v := range res.Verdicts {
		counts[v.Status]++
		if v.Status == ritualwitness.Outside {
			outside = append(outside, v)
		}
	}
	t.Logf("verdict counts: %d within, %d unattributable, %d outside; command log supplied: %t",
		counts[ritualwitness.Within], counts[ritualwitness.Unattributable], counts[ritualwitness.Outside], res.Log.OK)

	if res.Err != nil {
		t.Logf("the ritual answered: %v", res.Err)
	}
	if res.Exit != want.exit {
		t.Fatalf("%s exited %d, want %d: %v", decl.Ritual, res.Exit, want.exit, res.Err)
	}
	if want.exit == 0 {
		if res.Err != nil {
			t.Fatalf("%s completed with an error: %v", decl.Ritual, res.Err)
		}
	} else {
		if len(want.words) == 0 {
			t.Fatalf("the case expects a refusal (exit %d) but names none of its words (SI-334 (4))", want.exit)
		}
		for _, w := range want.words {
			if w == "" || res.Err == nil || !strings.Contains(res.Err.Error(), w) {
				t.Fatalf("%s's refusal = %v, want its own words %q", decl.Ritual, res.Err, w)
			}
		}
		requireNothingRemains(t, res)
	}

	switch {
	case want.scopedRefusal:
		if decl.IndexCarry != ws.CarryScoped || want.exit != 2 {
			t.Fatalf("SI-348 (2)'s allowance applies only to a scoped declaration's exit-2 refusal; %s declares %s and the case expects exit %d", decl.Ritual, decl.IndexCarry, want.exit)
		}
		if len(outside) != 1 || outside[0] != scopedRefusalVerdict() {
			t.Errorf("outside verdicts = %v, want exactly %v (SI-348 (2))", outside, scopedRefusalVerdict())
		}
	case len(outside) > 0:
		t.Errorf("%d effect(s) outside %s's declaration: %v", len(outside), decl.Ritual, outside)
	}

	if counts[ritualwitness.Unattributable] > 0 && res.Log.OK {
		t.Errorf("%d unattributable verdict(s) with a command log supplied: the tolerance ends once the log exists (SI-341 (1))", counts[ritualwitness.Unattributable])
	}
}

// requireNothingRemains fails the test unless the run left no git
// mutation and created no commit object: a refusal is a guard before any
// mutation (SI-325 (3), SI-329 (3′), SI-348 (2)).
func requireNothingRemains(t *testing.T, res ritualwitness.Result) {
	t.Helper()
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
			t.Errorf("the refusal left %s changed:\nbefore %+v\nafter  %+v", c.what, c.before, c.after)
		}
	}
	if created := createdCommits(res); len(created) != 0 {
		t.Errorf("the refusal created commits %v", created)
	}
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
