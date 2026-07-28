package reclaim

import (
	"strconv"
	"strings"
	"testing"
)

// TestRow_Line_OneTemplatePerKind proves dc-4's own line shapes render
// distinctly, one line per unit, mirroring internal/wtmanager.Result.Line()'s
// own per-Decision-template precedent — a worktree+branch unit and a
// branch-only unit, for the kinds that apply to both.
func TestRow_Line_OneTemplatePerKind(t *testing.T) {
	wtUnit := Unit{Branch: "design/x", WorktreePath: "/store/wt/x"}
	branchOnlyUnit := Unit{Branch: "close/y"}

	cases := []struct {
		name string
		row  Row
		want []string // every substring that must appear
		none []string // substrings that must NOT appear (cross-kind leakage guard)
	}{
		{
			name: "eligible worktree+branch",
			row:  Row{Kind: KindEligible, Unit: wtUnit},
			want: []string{"eligible:", "worktree /store/wt/x", "branch design/x"},
			none: []string{"tip", "kept", "reclaimed", "refused", "partial"},
		},
		{
			name: "eligible branch-only",
			row:  Row{Kind: KindEligible, Unit: branchOnlyUnit},
			want: []string{"eligible:", "branch close/y"},
			none: []string{"worktree", "tip"},
		},
		{
			name: "kept: unmerged",
			row:  Row{Kind: KindKept, Unit: wtUnit, Reason: KeptUnmerged},
			want: []string{"kept:", "worktree /store/wt/x", "branch design/x", "unmerged"},
		},
		{
			name: "kept: unresolved-state names residue's own Reason detail",
			row:  Row{Kind: KindKept, Unit: wtUnit, Reason: KeptUnresolvedState, Detail: "prunable: gitdir file points to non-existent location"},
			want: []string{"kept:", "unresolved-state", "prunable: gitdir file points to non-existent location"},
		},
		{
			name: "kept: branch-only invoking",
			row:  Row{Kind: KindKept, Unit: branchOnlyUnit, Reason: KeptInvoking},
			want: []string{"kept:", "branch close/y", "invoking"},
			none: []string{"worktree"},
		},
		{
			name: "kept: detached names the worktree path, never a dangling empty branch",
			row:  Row{Kind: KindKept, Unit: Unit{WorktreePath: "/store/wt/detached"}, Reason: KeptDetached},
			want: []string{"kept:", "worktree /store/wt/detached", "detached"},
			none: []string{"+ branch", "branch "},
		},
		{
			name: "reclaimed: worktree+branch names the tip",
			row:  Row{Kind: KindReclaimed, Unit: wtUnit, Tip: "deadbeef"},
			want: []string{"reclaimed:", "worktree /store/wt/x", "branch design/x", "tip deadbeef"},
		},
		{
			name: "reclaimed: branch-only names the tip",
			row:  Row{Kind: KindReclaimed, Unit: branchOnlyUnit, Tip: "cafebabe"},
			want: []string{"reclaimed:", "branch close/y", "tip cafebabe"},
			none: []string{"worktree"},
		},
		{
			name: "refused: worktree-remove step, branch-delete never attempted",
			row:  Row{Kind: KindRefused, Unit: wtUnit, Detail: "worktree has uncommitted changes"},
			want: []string{"refused:", "worktree /store/wt/x", "worktree removal refused", "worktree has uncommitted changes"},
		},
		{
			name: "refused: branch-only, branch-delete step",
			row:  Row{Kind: KindRefused, Unit: branchOnlyUnit, Detail: "checked out at '/store/primary'"},
			want: []string{"refused:", "branch close/y", "branch deletion refused", "checked out at"},
		},
		{
			name: "partial: worktree removed, branch NOT deleted",
			row:  Row{Kind: KindPartial, Unit: wtUnit, Detail: "not fully merged"},
			want: []string{"partial:", "worktree /store/wt/x removed", "branch design/x", "NOT deleted", "not fully merged"},
			none: []string{"reclaimed", "eligible"},
		},
	}

	var allLines []string
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.row.Line()
			for _, want := range c.want {
				if !strings.Contains(got, want) {
					t.Errorf("Line() = %q, want it to contain %q", got, want)
				}
			}
			for _, notWant := range c.none {
				if strings.Contains(got, notWant) {
					t.Errorf("Line() = %q, must NOT contain %q", got, notWant)
				}
			}
			allLines = append(allLines, got)
		})
	}

	seen := map[string]bool{}
	for i, line := range allLines {
		if seen[line] {
			t.Errorf("case %d produced a line duplicating an earlier case: %q", i, line)
		}
		seen[line] = true
	}
}

// TestRow_Line_UnhandledKind_FailsClosed pins Line()'s default arm (M3):
// a Row carrying a Kind value outside dc-4's closed set (KindEligible ..
// KindPartial) must render a legible, self-naming "internal error:
// unhandled row kind" line — never silently produce an empty string or one
// of the real templates — mirroring this package's own fail-closed
// convention (internal/artifactview.DecodeMeta's unhandled-kind guard). A
// human reading gc output sees the numeric kind and the unit's identity,
// so an unmapped kind is a loud diagnostic, not a silent blank. The switch
// enumerates 0..4; a value past the enum (here 99) and a negative value
// both exercise the arm.
func TestRow_Line_UnhandledKind_FailsClosed(t *testing.T) {
	wtUnit := Unit{Branch: "design/x", WorktreePath: "/store/wt/x"}
	for _, bogus := range []Kind{Kind(99), Kind(-1)} {
		row := Row{Kind: bogus, Unit: wtUnit}
		got := row.Line()
		for _, want := range []string{
			"reclaim: internal error",
			"unhandled row kind",
			// the numeric kind, so the diagnostic names exactly which value fell through
			strconv.Itoa(int(bogus)),
			// the unit's own identity, via the same unitDesc every real arm uses
			"worktree /store/wt/x + branch design/x",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("Row{Kind: %d}.Line() = %q, want it to contain %q (fail-closed diagnostic)", int(bogus), got, want)
			}
		}
		// A bogus kind must never masquerade as one of the real report leads.
		for _, leak := range []string{"eligible:", "kept:", "reclaimed:", "refused:", "partial:"} {
			if strings.HasPrefix(got, leak) {
				t.Errorf("Row{Kind: %d}.Line() = %q, must not render as the real %q template", int(bogus), got, leak)
			}
		}
	}
}

// TestPlan_DryRunRows_PreservesOrderAndClassification is a small, pure
// (no fixturegit) proof that DryRunRows is a straight, order-preserving
// projection of Plan.Items.
func TestPlan_DryRunRows_PreservesOrderAndClassification(t *testing.T) {
	plan := Plan{Items: []PlanItem{
		{Unit: Unit{Branch: "a"}, Eligible: true},
		{Unit: Unit{Branch: "b"}, Eligible: false, Reason: KeptDirty},
		{Unit: Unit{Branch: "c"}, Eligible: true},
	}}
	rows := plan.DryRunRows()
	if len(rows) != 3 {
		t.Fatalf("DryRunRows produced %d rows, want 3", len(rows))
	}
	wantKinds := []Kind{KindEligible, KindKept, KindEligible}
	wantBranches := []string{"a", "b", "c"}
	for i, row := range rows {
		if row.Kind != wantKinds[i] {
			t.Errorf("row %d: Kind = %v, want %v", i, row.Kind, wantKinds[i])
		}
		if row.Unit.Branch != wantBranches[i] {
			t.Errorf("row %d: Branch = %q, want %q", i, row.Unit.Branch, wantBranches[i])
		}
	}
	if rows[1].Reason != KeptDirty {
		t.Errorf("row 1: Reason = %v, want KeptDirty", rows[1].Reason)
	}
}
