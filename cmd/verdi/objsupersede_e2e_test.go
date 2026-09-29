// objsupersede_e2e_test.go proves closed-spec object supersession through
// the built binary (design §8, "Test layers": CLI paths are end-to-end Go
// tests driving the built binary), over the committed scenario fixture
// testdata/objsupersede and its builder: lint, align, and gate on the
// successor's design branch, acceptance, the refusals §8 lists, a carried
// replacement through revisions, unrelated reuse, a hand-typed
// disposition, and each landing shape's outcome. The judged sweep runs
// unconfigured (the scenario store sets no align.judge_cmd, and
// judge_required defaults false), and its absence finding is dispositioned
// by hand, as a reviewer does, so the gate turns on the computed section.
package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/store"
)

// cssHermeticEnv keeps a developer's forge credentials and CI variables
// from reaching the binary: no forge is built, the default branch
// resolves from the fixture's own origin/main, and lint's CI context
// (lint.ReadCIEnv) is empty, so a PR's target branch marks no fixture
// branch a PR boundary.
var cssHermeticEnv = []string{"GITHUB_TOKEN=", "CI_JOB_TOKEN=", "CI_PROJECT_ID=", "CI_DEFAULT_BRANCH=",
	"CI=", "GITHUB_ACTIONS=", "GITHUB_BASE_REF=", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME="}

const (
	cssNew = "records match; takes effect when spec/successor is accepted"
	// A resolved finding's note is the edge it resolves.
	cssNoteDC1 = "decision dc-1 supersedes spec/closed-feature#dc-1"
	cssNoteDC2 = "decision dc-2 supersedes spec/closed-story#ac-1"
)

// cssFinding is an expected computed finding: resolved (SUPERSEDED) or not,
// its exact text, and its exact note (none on an unresolved finding, so a
// note typed onto one is caught).
type cssFinding struct {
	resolved   bool
	text, note string
}

var (
	cssNewDC1 = cssFinding{true, cssNew, cssNoteDC1}
	cssNewDC2 = cssFinding{true, cssNew, cssNoteDC2}
)

// cssUnresolved is an expected unresolved finding with its reason's text.
func cssUnresolved(text string) cssFinding { return cssFinding{false, text, ""} }

// cssRun runs the built binary in dir and fails t unless it exits want.
func cssRun(t *testing.T, bin, dir string, want int, args ...string) string {
	t.Helper()
	stdout, stderr, code := runVerdiBinary(t, bin, dir, cssHermeticEnv, args...)
	if code != want {
		t.Fatalf("verdi %v = %d, want %d\nstdout:\n%s\nstderr:\n%s", args, code, want, stdout, stderr)
	}
	return stdout + stderr
}

// cssAlign runs `verdi align` on dir's design branch, checks the report's
// computed findings against want (every computed finding named, none
// else), and dispositions the judged finding by hand.
func cssAlign(t *testing.T, bin, dir, spec string, want map[string]cssFinding) string {
	t.Helper()
	cssRun(t, bin, dir, 0, "align")
	path := store.DecisionConflictReportPath(dir, store.ZoneActive, spec)
	got := map[string]cssFinding{}
	for _, f := range decodeDecisionReportFile(t, path).Findings {
		if f.Kind == artifact.FindingComputed {
			if f.Dispositioned() && f.Disposition != artifact.ConflictSuperseded {
				t.Fatalf("computed finding %+v: a closed-spec object edge resolves only SUPERSEDED", f)
			}
			got[f.ID] = cssFinding{resolved: f.Dispositioned(), text: f.Text, note: f.Note}
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("computed findings:\n got %+v\nwant %+v", got, want)
	}
	dispositionJudged(t, path)
	return path
}

// cssGate runs `verdi gate` on dir's design branch: it passes when every
// computed finding in want is resolved, and otherwise fails naming exactly
// the unresolved ids — a recomputed difference would print "differs"
// instead, so a failure here also proves align and the gate agree.
func cssGate(t *testing.T, bin, dir string, want map[string]cssFinding) {
	t.Helper()
	var open []string
	for id, f := range want {
		if !f.resolved {
			open = append(open, id)
		}
	}
	if len(open) == 0 {
		if out := cssRun(t, bin, dir, 0, "gate"); !strings.Contains(out, "gate: PASS") {
			t.Fatalf("gate output = %s, want gate: PASS", out)
		}
		return
	}
	sort.Strings(open)
	out := cssRun(t, bin, dir, 1, "gate")
	if w := "undispositioned/unresolved finding(s): [" + strings.Join(open, " ") + "]"; !strings.Contains(out, w) {
		t.Fatalf("gate output = %s\nwant it to name %q", out, w)
	}
}

// TestObjSupersedeE2E_AlignAndGate drives lint, align, and gate through the
// built binary on each scenario's design branch: both new replacements (a
// closed feature's decision, a closed story's criterion) resolve SUPERSEDED
// as proposed, never as in force, and the gate passes; a revision carries
// them under S1's conflict and date, with no new conflict; and every §8
// refusal leaves its edge unresolved with its named reason, so the gate
// fails naming it.
func TestObjSupersedeE2E_AlignAndGate(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	carried := func(conflict, note string) cssFinding {
		return cssFinding{true, "carries the replacement established by spec/successor (conflict/" + conflict + ", since 2024-02-15)", note}
	}
	tests := []struct {
		name, scenario, branch, spec string
		want                         map[string]cssFinding
	}{
		{"not yet accepted: new replacements take effect at acceptance", "proposed", "", "successor",
			map[string]cssFinding{gdcDC1: cssNewDC1, gdcDC2: cssNewDC2}},
		{"carried through a revision", "chain", "design/successor-v2", "successor-v2",
			map[string]cssFinding{gdcDC1: carried("successor-closed-feature", cssNoteDC1), gdcDC2: carried("successor-closed-story", cssNoteDC2)}},
		{"carried through an amending revision", "chain", "design/successor-v3", "successor-v3",
			map[string]cssFinding{gdcDC1: carried("successor-closed-feature", cssNoteDC1), gdcDC2: carried("successor-closed-story", cssNoteDC2)}},
		{"no conflict", "no-conflict", "", "successor",
			map[string]cssFinding{gdcDC1: cssUnresolved("no conflict challenges spec/closed-feature#dc-1"), gdcDC2: cssNewDC2}},
		{"open conflict", "conflict-open", "", "successor",
			map[string]cssFinding{gdcDC1: cssUnresolved("the conflict conflict/successor-closed-feature is not superseded"), gdcDC2: cssNewDC2}},
		{"dismissed conflict", "conflict-dismissed", "", "successor",
			map[string]cssFinding{gdcDC1: cssUnresolved("the conflict conflict/successor-closed-feature is not superseded"), gdcDC2: cssNewDC2}},
		{"resolved_by another spec", "resolved-by-other", "", "successor",
			map[string]cssFinding{gdcDC1: cssUnresolved("the conflict conflict/successor-closed-feature's resolved_by names another spec, spec/other-feature"), gdcDC2: cssNewDC2}},
		{"a conflict challenging another spec's object", "conflict-spans-specs", "", "successor",
			map[string]cssFinding{gdcDC1: cssUnresolved("the conflict challenges objects of more than one spec"), gdcDC2: cssUnresolved("the conflict challenges objects of more than one spec")}},
		{"a challenged fragment with no edge", "unmatched-challenge", "", "successor",
			map[string]cssFinding{gdcDC1: cssNewDC1, gdcDC2: cssNewDC2,
				"completeness-conflict--successor-closed-feature-spec--closed-feature-ac-1": cssUnresolved("conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge")}},
		{"an edge to an undeclared object", "undeclared-object", "", "successor",
			map[string]cssFinding{"edge-dc-1-supersedes-spec--closed-feature-dc-9": cssUnresolved("the object spec/closed-feature#dc-9 is not declared"), gdcDC2: cssNewDC2}},
		{"an edge to a constraint", "constraint-target", "", "successor",
			map[string]cssFinding{"edge-dc-1-supersedes-spec--closed-feature-co-1": cssUnresolved("the object spec/closed-feature#co-1 is not an acceptance criterion or a decision"), gdcDC2: cssNewDC2}},
		{"target not closed", "target-not-closed", "", "successor",
			map[string]cssFinding{"edge-dc-1-supersedes-spec--other-feature-dc-1": cssUnresolved("the target spec spec/other-feature is not closed"), gdcDC2: cssNewDC2}},
		{"target already superseded", "already-superseded", "", "successor",
			map[string]cssFinding{gdcDC1: cssUnresolved("the object spec/closed-feature#dc-1 is already superseded by spec/prior-successor (conflict/prior-successor-closed-feature)"), gdcDC2: cssNewDC2}},
		{"unrelated reuse of S1's conflict", "unrelated", "", "unrelated",
			map[string]cssFinding{gdcDC1: cssUnresolved("the object spec/closed-feature#dc-1 is already superseded by spec/successor (conflict/successor-closed-feature)")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := scenario.Build(t, tc.scenario).Dir
			if tc.branch != "" {
				gdcGit(t, dir, "checkout", "-q", tc.branch)
			}
			cssAlign(t, bin, dir, tc.spec, tc.want)
			cssGate(t, bin, dir, tc.want)
			if tc.scenario == "chain" {
				entries, err := os.ReadDir(filepath.Join(dir, ".verdi", "conflicts"))
				if err != nil {
					t.Fatal(err)
				}
				var names []string
				for _, e := range entries {
					names = append(names, e.Name())
				}
				if want := []string{"successor-closed-feature.md", "successor-closed-story.md"}; !reflect.DeepEqual(names, want) {
					t.Fatalf("conflicts = %v, want only S1's %v (a carried replacement files no new conflict)", names, want)
				}
			}
		})
	}
}

// cssView is what the built `verdi spec doc` prints, from the default
// branch, beside one object or decision id of the document ref: the
// view's supersession lines joined by " | ", as TestIndex_Scenarios pins
// them, or "" for none.
type cssView struct{ ref, id, want string }

// cssViewLine matches one supersession line of `verdi spec doc`'s
// Markdown: a span whose test id starts with the object or decision id,
// holding the view's line with the refs it names linked in place. The
// trailing conflict link beside a since line is not part of the line.
var (
	cssViewLine = regexp.MustCompile(`<span class="objsupersede objsupersede--[^"]*" data-testid="objsupersede-([^"]*)" data-state="[^"]*">(.*?)</span>`)
	cssTag      = regexp.MustCompile(`<[^>]*>`)
)

// cssViews runs `verdi spec doc` on dir for each view's document, which it
// reads from the default branch, and checks the lines beside the view's id.
// The id's own anchor must be in the document, so a view that expects no
// line stands on its own: the object was rendered, and nothing is beside it.
func cssViews(t *testing.T, bin, dir string, repo *scenario.Repo, views []cssView) {
	t.Helper()
	docs := map[string]string{}
	for _, v := range views {
		doc, ok := docs[v.ref]
		if !ok {
			doc = cssRun(t, bin, dir, 0, "spec", "doc", v.ref, "--no-readiness")
			docs[v.ref] = doc
		}
		if anchor := `<a id="` + v.id + `"></a>`; !strings.Contains(doc, anchor) {
			t.Errorf("verdi spec doc %s renders no %s anchor for %s", v.ref, anchor, v.id)
		}
		var lines []string
		for _, m := range cssViewLine.FindAllStringSubmatch(doc, -1) {
			if strings.HasPrefix(m[1], v.id+"-") {
				lines = append(lines, html.UnescapeString(cssTag.ReplaceAllString(m[2], "")))
			}
		}
		if got, want := strings.Join(lines, " | "), cssAt(repo, v.want); got != want {
			t.Errorf("verdi spec doc %s, %s:\n got %q\nwant %q", v.ref, v.id, got, want)
		}
	}
}

// cssAt replaces "{step N}" in s with step N's commit as a reason names the
// point it evaluated (its 12-hex prefix, SI-281), and "{commit N}" with the
// whole commit, as a tie's witness names it.
func cssAt(repo *scenario.Repo, s string) string {
	for i, c := range repo.Steps {
		s = strings.ReplaceAll(s, fmt.Sprintf("{step %d}", i), c[:12])
		s = strings.ReplaceAll(s, fmt.Sprintf("{commit %d}", i), c)
	}
	return s
}

// TestObjSupersedeE2E_Landings drives each landing shape of the scenario
// fixture through the built binary (whole-wave review N-2). align and the
// gate evaluate a successor's own edges as new replacements, so they never
// show its in-force point; that point, and the "as of commit" witness of a
// supersession not in force, reach the CLI in `verdi spec doc`'s lines,
// which it reads from the default branch. Each row checks those lines as
// TestIndex_Scenarios pins them, then runs align and the gate on a design
// branch, asserting the outcome the package tests pin for that branch's
// head. That is TestEvaluate_EveryScenario's outcome for the scenario,
// except in late-close: its design branch is the pre-merge head, the
// commit target-not-closed checks out, so the row asserts that scenario's
// pinned outcome. A design branch the scenario does not define is cut at
// main, so align and the gate read the tree TestEvaluate_EveryScenario
// evaluates.
func TestObjSupersedeE2E_Landings(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	const (
		govF = "governed spec/closed-feature's completed work (closed 2024-01-10) | "
		govS = "governed spec/closed-story's completed work (closed 2024-01-10) | "
		govO = "governed spec/other-feature's completed work "
		byS  = "superseded since 2024-02-15 by spec/successor#dc-2"
		notE = "supersession not established: spec/successor's supersession was not in force at its acceptance: "
	)
	tests := []struct {
		name, scenario, branch, spec string
		cut                          bool // the test cuts branch at main
		want                         map[string]cssFinding
		views                        []cssView
	}{
		{name: "fast-forward: in force from the series' completing commit", scenario: "ff-landing", branch: "design/successor", spec: "successor",
			want: map[string]cssFinding{gdcDC1: cssNewDC1, gdcDC2: cssNewDC2},
			views: []cssView{
				{"spec/closed-feature", "dc-1", govF + "superseded since 2024-02-10 by spec/successor#dc-1"},
				{"spec/closed-story", "ac-1", govS + "superseded since 2024-02-10 by spec/successor#dc-2"},
				{"spec/closed-feature", "ac-1", ""},
				{"spec/successor", "dc-1", "supersedes spec/closed-feature#dc-1"},
				{"spec/successor", "dc-2", "supersedes spec/closed-story#ac-1"},
			}},
		{name: "rebase: in force from the replayed completing commit", scenario: "rebase-landing", branch: "design/successor", spec: "successor",
			want: map[string]cssFinding{gdcDC1: cssNewDC1, gdcDC2: cssNewDC2},
			views: []cssView{
				{"spec/closed-feature", "dc-1", govF + "superseded since 2024-02-15 by spec/successor#dc-1"},
				{"spec/closed-story", "ac-1", govS + "superseded since 2024-02-15 by spec/successor#dc-2"},
				{"spec/successor", "dc-1", "supersedes spec/closed-feature#dc-1"},
				{"spec/successor", "dc-2", "supersedes spec/closed-story#ac-1"},
			}},
		// F-1: the stale-base merge never puts the closed feature's
		// supersession in force, and a later successor replaces the object.
		{name: "stale base: not in force as of the merge, replaceable by a later successor", scenario: "stale-base", branch: "design/unrelated", spec: "unrelated",
			want: map[string]cssFinding{gdcDC1: {true, "records match; takes effect when spec/unrelated is accepted", cssNoteDC1}},
			views: []cssView{
				{"spec/closed-feature", "dc-1", ""},
				{"spec/closed-feature", "ac-1", ""},
				{"spec/closed-story", "ac-1", govS + byS},
				{"spec/successor", "dc-1", notE + "as of commit {step 3}, conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge"},
				{"spec/successor", "dc-2", "supersedes spec/closed-story#ac-1"},
			}},
		// BL-94: the series' first commit is the point, so the later edge
		// is not in force, and its reason names that commit.
		{name: "widening series: in force from its first commit; the later edge is not", scenario: "ff-widening-series", branch: "design/successor", spec: "successor",
			want: map[string]cssFinding{gdcDC1: cssNewDC1, gdcDC2: cssNewDC2,
				"edge-dc-3-supersedes-spec--closed-feature-ac-1": {true, cssNew, "decision dc-3 supersedes spec/closed-feature#ac-1"}},
			views: []cssView{
				{"spec/closed-feature", "dc-1", govF + "superseded since 2024-02-01 by spec/successor#dc-1"},
				{"spec/closed-feature", "ac-1", ""},
				{"spec/successor", "dc-3", notE + "as of commit {step 0}, spec/successor carries no edge to spec/closed-feature#ac-1"},
			}},
		// The views are the rows pinned for late-close-then-conflict,
		// which is this scenario plus a later filing; the history test
		// pins both at the archive commit. The design branch is the
		// pre-merge head, which is the commit target-not-closed checks
		// out, so its gate refuses the edge while the target is open.
		{name: "late close: in force from the archive commit", scenario: "late-close", branch: "design/successor", spec: "successor",
			want: map[string]cssFinding{"edge-dc-1-supersedes-spec--other-feature-dc-1": cssUnresolved("the target spec spec/other-feature is not closed"), gdcDC2: cssNewDC2},
			views: []cssView{
				{"spec/other-feature", "dc-1", govO + "(closed 2024-03-01) | superseded since 2024-03-01 by spec/successor#dc-1"},
				{"spec/closed-story", "ac-1", govS + byS},
				{"spec/successor", "dc-1", "supersedes spec/other-feature#dc-1"},
			}},
		// The scenario's one design branch, design/two, names no spec, so
		// the test cuts design/successor at main; the gate fails naming
		// exactly dc-1's edge. The text wraps the R2 witness that the
		// history test pins for the rival, spec/unrelated, in condition 5's
		// establishment reason (match_test) and the acceptance-unproven
		// text (reason_test).
		{name: "same-commit tie: acceptance unproven", scenario: "same-commit-tie", branch: "design/successor", spec: "successor", cut: true,
			want: map[string]cssFinding{gdcDC2: cssNewDC2,
				gdcDC1: cssUnresolved("acceptance unproven: spec/unrelated's establishment: spec/unrelated and spec/successor first match for spec/closed-feature#dc-1 at the same commit {commit 1}, so neither takes effect before the other")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := scenario.Build(t, tc.scenario)
			dir := repo.Dir
			cssViews(t, bin, dir, repo, tc.views)
			if tc.cut {
				gdcGit(t, dir, "checkout", "-q", "-b", tc.branch, "main")
			} else {
				gdcGit(t, dir, "checkout", "-q", tc.branch)
			}
			want := map[string]cssFinding{}
			for id, f := range tc.want {
				f.text = cssAt(repo, f.text)
				want[id] = f
			}
			cssAlign(t, bin, dir, tc.spec, want)
			cssGate(t, bin, dir, want)
		})
	}
}

// TestObjSupersedeE2E_RepeatedChallenge proves a conflict listing the same
// unmatched challenged fragment more than once is a verdict, never an
// operational failure (L4 review a, I-1): with the repeat committed, `verdi
// align` exits 0 and writes ONE completeness finding for that (conflict,
// fragment), and `verdi gate` exits 1 naming it.
func TestObjSupersedeE2E_RepeatedChallenge(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	const ac1 = "  - { type: challenges, ref: \"spec/closed-feature#ac-1\" }\n"
	want := map[string]cssFinding{gdcDC1: cssNewDC1, gdcDC2: cssNewDC2,
		"completeness-conflict--successor-closed-feature-spec--closed-feature-ac-1": cssUnresolved("conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge")}
	for _, tc := range []struct {
		name  string
		times int
	}{{"listed twice", 2}, {"listed three times", 3}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := scenario.Build(t, "unmatched-challenge").Dir
			p := filepath.Join(dir, ".verdi", "conflicts", "successor-closed-feature.md")
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), ac1) {
				t.Fatalf("test setup: the fixture conflict has no %q link", ac1)
			}
			if err := os.WriteFile(p, []byte(strings.Replace(string(b), ac1, strings.Repeat(ac1, tc.times), 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			gdcGit(t, dir, "commit", "-q", "--no-verify", "-am", "Repeat a challenged fragment")
			cssAlign(t, bin, dir, "successor", want)
			cssGate(t, bin, dir, want)
		})
	}
}

// TestObjSupersedeE2E_Lint proves VL-026 through the binary: the proposed
// successor lints clean, and each top-level-link refusal reports exactly
// one VL-026 finding, naming its clause and reason. The refusal there is
// lint's alone: a top-level link is not a declared edge on a decision
// object (03), so align computes only the decision edges, which resolve,
// and `verdi gate` passes.
func TestObjSupersedeE2E_Lint(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	for _, tc := range []struct {
		scenario string
		finding  string // the one VL-026 finding's clause and reason; "" lints clean
		computed map[string]cssFinding
	}{
		{"proposed", "", nil},
		{"top-level-supersedes",
			`clause (b): top-level supersedes link "spec/closed-feature#dc-1" targets #dc-1 of closed spec spec/closed-feature, but a top-level supersedes link never targets an object of a closed spec: that edge belongs on a decision (02 §Link taxonomy; VL-026)`,
			map[string]cssFinding{gdcDC2: cssNewDC2}},
		{"feature-fragment-link",
			`clause (a): top-level links[].ref "spec/closed-feature#ac-1" targets object fragment #ac-1, but a feature spec's top-level links target no object fragment (02 §Link taxonomy; VL-026)`,
			map[string]cssFinding{gdcDC1: cssNewDC1, gdcDC2: cssNewDC2}},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()
			dir := scenario.Build(t, tc.scenario).Dir
			code := 0
			if tc.finding != "" {
				code = 1
			}
			var vl026 []string
			for _, line := range strings.Split(cssRun(t, bin, dir, code, "lint"), "\n") {
				if strings.Contains(line, "VL-026") {
					vl026 = append(vl026, line)
				}
			}
			if tc.finding == "" {
				if len(vl026) != 0 {
					t.Fatalf("VL-026 findings = %q, want none", vl026)
				}
				return
			}
			if len(vl026) != 1 || !strings.Contains(vl026[0], tc.finding) {
				t.Fatalf("VL-026 findings = %q, want exactly one containing %q", vl026, tc.finding)
			}
			cssAlign(t, bin, dir, "successor", tc.computed)
			cssGate(t, bin, dir, tc.computed)
		})
	}
}

// TestObjSupersedeE2E_AcceptanceKeepsArchivedBytes proves the happy path
// changes no archived byte: after lint, align, and a passing gate on the
// successor's design branch, the branch (report included) is committed and
// merged into the default branch — acceptance — and every file of the
// closed specs' archive zone and obligations equals the fixture's bytes,
// with none added.
func TestObjSupersedeE2E_AcceptanceKeepsArchivedBytes(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := scenario.Build(t, "proposed").Dir
	cssRun(t, bin, dir, 0, "lint")
	cssAlign(t, bin, dir, "successor", map[string]cssFinding{gdcDC1: cssNewDC1, gdcDC2: cssNewDC2})
	cssRun(t, bin, dir, 0, "gate")
	gdcGit(t, dir, "add", "-A")
	gdcGit(t, dir, "commit", "-q", "--no-verify", "-m", "Record the decision-conflict report")
	gdcGit(t, dir, "checkout", "-q", "main")
	gdcGit(t, dir, "merge", "-q", "--no-ff", "--no-verify", "-m", "Accept spec/successor", "design/successor")

	m, err := scenario.Load(scenario.Dir())
	if err != nil {
		t.Fatal(err)
	}
	base, err := m.BaseFiles(scenario.Dir())
	if err != nil {
		t.Fatal(err)
	}
	want, got := map[string]string{}, map[string]string{}
	for _, prefix := range []string{".verdi/specs/archive/", ".verdi/obligations/"} {
		for p, content := range base {
			if strings.HasPrefix(p, prefix) {
				want[p] = content
			}
		}
		err := filepath.WalkDir(filepath.Join(dir, filepath.FromSlash(prefix)), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(p)
			rel, _ := filepath.Rel(dir, p)
			got[filepath.ToSlash(rel)] = string(data)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(want) == 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("archived bytes after acceptance differ from the fixture:\n got %v\nwant %v", keysOf(got), keysOf(want))
	}
}

func keysOf(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestObjSupersedeE2E_HandTypedDisposition proves a hand-typed `superseded`
// on a computed finding resolves nothing (design §5, BL-67): `verdi align`
// drops it, and a report presented to `verdi gate` with it unchanged by
// align fails, naming the difference.
func TestObjSupersedeE2E_HandTypedDisposition(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := scenario.Build(t, "no-conflict").Dir
	want := map[string]cssFinding{gdcDC1: cssUnresolved("no conflict challenges spec/closed-feature#dc-1"), gdcDC2: cssNewDC2}
	path := cssAlign(t, bin, dir, "successor", want)
	typeSuperseded := func() {
		editDecisionReport(t, path, func(fm *artifact.DecisionConflictFrontmatter) {
			f := gdcFinding(t, fm, gdcDC1)
			f.Disposition, f.Note = artifact.ConflictSuperseded, "typed by hand"
		})
	}

	typeSuperseded()
	cssAlign(t, bin, dir, "successor", want) // align drops the typed disposition and note

	typeSuperseded()
	out := cssRun(t, bin, dir, 1, "gate")
	if w := gdcDC1 + `: disposition is "superseded", the records compute none`; !strings.Contains(out, w) {
		t.Fatalf("gate output = %s\nwant it to name %q", out, w)
	}
}
