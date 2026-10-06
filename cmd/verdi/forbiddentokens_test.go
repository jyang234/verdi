package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/gitforbid"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/ritualwitness"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// TestForbiddenTokens_EveryVerbCommandLog is the producer of obligation
// gitx-recorder-seam--ac-2--behavioral (spec/gitx-recorder-seam ac-2; parent
// spec/ritual-write-scope-v3 ac-3, dc-5, dc-10). It proves the obligation's
// whole claim, one part per clause:
//
//   - every declared verb, run by the ritual-effect harness through its real
//     entry point (the built binary, the workbench handler on loopback, or the
//     MCP server in process) on every publication path in both seeded states,
//     leaves a command log free of every forbidden token: the table's coverage
//     law holds (checkRitualCoverage), every run carries its log, and no logged
//     call's argv carries a token of internal/gitforbid, the one list recovery
//     imports too (forbiddenTokenViolations);
//   - a test verb made to emit a forbidden token fails the witness: one per
//     token, each through the harness (SI-359 (6));
//   - and the built binary writes its log to VERDI_GITLOG only when it is set
//     (TestGitLogE2E_RecordsOnlyWhenSet, run here as a subtest).
//
// The runs are the ritual-effect producer's own (forEachRitualEffectsRun):
// whichever of the two tests runs first runs the table once per test binary
// and the other reads the runs back (ledger SI-359 (8); backlog BL-159). Each
// in-process run's log is first checked against a process-wide VERDI_GITLOG
// record for record (SI-359 (7)), so a context root that dropped a call fails
// the run here too.
//
// Disclosed, not proven:
//
//   - SI-359 (4b): git that a child program the binary starts runs itself (the
//     go toolchain, make, npx) is outside the log and so outside this witness;
//   - SI-359 (13): matching is whole-element and fail-closed over every argv
//     element, data included, so a commit message or branch name spelt exactly
//     as a token (`commit -m reset`) fails the witness; the fixtures keep such
//     names out, and recovery's refusal of them is unchanged;
//   - SI-359 (15): the Binary driver keeps only the records of the process it
//     started, so the calls of a verdi process the binary itself starts (none
//     does today) would be dropped from the log and from this witness.
func TestForbiddenTokens_EveryVerbCommandLog(t *testing.T) {
	bin := buildVerdiBinary(t)
	table := ritualEffectsTable(t, bin)
	if err := checkRitualCoverage(ws.Registry(), table); err != nil {
		t.Fatal(err)
	}

	var tally forbiddenTokenTally
	forEachRitualEffectsRun(t, bin, table, func(t *testing.T, _ ritualCase, _ ws.Declaration, _ ritualwitness.SeedState, run ritualEffectsRun) {
		tally.record(run)
		for _, v := range forbiddenTokenViolations(run) {
			t.Error(v)
		}
		for _, g := range run.LogGaps {
			t.Error(g)
		}
	})
	for _, line := range tally.lines() {
		t.Log(line)
	}

	t.Run("a test verb made to emit a forbidden token fails the witness", func(t *testing.T) {
		// The verb commits in process, so it reads this process's git
		// configuration: isolate it from the ambient one, as the table's
		// runs are.
		t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
		t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		for _, token := range gitforbid.Tokens() {
			t.Run(token, func(t *testing.T) {
				run := runTokenEmittingVerb(t, token)
				got := strings.Join(forbiddenTokenViolations(run), "\n")
				if got == "" || !strings.Contains(got, fmt.Sprintf("%q", token)) {
					t.Fatalf("the witness passed a verb that emitted %q (violations: %q; log %+v)", token, got, run.Log)
				}
			})
		}
		t.Run("the same verb without a token passes", func(t *testing.T) {
			run := runTokenEmittingVerb(t, "a plain message")
			if got := forbiddenTokenViolations(run); len(got) != 0 {
				t.Fatalf("the witness failed a verb that emitted no token: %q", got)
			}
		})
	})

	t.Run("the built binary writes its log to VERDI_GITLOG only when it is set", TestGitLogE2E_RecordsOnlyWhenSet)
}

// forbiddenTokenViolations is the witness's law over one run (SI-359 (6),
// (13)): a run whose driver supplied no command log cannot show its log is
// free of tokens, and every logged call whose argv carries a forbidden token
// (gitforbid.Forbids, whole-element) is named with each element that does.
func forbiddenTokenViolations(run ritualEffectsRun) []string {
	if !run.Log.OK {
		return []string{fmt.Sprintf("%s run of %s (%s): no command log, so the witness cannot show it free of forbidden tokens: %s", run.Driver, run.Verb, run.State, run.Log.Reason)}
	}
	var out []string
	for i, c := range run.Log.Calls {
		if !gitforbid.Forbids(c.Args) {
			continue
		}
		var elements []string
		for _, arg := range c.Args {
			if gitforbid.Forbids([]string{arg}) {
				elements = append(elements, fmt.Sprintf("%q", arg))
			}
		}
		out = append(out, fmt.Sprintf("%s run of %s (%s): logged call %d `git %s` in %s carries forbidden element(s) %s",
			run.Driver, run.Verb, run.State, i+1, strings.Join(c.Args, " "), c.Dir, strings.Join(elements, ", ")))
	}
	return out
}

// runTokenEmittingVerb runs a test verb through the harness, in process,
// over a fresh fixture: it commits through gitx with message as its one
// argument, so its logged argv carries message as one whole element
// (`commit -m <message>`), and git's own answer does not matter, since gitx
// logs a call before it runs.
func runTokenEmittingVerb(t *testing.T, message string) ritualEffectsRun {
	t.Helper()
	ctx := context.Background()
	verb := ritualwitness.InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		if _, err := gitx.CreateCommit(ctx, dir, message); err != nil {
			return 2, err
		}
		return 0, nil
	}}
	// The witness judges the log alone, so the declaration only has to be
	// valid for the harness to run the verb.
	decl := ws.Declaration{Ritual: "token_emitter", Verbs: []ws.Verb{ws.CLI("token-emitter")}, RefsMove: []ws.RefPattern{ws.RefCheckedOut}, IndexCarry: ws.CarryNoCommit}
	res := ritualwitness.Run(t, ctx, verb, decl, ritualwitness.SeedClean)
	return newRitualEffectsRun(ws.CLI("token-emitter"), ritualCase{path: pathRoot, ritual: decl.Ritual}, ritualwitness.SeedClean, verb, res)
}

// forbiddenTokenTally counts the table's runs and logged calls per driver
// kind. Its zero value is ready; the parallel cases record into it
// concurrently.
type forbiddenTokenTally struct {
	mu    sync.Mutex
	runs  map[string]int
	calls map[string]int
}

// record notes one run.
func (f *forbiddenTokenTally) record(run ritualEffectsRun) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.runs == nil {
		f.runs, f.calls = map[string]int{}, map[string]int{}
	}
	f.runs[run.Driver]++
	f.calls[run.Driver] += len(run.Log.Calls)
}

// lines is the tally, one line per driver kind, sorted.
func (f *forbiddenTokenTally) lines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var kinds []string
	for k := range f.runs {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	var out []string
	for _, k := range kinds {
		out = append(out, fmt.Sprintf("driver %s: %d run(s), %d logged call(s), each checked against %d forbidden tokens", k, f.runs[k], f.calls[k], len(gitforbid.Tokens())))
	}
	return out
}
