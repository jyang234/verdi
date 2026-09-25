// objsupersede_e2e_test.go proves closed-spec object supersession through
// the built binary (design §8, "Test layers": CLI paths are end-to-end Go
// tests driving the built binary), over the committed scenario fixture
// testdata/objsupersede and its builder: lint, align, and gate on the
// successor's design branch, acceptance, the refusals §8 lists, a carried
// replacement through revisions, unrelated reuse, and a hand-typed
// disposition. The judged sweep runs unconfigured (the scenario store sets
// no align.judge_cmd, and judge_required defaults false), and its absence
// finding is dispositioned by hand, as a reviewer does, so the gate turns
// on the computed section.
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/store"
)

// cssHermeticEnv keeps a developer's forge credentials and CI variables
// from reaching the binary: no forge is built, and the default branch
// resolves from the fixture's own origin/main.
var cssHermeticEnv = []string{"GITHUB_TOKEN=", "CI_JOB_TOKEN=", "CI_PROJECT_ID=", "CI_DEFAULT_BRANCH="}

const cssNew = "records match; takes effect when spec/successor is accepted"

// cssFinding is an expected computed finding: resolved (SUPERSEDED) or not,
// and its exact text.
type cssFinding struct {
	resolved bool
	text     string
}

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
			got[f.ID] = cssFinding{resolved: f.Dispositioned(), text: f.Text}
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
	carried := func(conflict string) cssFinding {
		return cssFinding{true, "carries the replacement established by spec/successor (conflict/" + conflict + ", since 2024-02-15)"}
	}
	newDC2 := cssFinding{true, cssNew}
	unresolved := func(text string) cssFinding { return cssFinding{false, text} }
	tests := []struct {
		name, scenario, branch, spec string
		want                         map[string]cssFinding
	}{
		{"not yet accepted: new replacements take effect at acceptance", "proposed", "", "successor",
			map[string]cssFinding{gdcDC1: {true, cssNew}, gdcDC2: newDC2}},
		{"carried through a revision", "chain", "design/successor-v2", "successor-v2",
			map[string]cssFinding{gdcDC1: carried("successor-closed-feature"), gdcDC2: carried("successor-closed-story")}},
		{"carried through an amending revision", "chain", "design/successor-v3", "successor-v3",
			map[string]cssFinding{gdcDC1: carried("successor-closed-feature"), gdcDC2: carried("successor-closed-story")}},
		{"no conflict", "no-conflict", "", "successor",
			map[string]cssFinding{gdcDC1: unresolved("no conflict challenges spec/closed-feature#dc-1"), gdcDC2: newDC2}},
		{"open conflict", "conflict-open", "", "successor",
			map[string]cssFinding{gdcDC1: unresolved("the conflict conflict/successor-closed-feature is not superseded"), gdcDC2: newDC2}},
		{"dismissed conflict", "conflict-dismissed", "", "successor",
			map[string]cssFinding{gdcDC1: unresolved("the conflict conflict/successor-closed-feature is not superseded"), gdcDC2: newDC2}},
		{"resolved_by another spec", "resolved-by-other", "", "successor",
			map[string]cssFinding{gdcDC1: unresolved("the conflict conflict/successor-closed-feature's resolved_by names another spec, spec/other-feature"), gdcDC2: newDC2}},
		{"a conflict challenging another spec's object", "conflict-spans-specs", "", "successor",
			map[string]cssFinding{gdcDC1: unresolved("the conflict challenges objects of more than one spec"), gdcDC2: unresolved("the conflict challenges objects of more than one spec")}},
		{"a challenged fragment with no edge", "unmatched-challenge", "", "successor",
			map[string]cssFinding{gdcDC1: {true, cssNew}, gdcDC2: newDC2,
				"completeness-conflict--successor-closed-feature-spec--closed-feature-ac-1": unresolved("conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge")}},
		{"an edge to an undeclared object", "undeclared-object", "", "successor",
			map[string]cssFinding{"edge-dc-1-supersedes-spec--closed-feature-dc-9": unresolved("the object spec/closed-feature#dc-9 is not declared"), gdcDC2: newDC2}},
		{"an edge to a constraint", "constraint-target", "", "successor",
			map[string]cssFinding{"edge-dc-1-supersedes-spec--closed-feature-co-1": unresolved("the object spec/closed-feature#co-1 is not an acceptance criterion or a decision"), gdcDC2: newDC2}},
		{"target not closed", "target-not-closed", "", "successor",
			map[string]cssFinding{"edge-dc-1-supersedes-spec--other-feature-dc-1": unresolved("the target spec spec/other-feature is not closed"), gdcDC2: newDC2}},
		{"target already superseded", "already-superseded", "", "successor",
			map[string]cssFinding{gdcDC1: unresolved("the object spec/closed-feature#dc-1 is already superseded by spec/prior-successor (conflict/prior-successor-closed-feature)"), gdcDC2: newDC2}},
		{"unrelated reuse of S1's conflict", "unrelated", "", "unrelated",
			map[string]cssFinding{gdcDC1: unresolved("the object spec/closed-feature#dc-1 is already superseded by spec/successor (conflict/successor-closed-feature)")}},
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

// TestObjSupersedeE2E_RepeatedChallenge proves a conflict listing the same
// unmatched challenged fragment more than once is a verdict, never an
// operational failure (L4 review a, I-1): with the repeat committed, `verdi
// align` exits 0 and writes ONE completeness finding for that (conflict,
// fragment), and `verdi gate` exits 1 naming it.
func TestObjSupersedeE2E_RepeatedChallenge(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	const ac1 = "  - { type: challenges, ref: \"spec/closed-feature#ac-1\" }\n"
	want := map[string]cssFinding{gdcDC1: {true, cssNew}, gdcDC2: {true, cssNew},
		"completeness-conflict--successor-closed-feature-spec--closed-feature-ac-1": {false, "conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge"}}
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
// successor lints clean, and the two top-level-link refusals report VL-026.
func TestObjSupersedeE2E_Lint(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	for _, tc := range []struct {
		scenario string
		code     int
	}{{"proposed", 0}, {"top-level-supersedes", 1}, {"feature-fragment-link", 1}} {
		t.Run(tc.scenario, func(t *testing.T) {
			t.Parallel()
			out := cssRun(t, bin, scenario.Build(t, tc.scenario).Dir, tc.code, "lint")
			if got := strings.Contains(out, "VL-026"); got != (tc.code == 1) {
				t.Fatalf("lint output = %s; VL-026 reported = %v, want %v", out, got, tc.code == 1)
			}
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
	cssAlign(t, bin, dir, "successor", map[string]cssFinding{gdcDC1: {true, cssNew}, gdcDC2: {true, cssNew}})
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
	want := map[string]cssFinding{gdcDC1: {false, "no conflict challenges spec/closed-feature#dc-1"}, gdcDC2: {true, cssNew}}
	path := cssAlign(t, bin, dir, "successor", want)
	typeSuperseded := func() {
		editDecisionReport(t, path, func(fm *artifact.DecisionConflictFrontmatter) {
			f := gdcFinding(t, fm, gdcDC1)
			f.Disposition, f.Note = artifact.ConflictSuperseded, "typed by hand"
		})
	}

	typeSuperseded()
	cssAlign(t, bin, dir, "successor", want) // align drops the typed disposition

	typeSuperseded()
	out := cssRun(t, bin, dir, 1, "gate")
	if w := gdcDC1 + `: disposition is "superseded", the records compute none`; !strings.Contains(out, w) {
		t.Fatalf("gate output = %s\nwant it to name %q", out, w)
	}
}
