package lint

import (
	"context"
	"os/exec"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

func TestReadCIEnv(t *testing.T) {
	t.Run("gitlab MR pipeline", func(t *testing.T) {
		t.Setenv("CI", "true")
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("GITHUB_BASE_REF", "")

		e := ReadCIEnv()
		if !e.InCI || e.DefaultBranch != "main" || e.TargetBranch != "main" {
			t.Fatalf("got %+v, want InCI=true DefaultBranch=main TargetBranch=main", e)
		}
	})

	t.Run("github PR workflow falls back to GITHUB_BASE_REF", func(t *testing.T) {
		t.Setenv("CI", "")
		t.Setenv("CI_DEFAULT_BRANCH", "")
		t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("GITHUB_BASE_REF", "main")

		e := ReadCIEnv()
		if !e.InCI || e.TargetBranch != "main" {
			t.Fatalf("got %+v, want InCI=true TargetBranch=main", e)
		}
	})

	t.Run("no CI environment at all", func(t *testing.T) {
		t.Setenv("CI", "")
		t.Setenv("CI_DEFAULT_BRANCH", "")
		t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "")
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("GITHUB_BASE_REF", "")

		e := ReadCIEnv()
		if e.InCI || e.DefaultBranch != "" || e.TargetBranch != "" {
			t.Fatalf("got %+v, want all zero", e)
		}
	})
}

// TestResolveDefaultBranch proves D6-6's fix: ResolveDefaultBranch's full
// precedence chain — (1) CI_DEFAULT_BRANCH env, (2) origin/HEAD symbolic
// ref, (3) NEW: the hermetic local fallback (exactly one of
// refs/remotes/origin/main or refs/remotes/origin/master), (4) "" unknown
// — against real git state via fixturegit fake remotes (no network: every
// ref here is fabricated directly with gitx.UpdateRef/symbolic-ref, never
// a real clone or `ls-remote`).
func TestResolveDefaultBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("(a) env wins even when refs disagree", func(t *testing.T) {
		repo := buildBranchRepo(t)
		fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
		fabricateRemoteRef(t, repo.Dir, "master", repo.Head)
		setSymbolicRef(t, repo.Dir, "refs/remotes/origin/HEAD", "refs/remotes/origin/master")
		t.Setenv("CI_DEFAULT_BRANCH", "release-line")

		if got := ResolveDefaultBranch(ctx, repo.Dir); got != "release-line" {
			t.Fatalf("ResolveDefaultBranch = %q, want %q (env must win even though origin/HEAD and refs disagree)", got, "release-line")
		}
	})

	t.Run("(b) origin/HEAD symbolic ref is used when no env", func(t *testing.T) {
		repo := buildBranchRepo(t)
		fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
		setSymbolicRef(t, repo.Dir, "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
		t.Setenv("CI_DEFAULT_BRANCH", "")

		if got := ResolveDefaultBranch(ctx, repo.Dir); got != "main" {
			t.Fatalf("ResolveDefaultBranch = %q, want %q", got, "main")
		}
	})

	t.Run("(c) only origin/main ref resolves via the D6-6 fallback", func(t *testing.T) {
		repo := buildBranchRepo(t)
		fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
		t.Setenv("CI_DEFAULT_BRANCH", "")

		if got := ResolveDefaultBranch(ctx, repo.Dir); got != "main" {
			t.Fatalf("ResolveDefaultBranch = %q, want %q (fresh-GitHub-checkout shape: no origin/HEAD, only origin/main)", got, "main")
		}
	})

	t.Run("(d) only origin/master ref resolves via the D6-6 fallback", func(t *testing.T) {
		repo := buildBranchRepo(t)
		fabricateRemoteRef(t, repo.Dir, "master", repo.Head)
		t.Setenv("CI_DEFAULT_BRANCH", "")

		if got := ResolveDefaultBranch(ctx, repo.Dir); got != "master" {
			t.Fatalf("ResolveDefaultBranch = %q, want %q (fresh-GitHub-checkout shape: no origin/HEAD, only origin/master)", got, "master")
		}
	})

	t.Run("(e) both main and master present, no HEAD, is ambiguous and fails closed", func(t *testing.T) {
		repo := buildBranchRepo(t)
		fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
		fabricateRemoteRef(t, repo.Dir, "master", repo.Head)
		t.Setenv("CI_DEFAULT_BRANCH", "")

		if got := ResolveDefaultBranch(ctx, repo.Dir); got != "" {
			t.Fatalf("ResolveDefaultBranch = %q, want empty (ambiguous main+master must refuse, never guess)", got)
		}
	})

	t.Run("(f) nothing resolves at all fails closed", func(t *testing.T) {
		repo := buildBranchRepo(t)
		t.Setenv("CI_DEFAULT_BRANCH", "")

		if got := ResolveDefaultBranch(ctx, repo.Dir); got != "" {
			t.Fatalf("ResolveDefaultBranch = %q, want empty", got)
		}
	})
}

// buildBranchRepo builds a minimal one-layer fixturegit repo (fixturegit's
// own --initial-branch=main default) with no "origin" remote configured at
// all — the same starting shape internal/gitx's own buildRepo provides,
// duplicated here (not exported/shared) because internal/lint's tests
// otherwise pull in buildLintRepo's full corpus scaffold, which this
// git-refs-only resolution test doesn't need.
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
// via gitx.UpdateRef — no clone, no fetch, no network — modeling exactly
// what actions/checkout's specific-ref fetch leaves behind on a fresh
// GitHub checkout (D6-6): the remote-tracking ref for the fetched branch,
// but no refs/remotes/origin/HEAD symbolic ref.
func fabricateRemoteRef(t *testing.T, dir, branch, commit string) {
	t.Helper()
	if err := gitx.UpdateRef(context.Background(), dir, "refs/remotes/origin/"+branch, commit); err != nil {
		t.Fatalf("seeding refs/remotes/origin/%s: %v", branch, err)
	}
}

// setSymbolicRef points a symbolic ref (e.g. refs/remotes/origin/HEAD) at
// another ref name directly (`git symbolic-ref <name> <target>`) — the
// same on-disk state `git remote set-head origin -a` produces, without
// needing a real configured remote to query.
func setSymbolicRef(t *testing.T, dir, name, target string) {
	t.Helper()
	cmd := exec.Command("git", "symbolic-ref", name, target)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref %s %s: %v\n%s", name, target, err, out)
	}
}
