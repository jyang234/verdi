package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// diffVariants are the two exported seams over diffNameStatus. Every
// configuration row below runs against both, because both are consulted
// by safety checks (the SI-231 predicate and VL-010/VL-016/spec-import read
// DiffNameStatus; the sealed-execution handback reads DiffNameStatusCopies).
var diffVariants = []struct {
	name string
	diff func(ctx context.Context, dir, base, head string) ([]DiffEntry, error)
}{
	{name: "DiffNameStatus", diff: DiffNameStatus},
	{name: "DiffNameStatusCopies", diff: DiffNameStatusCopies},
}

// .gitmodules formats for the lib submodule; %s is its url (a local path).
const (
	diffPlainGitmodules    = "[submodule \"lib\"]\n\tpath = lib\n\turl = %s\n"
	diffIgnoringGitmodules = "[submodule \"lib\"]\n\tpath = lib\n\turl = %s\n\tignore = all\n"
)

// submoduleDiffRepo is buildSubmoduleDiffRepo's fixture: Base..Head bumps
// the gitlink lib from one commit to another and edits one file at the
// repository root and one under sub/.
type submoduleDiffRepo struct {
	Dir, Base, Head string
}

// buildSubmoduleDiffRepo builds, with no network and no clone, a
// superproject whose Base commit records the gitlink (mode 160000) lib at
// commit A of a second, local repository, described by a committed
// .gitmodules (gitmodulesFormat, its url the local repository's path), and
// whose Head commit bumps lib to that repository's commit B while also
// editing top.txt (at the root) and sub/inner.txt (below sub/). The gitlink
// is written with `git update-index --cacheinfo`, the shape `git submodule
// update` + `git add lib` leaves; lib/ stays an empty directory, as an
// uninitialized submodule's does, so `git add -A` leaves the entry alone.
func buildSubmoduleDiffRepo(t *testing.T, gitmodulesFormat string) submoduleDiffRepo {
	t.Helper()
	lib := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"lib.go": "package lib // A\n"}, Message: "lib commit A"},
		{Files: map[string]string{"lib.go": "package lib // B\n"}, Message: "lib commit B"},
	})
	libA, libB := lib.Heads[0], lib.Heads[1]

	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			"top.txt":       "top\n",
			"sub/inner.txt": "inner\n",
			".gitmodules":   fmt.Sprintf(gitmodulesFormat, lib.Dir),
		},
		Message: "superproject files",
	}})
	if err := os.Mkdir(filepath.Join(repo.Dir, "lib"), 0o755); err != nil {
		t.Fatalf("mkdir lib: %v", err)
	}
	runGitForTest(t, repo.Dir, "update-index", "--add", "--cacheinfo", "160000,"+libA+",lib")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "record lib at commit A")
	base := strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))

	for rel, content := range map[string]string{"top.txt": "top changed\n", "sub/inner.txt": "inner changed\n"} {
		if err := os.WriteFile(filepath.Join(repo.Dir, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", rel, err)
		}
	}
	runGitForTest(t, repo.Dir, "add", "--", "top.txt", "sub/inner.txt")
	runGitForTest(t, repo.Dir, "update-index", "--cacheinfo", "160000,"+libB+",lib")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "bump lib to commit B and edit two files")
	head := strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "HEAD"))

	return submoduleDiffRepo{Dir: repo.Dir, Base: base, Head: head}
}

// TestDiffNameStatus_ConfigurationIndependent is the config matrix for
// diffNameStatus: its answer is the full, repository-root-relative diff
// whatever the user's or repository's diff settings say. Two families of
// ordinary settings change which paths plain `git diff --name-status`
// reports:
//
//   - every submodule `ignore = all` scope (diff.ignoreSubmodules in local or
//     global config, submodule.<name>.ignore, or a COMMITTED .gitmodules that
//     every clone inherits) hides a gitlink bump, so a caller that asks "what
//     does this commit change?" is told less than the commit changes (the
//     SI-231 one-behind predicate then accepted a commit that also bumped a
//     submodule: L3b re-review N-1);
//   - diff.relative=true, with git run from a subdirectory (every gitx call
//     runs with cmd.Dir = the store root, which may sit below the git root),
//     hides every path outside it and re-bases the rest onto it.
//
// Every row asserts the exact entries for both variants, reporting each
// variant's failure on its own, and first proves that plain git really is
// misled by the row's configuration, so a row can never pass because its
// setting silently did nothing.
func TestDiffNameStatus_ConfigurationIndependent(t *testing.T) {
	ctx := context.Background()
	want := []DiffEntry{
		{Status: "M", Path: "lib"},
		{Status: "M", Path: "sub/inner.txt"},
		{Status: "M", Path: "top.txt"},
	}
	const fullPlainDiff = "M\tlib\nM\tsub/inner.txt\nM\ttop.txt"

	cases := []struct {
		name       string
		gitmodules string
		// local are repository-config settings; global, when non-empty, is
		// the content of the file GIT_CONFIG_GLOBAL names.
		local  [][2]string
		global string
		// from is the directory, relative to the repository root, git runs
		// in; "" is the root itself.
		from string
		// plainDiff is what plain `git diff --name-status -M base head`
		// reports from that directory under this configuration.
		plainDiff string
	}{
		{
			name:       "no configuration (baseline)",
			gitmodules: diffPlainGitmodules,
			plainDiff:  fullPlainDiff,
		},
		{
			name:       "local diff.ignoreSubmodules=all",
			gitmodules: diffPlainGitmodules,
			local:      [][2]string{{"diff.ignoreSubmodules", "all"}},
			plainDiff:  "M\tsub/inner.txt\nM\ttop.txt",
		},
		{
			name:       "global diff.ignoreSubmodules=all",
			gitmodules: diffPlainGitmodules,
			global:     "[diff]\n\tignoreSubmodules = all\n",
			plainDiff:  "M\tsub/inner.txt\nM\ttop.txt",
		},
		{
			name:       "local submodule.<name>.ignore=all",
			gitmodules: diffPlainGitmodules,
			local:      [][2]string{{"submodule.lib.ignore", "all"}},
			plainDiff:  "M\tsub/inner.txt\nM\ttop.txt",
		},
		{
			name:       "committed .gitmodules ignore = all (every clone inherits it)",
			gitmodules: diffIgnoringGitmodules,
			plainDiff:  "M\tsub/inner.txt\nM\ttop.txt",
		},
		{
			name:       "diff.relative=true run from a subdirectory",
			gitmodules: diffPlainGitmodules,
			local:      [][2]string{{"diff.relative", "true"}},
			from:       "sub",
			plainDiff:  "M\tinner.txt",
		},
		{
			name:       "every setting at once, run from a subdirectory",
			gitmodules: diffIgnoringGitmodules,
			local:      [][2]string{{"diff.ignoreSubmodules", "all"}, {"submodule.lib.ignore", "all"}, {"diff.relative", "true"}},
			global:     "[diff]\n\tignoreSubmodules = all\n\trelative = true\n",
			from:       "sub",
			plainDiff:  "M\tinner.txt",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildSubmoduleDiffRepo(t, tc.gitmodules)
			for _, kv := range tc.local {
				runGitForTest(t, repo.Dir, "config", kv[0], kv[1])
			}
			if tc.global != "" {
				globalPath := filepath.Join(t.TempDir(), "gitconfig")
				if err := os.WriteFile(globalPath, []byte(tc.global), 0o644); err != nil {
					t.Fatalf("writing global config: %v", err)
				}
				t.Setenv("GIT_CONFIG_GLOBAL", globalPath)
			}
			dir := filepath.Join(repo.Dir, filepath.FromSlash(tc.from))

			if got := strings.TrimSpace(runGitForTest(t, dir, "diff", "--name-status", "-M", repo.Base, repo.Head)); got != tc.plainDiff {
				t.Fatalf("fixture: plain `git diff --name-status` = %q, want %q — the row's configuration must really change what plain git reports", got, tc.plainDiff)
			}

			for _, v := range diffVariants {
				got, err := v.diff(ctx, dir, repo.Base, repo.Head)
				if err != nil {
					t.Errorf("%s: %v", v.name, err)
					continue
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s = %+v, want %+v — the full repository-root diff regardless of configuration", v.name, got, want)
				}
			}
		})
	}
}

// TestDiffNameStatus_MalformedOutputErrors is the parser's negative path:
// a line git's --name-status format can never produce (no tab-separated
// path, or a rename/copy carrying one path instead of two) is an error for
// both variants, never a silently short answer. A stand-in git on PATH
// supplies the output, since real git never emits it.
func TestDiffNameStatus_MalformedOutputErrors(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		output string
	}{
		{name: "a line with no tab-separated path", output: "M\n"},
		{name: "a rename line carrying one path", output: "R100\told.txt\n"},
		{name: "a copy line carrying one path", output: "C075\tsource.txt\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shimDir := t.TempDir()
			outputPath := filepath.Join(shimDir, "output")
			if err := os.WriteFile(outputPath, []byte(tc.output), 0o644); err != nil {
				t.Fatalf("writing stand-in output: %v", err)
			}
			script := "#!/bin/sh\ncat '" + outputPath + "'\n"
			if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
				t.Fatalf("writing stand-in git: %v", err)
			}
			t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			for _, v := range diffVariants {
				got, err := v.diff(ctx, t.TempDir(), "base", "head")
				if err == nil {
					t.Errorf("%s over %q = %+v, nil; want a malformed-line error", v.name, tc.output, got)
					continue
				}
				if !strings.Contains(err.Error(), "malformed") {
					t.Errorf("%s over %q: err = %v, want it to name the malformed line", v.name, tc.output, err)
				}
			}
		})
	}
}
