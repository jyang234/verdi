package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestStatusDirty table-drives StatusDirty's own documented contract —
// "any uncommitted change (staged, unstaged, or untracked-and-unignored)"
// — including the row that contract only actually holds for once the
// query is configuration-independent (R-RR3-28): an untracked, unignored
// file in a repository whose operator set the ordinary DISPLAY setting
// `status.showUntrackedFiles=no`. A plain `git status --porcelain`
// honors that setting and answers "clean" with the operator's work
// sitting on disk; `--untracked-files=all` overrides it at the query, so
// every consumer of this single dirty check reads the same answer
// whatever the repository's configuration says.
func TestStatusDirty(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "clean tree → clean",
			setup: func(*testing.T, string) {},
			want:  false,
		},
		{
			name: "modified tracked file → dirty",
			setup: func(t *testing.T, dir string) {
				writeFixtureFile(t, filepath.Join(dir, "a.txt"), "changed\n")
			},
			want: true,
		},
		{
			name: "untracked unignored file → dirty",
			setup: func(t *testing.T, dir string) {
				writeFixtureFile(t, filepath.Join(dir, "unfinished.txt"), "operator work\n")
			},
			want: true,
		},
		{
			name: "ignored file → clean",
			setup: func(t *testing.T, dir string) {
				writeFixtureFile(t, filepath.Join(dir, ".gitignore"), "ignored.txt\n")
				gitConfigure(t, dir, "add", ".gitignore")
				gitConfigure(t, dir, "commit", "--quiet", "-m", "ignore ignored.txt")
				writeFixtureFile(t, filepath.Join(dir, "ignored.txt"), "build output\n")
			},
			want: false,
		},
		{
			name: "untracked file hidden by status.showUntrackedFiles=no → dirty",
			setup: func(t *testing.T, dir string) {
				gitConfigure(t, dir, "config", "status.showUntrackedFiles", "no")
				writeFixtureFile(t, filepath.Join(dir, "unfinished.txt"), "operator work\n")
			},
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildRepo(t)
			tc.setup(t, repo.Dir)

			got, err := StatusDirty(ctx, repo.Dir)
			if err != nil {
				t.Fatalf("StatusDirty: %v", err)
			}
			if got != tc.want {
				t.Fatalf("StatusDirty = %v, want %v", got, tc.want)
			}
		})
	}
}

// writeFixtureFile writes a fixture file, failing the test on error.
func writeFixtureFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// gitConfigure runs one git command against dir for fixture setup only —
// deliberately NOT through this package's own run(), so a fixture never
// depends on the very seam the test is measuring.
func gitConfigure(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestStatusDirty_Negative(t *testing.T) {
	if _, err := StatusDirty(context.Background(), t.TempDir()); err == nil {
		t.Fatal("StatusDirty outside a repo: want error")
	}
}

func TestLocalBranches(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	branches, err := LocalBranches(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}
	want := map[string]bool{"main": true, "design/x": true}
	if len(branches) != 2 || !want[branches[0]] || !want[branches[1]] {
		t.Fatalf("LocalBranches = %v, want main + design/x", branches)
	}
}

func TestLocalBranches_Negative(t *testing.T) {
	if _, err := LocalBranches(context.Background(), t.TempDir()); err == nil {
		t.Fatal("LocalBranches outside a repo: want error")
	}
}

func TestCheckout_GuardsDirtyTree(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	// Clean: the switch works.
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout (clean): %v", err)
	}
	got, _ := CurrentBranch(ctx, repo.Dir)
	if got != "main" {
		t.Fatalf("CurrentBranch after checkout = %q, want main", got)
	}

	// Dirty: the guard blocks before git runs.
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("mid-edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Checkout(ctx, repo.Dir, "design/x"); err == nil {
		t.Fatal("Checkout with a dirty tree succeeded, want guard error")
	}
	got, _ = CurrentBranch(ctx, repo.Dir)
	if got != "main" {
		t.Fatalf("guarded checkout still switched branch to %q", got)
	}
}

func TestCheckout_Negative_UnknownBranch(t *testing.T) {
	repo := buildRepo(t)
	if err := Checkout(context.Background(), repo.Dir, "no-such-branch"); err == nil {
		t.Fatal("Checkout of a missing branch succeeded")
	}
}

func TestWorktreeAdd_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}

	wtPath := filepath.Join(t.TempDir(), "x")
	if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/x"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wtPath, "a.txt")); err != nil {
		t.Fatalf("cut worktree missing expected file: %v", err)
	}
	got, err := CurrentBranch(ctx, wtPath)
	if err != nil {
		t.Fatalf("CurrentBranch(cut worktree): %v", err)
	}
	if got != "design/x" {
		t.Fatalf("cut worktree's branch = %q, want design/x", got)
	}

	// The serving checkout's own branch is untouched.
	rootBranch, _ := CurrentBranch(ctx, repo.Dir)
	if rootBranch != "main" {
		t.Fatalf("WorktreeAdd changed the serving checkout's own branch to %q", rootBranch)
	}
}

func TestWorktreeAdd_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("nonexistent branch", func(t *testing.T) {
		wtPath := filepath.Join(t.TempDir(), "nope")
		if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/nope"); err == nil {
			t.Fatal("WorktreeAdd(nonexistent branch): want error, got nil")
		}
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Fatalf("WorktreeAdd(nonexistent branch) left a directory behind: err=%v", err)
		}
	})

	t.Run("branch already checked out at dir itself", func(t *testing.T) {
		if err := CheckoutNewBranch(ctx, repo.Dir, "design/here"); err != nil {
			t.Fatalf("CheckoutNewBranch: %v", err)
		}
		wtPath := filepath.Join(t.TempDir(), "here")
		err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/here")
		if err == nil {
			t.Fatal("WorktreeAdd(branch checked out at dir): want error, got nil")
		}
		// Assert the typed refusal, not git's version-dependent stderr
		// text: the proactive current-branch check must classify this as
		// ErrBranchCheckedOut regardless of how the installed git words it.
		if !errors.Is(err, ErrBranchCheckedOut) {
			t.Fatalf("WorktreeAdd error = %v, want ErrBranchCheckedOut", err)
		}
		// The proactive guard runs before `git worktree add`, so no
		// worktree directory is ever left behind.
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Fatalf("WorktreeAdd(branch checked out at dir) left a directory behind: err=%v", err)
		}
	})
}

func TestWorktreeRemove_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "x")
	if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/x"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	if err := WorktreeRemove(ctx, repo.Dir, wtPath); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree directory still present after WorktreeRemove: err=%v", err)
	}
}

// TestWorktreeRemove_Negative_DirtyRefusedWithoutForce table-drives
// git's OWN dirty-tree refusal — the second, independent guard behind
// every caller's gitx.StatusDirty check. The config row is R-RR3-29: git
// resolves `worktree remove`'s cleanliness question through its own
// `git status`, which honors status.showUntrackedFiles exactly as
// StatusDirty's plain query did, so before this ruling BOTH guards failed
// together on the same ordinary display setting and the operator's
// untracked file was deleted. Passing -c status.showUntrackedFiles=all
// before the subcommand makes the second guard independent too; it is a
// git global option, never a force flag.
func TestWorktreeRemove_Negative_DirtyRefusedWithoutForce(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, repoDir, wtPath string)
	}{
		{
			name: "modified tracked file",
			setup: func(t *testing.T, _, wtPath string) {
				writeFixtureFile(t, filepath.Join(wtPath, "a.txt"), "dirty\n")
			},
		},
		{
			name: "untracked unignored file",
			setup: func(t *testing.T, _, wtPath string) {
				writeFixtureFile(t, filepath.Join(wtPath, "unfinished.txt"), "operator work\n")
			},
		},
		{
			name: "untracked file hidden by status.showUntrackedFiles=no",
			setup: func(t *testing.T, repoDir, wtPath string) {
				gitConfigure(t, repoDir, "config", "status.showUntrackedFiles", "no")
				writeFixtureFile(t, filepath.Join(wtPath, "unfinished.txt"), "operator work\n")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildRepo(t)
			if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
				t.Fatalf("CheckoutNewBranch: %v", err)
			}
			if err := Checkout(ctx, repo.Dir, "main"); err != nil {
				t.Fatalf("Checkout(main): %v", err)
			}
			wtPath := filepath.Join(t.TempDir(), "x")
			if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/x"); err != nil {
				t.Fatalf("WorktreeAdd: %v", err)
			}
			tc.setup(t, repo.Dir, wtPath)

			if err := WorktreeRemove(ctx, repo.Dir, wtPath); err == nil {
				t.Fatal("WorktreeRemove(dirty worktree, no --force): want error, got nil")
			}
			if _, err := os.Stat(wtPath); err != nil {
				t.Fatalf("dirty worktree removed despite refusal: %v", err)
			}
		})
	}
}

// TestWorktreeRemove_ArgvCarriesNoForceFlag pins R-RR3-29's exact argv
// through the observer seam: the configuration override is passed as
// git's global -c option BEFORE the subcommand, and the argv still
// carries no --force / -f (SI-224's forbidden-token floor is unchanged).
func TestWorktreeRemove_ArgvCarriesNoForceFlag(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "x")
	if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/x"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	// recordingObserver (observer_test.go) records dir as calls[i][0], so
	// the argv proper starts at index 1.
	rec := &recordingObserver{}
	if err := WorktreeRemove(WithObserver(ctx, rec), repo.Dir, wtPath); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	want := [][]string{{repo.Dir, "-c", "status.showUntrackedFiles=all", "worktree", "remove", wtPath}}
	if !reflect.DeepEqual(rec.calls, want) {
		t.Fatalf("recorded calls = %v, want exactly %v", rec.calls, want)
	}
	for _, arg := range rec.calls[0][1:] {
		if arg == "--force" || arg == "-f" {
			t.Fatalf("WorktreeRemove argv %v carries a force flag", rec.calls[0][1:])
		}
	}
}

func TestWorktreeRemove_Negative_NoSuchWorktree(t *testing.T) {
	repo := buildRepo(t)
	if err := WorktreeRemove(context.Background(), repo.Dir, filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("WorktreeRemove(never-added path): want error, got nil")
	}
}

func TestPushAndHasRemote(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	// No remote yet.
	has, err := HasRemote(ctx, repo.Dir, "origin")
	if err != nil {
		t.Fatalf("HasRemote: %v", err)
	}
	if has {
		t.Fatal("HasRemote true with no remotes")
	}
	if err := Push(ctx, repo.Dir); err == nil {
		t.Fatal("Push with no origin succeeded, want error")
	}

	// A local bare origin: push round-trips (hermetic — no network).
	bare := t.TempDir()
	if err := exec.Command("git", "init", "--bare", "--quiet", "--initial-branch=main", bare).Run(); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	if err := exec.Command("git", "-C", repo.Dir, "remote", "add", "origin", bare).Run(); err != nil {
		t.Fatalf("git remote add: %v", err)
	}
	has, err = HasRemote(ctx, repo.Dir, "origin")
	if err != nil {
		t.Fatalf("HasRemote: %v", err)
	}
	if !has {
		t.Fatal("HasRemote false after remote add")
	}
	if err := Push(ctx, repo.Dir); err != nil {
		t.Fatalf("Push: %v", err)
	}
	out, err := exec.Command("git", "-C", bare, "rev-parse", "main").Output()
	if err != nil || len(out) == 0 {
		t.Fatalf("bare origin has no main after push: %v", err)
	}
}

// TestWorktreeAdd_InUseElsewhere_TypedRefusal pins WorktreeAdd's fallback
// over both wordings git gives a branch in use by another worktree (ledger
// SI-354 (1)): "is already checked out at" before git 2.42, and "is
// already used by worktree at" from git 2.42 on, which CI's git prints.
// Both are the typed ErrBranchCheckedOut; any other refusal keeps git's
// own words. A stand-in git answers dir's current branch as main and
// refuses the add, so each wording is pinned whatever git is installed.
func TestWorktreeAdd_InUseElsewhere_TypedRefusal(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		stderr string
		typed  bool
	}{
		{"git before 2.42", "Preparing worktree (checking out 'design/x')\nfatal: 'design/x' is already checked out at '/elsewhere'", true},
		{"git 2.42 and later", "Preparing worktree (checking out 'design/x')\nfatal: 'design/x' is already used by worktree at '/elsewhere'", true},
		{"another refusal", "fatal: invalid reference: design/x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shimDir := t.TempDir()
			stderrPath := filepath.Join(shimDir, "stderr")
			if err := os.WriteFile(stderrPath, []byte(tt.stderr+"\n"), 0o644); err != nil {
				t.Fatalf("writing stand-in stderr: %v", err)
			}
			script := "#!/bin/sh\nif [ \"$1\" = symbolic-ref ]; then echo main; exit 0; fi\ncat '" + stderrPath + "' >&2\nexit 128\n"
			if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
				t.Fatalf("writing stand-in git: %v", err)
			}
			t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			err := WorktreeAdd(ctx, t.TempDir(), filepath.Join(t.TempDir(), "x"), "design/x")
			if err == nil {
				t.Fatal("WorktreeAdd over a refusing git = nil, want an error")
			}
			if got := errors.Is(err, ErrBranchCheckedOut); got != tt.typed {
				t.Fatalf("errors.Is(%v, ErrBranchCheckedOut) = %v, want %v", err, got, tt.typed)
			}
			if !tt.typed && !strings.Contains(err.Error(), "invalid reference: design/x") {
				t.Fatalf("WorktreeAdd error = %v, want git's own words", err)
			}
		})
	}
}

// TestWorktreeAdd_HeldMidOperation is the same refusal against the
// installed git (ledger SI-354 (1)): a linked worktree holding the branch,
// plainly, mid-rebase, or mid-bisect, where `worktree list --porcelain`
// reports it detached, still holds it for git, and WorktreeAdd answers the
// typed ErrBranchCheckedOut with no directory cut. Run it under each git
// the build meets (CI's 2.55 words the refusal the 2.42+ way).
func TestWorktreeAdd_HeldMidOperation(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		hold func(t *testing.T, wt string)
	}{
		{"checked out", func(*testing.T, string) {}},
		{"mid-rebase", func(t *testing.T, wt string) {
			if out, err := gitInDir(wt, "rebase", "--force-rebase", "--exec", "false", "main"); err == nil {
				t.Fatalf("the rebase did not stop:\n%s", out)
			}
		}},
		{"mid-bisect", func(t *testing.T, wt string) {
			if out, err := gitInDir(wt, "bisect", "start", "HEAD", "HEAD~2"); err != nil {
				t.Fatalf("git bisect start: %v\n%s", err, out)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := buildRepo(t)
			if out, err := gitInDir(repo.Dir, "branch", "design/held", "main"); err != nil {
				t.Fatalf("git branch: %v\n%s", err, out)
			}
			wt := filepath.Join(t.TempDir(), "holder")
			if out, err := gitInDir(repo.Dir, "worktree", "add", "--quiet", wt, "design/held"); err != nil {
				t.Fatalf("git worktree add: %v\n%s", err, out)
			}
			if err := os.WriteFile(filepath.Join(wt, "held.txt"), []byte("held\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"add", "held.txt"}, {"commit", "--quiet", "-m", "held"}} {
				if out, err := gitInDir(wt, args...); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}
			tt.hold(t, wt)

			path := filepath.Join(t.TempDir(), "cut")
			err := WorktreeAdd(ctx, repo.Dir, path, "design/held")
			if !errors.Is(err, ErrBranchCheckedOut) {
				t.Fatalf("WorktreeAdd for a branch held %s = %v, want ErrBranchCheckedOut", tt.name, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("WorktreeAdd left %s behind (stat err %v)", path, err)
			}
		})
	}
}

// gitInDir runs git in dir with a fixed identity and no editor, returning
// its combined output.
func gitInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
		"GIT_EDITOR=true", "GIT_SEQUENCE_EDITOR=true")
	out, err := cmd.CombinedOutput()
	return string(out), err
}
