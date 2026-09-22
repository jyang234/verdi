package branchbase

import (
	"context"
	"os/exec"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// buildRepo pins CI_DEFAULT_BRANCH to empty (2A-I1): the three non-env
// tests below all rely on the name-resolution chain finding nothing, but
// CI_DEFAULT_BRANCH is that chain's own FIRST link
// (internal/specstate/defaultbranch.go), and this repo's test suites
// leak it into child processes routinely (GitLab CI also defines it as a
// predefined variable) — cmd/verdi/design_test.go pins it at every
// non-env site for the same reason. TestResolve_CIDefaultBranchEnv keeps
// its own explicit t.Setenv, which still wins (it runs after this one in
// its own subtest's environment).
func buildRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "")
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "layer 1"},
	})
}

func fabricateRemoteRef(t *testing.T, dir, branch, commit string) {
	t.Helper()
	if err := gitx.UpdateRef(context.Background(), dir, "refs/remotes/origin/"+branch, commit); err != nil {
		t.Fatalf("seeding refs/remotes/origin/%s: %v", branch, err)
	}
}

func setSymbolicRef(t *testing.T, dir, name, target string) {
	t.Helper()
	cmd := exec.Command("git", "symbolic-ref", name, target)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref %s %s: %v\n%s", name, target, err, out)
	}
}

func addOriginRemote(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "remote", "add", "origin", "https://example.invalid/repo.git")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add origin: %v\n%s", err, out)
	}
}

func TestResolve_ResolvedDefault(t *testing.T) {
	repo := buildRepo(t)
	fabricateRemoteRef(t, repo.Dir, "main", repo.Head)
	setSymbolicRef(t, repo.Dir, "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	got, err := Resolve(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Kind != ResolvedDefault || got.Ref != "origin/main" || got.BranchName != "main" || got.Commit != repo.Head {
		t.Fatalf("Resolve = %+v, want ResolvedDefault origin/main @ %s", got, repo.Head)
	}
}

func TestResolve_HeadFallback_NoOrigin(t *testing.T) {
	repo := buildRepo(t)

	got, err := Resolve(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Kind != HeadFallback || got.Ref != "HEAD" || got.Commit != repo.Head {
		t.Fatalf("Resolve = %+v, want HeadFallback @ %s", got, repo.Head)
	}
}

func TestResolve_Unresolvable_OriginConfiguredButNoDefault(t *testing.T) {
	repo := buildRepo(t)
	addOriginRemote(t, repo.Dir)

	got, err := Resolve(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Kind != Unresolvable {
		t.Fatalf("Resolve = %+v, want Unresolvable", got)
	}
}

func TestResolve_CIDefaultBranchEnv(t *testing.T) {
	repo := buildRepo(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	got, err := Resolve(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Kind != ResolvedDefault || got.Ref != "main" || got.BranchName != "main" || got.Commit != repo.Head {
		t.Fatalf("Resolve = %+v, want ResolvedDefault main/main @ %s", got, repo.Head)
	}
}

func TestResolve_NotARepository(t *testing.T) {
	if _, err := Resolve(context.Background(), t.TempDir()); err == nil {
		t.Fatal("Resolve(non-repo) = nil error, want an error")
	}
}
