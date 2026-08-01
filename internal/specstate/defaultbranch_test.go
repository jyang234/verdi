package specstate

import (
	"context"
	"os/exec"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// buildBranchRepo builds a minimal one-layer fixturegit repo (fixturegit's
// own --initial-branch=main default) with no "origin" remote configured at
// all — mirroring internal/lint/cienv_test.go's own helper of the same
// name (deliberately duplicated, not shared, per that file's own
// documented reasoning: this git-refs-only resolution test doesn't need a
// full corpus scaffold).
func buildBranchRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{"a.txt": "hello\n"},
			Message: "layer 1",
		},
	})
}

// fabricateRemoteRef seeds refs/remotes/origin/<branch> at commit directly
// via gitx.UpdateRef — no clone, no fetch, no network.
func fabricateRemoteRef(t *testing.T, dir, branch, commit string) {
	t.Helper()
	if err := gitx.UpdateRef(context.Background(), dir, "refs/remotes/origin/"+branch, commit); err != nil {
		t.Fatalf("seeding refs/remotes/origin/%s: %v", branch, err)
	}
}

// fabricateLocalBranch seeds a LOCAL branch ref (refs/heads/<branch>) at
// commit directly, without checking it out — modeling "CI_DEFAULT_BRANCH
// names some other branch that does exist locally".
func fabricateLocalBranch(t *testing.T, dir, branch, commit string) {
	t.Helper()
	if err := gitx.UpdateRef(context.Background(), dir, "refs/heads/"+branch, commit); err != nil {
		t.Fatalf("seeding refs/heads/%s: %v", branch, err)
	}
}

// renameCurrentBranch renames dir's checked-out branch, used to get a
// fixturegit repo OFF its default local "main" branch so a test can prove
// remote-tracking-only resolution without a same-named local branch
// shadowing it.
func renameCurrentBranch(t *testing.T, dir, newName string) {
	t.Helper()
	cmd := exec.Command("git", "branch", "-m", newName)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch -m %s: %v\n%s", newName, err, out)
	}
}

// setSymbolicRef points a symbolic ref (e.g. refs/remotes/origin/HEAD) at
// another ref name directly (`git symbolic-ref <name> <target>`).
func setSymbolicRef(t *testing.T, dir, name, target string) {
	t.Helper()
	cmd := exec.Command("git", "symbolic-ref", name, target)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref %s %s: %v\n%s", name, target, err, out)
	}
}

// TestResolveDefaultBranch covers the task-3 brief's six named
// default-branch-resolution outcomes, plus a "not a repository" safety
// case.
func TestResolveDefaultBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("CI_DEFAULT_BRANCH plus a matching local branch gives {name, name}", func(t *testing.T) {
		repo := buildBranchRepo(t)
		fabricateLocalBranch(t, repo.Dir, "release-line", repo.Head)
		t.Setenv("CI_DEFAULT_BRANCH", "release-line")

		got, ok := ResolveDefaultBranch(ctx, repo.Dir)
		want := Branch{Name: "release-line", Ref: "release-line"}
		if !ok || got != want {
			t.Fatalf("ResolveDefaultBranch = (%+v, %v), want (%+v, true)", got, ok, want)
		}
	})

	t.Run("remote-only origin/main (no local main) gives {main, origin/main}", func(t *testing.T) {
		repo := buildBranchRepo(t)
		renameCurrentBranch(t, repo.Dir, "trunk") // no local branch literally named "main"
		fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
		t.Setenv("CI_DEFAULT_BRANCH", "")

		got, ok := ResolveDefaultBranch(ctx, repo.Dir)
		want := Branch{Name: "main", Ref: "origin/main"}
		if !ok || got != want {
			t.Fatalf("ResolveDefaultBranch = (%+v, %v), want (%+v, true)", got, ok, want)
		}
	})

	t.Run("configured origin/HEAD gives its actual target, not a hardcoded main", func(t *testing.T) {
		repo := buildBranchRepo(t) // local branch stays "main"
		fabricateRemoteRef(t, repo.Dir, "master", repo.Head)
		setSymbolicRef(t, repo.Dir, "refs/remotes/origin/HEAD", "refs/remotes/origin/master")
		t.Setenv("CI_DEFAULT_BRANCH", "")

		got, ok := ResolveDefaultBranch(ctx, repo.Dir)
		want := Branch{Name: "master", Ref: "origin/master"}
		if !ok || got != want {
			t.Fatalf("ResolveDefaultBranch = (%+v, %v), want (%+v, true) (origin/HEAD names master, no local master exists)", got, ok, want)
		}
	})

	t.Run("both origin/main and origin/master, no other signal, is ambiguous and refuses", func(t *testing.T) {
		repo := buildBranchRepo(t)
		fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
		fabricateRemoteRef(t, repo.Dir, "master", repo.Head)
		t.Setenv("CI_DEFAULT_BRANCH", "")

		if _, ok := ResolveDefaultBranch(ctx, repo.Dir); ok {
			t.Fatal("ResolveDefaultBranch: want ambiguous (false), got ok=true")
		}
	})

	t.Run("no signal at all returns false", func(t *testing.T) {
		repo := buildBranchRepo(t)
		t.Setenv("CI_DEFAULT_BRANCH", "")

		if _, ok := ResolveDefaultBranch(ctx, repo.Dir); ok {
			t.Fatal("ResolveDefaultBranch: want unresolved (false), got ok=true")
		}
	})

	t.Run("a named branch whose ref cannot resolve returns false", func(t *testing.T) {
		repo := buildBranchRepo(t)
		t.Setenv("CI_DEFAULT_BRANCH", "ghost-branch") // no local or remote ref named this

		if _, ok := ResolveDefaultBranch(ctx, repo.Dir); ok {
			t.Fatal("ResolveDefaultBranch: want unresolved (false) for a name with no resolvable ref, got ok=true")
		}
	})

	t.Run("not a repository at all fails closed", func(t *testing.T) {
		notARepo := t.TempDir()
		t.Setenv("CI_DEFAULT_BRANCH", "")

		if _, ok := ResolveDefaultBranch(ctx, notARepo); ok {
			t.Fatal("ResolveDefaultBranch outside a repo: want unresolved (false), got ok=true")
		}
	})
}
