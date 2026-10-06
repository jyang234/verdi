package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/pathcanon"
	"github.com/jyang234/verdi/internal/ritualwitness"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// The ritual-effect table's runs, shared by the two tests that read them
// (ledger SI-359 (8); backlog BL-159): TestRitualEffects_EveryDeclaredRitual
// judges each run's effects against its declaration, and
// TestForbiddenTokens_EveryVerbCommandLog judges each run's git command log
// against the forbidden tokens. The table runs at most once per test
// binary: the first of the two to run it keeps the runs in a file beside
// the shared built binary (buildVerdiBinary's per-process directory, which
// TestMain removes), and the other reads them back. Running either test
// alone runs the table once. A file is kept only when every case ran in
// every state, so a run cut short, or a -run pattern that selects some
// cases only, leaves the next test to run the table itself.
//
// The table runs with a process-wide VERDI_GITLOG set, so every gitx call
// the test process makes is also recorded there (spec/gitx-recorder-seam
// ac-2, dc-2). Each in-process run (Workbench, MCP) is then checked for
// completeness (ledger SI-359 (7)): the records the process appended while
// the driver ran, in the run's own directories, must equal the driver's
// observer log record for record, so no context root dropped a call.

// ritualEffectsRun is one run of one case in one seeded state: the case's
// identity, the driver's kind, the harness's Result, and the completeness
// check's findings. It is kept as JSON between the two tests.
type ritualEffectsRun struct {
	Verb     string
	Path     string
	Ritual   string
	State    string
	Driver   string
	Exit     int
	Err      *string
	Log      ritualwitness.CommandLog
	Before   ritualwitness.Snapshot
	After    ritualwitness.Snapshot
	Verdicts []ritualwitness.Verdict
	// InProcess reports a run whose driver runs the code in this process
	// (Workbench, MCP), whose log the completeness check compared with
	// Compared process-wide records; LogGaps is every difference.
	InProcess bool
	Compared  int
	LogGaps   []string
}

// newRitualEffectsRun is res, run by d, as one case's run in state.
func newRitualEffectsRun(verb ws.Verb, c ritualCase, state ritualwitness.SeedState, d ritualwitness.Driver, res ritualwitness.Result) ritualEffectsRun {
	run := ritualEffectsRun{
		Verb: verb.String(), Path: c.path, Ritual: c.ritual, State: state.String(), Driver: driverKind(d),
		Exit: res.Exit, Log: res.Log, Before: res.Before, After: res.After, Verdicts: res.Verdicts,
	}
	if res.Err != nil {
		msg := res.Err.Error()
		run.Err = &msg
	}
	return run
}

// result is the run as the harness reported it.
func (r ritualEffectsRun) result() ritualwitness.Result {
	res := ritualwitness.Result{Exit: r.Exit, Log: r.Log, Before: r.Before, After: r.After, Verdicts: r.Verdicts}
	if r.Err != nil {
		res.Err = errors.New(*r.Err)
	}
	return res
}

// key identifies the run among the table's.
func (r ritualEffectsRun) key() string { return ritualRunKey(r.Ritual, r.Path, r.Verb, r.State) }

// ritualRunKey is a run's identity: its case's subtest name, then its
// state's.
func ritualRunKey(ritual, path, verb, state string) string {
	return ritual + " " + path + " " + verb + "/" + state
}

// driverKind names a driver's type: Binary, Workbench, MCP, or InProcess.
func driverKind(d ritualwitness.Driver) string {
	name := fmt.Sprintf("%T", d)
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimPrefix(name, "*")
}

// inProcessDriver reports a driver that runs the code under test in this
// process, so its log is a gitx.Observer's.
func inProcessDriver(d ritualwitness.Driver) bool {
	switch d.(type) {
	case ritualwitness.Workbench, ritualwitness.MCP:
		return true
	}
	return false
}

// ritualEffectsRunsFile is where the first test keeps the runs, in the
// shared binary's per-process directory.
func ritualEffectsRunsFile(bin string) string {
	return filepath.Join(filepath.Dir(bin), "ritual-effects-runs.json")
}

// forEachRitualEffectsRun runs, or reads back, every case of table in
// every state it runs in, and calls judge on each run inside the case's
// own subtest, "cases/<ritual> <path> <verb>/<state>". A run read back is
// judged at once; a run made here is judged as it ends, the cases in
// parallel, and t's environment pins (git configuration isolation, the CI
// variables, the process-wide VERDI_GITLOG) hold for the whole table.
func forEachRitualEffectsRun(t *testing.T, bin string, table map[ws.Verb][]ritualCase, judge func(t *testing.T, c ritualCase, decl ws.Declaration, state ritualwitness.SeedState, run ritualEffectsRun)) {
	t.Helper()
	decls := map[string]ws.Declaration{}
	for _, d := range ws.Registry() {
		decls[d.Ritual] = d
	}
	file := ritualEffectsRunsFile(bin)
	if kept, ok := readRitualEffectsRuns(t, file, table); ok {
		t.Logf("judging the %d runs another test made in this test binary (%s; ledger SI-359 (8))", len(kept), file)
		t.Run("cases", func(t *testing.T) {
			for _, verb := range sortedVerbs(table) {
				for _, c := range table[verb] {
					t.Run(c.ritual+" "+c.path+" "+verb.String(), func(t *testing.T) {
						for _, state := range c.states() {
							t.Run(state.String(), func(t *testing.T) {
								judge(t, c, decls[c.ritual], state, kept[ritualRunKey(c.ritual, c.path, verb.String(), state.String())])
							})
						}
					})
				}
			}
		})
		return
	}

	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	ritualwitness.PinCIEnv(t, ritualwitness.CIEnv{})
	sink := filepath.Join(t.TempDir(), "process-gitlog.jsonl")
	t.Setenv(gitx.GitLogEnv, sink)

	var mu sync.Mutex
	made := map[string]ritualEffectsRun{}
	// The cases run in parallel inside one sequential subtest, which
	// returns only once every case has finished.
	t.Run("cases", func(t *testing.T) {
		for _, verb := range sortedVerbs(table) {
			for _, c := range table[verb] {
				decl := decls[c.ritual]
				t.Run(c.ritual+" "+c.path+" "+verb.String(), func(t *testing.T) {
					t.Parallel()
					for _, state := range c.states() {
						t.Run(state.String(), func(t *testing.T) {
							run := runRitualCase(t, verb, c, decl, state, sink)
							mu.Lock()
							made[run.key()] = run
							mu.Unlock()
							judge(t, c, decl, state, run)
						})
					}
				})
			}
		}
	})
	if missing := missingRitualRuns(table, made); len(missing) > 0 {
		named := missing
		if len(named) > 3 {
			named = append(named[:3:3], "…")
		}
		t.Logf("the table's runs are not kept for the other test: %d of its %d runs did not finish (%s)", len(missing), len(ritualRunKeys(table)), strings.Join(named, "; "))
		return
	}
	if err := writeRitualEffectsRuns(file, made); err != nil {
		t.Errorf("keeping the table's runs for the other test: %v", err)
	}
}

// ritualCaseTimeout bounds one run, fixture to judgment.
const ritualCaseTimeout = 120 * time.Second

// runRitualCase builds, seeds, and runs one case in one state, and returns
// the run, with an in-process run's log checked against the process-wide
// VERDI_GITLOG at sink (ledger SI-359 (7)).
func runRitualCase(t *testing.T, verb ws.Verb, c ritualCase, decl ws.Declaration, state ritualwitness.SeedState, sink string) ritualEffectsRun {
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
	d := c.driver(t, ctx, fx)
	if !inProcessDriver(d) {
		return newRitualEffectsRun(verb, c, state, d, ritualwitness.RunOn(t, ctx, fx, d, decl))
	}
	w := &sinkWindow{inner: d, path: sink}
	res := ritualwitness.RunOn(t, ctx, fx, w, decl)
	run := newRitualEffectsRun(verb, c, state, d, res)
	run.InProcess = true
	run.Compared, run.LogGaps = w.compare(res, fx)
	return run
}

// missingRitualRuns is every run the table defines that made does not
// hold.
func missingRitualRuns(table map[ws.Verb][]ritualCase, made map[string]ritualEffectsRun) []string {
	var missing []string
	for _, verb := range sortedVerbs(table) {
		for _, c := range table[verb] {
			for _, state := range c.states() {
				k := ritualRunKey(c.ritual, c.path, verb.String(), state.String())
				if _, ok := made[k]; !ok {
					missing = append(missing, k)
				}
			}
		}
	}
	return missing
}

// writeRitualEffectsRuns keeps runs at file as one JSON object keyed by
// run.
func writeRitualEffectsRuns(file string, runs map[string]ritualEffectsRun) error {
	data, err := json.Marshal(runs)
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0o600)
}

// readRitualEffectsRuns reads back the runs kept at file, strictly
// decoded, reporting false when none were kept. Kept runs that are not
// exactly the table's fail the test: the table never changes within one
// test binary.
func readRitualEffectsRuns(t *testing.T, file string, table map[ws.Verb][]ritualCase) (map[string]ritualEffectsRun, bool) {
	t.Helper()
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false
	}
	if err != nil {
		t.Fatalf("reading the table's kept runs: %v", err)
	}
	var runs map[string]ritualEffectsRun
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&runs); err != nil {
		t.Fatalf("decoding the table's kept runs %s: %v", file, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		t.Fatalf("decoding the table's kept runs %s: trailing data", file)
	}
	if missing := missingRitualRuns(table, runs); len(missing) > 0 || len(runs) != len(ritualRunKeys(table)) {
		t.Fatalf("the kept runs at %s are not the table's: %d kept, %d defined, missing %v", file, len(runs), len(ritualRunKeys(table)), missing)
	}
	return runs, true
}

// ritualRunKeys is every run the table defines.
func ritualRunKeys(table map[ws.Verb][]ritualCase) []string {
	var keys []string
	for _, verb := range sortedVerbs(table) {
		for _, c := range table[verb] {
			for _, state := range c.states() {
				keys = append(keys, ritualRunKey(c.ritual, c.path, verb.String(), state.String()))
			}
		}
	}
	return keys
}

// sinkWindow wraps an in-process driver and reads the process-wide
// VERDI_GITLOG file around the driver's own Run, so the window holds what
// the test process recorded while the ritual ran and nothing the harness's
// snapshots or the case's seeding did.
type sinkWindow struct {
	inner ritualwitness.Driver
	path  string

	window []gitx.GitLogRecord
	err    error
}

// Run implements ritualwitness.Driver.
func (w *sinkWindow) Run(ctx context.Context, dir string) (int, ritualwitness.CommandLog, error) {
	before, err := sinkRecords(w.path)
	exit, log, runErr := w.inner.Run(ctx, dir)
	after, err2 := sinkRecords(w.path)
	switch {
	case err != nil:
		w.err = err
	case err2 != nil:
		w.err = err2
	case len(after) < len(before):
		w.err = fmt.Errorf("the process-wide VERDI_GITLOG shrank from %d to %d records", len(before), len(after))
	default:
		w.window = after[len(before):]
	}
	return exit, log, runErr
}

// sinkRecords strictly decodes every whole line of the VERDI_GITLOG file
// at path; a last line still being written, with no newline yet, is left
// for a later read. A missing file holds no record.
func sinkRecords(path string) ([]gitx.GitLogRecord, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the process-wide VERDI_GITLOG: %w", err)
	}
	whole := data[:bytes.LastIndexByte(data, '\n')+1]
	if len(whole) == 0 {
		return nil, nil
	}
	var out []gitx.GitLogRecord
	for i, line := range strings.Split(strings.TrimSuffix(string(whole), "\n"), "\n") {
		var rec gitx.GitLogRecord
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&rec); err != nil {
			return nil, fmt.Errorf("the process-wide VERDI_GITLOG's line %d %q: %w", i+1, line, err)
		}
		if _, err := dec.Token(); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("the process-wide VERDI_GITLOG's line %d %q: trailing data", i+1, line)
		}
		out = append(out, rec)
	}
	return out, nil
}

// compare is the completeness check (ledger SI-359 (7)): the records this
// process appended to the process-wide VERDI_GITLOG while the driver ran,
// in a directory of the run's own (the fixture's repository, its bare
// remote, a worktree registered before or after the run, or a directory
// the observer logged), must equal the observer's log record for record,
// each record its canonical directory and its argv. It returns how many
// process-wide records it compared and every difference. Records of other
// processes (a binary a seed runs) and of other runs (in their own
// fixtures, which run in parallel) are outside the window's filter.
func (w *sinkWindow) compare(res ritualwitness.Result, fx *ritualwitness.Fixture) (int, []string) {
	if w.err != nil {
		return 0, []string{fmt.Sprintf("the completeness check could not read the process-wide VERDI_GITLOG: %v", w.err)}
	}
	if !res.Log.OK {
		return 0, []string{fmt.Sprintf("the in-process driver supplied no command log to compare with the process-wide VERDI_GITLOG: %s", res.Log.Reason)}
	}
	dirs := []string{pathcanon.Canonical(res.After.Root), pathcanon.Canonical(fx.Bare)}
	for _, wt := range append(append([]ritualwitness.Worktree(nil), res.Before.Worktrees...), res.After.Worktrees...) {
		dirs = append(dirs, pathcanon.Canonical(wt.Path))
	}
	record := func(dir string, args []string) string {
		return pathcanon.Canonical(dir) + "\x00" + strings.Join(args, "\x00")
	}
	observed := map[string]int{}
	for _, c := range res.Log.Calls {
		observed[record(c.Dir, c.Args)]++
		dirs = append(dirs, pathcanon.Canonical(c.Dir))
	}
	ours := func(dir string) bool {
		for _, d := range dirs {
			if rel, err := filepath.Rel(d, dir); err == nil && rel != ".." && !strings.HasPrefix(filepath.ToSlash(rel), "../") {
				return true
			}
		}
		return false
	}
	sunk := map[string]int{}
	compared := 0
	for _, rec := range w.window {
		if rec.PID != os.Getpid() {
			continue
		}
		dir := pathcanon.Canonical(rec.Dir)
		if !ours(dir) {
			continue
		}
		compared++
		sunk[record(dir, rec.Args)]++
	}
	var gaps []string
	keys := map[string]bool{}
	for k := range observed {
		keys[k] = true
	}
	for k := range sunk {
		keys[k] = true
	}
	var sorted []string
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		dir, argv, _ := strings.Cut(k, "\x00")
		call := fmt.Sprintf("`git %s` in %s", strings.ReplaceAll(argv, "\x00", " "), dir)
		switch n := sunk[k] - observed[k]; {
		case n > 0:
			gaps = append(gaps, fmt.Sprintf("the process-wide VERDI_GITLOG recorded %s %d more time(s) than the driver's observer: a context root dropped the observer (ledger SI-359 (7))", call, n))
		case n < 0:
			gaps = append(gaps, fmt.Sprintf("the driver's observer logged %s %d more time(s) than the process-wide VERDI_GITLOG recorded (ledger SI-359 (7))", call, -n))
		}
	}
	return compared, gaps
}
