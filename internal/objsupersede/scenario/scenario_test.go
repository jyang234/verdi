package scenario

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// wantRootCommit is the store's root commit, the first base step's commit.
// Every scenario shares it and every committed frozen stamp names it, so a
// change to that step must fail here first.
const wantRootCommit = "d49dd630388ff05fe4cd7d4084c785045ba15689"

// wantChainBase and wantChainSteps pin the chain scenario's commits: the
// manifest's identity, dates, messages, and records determine them, and no
// ambient git state may move them (lane L3 review a M-2, M-7).
var (
	wantChainBase  = []string{wantRootCommit, "c6c5d8b9382b37e3b9a8a4569c41e464007830c4", "d1c45a0dec59ca0d50c7427032f4df5e4c1dbd75"}
	wantChainSteps = []string{"f648735b38fc62fb8c1991a623e10c6f6d193dfc", "f395292cec946de6f9d34b90e8925e5e7e1a7320",
		"ea27f1c8c10a37dd5664fc703f6a6a6e04384783", "ebf6bfbca5b3e42fad50114571fe4dd6fb5edd69",
		"02e7229636df4bdb1f42bac4b447677f513420f2", "0f5e7dbd8466d52bbadb973e9fe7682b4986a95d",
		"c6ed8609b67411d97b2fe251dc745259c941cebe"}
)

func TestLoad_CommittedManifest(t *testing.T) {
	m, err := Load(Dir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	names := sortedKeys(m.Scenarios)
	want := []string{"accepted", "already-superseded", "chain", "chain-drop", "chain-not-in-force", "conflict-dismissed",
		"conflict-open", "conflict-spans-specs", "constraint-target", "feature-fragment-link", "ff-landing", "no-conflict",
		"proposed", "rebase-landing", "resolved-by-other", "target-not-closed", "top-level-supersedes", "undeclared-object",
		"unmatched-challenge", "unrelated", "unrelated-accepted"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("scenarios %v, want %v", names, want)
	}
	files, err := m.Files(Dir(), "closed", "successor")
	if err != nil || len(files) != 5 || !strings.Contains(files[".verdi/specs/active/successor/spec.md"], "id: spec/successor\n") {
		t.Fatalf("Files(closed, successor) = %d files, %v", len(files), err)
	}
	base, err := m.BaseFiles(Dir())
	if err != nil {
		t.Fatal(err)
	}
	got := sortedKeys(base)
	wantBase := []string{".gitattributes", ".verdi/obligations/closed-story/ac-1--attestation.md",
		".verdi/specs/active/other-feature/spec.md", ".verdi/specs/archive/closed-feature/spec.md",
		".verdi/specs/archive/closed-story/spec.md", ".verdi/verdi.yaml"}
	if strings.Join(got, ",") != strings.Join(wantBase, ",") {
		t.Fatalf("BaseFiles %v, want %v", got, wantBase)
	}
}

// mutated writes the committed manifest, changed by mutate, into a fresh
// fixture directory that shares the committed records.
func mutated(t *testing.T, mutate func(m map[string]any)) string {
	t.Helper()
	good, err := os.ReadFile(filepath.Join(Dir(), "scenarios.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(good, &m); err != nil {
		t.Fatal(err)
	}
	mutate(m)
	bad, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(Dir(), "records"), filepath.Join(dir, "records")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenarios.json"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// obj and step reach into a decoded manifest.
func obj(v any, key string) map[string]any { return v.(map[string]any)[key].(map[string]any) }

func step(m map[string]any, scenario string, i int) map[string]any {
	if scenario == "" {
		return m["base"].([]any)[i].(map[string]any)
	}
	return obj(obj(m, "scenarios"), scenario)["steps"].([]any)[i].(map[string]any)
}

// retype makes st a step of another operation: it drops st's layers, moves,
// and merge and sets op to branch. It returns st.
func retype(st map[string]any, op, branch string) map[string]any {
	for _, k := range []string{"layers", "moves", "merge"} {
		delete(st, k)
	}
	st[op] = branch
	return st
}

func TestLoad_Negative(t *testing.T) {
	const identity, twoOps, badMove = "commit needs a name, an email, and an initial branch", "exactly one", "needs two distinct clean relative paths"
	const noFFDate = "a fast-forward step makes no commit, so it carries no date"
	tests := []struct {
		name   string
		mutate func(m map[string]any)
		want   string // the error's own check, so no other check masks it (re-review a m-4)
	}{
		{"unknown field", func(m map[string]any) { m["extra"] = 1 }, `"extra"`},
		{"no identity name", func(m map[string]any) { obj(m, "commit")["name"] = "" }, identity},
		{"no identity email", func(m map[string]any) { obj(m, "commit")["email"] = "" }, identity},
		{"no base step", func(m map[string]any) { m["base"] = []any{} }, "no base step"},
		{"a base step off the initial branch", func(m map[string]any) { step(m, "", 0)["branch"] = "design/x" }, "base step 0 must write layers or move paths on main"},
		{"a base step that merges", func(m map[string]any) {
			delete(step(m, "", 2), "moves")
			step(m, "", 2)["merge"] = "main"
		}, "base step 2 must write layers or move paths on main"},
		{"undefined base layer", func(m map[string]any) { step(m, "", 1)["layers"] = []any{"nope"} }, `base step 1: layer "nope" is not defined`},
		{"missing record file", func(m map[string]any) {
			obj(obj(m, "layers"), "unrelated")[".verdi/specs/active/unrelated/spec.md"] = "specs/no-such.md"
		}, `layer "unrelated": stat`},
		{"unclean repo path", func(m map[string]any) {
			obj(m, "layers")["x"] = map[string]any{"../unrelated/spec.md": "specs/unrelated.md"}
		}, `repo path "../unrelated/spec.md" is not a clean relative path`},
		{"absolute repo path", func(m map[string]any) { obj(m, "layers")["x"] = map[string]any{"/abs/spec.md": "specs/unrelated.md"} }, `repo path "/abs/spec.md" is not a clean relative path`},
		{"undefined step layer", func(m map[string]any) { step(m, "accepted", 0)["layers"] = []any{"no-such-layer"} }, `layer "no-such-layer" is not defined`},
		{"non-UTC date", func(m map[string]any) { step(m, "accepted", 1)["date"] = "2024-02-15T09:00:00+01:00" }, "is not UTC"},
		{"malformed author date", func(m map[string]any) { step(m, "chain", 1)["author_date"] = "yesterday" }, `date "yesterday"`},
		{"merge and layers together", func(m map[string]any) { step(m, "accepted", 1)["layers"] = []any{"successor"} }, twoOps},
		{"no operation", func(m map[string]any) { delete(step(m, "accepted", 0), "layers") }, twoOps},
		{"an unclean move", func(m map[string]any) { step(m, "", 2)["moves"].([]any)[0].(map[string]any)["to"] = "../x" }, badMove},
		{"a move onto itself", func(m map[string]any) {
			mv := step(m, "", 2)["moves"].([]any)[0].(map[string]any)
			mv["to"] = mv["from"]
		}, badMove},
		{"a step without a branch", func(m map[string]any) { step(m, "accepted", 0)["branch"] = "" }, `scenario "accepted" step 0: a step needs a branch and a message`},
		{"a step without a message", func(m map[string]any) { step(m, "accepted", 0)["message"] = "" }, `scenario "accepted" step 0: a step needs a branch and a message`},
		{"a scenario without a checkout", func(m map[string]any) { obj(obj(m, "scenarios"), "accepted")["checkout"] = "" }, `scenario "accepted" needs a checkout branch and at least one step`},
		{"a scenario without steps", func(m map[string]any) { obj(obj(m, "scenarios"), "accepted")["steps"] = []any{} }, `scenario "accepted" needs a checkout branch and at least one step`},
		{"a merge of a branch no step commits to", func(m map[string]any) { step(m, "accepted", 1)["merge"] = "design/nowhere" }, `merges "design/nowhere" before any step commits to it`},
		{"a checkout no step commits to", func(m map[string]any) {
			obj(obj(m, "scenarios"), "accepted")["checkout"] = "design/nowhere"
		}, `checks out "design/nowhere", which no step commits to`},
		{"a fast-forward step with a date", func(m map[string]any) {
			retype(step(m, "accepted", 1), "fast_forward", "design/successor")
		}, noFFDate},
		{"a fast-forward step with an author date", func(m map[string]any) {
			delete(retype(step(m, "chain", 1), "fast_forward", "design/successor"), "date")
		}, noFFDate},
		{"a rebase step with an author date", func(m map[string]any) {
			retype(step(m, "chain", 1), "rebase", "design/successor")
		}, "a rebase step keeps each replayed commit's author date, so it carries none"},
		{"a rebase step without a date", func(m map[string]any) {
			st := retype(step(m, "chain", 1), "rebase", "design/successor")
			delete(st, "author_date")
			delete(st, "date")
		}, `date ""`},
		{"a base step that fast-forwards", func(m map[string]any) {
			retype(step(m, "", 2), "fast_forward", "main")
		}, "base step 2 must write layers or move paths on main"},
		{"a base step that rebases", func(m map[string]any) { retype(step(m, "", 2), "rebase", "main") }, "base step 2 must write layers or move paths on main"},
		{"a fast-forward and a merge together", func(m map[string]any) { step(m, "accepted", 1)["fast_forward"] = "design/successor" }, twoOps},
		{"a rebase and layers together", func(m map[string]any) { step(m, "accepted", 0)["rebase"] = "main" }, twoOps},
		{"a fast-forward to a branch no step commits to", func(m map[string]any) {
			delete(retype(step(m, "accepted", 1), "fast_forward", "design/nowhere"), "date")
		}, `fast-forwards to "design/nowhere" before any step commits to it`},
		{"a rebase onto a branch no step commits to", func(m map[string]any) {
			retype(step(m, "accepted", 1), "rebase", "design/nowhere")
		}, `rebases onto "design/nowhere" before any step commits to it`},
		{"a rebase of a branch no step commits to", func(m map[string]any) {
			retype(step(m, "chain", 3), "rebase", "main")
		}, `rebases "design/successor-v2" before any step commits to it`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(mutated(t, tc.mutate)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load: %v, want an error containing %q", err, tc.want)
			}
		})
	}
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("Load accepted a directory with no manifest")
	}
	if _, err := (&Manifest{}).Files(Dir(), "nope"); err == nil {
		t.Fatal("Files accepted an undefined layer")
	}
}

// TestLoad_ErrorsAreDeterministic pins that Load reports the first problem
// in sorted order, whatever the map iteration order (review a M-8).
func TestLoad_ErrorsAreDeterministic(t *testing.T) {
	dir := mutated(t, func(m map[string]any) {
		for _, name := range []string{"unrelated", "accepted", "chain"} {
			obj(obj(m, "scenarios"), name)["checkout"] = ""
		}
	})
	for i := 0; i < 20; i++ {
		if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), `scenario "accepted"`) {
			t.Fatalf("run %d: %v, want the first scenario in sorted order", i, err)
		}
	}
}

// hostileGit sets hostile ambient git state: exported identities and dates
// (as git hooks export them), `git -c` settings passed down through
// GIT_CONFIG_COUNT, a global config reached through GIT_CONFIG_GLOBAL or
// HOME that sets merge.log and a commit encoding, and a non-UTC TZ.
func hostileGit(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(cfg, []byte("[merge]\n\tlog = true\n[i18n]\n\tcommitEncoding = ISO-8859-1\n[user]\n\tname = Ambient\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]string{{"GIT_AUTHOR_NAME", "Hook Author"}, {"GIT_AUTHOR_EMAIL", "hook@example.invalid"},
		{"GIT_AUTHOR_DATE", "@1790364264 -0400"}, {"GIT_COMMITTER_NAME", "Hook Author"},
		{"GIT_COMMITTER_EMAIL", "hook@example.invalid"}, {"GIT_COMMITTER_DATE", "@1790364264 -0400"},
		{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "i18n.commitEncoding"}, {"GIT_CONFIG_VALUE_0", "ISO-8859-1"},
		{"GIT_CONFIG_GLOBAL", cfg}, {"HOME", home}, {"XDG_CONFIG_HOME", home}, {"TZ", "America/New_York"}} {
		t.Setenv(kv[0], kv[1])
	}
}

// wantAllCommits is the SHA-256 of every scenario's base and step commits,
// one "name base steps" line per scenario in name order, from a build with
// no ambient git state. TestBuild_EveryScenario reproduces it under
// hostileGit, so no ambient setting moves any SHA of any scenario.
const wantAllCommits = "8e77b4ecf06e14689f201e8a79cf7d66d3a2695b2e6c325f352f473b4b79cfd3"

func TestBuild_EveryScenario(t *testing.T) {
	hostileGit(t)
	m, err := Load(Dir())
	if err != nil {
		t.Fatal(err)
	}
	var commits strings.Builder
	for _, name := range sortedKeys(m.Scenarios) {
		sc := m.Scenarios[name]
		t.Run(name, func(t *testing.T) {
			repo := Build(t, name)
			if repo.Base[0] != wantRootCommit || len(repo.Base) != len(m.Base) {
				t.Fatalf("base commits %v, want %d starting %s", repo.Base, len(m.Base), wantRootCommit)
			}
			if got := gitOut(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD"); got != sc.Checkout {
				t.Fatalf("checked out %q, want %q", got, sc.Checkout)
			}
			if got := gitOut(t, repo.Dir, "rev-parse", "refs/remotes/origin/main"); got != gitOut(t, repo.Dir, "rev-parse", "main") {
				t.Fatalf("origin/main %s does not track main", got)
			}
			if len(repo.Steps) != len(sc.Steps) {
				t.Fatalf("%d step commits, want %d", len(repo.Steps), len(sc.Steps))
			}
			fmt.Fprintf(&commits, "%s %s %s\n", name, strings.Join(repo.Base, ","), strings.Join(repo.Steps, ","))
		})
	}
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(commits.String()))); got != wantAllCommits {
		t.Fatalf("every scenario's commits hash to %s, want %s:\n%s", got, wantAllCommits, commits.String())
	}
}

// TestBuild_PinnedCommits pins the chain scenario's SHAs in a clean
// environment and under hostileGit.
func TestBuild_PinnedCommits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		hostile bool
	}{{"clean", false}, {"hostile ambient state", true}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hostile {
				hostileGit(t)
			}
			repo := Build(t, "chain")
			if strings.Join(repo.Base, ",") != strings.Join(wantChainBase, ",") || strings.Join(repo.Steps, ",") != strings.Join(wantChainSteps, ",") {
				t.Fatalf("chain commits\nbase  %q\nsteps %q\nwant  %q\n      %q", repo.Base, repo.Steps, wantChainBase, wantChainSteps)
			}
		})
	}
}

// TestMaterialize_ManifestDrivesCommits pins that the manifest's recorded
// identity and dates are the ones the commits carry: changing one moves
// exactly the SHAs that depend on it.
func TestMaterialize_ManifestDrivesCommits(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		mutate     func(m map[string]any)
		rootMoves  bool
		firstMoved int // the first chain step whose commit moves
	}{
		{"the identity", func(m map[string]any) { obj(m, "commit")["email"] = "other@verdi.invalid" }, true, 0},
		{"a merge's author date", func(m map[string]any) { step(m, "chain", 1)["author_date"] = "2024-02-13T09:00:00Z" }, false, 1},
		{"a step's message", func(m map[string]any) { step(m, "chain", 2)["message"] = "Edit it" }, false, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := mutated(t, tc.mutate)
			repo, err := Materialize(ctx, dir, t.TempDir(), "chain")
			if err != nil {
				t.Fatal(err)
			}
			if (repo.Base[0] != wantRootCommit) != tc.rootMoves {
				t.Fatalf("root %s, moved want %v", repo.Base[0], tc.rootMoves)
			}
			m, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			who := m.Commit.Name + " <" + m.Commit.Email + ">"
			for _, got := range strings.Split(gitOut(t, repo.Dir, "log", "--all", "--format=%an <%ae>|%cn <%ce>"), "\n") {
				if got != who+"|"+who {
					t.Fatalf("a commit by %q, want the manifest's %q as author and committer", got, who)
				}
			}
			for i, c := range repo.Steps {
				if (c != wantChainSteps[i]) != (i >= tc.firstMoved) {
					t.Fatalf("step %d commit %s vs pinned %s, want moved only from step %d", i, c, wantChainSteps[i], tc.firstMoved)
				}
			}
		})
	}
	if _, err := Materialize(ctx, Dir(), t.TempDir(), "no-such-scenario"); err == nil {
		t.Fatal("an undefined scenario materialized")
	}
}

// landingSteps is a successor's two-commit series (the spec, then its
// resolved conflicts) and its landing on main: a fast-forward, or a
// rebase onto a main that moved, then a fast-forward.
func landingSteps(rebase bool) []any {
	steps := []any{
		map[string]any{"branch": "design/successor", "date": "2024-02-01T09:00:00Z", "message": "Propose spec/successor", "layers": []any{"successor"}},
		map[string]any{"branch": "design/successor", "date": "2024-02-10T09:00:00Z", "message": "Resolve the conflicts", "layers": []any{"conflict-feature", "conflict-story"}},
	}
	if rebase {
		steps = append(steps,
			map[string]any{"branch": "main", "date": "2024-02-12T09:00:00Z", "message": "Move main", "layers": []any{"unrelated"}},
			map[string]any{"branch": "design/successor", "date": "2024-02-15T09:00:00Z", "message": "Rebase", "rebase": "main"})
	}
	return append(steps, map[string]any{"branch": "main", "message": "Land", "fast_forward": "design/successor"})
}

// checkLanding pins a landing without a merge commit: main is the series'
// last commit, main's history has no merge commit, main's first-parent
// chain is want (newest first) above the base, and each of those commits
// carries the author date, committer date, and subject in dates.
func checkLanding(t *testing.T, repo *Repo, want []string, dates []string) {
	t.Helper()
	last := repo.Steps[len(repo.Steps)-1]
	if got := gitOut(t, repo.Dir, "rev-parse", "main"); got != last || last != want[0] {
		t.Fatalf("main at %s, last step at %s, want %s", got, last, want[0])
	}
	if merges := gitOut(t, repo.Dir, "rev-list", "--merges", "main"); merges != "" {
		t.Fatalf("main's history has merge commits: %s", merges)
	}
	chain := strings.Fields(gitOut(t, repo.Dir, "rev-list", "--first-parent", "main"))
	base := []string{repo.Base[2], repo.Base[1], repo.Base[0]}
	if strings.Join(chain, ",") != strings.Join(append(slices.Clone(want), base...), ",") {
		t.Fatalf("main's first-parent chain %v, want %v then the base", chain, want)
	}
	for i, c := range want {
		if got := gitOut(t, repo.Dir, "log", "-1", "--format=%aI %cI %s", c); got != dates[i] {
			t.Errorf("commit %s: %q, want %q", c, got, dates[i])
		}
	}
}

// TestMaterialize_Landings pins the fast-forward and rebase operations
// (SI-270 as amended; whole-wave review F-4): each lands the series on
// main with no merge commit; a rebase replays it onto the moved main, each
// commit keeping its author date and taking the step's committer date.
func TestMaterialize_Landings(t *testing.T) {
	ctx := context.Background()
	for _, rebase := range []bool{false, true} {
		t.Run(fmt.Sprintf("rebase=%v", rebase), func(t *testing.T) {
			dir := mutated(t, func(m map[string]any) { obj(obj(m, "scenarios"), "accepted")["steps"] = landingSteps(rebase) })
			repo, err := Materialize(ctx, dir, t.TempDir(), "accepted")
			if err != nil {
				t.Fatal(err)
			}
			if !rebase {
				checkLanding(t, repo, []string{repo.Steps[1], repo.Steps[0]}, []string{
					"2024-02-10T09:00:00+00:00 2024-02-10T09:00:00+00:00 Resolve the conflicts",
					"2024-02-01T09:00:00+00:00 2024-02-01T09:00:00+00:00 Propose spec/successor"})
				return
			}
			replayed := gitOut(t, repo.Dir, "rev-parse", repo.Steps[3]+"^")
			if replayed == repo.Steps[0] || repo.Steps[3] == repo.Steps[1] {
				t.Fatalf("the rebase replayed nothing: %v", repo.Steps)
			}
			checkLanding(t, repo, []string{repo.Steps[3], replayed, repo.Steps[2]}, []string{
				"2024-02-10T09:00:00+00:00 2024-02-15T09:00:00+00:00 Resolve the conflicts",
				"2024-02-01T09:00:00+00:00 2024-02-15T09:00:00+00:00 Propose spec/successor",
				"2024-02-12T09:00:00+00:00 2024-02-12T09:00:00+00:00 Move main"})
		})
	}
}

// TestBuild_Landings pins the committed landings without a merge commit
// (whole-wave review F-4): ff-landing fast-forwards main to the
// successor's second commit, and rebase-landing replays both commits onto
// a main that moved, then fast-forwards.
func TestBuild_Landings(t *testing.T) {
	const propose, resolve = " Propose spec/successor", " Resolve the conflicts spec/successor challenges"
	ff := Build(t, "ff-landing")
	checkLanding(t, ff, []string{ff.Steps[1], ff.Steps[0]}, []string{
		"2024-02-10T09:00:00+00:00 2024-02-10T09:00:00+00:00" + resolve,
		"2024-02-01T09:00:00+00:00 2024-02-01T09:00:00+00:00" + propose})
	rb := Build(t, "rebase-landing")
	checkLanding(t, rb, []string{rb.Steps[3], gitOut(t, rb.Dir, "rev-parse", rb.Steps[3]+"^"), rb.Steps[2]}, []string{
		"2024-02-10T09:00:00+00:00 2024-02-15T09:00:00+00:00" + resolve,
		"2024-02-01T09:00:00+00:00 2024-02-15T09:00:00+00:00" + propose,
		"2024-02-12T09:00:00+00:00 2024-02-12T09:00:00+00:00 Move main"})
}

// TestMaterialize_GitErrors pins that a failing git step is returned as an
// error naming its command, never ignored (lane L3 re-review a m-5).
func TestMaterialize_GitErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(m map[string]any)
		want   string
	}{
		{"a move of a missing path", func(m map[string]any) {
			step(m, "", 2)["moves"].([]any)[0].(map[string]any)["from"] = ".verdi/specs/active/nowhere"
		}, "scenario: git mv .verdi/specs/active/nowhere"},
		{"a commit with nothing to commit", func(m map[string]any) { step(m, "accepted", 0)["layers"] = []any{"store"} }, "scenario: git commit"},
		{"a merge that conflicts", func(m map[string]any) {
			steps := step(m, "accepted", 0)
			obj(obj(m, "scenarios"), "accepted")["steps"] = []any{
				map[string]any{"branch": "design/a", "date": steps["date"], "message": "Write successor", "layers": []any{"successor"}},
				map[string]any{"branch": "main", "date": steps["date"], "message": "Write it otherwise", "layers": []any{"successor-edited"}},
				map[string]any{"branch": "main", "date": steps["date"], "message": "Merge", "merge": "design/a"},
			}
		}, "scenario: git merge"},
		{"a fast-forward of a branch that diverged", func(m map[string]any) {
			date := step(m, "accepted", 0)["date"]
			obj(obj(m, "scenarios"), "accepted")["steps"] = []any{
				map[string]any{"branch": "design/a", "date": date, "message": "Write successor", "layers": []any{"successor"}},
				map[string]any{"branch": "main", "date": date, "message": "Move main", "layers": []any{"unrelated"}},
				map[string]any{"branch": "main", "message": "Fast-forward", "fast_forward": "design/a"},
			}
		}, "scenario: git merge -q --ff-only design/a"},
		{"a rebase that conflicts", func(m map[string]any) {
			date := step(m, "accepted", 0)["date"]
			obj(obj(m, "scenarios"), "accepted")["steps"] = []any{
				map[string]any{"branch": "design/a", "date": date, "message": "Write successor", "layers": []any{"successor"}},
				map[string]any{"branch": "main", "date": date, "message": "Write it otherwise", "layers": []any{"successor-edited"}},
				map[string]any{"branch": "design/a", "date": date, "message": "Rebase", "rebase": "main"},
			}
		}, "scenario: git rebase -q main"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Materialize(context.Background(), mutated(t, tc.mutate), t.TempDir(), "accepted")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Materialize: %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

// TestBuild_Dates pins the dates SI-270 reads: the chain's accepting merge
// and the base closing step each carry a committer date distinct from
// their author date, and the closed specs are accepted in the active zone
// before a later, distinct archive move (review a I-2).
func TestBuild_Dates(t *testing.T) {
	repo := Build(t, "chain")
	for _, tc := range []struct{ commit, want string }{
		{repo.Steps[1], "2024-02-14T09:00:00+00:00 2024-02-15T09:00:00+00:00 Accept spec/successor"},
		{repo.Base[1], "2024-01-01T00:00:00+00:00 2024-01-01T00:00:00+00:00 Base layer closed"},
		{repo.Base[2], "2024-01-09T09:00:00+00:00 2024-01-10T09:00:00+00:00 Close spec/closed-feature and spec/closed-story"},
	} {
		if got := gitOut(t, repo.Dir, "log", "-1", "--format=%aI %cI %s", tc.commit); got != tc.want {
			t.Errorf("commit %s: %q, want %q", tc.commit, got, tc.want)
		}
	}
	for _, p := range []string{".verdi/specs/active/closed-feature/spec.md", ".verdi/specs/active/closed-story/spec.md"} {
		if gitOut(t, repo.Dir, "ls-tree", "--name-only", repo.Base[1], "--", p) != p || gitOut(t, repo.Dir, "ls-tree", "--name-only", repo.Base[2], "--", p) != "" {
			t.Errorf("%s is not active at the closed layer and gone after the archive move", p)
		}
	}
	names := strings.Fields(gitOut(t, repo.Dir, "ls-tree", "-r", "--name-only", repo.Base[2], "--", ".verdi/specs/archive"))
	sort.Strings(names)
	if strings.Join(names, ",") != ".verdi/specs/archive/closed-feature/spec.md,.verdi/specs/archive/closed-story/spec.md" {
		t.Errorf("archive after the move: %v", names)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
