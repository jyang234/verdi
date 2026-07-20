package reclaim

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/residue"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// TestLooksManagedAnywhere is the direct unit witness for the segment
// predicate: any "<root>/.verdi/data/worktrees/<name>" path is recognized as
// managed regardless of which checkout's root it is, while boundary-adjacent
// look-alikes are not. The segment is derived from wtmanager.WorktreesRoot
// (internal/wtmanager/naming.go), so this also pins that derivation.
func TestLooksManagedAnywhere(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "own checkout's managed leaf", path: "/store/primary/.verdi/data/worktrees/x", want: true},
		{name: "foreign checkout's managed leaf", path: "/store/checkout-b/.verdi/data/worktrees/foreign", want: true},
		{name: "nested below a managed leaf still matches (safe keep direction)", path: "/store/checkout-b/.verdi/data/worktrees/foreign/sub", want: true},
		{name: "unmanaged sibling outside any data zone", path: "/store/verdi-wt/other", want: false},
		{name: "trailing-boundary collision: worktrees-scratch", path: "/store/primary/.verdi/data/worktrees-scratch/x", want: false},
		{name: "leading-boundary collision: prefix.verdi", path: "/store/primary/prefix.verdi/data/worktrees/x", want: false},
		{name: "bare data-zone root, name-less, never a real worktree", path: "/store/primary/.verdi/data/worktrees", want: false},
		{name: "empty path", path: "", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := looksManagedAnywhere(c.path); got != c.want {
				t.Errorf("looksManagedAnywhere(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

// TestClassifyWorktreeRow_ForeignManagedSegment_KeptManaged is the unit
// witness for align finding judged-managed-jurisdiction-is-invoking-root-
// relative (controller adjudication R4-I-82): a worktree row whose
// residue-resolved Managed is FALSE — because internal/residue/survey.go
// resolves Managed against the INVOKING checkout's own WorktreesRoot, while
// `git worktree list` is repo-global — but whose path structurally sits
// under ANOTHER linked checkout's .verdi/data/worktrees/ must still be
// kept:managed, never eligible for --apply deletion. A genuinely unmanaged
// row (neither Managed nor a managed-segment path) stays eligible, proving
// the segment guard is not over-broad.
func TestClassifyWorktreeRow_ForeignManagedSegment_KeptManaged(t *testing.T) {
	const invokingRoot = "/store/primary"

	// Merged, clean, branched, and NOT the invoking path — so the row reaches
	// the managed/eligible decision (every earlier exclusion in the fixed
	// switch is already false).
	base := residue.Worktree{
		Branch: "design/foreign",
		Merged: true,
	}

	cases := []struct {
		name       string
		wt         residue.Worktree
		wantElig   bool
		wantReason KeptReason
	}{
		{
			name:       "own-managed: residue already resolved Managed=true -> kept:managed (the wt.Managed arm)",
			wt:         withField(base, func(w *residue.Worktree) { w.Managed = true; w.Path = "/store/verdi-wt/other" }),
			wantElig:   false,
			wantReason: KeptManaged,
		},
		{
			name:       "foreign-managed: Managed=false but path is under ANOTHER checkout's .verdi/data/worktrees/ -> kept:managed (cross-checkout defense-in-depth)",
			wt:         withField(base, func(w *residue.Worktree) { w.Path = "/store/other-checkout/.verdi/data/worktrees/foreign" }),
			wantElig:   false,
			wantReason: KeptManaged,
		},
		{
			name:     "genuinely-unmanaged: neither Managed nor a managed-segment path -> stays eligible (guard is not over-broad)",
			wt:       withField(base, func(w *residue.Worktree) { w.Path = "/store/elsewhere/wt/foreign" }),
			wantElig: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			elig, reason, _ := classifyWorktreeRow(c.wt, invokingRoot)
			if elig != c.wantElig {
				t.Errorf("eligible = %v, want %v", elig, c.wantElig)
			}
			if !elig && reason != c.wantReason {
				t.Errorf("reason = %v (%s), want %v (%s)", reason, reason, c.wantReason, c.wantReason)
			}
		})
	}
}

// TestCompute_ForeignCheckoutManagedWorktree_KeptNotReclaimed is the
// behavioral, end-to-end witness of the same finding against real git: a
// worktree that is MANAGED from a SECOND linked checkout's perspective (it
// lives under that checkout's .verdi/data/worktrees/) must be kept:managed
// when the reclaim sweep runs from the PRIMARY checkout — never eligible.
//
// `git worktree list` is repo-global, so residue surfaces the foreign
// checkout's managed worktree with Managed=false (survey.go resolves Managed
// against the invoking root only). Under pre-fix code that made it
// merged+clean+ELIGIBLE, and `gc --reclaim-unmanaged --apply` would delete
// another checkout's managed worktree behind its manager's back (this test
// is RED there); classifyWorktreeRow's managed-segment guard keeps it
// kept:managed (GREEN).
func TestCompute_ForeignCheckoutManagedWorktree_KeptNotReclaimed(t *testing.T) {
	root := newReclaimTestRepo(t)
	ctx := context.Background()

	// A merged design branch. Its managed worktree is cut BELOW by a second,
	// linked checkout, so the worktree lives under THAT checkout's data zone.
	const foreignBranch = "design/foreign"
	if err := gitx.CheckoutNewBranch(ctx, root, foreignBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", foreignBranch, err)
	}
	mustWriteFile(t, root, "foreign.txt", "f\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "foreign work")
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+foreignBranch, foreignBranch)

	// Second linked checkout, detached at main's tip (so it carries the
	// committed .verdi/.gitignore 'data/' rule and its own managed data zone
	// stays out of git) — the many-checkouts topology this repo lives in.
	checkoutB := filepath.Join(t.TempDir(), "checkout-b")
	runGit(t, root, "worktree", "add", "--detach", "--quiet", checkoutB, "main")

	// checkoutB cuts design/foreign's managed worktree through the real
	// production entry point, landing it under checkoutB/.verdi/data/
	// worktrees/foreign — MANAGED from B's perspective, foreign to root's.
	foreignWTPath, err := wtmanager.EnsureWorktree(ctx, checkoutB, foreignBranch)
	if err != nil {
		t.Fatalf("EnsureWorktree(%s from checkoutB): %v", foreignBranch, err)
	}
	// Fixture premise: the foreign worktree is NOT under root's own managed
	// root — otherwise this would not exercise the cross-checkout gap at all.
	rootManaged := realOrSelf(wtmanager.WorktreesRoot(root)) + string(filepath.Separator)
	if strings.HasPrefix(realOrSelf(foreignWTPath), rootManaged) {
		t.Fatalf("fixture invalid: foreign worktree %q is under root's managed root %q; the bug requires it foreign to root", foreignWTPath, rootManaged)
	}

	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatalf("residue.Scan: %v", err)
	}

	// The mis-jurisdiction itself, asserted at the residue seam: the survey,
	// run from root, reports the foreign worktree Managed=false, yet
	// merged+clean (reclaim-eligible-shaped). This is precisely the input the
	// predicate must defend against.
	var foreignRow *residue.Worktree
	for i := range res.Worktrees {
		if res.Worktrees[i].Branch == foreignBranch {
			foreignRow = &res.Worktrees[i]
		}
	}
	if foreignRow == nil {
		t.Fatalf("residue.Scan did not surface the foreign managed worktree %s: %+v", foreignBranch, res.Worktrees)
	}
	if foreignRow.Managed {
		t.Fatalf("fixture premise broken: residue reports the foreign worktree Managed=true; the cross-checkout bug requires the invoking-root survey to miss it (Managed=false)")
	}
	if !foreignRow.Merged || foreignRow.Dirty || foreignRow.MergedUnresolved || foreignRow.DirtyUnresolved {
		t.Fatalf("fixture premise broken: foreign worktree must be merged+clean to be reclaim-eligible-shaped, got %+v", *foreignRow)
	}

	plan := Compute(res, root, "main")

	item := planItemFor(t, plan, foreignBranch)
	if item.Eligible {
		t.Fatalf("foreign-checkout managed worktree %s classified ELIGIBLE; --apply would delete another checkout's managed worktree behind its manager's back (align finding judged-managed-jurisdiction-is-invoking-root-relative / R4-I-82)", foreignBranch)
	}
	if item.Reason != KeptManaged {
		t.Fatalf("foreign-checkout managed worktree %s kept:%s, want kept:managed", foreignBranch, item.Reason)
	}
}
