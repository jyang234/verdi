package gitx

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// runExactRefGit runs one fixture-setup git command in dir. It is this
// file's own helper so the test depends on no other gitx test file.
func runExactRefGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestResolveExactRef_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	first, second := repo.Heads[0], repo.Heads[1]
	runExactRefGit(t, repo.Dir, "update-ref", "refs/remotes/origin/close/spec-x", first)
	runExactRefGit(t, repo.Dir, "update-ref", "refs/remotes/origin/main", second)
	runExactRefGit(t, repo.Dir, "pack-refs", "--all")
	runExactRefGit(t, repo.Dir, "update-ref", "refs/remotes/origin/loose", second)

	tests := []struct {
		name, ref, want string
	}{
		{name: "packed remote-tracking ref", ref: "refs/remotes/origin/close/spec-x", want: first},
		{name: "packed ref at another commit", ref: "refs/remotes/origin/main", want: second},
		{name: "loose remote-tracking ref", ref: "refs/remotes/origin/loose", want: second},
		{name: "local branch by its full name", ref: "refs/heads/main", want: repo.Head},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveExactRef(ctx, repo.Dir, tt.ref)
			if err != nil {
				t.Fatalf("ResolveExactRef(%q): %v", tt.ref, err)
			}
			if got != tt.want {
				t.Fatalf("ResolveExactRef(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

// TestResolveExactRef_Negative includes the E4a review's M-1 probe: with
// the exact refs/remotes/origin/<name> absent, a tag or a branch literally
// named refs/remotes/origin/<name> at HEAD makes `git rev-parse --verify`
// resolve through its lookup rules — ResolveExactRef must not.
func TestResolveExactRef_Negative(t *testing.T) {
	ctx := context.Background()

	t.Run("lookup-rule decoys", func(t *testing.T) {
		repo := buildRepo(t)
		runExactRefGit(t, repo.Dir, "tag", "refs/remotes/origin/tag-decoy", repo.Head)
		runExactRefGit(t, repo.Dir, "branch", "refs/remotes/origin/branch-decoy", repo.Head)
		for _, ref := range []string{"refs/remotes/origin/tag-decoy", "refs/remotes/origin/branch-decoy"} {
			// The decoy is live: rev-parse, with its lookup rules, resolves it.
			if got, err := RevParse(ctx, repo.Dir, ref); err != nil || got != repo.Head {
				t.Fatalf("test setup: RevParse(%q) = %q, %v; want the decoy at HEAD", ref, got, err)
			}
			if got, err := ResolveExactRef(ctx, repo.Dir, ref); err == nil {
				t.Fatalf("ResolveExactRef(%q) = %q, nil; want an error (the exact ref is absent)", ref, got)
			}
		}
	})

	t.Run("absent ref", func(t *testing.T) {
		repo := buildRepo(t)
		if got, err := ResolveExactRef(ctx, repo.Dir, "refs/remotes/origin/absent"); err == nil {
			t.Fatalf("ResolveExactRef(absent) = %q, nil; want an error", got)
		}
	})

	t.Run("not a full refname", func(t *testing.T) {
		repo := buildRepo(t)
		for _, ref := range []string{"", "HEAD", "main", "origin/main", "remotes/origin/main", "-h", "--all", repo.Head, "main~1"} {
			if got, err := ResolveExactRef(ctx, repo.Dir, ref); err == nil {
				t.Fatalf("ResolveExactRef(%q) = %q, nil; want an error", ref, got)
			}
		}
	})

	t.Run("revision syntax under refs/", func(t *testing.T) {
		repo := buildRepo(t)
		runExactRefGit(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Head)
		for _, ref := range []string{"refs/remotes/origin/main~1", "refs/remotes/origin/main^", "refs/remotes/origin/main^{commit}", "refs/remotes/origin/main@{0}"} {
			if got, err := ResolveExactRef(ctx, repo.Dir, ref); err == nil {
				t.Fatalf("ResolveExactRef(%q) = %q, nil; want an error (no revision expression is evaluated)", ref, got)
			}
		}
	})

	t.Run("not a repository", func(t *testing.T) {
		if got, err := ResolveExactRef(ctx, t.TempDir(), "refs/heads/main"); err == nil {
			t.Fatalf("ResolveExactRef outside a repository = %q, nil; want an error", got)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		repo := buildRepo(t)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if got, err := ResolveExactRef(cancelled, repo.Dir, "refs/heads/main"); err == nil {
			t.Fatalf("ResolveExactRef with a cancelled context = %q, nil; want an error", got)
		}
	})
}
