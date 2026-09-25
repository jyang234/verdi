package scenario

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// wantRootCommit is the store's root commit, the first base layer's
// fixturegit commit. Every scenario shares it and every committed frozen
// stamp names it, so a change to that layer must fail here first.
const wantRootCommit = "d49dd630388ff05fe4cd7d4084c785045ba15689"

func TestLoad_CommittedManifest(t *testing.T) {
	m, err := Load(Dir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var names []string
	for n := range m.Scenarios {
		names = append(names, n)
	}
	sort.Strings(names)
	want := []string{"accepted", "already-superseded", "chain", "chain-drop", "conflict-dismissed", "conflict-open",
		"conflict-spans-specs", "constraint-target", "feature-fragment-link", "no-conflict", "proposed",
		"resolved-by-other", "target-not-closed", "top-level-supersedes", "undeclared-object",
		"unmatched-challenge", "unrelated", "unrelated-accepted"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("scenarios %v, want %v", names, want)
	}
	files, err := m.Files(Dir(), "closed", "successor")
	if err != nil || len(files) != 5 || !strings.Contains(files[".verdi/specs/active/successor/spec.md"], "id: spec/successor\n") {
		t.Fatalf("Files(closed, successor) = %d files, %v", len(files), err)
	}
}

func TestLoad_Negative(t *testing.T) {
	good, err := os.ReadFile(filepath.Join(Dir(), "scenarios.json"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, old, new string }{
		{"unknown field", `"base": [`, `"extra": 1, "base": [`},
		{"undefined base", `"closed"
  ],`, `"nope"
  ],`},
		{"missing record file", `"specs/closed-story.md"`, `"specs/no-such.md"`},
		{"undefined step layer", `"prior"
          ]`, `"no-such-layer"
          ]`},
		{"non-UTC date", `"2024-02-15T09:00:00Z"`, `"2024-02-15T09:00:00+01:00"`},
		{"merge and layers together", `"merge": "design/successor",`, `"merge": "design/successor", "layers": ["successor"],`},
		{"unclean repo path", `".verdi/specs/active/unrelated/spec.md"`, `"../unrelated/spec.md"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad := strings.Replace(string(good), tc.old, tc.new, 1)
			if bad == string(good) {
				t.Fatalf("mutation %q did not apply", tc.old)
			}
			dir := t.TempDir()
			if err := os.Symlink(filepath.Join(Dir(), "records"), filepath.Join(dir, "records")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "scenarios.json"), []byte(bad), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(dir); err == nil {
				t.Fatal("Load accepted a broken manifest")
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

func TestBuild_EveryScenario(t *testing.T) {
	m, err := Load(Dir())
	if err != nil {
		t.Fatal(err)
	}
	for name, sc := range m.Scenarios {
		t.Run(name, func(t *testing.T) {
			repo := Build(t, name)
			if repo.Base[0] != wantRootCommit {
				t.Fatalf("root commit %s, want %s", repo.Base[0], wantRootCommit)
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
		})
	}
}

func TestBuild_Deterministic(t *testing.T) {
	a, b := Build(t, "chain"), Build(t, "chain")
	if strings.Join(a.Steps, ",") != strings.Join(b.Steps, ",") {
		t.Fatalf("two builds of one scenario differ: %v vs %v", a.Steps, b.Steps)
	}
	dates := gitOut(t, a.Dir, "log", "--first-parent", "--format=%cI %s", "main")
	for _, want := range []string{"2024-02-15T09:00:00Z Accept spec/successor", "2024-01-01T00:00:00Z"} {
		if !strings.Contains(strings.ReplaceAll(dates, "+00:00", "Z"), want) {
			t.Fatalf("main's first-parent log lacks %q:\n%s", want, dates)
		}
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
