package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/pathcanon"
	"github.com/jyang234/verdi/internal/ritualwitness"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// The laws TestRitualEffects_EveryDeclaredRitual applies, driven with
// synthetic results and mutated tables (ledger SI-354 (6)): the per-run
// law (ritualRunViolations), the coverage law (checkRitualCoverage), and
// the abstain decision (ritualEffectsGaps, SI-354 (3)).

// TestRitualRunViolations is the per-run law's table: a conforming run has
// no violation, and every way of breaking the law is named.
func TestRitualRunViolations(t *testing.T) {
	within := ritualwitness.Verdict{Field: "index_carry", Status: ritualwitness.Within, Detail: "declares scoped; observed scoped"}
	unattributable := ritualwitness.Verdict{Field: "refs_create", Status: ritualwitness.Unattributable, Detail: "refs/heads/design/x created"}
	outsideConfig := ritualwitness.Verdict{Field: "config", Status: ritualwitness.Outside, Detail: "branch.x.remote set"}
	logged := ritualwitness.CommandLog{OK: true}
	answered := errors.New
	tests := []struct {
		name   string
		ritual string
		want   ritualRun
		res    ritualwitness.Result
		// violation is a word of the one violation the run shows; "" for
		// a conforming run.
		violation string
	}{
		{"a completion within its declaration", "design_start", completes(),
			ritualwitness.Result{Verdicts: []ritualwitness.Verdict{within}}, ""},
		{"an unattributable verdict while no command log is supplied", "design_start", completes(),
			ritualwitness.Result{Verdicts: []ritualwitness.Verdict{unattributable, within}}, ""},
		{"a refusal in its own words, nothing remaining", "close", refuses(2, "refusing to run"),
			ritualwitness.Result{Exit: 2, Err: answered("close: refusing to run with staged paths"), Verdicts: []ritualwitness.Verdict{within}}, ""},
		{"a named scoped refusal with exactly its verdict", "design_start", allowingScopedRefusal(refuses(2, "dirty")),
			ritualwitness.Result{Exit: 2, Err: answered("dirty context"), Verdicts: []ritualwitness.Verdict{scopedRefusalVerdict()}}, ""},
		{"a refusal standing in for a completion", "close", standingInForCompletion(refuses(1, "[FAIL] closure"), closeCompletionGap),
			ritualwitness.Result{Exit: 1, Err: answered("[FAIL] closure: countersign"), Verdicts: []ritualwitness.Verdict{within}}, ""},

		{"an unattributable verdict with a command log supplied", "design_start", completes(),
			ritualwitness.Result{Log: logged, Verdicts: []ritualwitness.Verdict{unattributable, within}}, "unattributable verdict(s) with a command log supplied"},
		{"the scoped refusal's verdict without the allowance", "design_start", refuses(2, "dirty"),
			ritualwitness.Result{Exit: 2, Err: answered("dirty context"), Verdicts: []ritualwitness.Verdict{scopedRefusalVerdict()}}, "effect(s) outside design_start's declaration"},
		{"the allowance on a declaration that is not scoped", "close", allowingScopedRefusal(refuses(2, "dirty")),
			ritualwitness.Result{Exit: 2, Err: answered("dirty context"), Verdicts: []ritualwitness.Verdict{scopedRefusalVerdict()}}, "applies only to a scoped declaration's exit-2 refusal"},
		{"the allowance on an exit-1 refusal", "design_start", allowingScopedRefusal(refuses(1, "dirty")),
			ritualwitness.Result{Exit: 1, Err: answered("dirty context"), Verdicts: []ritualwitness.Verdict{scopedRefusalVerdict()}}, "applies only to a scoped declaration's exit-2 refusal"},
		{"the allowance with a second outside verdict", "design_start", allowingScopedRefusal(refuses(2, "dirty")),
			ritualwitness.Result{Exit: 2, Err: answered("dirty context"), Verdicts: []ritualwitness.Verdict{scopedRefusalVerdict(), outsideConfig}}, "want exactly"},
		{"the allowance with no outside verdict", "design_start", allowingScopedRefusal(refuses(2, "dirty")),
			ritualwitness.Result{Exit: 2, Err: answered("dirty context"), Verdicts: []ritualwitness.Verdict{within}}, "want exactly"},
		{"a refusal in other words", "close", refuses(2, "refusing to run"),
			ritualwitness.Result{Exit: 2, Err: answered("close: something else"), Verdicts: []ritualwitness.Verdict{within}}, "want its own words"},
		{"a refusal answering no error", "close", refuses(2, "refusing to run"),
			ritualwitness.Result{Exit: 2, Verdicts: []ritualwitness.Verdict{within}}, "want its own words"},
		{"a refusal naming none of its words", "close", refuses(2),
			ritualwitness.Result{Exit: 2, Err: answered("close: refusing"), Verdicts: []ritualwitness.Verdict{within}}, "names none of its words"},
		{"a refusal that left a ref", "close", refuses(1, "refusing"),
			ritualwitness.Result{Exit: 1, Err: answered("refusing"),
				Before:   ritualwitness.Snapshot{Refs: map[string]ritualwitness.Ref{}},
				After:    ritualwitness.Snapshot{Refs: map[string]ritualwitness.Ref{"refs/heads/close/x": {Object: "abc"}}},
				Verdicts: []ritualwitness.Verdict{within}}, "the refusal left refs changed"},
		{"a refusal that changed the index", "close", refuses(1, "refusing"),
			ritualwitness.Result{Exit: 1, Err: answered("refusing"),
				After:    ritualwitness.Snapshot{Index: []ritualwitness.IndexEntry{{Tag: "H", Mode: "100644", Object: "abc", Path: "staged.txt"}}},
				Verdicts: []ritualwitness.Verdict{within}}, "the refusal left the index changed"},
		{"a refusal naming an empty word", "close", refuses(2, "refusing", ""),
			ritualwitness.Result{Exit: 2, Err: answered("close: refusing"), Verdicts: []ritualwitness.Verdict{within}}, `want its own words ""`},
		{"a refusal that created a commit", "close", refuses(1, "refusing"),
			ritualwitness.Result{Exit: 1, Err: answered("refusing"),
				After:    ritualwitness.Snapshot{Commits: map[string]ritualwitness.CommitObject{"abc": {}}},
				Verdicts: []ritualwitness.Verdict{within}}, "the refusal created commits [abc]"},
		{"a completion with an outside verdict", "gc", completes(),
			ritualwitness.Result{Verdicts: []ritualwitness.Verdict{outsideConfig}}, "effect(s) outside gc's declaration"},
		{"a refusal where completion is expected", "gc", completes(),
			ritualwitness.Result{Exit: 2, Err: answered("gc: refused")}, "gc exited 2, want 0"},
		{"a completion answering an error", "gc", completes(),
			ritualwitness.Result{Err: answered("gc: warned")}, "completed with an error"},
		{"a gap named on a completion", "close", standingInForCompletion(completes(), closeCompletionGap),
			ritualwitness.Result{Verdicts: []ritualwitness.Verdict{within}}, "only a refusal stands in for a completion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ritualRunViolations(ritualDeclaration(t, tt.ritual), tt.want, tt.res)
			if tt.violation == "" {
				if len(got) != 0 {
					t.Fatalf("violations = %q, want none", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], tt.violation) {
				t.Fatalf("violations = %q, want exactly one naming %q", got, tt.violation)
			}
		})
	}
}

// TestCheckRitualCoverage is the coverage law's table: the producer's own
// table conforms, and each mutated table or registry is refused with the
// problem named (dc-3; SI-341 (4), (7); SI-354 (5)).
func TestCheckRitualCoverage(t *testing.T) {
	base := ritualEffectsTable(t, "/nonexistent/verdi")
	if err := checkRitualCoverage(ws.Registry(), base); err != nil {
		t.Fatalf("the producer's table fails its own coverage law: %v", err)
	}
	pageB := ws.Workbench("/b/{branch}/board/spec/{name}")
	createB := ws.Workbench("/b/{branch}/board/spec/{name}/api/create")
	newB := ws.Workbench("/b/{branch}/board/spec/{name}/r3c-new")
	dropPath := func(cases []ritualCase, path string) []ritualCase {
		return slices.DeleteFunc(slices.Clone(cases), func(c ritualCase) bool { return c.path == path })
	}
	withVerb := func(ritual string, v ws.Verb) func() []ws.Declaration {
		return func() []ws.Declaration {
			decls := ws.Registry()
			for i := range decls {
				if decls[i].Ritual == ritual {
					decls[i].Verbs = append(slices.Clone(decls[i].Verbs), v)
				}
			}
			return decls
		}
	}
	tests := []struct {
		name  string
		decls func() []ws.Declaration
		table func(map[ws.Verb][]ritualCase)
		// want are words one problem line names together.
		want []string
	}{
		{"a registry verb with no case", nil, func(m map[ws.Verb][]ritualCase) { delete(m, ws.CLI("gc")) },
			[]string{"registry verb cli:gc (ritual gc) has no case"}},
		{"a stale key", nil, func(m map[ws.Verb][]ritualCase) {
			m[ws.CLI("sync")] = []ritualCase{{path: pathRoot, ritual: "design_start"}}
		}, []string{"table key cli:sync is no registry verb (a stale case)"}},
		{"a repeated path", nil, func(m map[ws.Verb][]ritualCase) { m[ws.CLI("gc")] = append(m[ws.CLI("gc")], m[ws.CLI("gc")][0]) },
			[]string{"cli:gc has two cases for path \"managed\""}},
		{"a case naming another ritual", nil, func(m map[ws.Verb][]ritualCase) {
			cases := slices.Clone(m[ws.CLI("gc")])
			cases[0].ritual = "recover"
			m[ws.CLI("gc")] = cases
		}, []string{"names ritual recover, but the registry declares gc"}},
		{"a /b/ verb missing its first use", nil, func(m map[ws.Verb][]ritualCase) { m[pageB] = dropPath(m[pageB], pathBFirstUse) },
			[]string{pageB.String(), `no case for publication path "b-first-use"`}},
		{"a /b/ verb missing its existing managed worktree", nil, func(m map[ws.Verb][]ritualCase) { m[pageB] = dropPath(m[pageB], pathBExisting) },
			[]string{pageB.String(), `no case for publication path "b-existing"`}},
		{"a /b/ verb missing its branch checked out here", nil, func(m map[ws.Verb][]ritualCase) { m[createB] = dropPath(m[createB], pathBHere) },
			[]string{createB.String(), `no case for publication path "b-here"`}},
		{"a /b/ verb missing its branch held elsewhere", nil, func(m map[ws.Verb][]ritualCase) { m[pageB] = dropPath(m[pageB], pathBElsewhere) },
			[]string{pageB.String(), `no case for publication path "b-elsewhere"`}},
		{"a new /b/ registry verb with every dispatch branch but held elsewhere", withVerb("managed_worktree", newB), func(m map[ws.Verb][]ritualCase) {
			for _, p := range []string{pathBFirstUse, pathBExisting, pathBHere} {
				m[newB] = append(m[newB], ritualCase{path: p, ritual: "managed_worktree"})
			}
		}, []string{newB.String(), `no case for publication path "b-elsewhere"`}},
		{"a root route missing its root path", nil, func(m map[ws.Verb][]ritualCase) {
			v := ws.Workbench("/board/{key}/commit")
			m[v] = dropPath(m[v], pathRoot)
		}, []string{"/board/{key}/commit", "has no case"}},
		{"a named path missing (close's unwind)", nil, func(m map[ws.Verb][]ritualCase) {
			m[ws.CLI("close")] = dropPath(m[ws.CLI("close")], "unwind")
		}, []string{"cli:close", `no case for publication path "unwind"`}},
		{"a whole-tree guard missing its pristine completion", nil, func(m map[ws.Verb][]ritualCase) {
			v := ws.MCP("import_apply")
			m[v] = dropPath(m[v], pathRoot+pristineSuffix)
		}, []string{`no case for publication path "root-pristine"`}},
		{"a new registry verb with no case", withVerb("design_start", ws.CLI("design restart")), nil,
			[]string{"registry verb cli:design restart (ritual design_start) has no case"}},
		{"named paths pinned for a verb the registry dropped", func() []ws.Declaration {
			return slices.DeleteFunc(ws.Registry(), func(d ws.Declaration) bool { return d.Ritual == "gc" })
		}, nil, []string{"named paths are pinned for cli:gc, which is no registry verb"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := map[ws.Verb][]ritualCase{}
			for v, cases := range base {
				table[v] = slices.Clone(cases)
			}
			if tt.table != nil {
				tt.table(table)
			}
			decls := ws.Registry()
			if tt.decls != nil {
				decls = tt.decls()
			}
			err := checkRitualCoverage(decls, table)
			if err == nil {
				t.Fatalf("checkRitualCoverage = nil, want a problem naming %q", tt.want)
			}
			for _, line := range strings.Split(err.Error(), "\n") {
				if containsAll(line, tt.want) {
					return
				}
			}
			t.Fatalf("checkRitualCoverage = %v, want a problem naming all of %q", err, tt.want)
		})
	}
}

// containsAll reports whether s contains every one of words.
func containsAll(s string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}

// TestRitualEffectsGaps is the abstain decision's table (ledger SI-354
// (3)): the producer abstains while any run lacked the command log, or
// while a refusal stood in for a completion, naming each gap once, and may
// pass only when no gap is open.
func TestRitualEffectsGaps(t *testing.T) {
	logged := ritualwitness.Result{Log: ritualwitness.CommandLog{OK: true}}
	unlogged := ritualwitness.Result{}
	closeStandIn := standingInForCompletion(refuses(1, "gate"), closeCompletionGap)
	execStandIn := standingInForCompletion(refuses(2, "binding"), executionCompletionGap)
	type run struct {
		want ritualRun
		res  ritualwitness.Result
	}
	tests := []struct {
		name string
		runs []run
		want []string
	}{
		{"no run at all", nil, []string{noCommandLogGap}},
		{"every run logged, every completion proven", []run{{completes(), logged}, {refuses(2, "staged"), logged}}, nil},
		{"one run without the log", []run{{completes(), logged}, {completes(), unlogged}}, []string{noCommandLogGap}},
		{"no run with the log", []run{{completes(), unlogged}, {refuses(2, "x"), unlogged}}, []string{noCommandLogGap}},
		{"close's completion stood in for, logged", []run{{completes(), logged}, {closeStandIn, logged}}, []string{closeCompletionGap}},
		{"every gap, each named once", []run{{execStandIn, unlogged}, {closeStandIn, unlogged}, {execStandIn, unlogged}, {completes(), unlogged}},
			[]string{noCommandLogGap, closeCompletionGap, executionCompletionGap}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g ritualEffectsGaps
			for _, r := range tt.runs {
				g.record(r.want, r.res)
			}
			if got := g.open(); !slices.Equal(got, tt.want) {
				t.Fatalf("open gaps = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRitualEffectsTable_StandInsNameEachGap feeds every expectation of
// the producer's own table, per case and state, into the abstain decision
// with a command log on every run, so only the completion gaps can stay
// open (ledger SI-354 (3)). Exactly two are open, close's and the
// execution rituals', and each is marked on exactly the rows that stand in
// for it: close's four paths in the clean-index state, where close is
// declared to complete, and the execution rituals' three verbs in both
// states. A row that lost its mark, or a mark on another row, fails here.
func TestRitualEffectsTable_StandInsNameEachGap(t *testing.T) {
	table := ritualEffectsTable(t, "/nonexistent/verdi")
	var g ritualEffectsGaps
	marked := map[string][]string{}
	for _, verb := range sortedVerbs(table) {
		for _, c := range table[verb] {
			for _, state := range c.states() {
				want := c.want(state)
				g.record(want, ritualwitness.Result{Log: ritualwitness.CommandLog{OK: true}})
				if want.unprovenCompletion != "" {
					marked[want.unprovenCompletion] = append(marked[want.unprovenCompletion], verb.String()+" "+c.path+" "+state.String())
				}
			}
		}
	}
	if got, want := g.open(), []string{closeCompletionGap, executionCompletionGap}; !slices.Equal(got, want) {
		t.Fatalf("open gaps = %q, want exactly %q", got, want)
	}
	wantMarked := map[string][]string{
		closeCompletionGap: {"cli:close ci clean", "cli:close feature clean", "cli:close force-local clean", "cli:close unwind clean"},
		executionCompletionGap: {
			"cli:experiment resume input-binding-refusal full", "cli:experiment resume input-binding-refusal clean",
			"cli:experiment start input-binding-refusal full", "cli:experiment start input-binding-refusal clean",
			"mcp:experiment input-binding-refusal full", "mcp:experiment input-binding-refusal clean",
		},
	}
	for gap, rows := range wantMarked {
		got := slices.Clone(marked[gap])
		slices.Sort(got)
		want := slices.Clone(rows)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("rows marked as standing in for %q = %q, want %q", gap, got, want)
		}
	}
	if len(marked) != len(wantMarked) {
		t.Errorf("rows are marked for %d gap(s), want %d: %v", len(marked), len(wantMarked), marked)
	}
}

// TestSinkWindowCompare is the completeness check on synthetic runs (ledger
// SI-359 (7); R5c2 review R5C2R-1): the process-wide records of this
// process in the run's own directories must equal the observer's log
// record for record. A dropped call (recorded, not observed) and an extra
// call (observed, not recorded) are each named, with their counts; a
// record in another directory, or of another process, is outside the
// comparison; a run without a log, or a window that could not be read,
// is a gap of its own.
func TestSinkWindowCompare(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	bare := filepath.Join(t.TempDir(), "remote.git")
	other := filepath.Join(t.TempDir(), "another-run")
	wt := filepath.Join(t.TempDir(), "linked")
	pid := os.Getpid()
	rec := func(dir string, p int, args ...string) gitx.GitLogRecord {
		return gitx.GitLogRecord{Args: args, Dir: dir, PID: p}
	}
	call := func(dir string, args ...string) ritualwitness.Call { return ritualwitness.Call{Dir: dir, Args: args} }
	head := []string{"rev-parse", "--verify", "HEAD"}
	add := []string{"add", "--", "a.txt"}
	tests := []struct {
		name     string
		window   []gitx.GitLogRecord
		err      error
		log      ritualwitness.CommandLog
		compared int
		gaps     []string // a word of each gap, in order; nil for none
	}{
		{"equal, record for record", []gitx.GitLogRecord{rec(root, pid, head...), rec(root+"/sub", pid, add...), rec(bare, pid, "for-each-ref")},
			nil, ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{call(root, head...), call(root+"/sub", add...), call(bare, "for-each-ref")}}, 3, nil},
		{"equal in another order", []gitx.GitLogRecord{rec(root, pid, add...), rec(root, pid, head...)},
			nil, ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{call(root, head...), call(root, add...)}}, 2, nil},
		{"a linked worktree's records count", []gitx.GitLogRecord{rec(wt, pid, head...)},
			nil, ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{call(wt, head...)}}, 1, nil},
		{"a dropped call", []gitx.GitLogRecord{rec(root, pid, head...), rec(root, pid, add...)},
			nil, ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{call(root, head...)}}, 2,
			[]string{"recorded `git add -- a.txt` in " + pathcanon.Canonical(root) + " 1 more time(s) than the driver's observer"}},
		{"a call dropped twice", []gitx.GitLogRecord{rec(root, pid, head...), rec(root, pid, head...)},
			nil, ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{}}, 2,
			[]string{"`git rev-parse --verify HEAD` in " + pathcanon.Canonical(root) + " 2 more time(s)"}},
		{"an extra call", []gitx.GitLogRecord{rec(root, pid, head...)},
			nil, ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{call(root, head...), call(root, add...)}}, 1,
			[]string{"the driver's observer logged `git add -- a.txt` in " + pathcanon.Canonical(root) + " 1 more time(s)"}},
		{"records in another directory are outside", []gitx.GitLogRecord{rec(root, pid, head...), rec(other, pid, add...), rec(root+"-sibling", pid, add...)},
			nil, ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{call(root, head...)}}, 1, nil},
		{"records of another process are outside", []gitx.GitLogRecord{rec(root, pid, head...), rec(root, pid+1, add...)},
			nil, ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{call(root, head...)}}, 1, nil},
		{"no command log", nil, nil, ritualwitness.CommandLog{Reason: "gone"}, 0, []string{"supplied no command log to compare with the process-wide VERDI_GITLOG: gone"}},
		{"an unreadable window", nil, errors.New("line 3: trailing data"),
			ritualwitness.CommandLog{OK: true, Calls: []ritualwitness.Call{}}, 0, []string{"could not read the process-wide VERDI_GITLOG: line 3: trailing data"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &sinkWindow{window: tt.window, err: tt.err}
			res := ritualwitness.Result{Log: tt.log,
				Before: ritualwitness.Snapshot{Root: root},
				After:  ritualwitness.Snapshot{Root: root, Worktrees: []ritualwitness.Worktree{{Path: wt}}}}
			compared, gaps := w.compare(res, &ritualwitness.Fixture{Dir: root, Bare: bare})
			if compared != tt.compared {
				t.Errorf("compared %d record(s), want %d", compared, tt.compared)
			}
			if len(gaps) != len(tt.gaps) {
				t.Fatalf("gaps = %q, want %d naming %q", gaps, len(tt.gaps), tt.gaps)
			}
			for i, w := range tt.gaps {
				if !strings.Contains(gaps[i], w) {
					t.Errorf("gap %q, want it to name %q", gaps[i], w)
				}
			}
		})
	}
}

// TestSinkRecords: the process-wide VERDI_GITLOG is read strictly, whole
// lines only, so a record still being written is left for a later read; a
// missing file holds no record, and a malformed line is an error.
func TestSinkRecords(t *testing.T) {
	dir := t.TempDir()
	good := `{"args":["status"],"dir":"/r","pid":7}` + "\n"
	tests := []struct {
		name    string
		data    *string
		want    int
		wantErr bool
	}{
		{"a missing file", nil, 0, false},
		{"an empty file", ptr(""), 0, false},
		{"whole records", ptr(good + good), 2, false},
		{"a record still being written is left unread", ptr(good + `{"args":["sta`), 1, false},
		{"an unknown field", ptr(`{"args":["status"],"dir":"/r","pid":7,"x":1}` + "\n"), 0, true},
		{"trailing data", ptr(`{"args":["status"],"dir":"/r","pid":7} {}` + "\n"), 0, true},
		{"not JSON", ptr("garbage\n"), 0, true},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, fmt.Sprintf("log-%d.jsonl", i))
			if tt.data != nil {
				if err := os.WriteFile(path, []byte(*tt.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := sinkRecords(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("sinkRecords = %v, %v; want an error: %v", got, err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.want {
				t.Fatalf("sinkRecords = %+v, want %d record(s)", got, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }
