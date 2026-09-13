package refindex

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// The tests in this file prove WHICH ref name the default-branch walk
// (dc-4) and dc-5's merged-branch ancestry test actually read. The default
// branch is selected through specstate.ResolveDefaultBranch — the ONE
// shared default-branch resolution (env, origin/HEAD, single
// origin/main-or-master fallback; origin/<name> preferred over a
// same-named local branch, local name only when no remote-tracking ref
// exists; unresolved otherwise) — and that selected, git-resolvable ref
// name (a mutable ref, not an immutable SHA) is what every tree read and
// ancestry test reuses. A bare short name ("main") is no longer handed to
// git as a revision when origin/main exists: it does not resolve at all in
// a clone with no local main, and it resolves to a diverged local shadow
// when one exists, neither of which is the genuine remote default.

// neutralizeCIDefaultBranch clears CI_DEFAULT_BRANCH for the test so the
// fixture's own ref state — not the invoking shell's CI environment —
// decides the default branch (specstate's precedence step 1).
func neutralizeCIDefaultBranch(t *testing.T) {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "")
}

// createRemoteTrackingMain simulates a fetched refs/remotes/origin/main
// at commit — create-only ref plumbing, no remote, no network (co-2).
func createRemoteTrackingMain(t *testing.T, dir, commit string) {
	t.Helper()
	if err := gitx.UpdateRef(context.Background(), dir, "refs/remotes/origin/main", commit); err != nil {
		t.Fatalf("createRemoteTrackingMain: %v", err)
	}
}

func headSHA(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
}

// TestComputeIndex_RemoteOnlyDefaultBranch_NoLocalMain is the reported
// directory defect: a valid clone whose origin/HEAD points at origin/main
// and whose refs/remotes/origin/main exists, but which has NO local main
// (checked out on some other branch). The default-branch walk must read
// the remote-tracking revision — never fail because a bare "main" does
// not resolve.
func TestComputeIndex_RemoteOnlyDefaultBranch_NoLocalMain(t *testing.T) {
	neutralizeCIDefaultBranch(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/specs/active/alpha/spec.md": componentSpecMD("alpha", "active")},
		Message: "seed default-branch spec",
	}})
	createRemoteTrackingMain(t, repo.Dir, headSHA(t, repo.Dir))
	setDefaultBranchSymref(t, repo.Dir, "main")
	checkoutNewBranch(t, repo.Dir, "work")
	deleteLocalBranch(t, repo.Dir, "main")

	got, err := ComputeIndex(context.Background(), repo.Dir, NewGitRunner(), NewStateResolver())
	if err != nil {
		t.Fatalf("ComputeIndex with remote-only default branch: unexpected error: %v", err)
	}
	if want := []string{"spec/alpha"}; !reflect.DeepEqual(refs(got), want) {
		t.Fatalf("ComputeIndex refs = %v, want %v", refs(got), want)
	}
	if e := entryByRef(t, got, "spec/alpha"); e.Source != SourceDefault {
		t.Fatalf("spec/alpha Source = %q, want %q", e.Source, SourceDefault)
	}
}

// TestComputeIndex_DivergedLocalDefault_ReadsRemoteTrackingRevision proves
// a same-named LOCAL main that has diverged ahead of origin/main never
// substitutes for the genuine remote default: the tree walk reads
// origin/main (a spec only local main carries is not a default entry) and
// dc-5's ancestry test runs against origin/main (a design branch merged
// only into local main is still an UNMERGED draft).
func TestComputeIndex_DivergedLocalDefault_ReadsRemoteTrackingRevision(t *testing.T) {
	neutralizeCIDefaultBranch(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/specs/active/alpha/spec.md": componentSpecMD("alpha", "active")},
		Message: "seed default-branch spec",
	}})
	createRemoteTrackingMain(t, repo.Dir, headSHA(t, repo.Dir))
	setDefaultBranchSymref(t, repo.Dir, "main")

	// A design draft cut from the remote default, then fast-forwarded into
	// LOCAL main only — origin/main never advanced.
	checkoutNewBranch(t, repo.Dir, "design/zeta")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/zeta/spec.md": componentSpecMD("zeta", "draft")}, "zeta draft")
	checkoutExisting(t, repo.Dir, "main")
	runGit(t, repo.Dir, "merge", "--quiet", "--ff-only", "design/zeta")
	// And a spec that exists on local main alone.
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/local-only/spec.md": componentSpecMD("local-only", "active")}, "local-only spec")

	got, err := ComputeIndex(context.Background(), repo.Dir, NewGitRunner(), NewStateResolver())
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	if want := []string{"spec/alpha", "spec/zeta"}; !reflect.DeepEqual(refs(got), want) {
		t.Fatalf("ComputeIndex refs = %v, want %v (local-only lives on the diverged local main only; never a default entry)", refs(got), want)
	}
	if e := entryByRef(t, got, "spec/alpha"); e.Source != SourceDefault {
		t.Fatalf("spec/alpha Source = %q, want %q", e.Source, SourceDefault)
	}
	zeta := entryByRef(t, got, "spec/zeta")
	if zeta.Source != SourceLocal {
		t.Fatalf("spec/zeta Source = %q, want %q (merged into the diverged LOCAL main only — ancestry must run against origin/main, so it is still an unmerged draft)", zeta.Source, SourceLocal)
	}
	if zeta.StatusGroup != StatusGroupDraftsInProgress {
		t.Fatalf("spec/zeta StatusGroup = %q, want %q", zeta.StatusGroup, StatusGroupDraftsInProgress)
	}
}

// TestComputeIndex_DefaultBranchUnprovable_NoDefaultEntries pins the
// unresolved posture: origin/HEAD names origin/main but neither a
// refs/remotes/origin/main nor a local main exists. specstate reports the
// default branch unresolved; the walk contributes no default entries and
// raises no operational error (the existing "nothing to walk" contract
// TestComputeIndex_Fake_DefaultBranchUnconfigured already pins for the
// no-origin case) — it never guesses HEAD or any other revision instead.
func TestComputeIndex_DefaultBranchUnprovable_NoDefaultEntries(t *testing.T) {
	neutralizeCIDefaultBranch(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/specs/active/alpha/spec.md": componentSpecMD("alpha", "active")},
		Message: "seed spec",
	}})
	setDefaultBranchSymref(t, repo.Dir, "main") // dangling: no refs/remotes/origin/main is ever created
	checkoutNewBranch(t, repo.Dir, "work")
	deleteLocalBranch(t, repo.Dir, "main")

	got, err := ComputeIndex(context.Background(), repo.Dir, NewGitRunner(), NewStateResolver())
	if err != nil {
		t.Fatalf("ComputeIndex with unprovable default branch: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ComputeIndex = %v, want no entries (no default branch could be proven; HEAD is never a substitute)", refs(got))
	}
}

// TestGitRunner_DefaultBranch_PinsResolvedRevision fixes the production
// adapter's contract: the value handed to ListTree/Show/IsAncestor is the
// git-resolvable default-branch REF NAME specstate selected — "origin/main"
// when the remote-tracking ref exists (even with a local main present),
// "" when unresolved — and a non-repository directory is still an error.
// (The test name's "Pins" refers to fixing this contract, not to
// immutable-SHA pinning: the value is a mutable ref name.)
func TestGitRunner_DefaultBranch_PinsResolvedRevision(t *testing.T) {
	neutralizeCIDefaultBranch(t)
	ctx := context.Background()

	t.Run("remote-tracking ref wins over a same-named local branch", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"README.md": "x"}, Message: "seed"}})
		createRemoteTrackingMain(t, repo.Dir, headSHA(t, repo.Dir))
		setDefaultBranchSymref(t, repo.Dir, "main")
		got, err := NewGitRunner().DefaultBranch(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("DefaultBranch: %v", err)
		}
		if got != "origin/main" {
			t.Fatalf("DefaultBranch = %q, want %q (the resolved remote-tracking revision, never the bare short name)", got, "origin/main")
		}
	})

	t.Run("unresolved is empty, not an error", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"README.md": "x"}, Message: "seed"}})
		got, err := NewGitRunner().DefaultBranch(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("DefaultBranch: %v", err)
		}
		if got != "" {
			t.Fatalf("DefaultBranch = %q, want \"\" (no origin/HEAD, no origin/main|master: unresolved)", got)
		}
	})

	t.Run("not a repository is still an error", func(t *testing.T) {
		if _, err := NewGitRunner().DefaultBranch(ctx, t.TempDir()); err == nil {
			t.Fatal("DefaultBranch on a non-repository: want an error, got nil")
		}
	})
}
