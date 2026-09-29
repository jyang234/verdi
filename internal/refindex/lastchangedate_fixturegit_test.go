package refindex

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestComputeIndex_LastChangeDatesFixturegit is ac-1's behavioral
// obligation: over a fixturegit repository with a default-branch spec and a
// design branch each at known, DIFFERENT commit dates, ComputeIndex's real
// production adapters (NewGitRunner, NewStateResolver — no fakes) return
// exactly those dates, and the default-branch walk still runs once (the
// same repo already proves ComputeIndex is deterministic across two calls
// elsewhere in this package).
func TestComputeIndex_LastChangeDatesFixturegit(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/specs/active/root-spec/spec.md": componentSpecMD("root-spec", "active")},
			Message: "seed default branch",
		},
	})
	setDefaultBranchSymref(t, repo.Dir, "main")
	// fixturegit.Build pins every layer's commit to 2024-01-01T00:00:00Z
	// (its own fixedDate); gitx.CommitDate normalizes that to this exact
	// canonical form regardless of the host git version's own %cI rendering
	// ("Z" vs "+00:00" — CLAUDE.md: deterministic artifacts).
	const rootSpecWantDate = "2024-01-01T00:00:00+00:00"

	// A design branch committed at a KNOWN, DIFFERENT date — 14 days later
	// — proving the design-branch tip's OWN date is read, never the default
	// branch's.
	checkoutNewBranch(t, repo.Dir, "design/aged-draft")
	const agedDraftDate = "1705276800 +0000" // 2024-01-15T00:00:00Z
	writeAndCommitAt(t, repo.Dir, agedDraftDate, map[string]string{
		".verdi/specs/active/aged-draft/spec.md": componentSpecMD("aged-draft", "draft"),
	}, "aged-draft commit at a known, later date")
	checkoutExisting(t, repo.Dir, "main")
	const agedDraftWantDate = "2024-01-15T00:00:00+00:00"

	deps := NewGitRunner()
	resolver := NewStateResolver()
	ctx := context.Background()

	got, err := ComputeIndex(ctx, repo.Dir, deps, resolver)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}

	root := entryByRef(t, got, "spec/root-spec")
	if root.Date != rootSpecWantDate {
		t.Fatalf("spec/root-spec Date = %q, want %q (the fixturegit seed commit's own pinned date)", root.Date, rootSpecWantDate)
	}
	if root.DateDisclosed != nil {
		t.Fatalf("spec/root-spec DateDisclosed = %+v, want nil", root.DateDisclosed)
	}

	aged := entryByRef(t, got, "spec/aged-draft")
	if aged.Date != agedDraftWantDate {
		t.Fatalf("spec/aged-draft Date = %q, want %q (its own branch tip's pinned date, not the default branch's)", aged.Date, agedDraftWantDate)
	}
	if aged.DateDisclosed != nil {
		t.Fatalf("spec/aged-draft DateDisclosed = %+v, want nil", aged.DateDisclosed)
	}
	if aged.Date == root.Date {
		t.Fatalf("spec/aged-draft and spec/root-spec share the same Date %q, want distinct dates (the two commits were pinned 14 days apart)", aged.Date)
	}
}
