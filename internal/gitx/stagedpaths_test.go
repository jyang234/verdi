package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestStagedPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("lists staged additions modifications and deletions deterministically", func(t *testing.T) {
		repo := buildRepo(t)
		added := "z-added\nwith-newline.txt"

		if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("staged modification\n"), 0o644); err != nil {
			t.Fatalf("modifying a.txt: %v", err)
		}
		if err := os.Remove(filepath.Join(repo.Dir, "dir", "b.txt")); err != nil {
			t.Fatalf("deleting dir/b.txt: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo.Dir, added), []byte("staged addition\n"), 0o644); err != nil {
			t.Fatalf("adding %q: %v", added, err)
		}
		if err := AddPaths(ctx, repo.Dir, "a.txt", "dir/b.txt", added); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}

		got, err := StagedPaths(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("StagedPaths: %v", err)
		}
		want := []string{"a.txt", "dir/b.txt", added}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("StagedPaths = %#v, want sorted add/modify/delete paths %#v", got, want)
		}
	})

	t.Run("clean index is empty", func(t *testing.T) {
		repo := buildRepo(t)

		got, err := StagedPaths(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("StagedPaths: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("StagedPaths = %#v, want no paths", got)
		}
	})
}

func TestStagedPaths_OutsideRepository(t *testing.T) {
	_, err := StagedPaths(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("StagedPaths outside a repository: want operational error, got nil")
	}
}

// TestParseStagedStatus table-drives the porcelain-v1 -z parser over every
// status shape git can emit, including the ones no fixture repository reaches
// conveniently (an unmerged conflict, a copy, an ignored entry) and the
// malformed inputs that must surface as errors rather than as a silently
// short answer — an under-reported index is exactly the fail-open this guard
// exists to prevent.
func TestParseStagedStatus(t *testing.T) {
	nul := "\x00"
	cases := []struct {
		name    string
		out     string
		want    []string
		wantErr bool
	}{
		{name: "empty output", out: ""},
		{name: "trailing NUL only", out: nul},
		{name: "staged modification", out: "M  a.txt" + nul, want: []string{"a.txt"}},
		{name: "staged addition with worktree edit on top", out: "AM a.txt" + nul, want: []string{"a.txt"}},
		{name: "staged deletion", out: "D  dir/b.txt" + nul, want: []string{"dir/b.txt"}},
		{name: "worktree-only modification is not staged", out: " M a.txt" + nul},
		{name: "untracked is not staged", out: "?? scratch.txt" + nul},
		{name: "ignored is not staged", out: "!! build/out" + nul},
		{name: "unmerged conflict differs from HEAD", out: "UU merged.txt" + nul, want: []string{"merged.txt"}},
		{
			name: "rename contributes both sides",
			out:  "R  new.txt" + nul + "old.txt" + nul,
			want: []string{"new.txt", "old.txt"},
		},
		{
			name: "copy contributes only its destination",
			out:  "C  copy.txt" + nul + "source.txt" + nul,
			want: []string{"copy.txt"},
		},
		{
			name: "paths carrying newlines and non-ASCII bytes survive -z",
			out:  "A  café-\nnl.txt" + nul + "M  plain.txt" + nul,
			want: []string{"café-\nnl.txt", "plain.txt"},
		},
		{
			name: "mixed entries keep only the staged ones",
			out:  "M  a.txt" + nul + " M b.txt" + nul + "?? c.txt" + nul + "A  d.txt" + nul,
			want: []string{"a.txt", "d.txt"},
		},
		{name: "entry too short to carry a path", out: "M  " + nul, wantErr: true},
		{name: "entry missing the status separator", out: "MXa.txt" + nul, wantErr: true},
		{name: "rename with no original-path field", out: "R  new.txt" + nul, wantErr: true},
		{name: "rename with an empty original-path field", out: "R  new.txt" + nul + nul, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseStagedStatus([]byte(tc.out))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseStagedStatus(%q) = %#v, nil; want an error rather than a silently short answer", tc.out, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseStagedStatus(%q): %v", tc.out, err)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseStagedStatus(%q) = %#v, want %#v", tc.out, got, tc.want)
			}
		})
	}
}

// runGitForTest execs git in dir, failing the test on a non-zero exit. Used
// only to arrange fixture states gitx itself exposes no primitive for (a
// gitlink index entry, a repository-local config, a staged rename).
func runGitForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v (dir %s): %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// stageGitlinkBump seeds repo with a committed gitlink (mode 160000) entry at
// `sub` plus a .gitmodules describing it, then stages a pointer bump to a
// second commit — the exact shape a submodule pointer update takes, built
// entirely with `git update-index --cacheinfo` so the fixture stays hermetic
// (no clone, no file:// protocol, no network). Returns nothing: the staged
// index entry IS the fixture.
func stageGitlinkBump(t *testing.T, dir, gitmodules string) {
	t.Helper()
	first := strings.TrimSpace(runGitForTest(t, dir, "rev-parse", "HEAD"))
	runGitForTest(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+first+",sub")
	if err := os.WriteFile(filepath.Join(dir, ".gitmodules"), []byte(gitmodules), 0o644); err != nil {
		t.Fatalf("writing .gitmodules: %v", err)
	}
	runGitForTest(t, dir, "add", "--", ".gitmodules")
	runGitForTest(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "seed gitlink")

	tree := strings.TrimSpace(runGitForTest(t, dir, "rev-parse", "HEAD^{tree}"))
	second := strings.TrimSpace(runGitForTest(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit-tree", tree, "-p", first, "-m", "submodule tip"))
	runGitForTest(t, dir, "update-index", "--cacheinfo", "160000,"+second+",sub")
}

// TestStagedPaths_SeesStagedSubmoduleBumpUnderIgnoreConfigs is the red-first
// proof that the guard's primitive may not be `git diff --cached`: ORDINARY
// git configurations make that command report an empty index while `git
// commit` still records the staged submodule pointer bump, so a fail-open
// StagedPaths lets a closure commit absorb unowned staged work. verdi runs
// inside other people's repositories, so "this repo has no submodules" is not
// a defense.
//
// It also pins the reason stagedPathsArgs passes NO --ignore-submodules flag.
// Every one of these configurations leaves the INDEX column reporting the bump
// — that column is the only one parseStagedStatus reads — so forcing
// `--ignore-submodules=none` changes no answer the guard can see while making
// status recurse into every submodule worktree. Asserting the property here is
// what keeps the doc comment honest: deleting the flag used to leave this
// whole suite green, which meant a comment claiming a property no test proved.
func TestStagedPaths_SeesStagedSubmoduleBumpUnderIgnoreConfigs(t *testing.T) {
	ctx := context.Background()
	const plainGitmodules = "[submodule \"sub\"]\n\tpath = sub\n\turl = ./sub\n"
	const ignoringGitmodules = "[submodule \"sub\"]\n\tpath = sub\n\turl = ./sub\n\tignore = all\n"

	cases := []struct {
		name       string
		gitmodules string
		config     [][2]string
		// diffIndexAlsoBlind records whether `git diff-index --cached` — the
		// obvious "but surely the plumbing form is safe" answer — is blinded
		// too. Asserted only where the doc claims it, never in the other
		// direction, so this test never over-pins git's own behaviour.
		diffIndexAlsoBlind bool
	}{
		{
			name:       "diff.ignoreSubmodules=all in repository config",
			gitmodules: plainGitmodules,
			config:     [][2]string{{"diff.ignoreSubmodules", "all"}},
		},
		{
			name:               "committed .gitmodules carrying ignore = all (every clone inherits it)",
			gitmodules:         ignoringGitmodules,
			diffIndexAlsoBlind: true,
		},
		{
			name:               "submodule.<name>.ignore=all in repository config",
			gitmodules:         plainGitmodules,
			config:             [][2]string{{"submodule.sub.ignore", "all"}},
			diffIndexAlsoBlind: true,
		},
		{
			name:               "every ignore scope at once",
			gitmodules:         ignoringGitmodules,
			config:             [][2]string{{"diff.ignoreSubmodules", "all"}, {"submodule.sub.ignore", "all"}},
			diffIndexAlsoBlind: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildRepo(t)
			for _, kv := range tc.config {
				runGitForTest(t, repo.Dir, "config", kv[0], kv[1])
			}
			stageGitlinkBump(t, repo.Dir, tc.gitmodules)

			got, err := StagedPaths(ctx, repo.Dir)
			if err != nil {
				t.Fatalf("StagedPaths: %v", err)
			}
			found := false
			for _, p := range got {
				if p == "sub" {
					found = true
				}
			}
			if !found {
				t.Fatalf("StagedPaths = %#v, want the staged submodule pointer bump %q named; git would still commit it, so an empty answer here is a fail-open safety guard", got, "sub")
			}

			// The other half of the doc's claim, previously unasserted: the diff
			// forms this guard rejected really are blind here.
			if out := runGitForTest(t, repo.Dir, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "" {
				t.Fatalf("`git diff --cached --name-only` = %q, want empty — the doc's stated reason for rejecting it as the primitive", out)
			}
			if tc.diffIndexAlsoBlind {
				if out := runGitForTest(t, repo.Dir, "diff-index", "--cached", "--name-only", "HEAD"); strings.TrimSpace(out) != "" {
					t.Fatalf("`git diff-index --cached --name-only HEAD` = %q, want empty — the doc claims the plumbing form is measurably not a fix here either", out)
				}
			}
		})
	}
}

// TestStagedPaths_SeesStagedPathsAboveDirUnderDiffRelative is the red-first
// proof for the second fail-open configuration: with diff.relative=true and a
// store root BELOW the git root (store.FindRoot walks up to the nearest
// .verdi, and gitx runs with cmd.Dir = that root), `git diff --cached` hides
// every staged path outside dir — while `git commit` run from the same
// directory still records all of them.
func TestStagedPaths_SeesStagedPathsAboveDirUnderDiffRelative(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			"above.txt":         "above the store root\n",
			"store/inside.txt":  "inside the store root\n",
			"store/.verdi/keep": "store marker\n",
		},
		Message: "seed a store root below the git root",
	}})
	runGitForTest(t, repo.Dir, "config", "diff.relative", "true")

	if err := os.WriteFile(filepath.Join(repo.Dir, "above.txt"), []byte("staged edit above the store root\n"), 0o644); err != nil {
		t.Fatalf("editing above.txt: %v", err)
	}
	// Staged from the git root, exactly as an operator would; the guard runs
	// with cmd.Dir = the store root one level below.
	runGitForTest(t, repo.Dir, "add", "--", "above.txt")
	storeRoot := filepath.Join(repo.Dir, "store")

	got, err := StagedPaths(ctx, storeRoot)
	if err != nil {
		t.Fatalf("StagedPaths: %v", err)
	}
	want := []string{"above.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StagedPaths(store root below git root, diff.relative=true) = %#v, want %#v — a staged path above dir is still committed by git, so hiding it is a fail-open safety guard", got, want)
	}
}

// TestRepoPrefix pins the one relation between StagedPaths' repository-root-
// relative answers and the store-root vocabulary its callers reason in — the
// two are the same vocabulary only when dir IS the repository root, and a
// caller that assumes that silently reads every staged store path as foreign
// in the below-git-root layout StagedPaths' own doc comment names.
func TestRepoPrefix(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			"above.txt":         "above the store root\n",
			"store/.verdi/keep": "store marker\n",
		},
		Message: "seed a store root below the git root",
	}})

	t.Run("the repository root itself has no prefix", func(t *testing.T) {
		got, err := RepoPrefix(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("RepoPrefix: %v", err)
		}
		if got != "" {
			t.Fatalf("RepoPrefix(repository root) = %q, want %q", got, "")
		}
	})

	t.Run("a directory below the repository root", func(t *testing.T) {
		got, err := RepoPrefix(ctx, filepath.Join(repo.Dir, "store"))
		if err != nil {
			t.Fatalf("RepoPrefix: %v", err)
		}
		if got != "store/" {
			t.Fatalf("RepoPrefix(store root below the git root) = %q, want %q — the trailing separator is what makes it a safe prefix to strip", got, "store/")
		}
	})

	t.Run("a prefix a StagedPaths answer can actually be stripped with", func(t *testing.T) {
		// The property the two functions are used for together: every staged
		// path inside dir must begin with dir's own prefix.
		storeRoot := filepath.Join(repo.Dir, "store")
		if err := os.WriteFile(filepath.Join(storeRoot, ".verdi", "keep"), []byte("edited\n"), 0o644); err != nil {
			t.Fatalf("editing the store marker: %v", err)
		}
		runGitForTest(t, storeRoot, "add", "--", ".verdi/keep")

		prefix, err := RepoPrefix(ctx, storeRoot)
		if err != nil {
			t.Fatalf("RepoPrefix: %v", err)
		}
		staged, err := StagedPaths(ctx, storeRoot)
		if err != nil {
			t.Fatalf("StagedPaths: %v", err)
		}
		want := []string{"store/.verdi/keep"}
		if !reflect.DeepEqual(staged, want) {
			t.Fatalf("StagedPaths = %#v, want %#v", staged, want)
		}
		if rest, ok := strings.CutPrefix(staged[0], prefix); !ok || rest != ".verdi/keep" {
			t.Fatalf("stripping RepoPrefix %q from StagedPaths %q = (%q, %v), want (%q, true)", prefix, staged[0], rest, ok, ".verdi/keep")
		}
	})

	t.Run("outside a repository is an operational error", func(t *testing.T) {
		if _, err := RepoPrefix(ctx, t.TempDir()); err == nil {
			t.Fatal("RepoPrefix outside a repository: want an operational error, got nil")
		}
	})
}

// TestStagedPaths_NamesBothSidesOfAStagedRename pins the refusal message's
// completeness: `--name-only` reports only a rename's DESTINATION (R100
// old->new renders as `new` alone), so a guard built on it under-names the
// paths it is refusing over. Both sides differ from HEAD and both are what
// the operator must deal with.
func TestStagedPaths_NamesBothSidesOfAStagedRename(t *testing.T) {
	ctx := context.Background()
	repo := buildRepo(t)
	runGitForTest(t, repo.Dir, "mv", "a.txt", "renamed.txt")

	got, err := StagedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("StagedPaths: %v", err)
	}
	want := []string{"a.txt", "renamed.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("StagedPaths(staged rename) = %#v, want both sides %#v", got, want)
	}
}
