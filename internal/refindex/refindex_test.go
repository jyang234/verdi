package refindex

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestComputeIndex_OneEntryPerSpecOrDraft_MergedBranchExcluded is ac-1's
// behavioral obligation: N default-branch specs, M(>=2) unmerged design
// branches each with their own draft, and one ALREADY-MERGED-but-not-yet-
// deleted design branch — asserting N+M entries, no duplicate, no drop, and
// determinism across two calls.
func TestComputeIndex_OneEntryPerSpecOrDraft_MergedBranchExcluded(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/specs/active/alpha/spec.md": componentSpecMD("alpha", "active"),
				".verdi/specs/active/beta/spec.md":  componentSpecMD("beta", "active"),
			},
			Message: "seed default-branch specs",
		},
	})
	setDefaultBranchSymref(t, repo.Dir, "main")

	// Two unmerged design branches, each with its own draft at a distinct
	// commit.
	checkoutNewBranch(t, repo.Dir, "design/gamma")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/gamma/spec.md": componentSpecMD("gamma", "draft")}, "gamma draft")
	checkoutExisting(t, repo.Dir, "main")

	checkoutNewBranch(t, repo.Dir, "design/delta")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/delta/spec.md": componentSpecMD("delta", "draft")}, "delta draft")
	checkoutExisting(t, repo.Dir, "main")

	// A design branch whose work is already merged into main but not yet
	// deleted (dc-5) — its spec must be counted exactly once, from the
	// default-branch walk, not again from the design-branch walk.
	checkoutNewBranch(t, repo.Dir, "design/epsilon")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/epsilon/spec.md": componentSpecMD("epsilon", "active")}, "epsilon spec")
	checkoutExisting(t, repo.Dir, "main")
	runGit(t, repo.Dir, "merge", "--quiet", "--ff-only", "design/epsilon")

	deps := NewGitRunner()
	ctx := context.Background()

	got, err := ComputeIndex(ctx, repo.Dir, deps)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}

	want := []string{"spec/alpha", "spec/beta", "spec/delta", "spec/epsilon", "spec/gamma"}
	if got2 := refs(got); !reflect.DeepEqual(got2, want) {
		t.Fatalf("ComputeIndex refs = %v, want %v (5 entries: 3 default + 2 unmerged design, epsilon counted once)", got2, want)
	}

	epsilon := entryByRef(t, got, "spec/epsilon")
	if epsilon.Source != SourceDefault {
		t.Fatalf("spec/epsilon Source = %q, want %q (merged design branch contributes no design-walk entry)", epsilon.Source, SourceDefault)
	}
	gamma := entryByRef(t, got, "spec/gamma")
	if gamma.Source != SourceLocal {
		t.Fatalf("spec/gamma Source = %q, want %q", gamma.Source, SourceLocal)
	}

	// Determinism: an unmodified second call returns byte-identical output.
	got2, err := ComputeIndex(ctx, repo.Dir, deps)
	if err != nil {
		t.Fatalf("ComputeIndex (second call): %v", err)
	}
	if !reflect.DeepEqual(got, got2) {
		t.Fatalf("ComputeIndex is not deterministic:\nfirst:  %+v\nsecond: %+v", got, got2)
	}
}

// TestComputeIndex_Sources is ac-2's behavioral obligation: a local-only, a
// remote-only, and a both-sourced design branch each chip their real
// Source, and the both-sourced branch yields exactly one entry.
func TestComputeIndex_Sources(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/specs/active/root-spec/spec.md": componentSpecMD("root-spec", "active")},
			Message: "seed default branch",
		},
	})
	setDefaultBranchSymref(t, repo.Dir, "main")

	// Local-only.
	checkoutNewBranch(t, repo.Dir, "design/local-only")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/local-only/spec.md": componentSpecMD("local-only", "draft")}, "local-only draft")
	checkoutExisting(t, repo.Dir, "main")

	// Remote-only: authored on a throwaway local branch (so the commit
	// exists), simulated as remote-tracking-only by deleting the local ref
	// after pointing refs/remotes/origin/design/remote-only at its commit —
	// hermetic, no network fetch (CO-2).
	checkoutNewBranch(t, repo.Dir, "design/remote-only-tmp")
	remoteOnlySHA := writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/remote-only/spec.md": componentSpecMD("remote-only", "draft")}, "remote-only draft")
	checkoutExisting(t, repo.Dir, "main")
	createRemoteDesignRef(t, repo.Dir, "remote-only", remoteOnlySHA)
	deleteLocalBranch(t, repo.Dir, "design/remote-only-tmp")

	// Both: a local branch whose tip is also mirrored as a remote-tracking
	// ref at the identical commit (simulating "pushed and up to date").
	checkoutNewBranch(t, repo.Dir, "design/both")
	bothSHA := writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/both/spec.md": componentSpecMD("both", "draft")}, "both draft")
	checkoutExisting(t, repo.Dir, "main")
	createRemoteDesignRef(t, repo.Dir, "both", bothSHA)

	got, err := ComputeIndex(context.Background(), repo.Dir, NewGitRunner())
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}

	cases := []struct {
		ref  string
		want Source
	}{
		{"spec/local-only", SourceLocal},
		{"spec/remote-only", SourceRemote},
		{"spec/both", SourceBoth},
	}
	for _, tc := range cases {
		e := entryByRef(t, got, tc.ref) // fails if not exactly one entry
		if e.Source != tc.want {
			t.Fatalf("%s Source = %q, want %q", tc.ref, e.Source, tc.want)
		}
	}
}

// TestComputeIndex_StatusGroups is ac-3's behavioral obligation: an active
// component, an accepted-pending-build story, a terminal (superseded)
// component, and a design-branch draft each land in their ratified
// StatusGroup, deterministically across two calls — including the design
// draft's unconditional drafts-in-progress grouping despite its own content
// declaring a different status ("active"), proving the override is truly
// unconditional rather than coincidentally matching.
func TestComputeIndex_StatusGroups(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/specs/active/comp-active/spec.md":    componentSpecMD("comp-active", "active"),
				".verdi/specs/active/story-accepted/spec.md": storySpecAcceptedMD("story-accepted"),
				".verdi/specs/active/comp-terminal/spec.md":  componentSpecMD("comp-terminal", "superseded"),
			},
			Message: "seed mixed statuses",
		},
	})
	setDefaultBranchSymref(t, repo.Dir, "main")

	checkoutNewBranch(t, repo.Dir, "design/draft-x")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/draft-x/spec.md": componentSpecMD("draft-x", "active")}, "draft-x (content says active)")
	checkoutExisting(t, repo.Dir, "main")

	deps := NewGitRunner()
	ctx := context.Background()

	got, err := ComputeIndex(ctx, repo.Dir, deps)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}

	cases := []struct {
		ref  string
		want StatusGroup
	}{
		{"spec/comp-active", StatusGroupActiveComponents},
		{"spec/story-accepted", StatusGroupAcceptedPendingBuild},
		{"spec/comp-terminal", StatusGroupTerminal},
		{"spec/draft-x", StatusGroupDraftsInProgress},
	}
	for _, tc := range cases {
		e := entryByRef(t, got, tc.ref)
		if e.StatusGroup != tc.want {
			t.Fatalf("%s StatusGroup = %q, want %q", tc.ref, e.StatusGroup, tc.want)
		}
	}

	got2, err := ComputeIndex(ctx, repo.Dir, deps)
	if err != nil {
		t.Fatalf("ComputeIndex (second call): %v", err)
	}
	if !reflect.DeepEqual(got, got2) {
		t.Fatalf("ComputeIndex grouping is not deterministic:\nfirst:  %+v\nsecond: %+v", got, got2)
	}
}

// TestComputeIndex_DisclosedNoDraftSpec is ac-4's behavioral obligation: a
// design branch cut from the default branch but never given a spec.md
// commit (the "branch-cut-before-scaffold-commit window") yields nil error,
// exactly one entry, and a non-nil Disclosed field distinguishable from
// every ordinary draft entry.
//
// The branch is deliberately left with a tip IDENTICAL to the default
// branch's (no commit at all past the cut — `git branch`, no checkout, no
// new commit) — the hardest case: gitx.IsAncestor treats a commit as its
// own ancestor, so a naive merged-branch check applied before the spec.md
// existence probe would wrongly treat this as "already merged" and drop it
// silently, contradicting this AC's "never a silent absence". Ordering the
// existence probe first (refindex.go's computeDesignBranchEntries) is what
// keeps this fixture — realistic, not adversarial — correctly disclosed.
func TestComputeIndex_DisclosedNoDraftSpec(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/specs/active/root-spec/spec.md": componentSpecMD("root-spec", "active")},
			Message: "seed default branch",
		},
	})
	setDefaultBranchSymref(t, repo.Dir, "main")
	runGit(t, repo.Dir, "branch", "design/empty-draft") // cut, no checkout, no new commit

	got, err := ComputeIndex(context.Background(), repo.Dir, NewGitRunner())
	if err != nil {
		t.Fatalf("ComputeIndex: unexpected error: %v", err)
	}

	e := entryByRef(t, got, "spec/empty-draft")
	if e.Disclosed == nil {
		t.Fatal("spec/empty-draft: Disclosed = nil, want a populated disclosure (ac-4)")
	}
	if e.StatusGroup != StatusGroupDraftsInProgress {
		t.Fatalf("spec/empty-draft StatusGroup = %q, want %q", e.StatusGroup, StatusGroupDraftsInProgress)
	}
	if e.SpecStatus != "" {
		t.Fatalf("spec/empty-draft SpecStatus = %q, want empty (no content was readable)", e.SpecStatus)
	}

	// Distinguishable from an ordinary draft entry in the same result set.
	root := entryByRef(t, got, "spec/root-spec")
	if root.Disclosed != nil {
		t.Fatalf("spec/root-spec (an ordinary default-branch entry) has a non-nil Disclosed: %+v", root.Disclosed)
	}
}

// TestComputeIndex_NeverMovesHEAD is ac-5's behavioral obligation: the
// serving checkout's HEAD and working tree are byte-identical before and
// after a ComputeIndex run against a repo carrying multiple design
// branches.
func TestComputeIndex_NeverMovesHEAD(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/specs/active/root-spec/spec.md": componentSpecMD("root-spec", "active")},
			Message: "seed default branch",
		},
	})
	setDefaultBranchSymref(t, repo.Dir, "main")

	checkoutNewBranch(t, repo.Dir, "design/one")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/one/spec.md": componentSpecMD("one", "draft")}, "one draft")
	checkoutExisting(t, repo.Dir, "main")

	checkoutNewBranch(t, repo.Dir, "design/two")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/two/spec.md": componentSpecMD("two", "draft")}, "two draft")
	checkoutExisting(t, repo.Dir, "main")

	beforeBranch := strings.TrimSpace(runGit(t, repo.Dir, "symbolic-ref", "--short", "HEAD"))
	beforeHead := strings.TrimSpace(runGit(t, repo.Dir, "rev-parse", "HEAD"))
	beforeStatus := runGit(t, repo.Dir, "status", "--porcelain")
	beforeHash := hashWorkingTree(t, repo.Dir)

	if _, err := ComputeIndex(context.Background(), repo.Dir, NewGitRunner()); err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}

	afterBranch := strings.TrimSpace(runGit(t, repo.Dir, "symbolic-ref", "--short", "HEAD"))
	afterHead := strings.TrimSpace(runGit(t, repo.Dir, "rev-parse", "HEAD"))
	afterStatus := runGit(t, repo.Dir, "status", "--porcelain")
	afterHash := hashWorkingTree(t, repo.Dir)

	if beforeBranch != afterBranch {
		t.Fatalf("current branch changed: before %q, after %q", beforeBranch, afterBranch)
	}
	if beforeHead != afterHead {
		t.Fatalf("HEAD commit changed: before %q, after %q", beforeHead, afterHead)
	}
	if strings.TrimSpace(beforeStatus) != "" || strings.TrimSpace(afterStatus) != "" {
		t.Fatalf("working tree not clean: before %q, after %q", beforeStatus, afterStatus)
	}
	if beforeHash != afterHash {
		t.Fatal("working tree content hash changed across a ComputeIndex run")
	}
}
