package residue

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestScan_EmptyDefaultBranchRef_AssertsNothing(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                  "data/\n",
			".verdi/specs/active/widget/spec.md": storySpecMD("widget", "accepted-pending-build", "feature-x"),
		},
		Message: "seed a spec that would otherwise be scannable",
	}})

	got, err := Scan(context.Background(), repo.Dir, "")
	if err != nil {
		t.Fatalf("Scan(defaultBranchRef=\"\"): unexpected error: %v", err)
	}
	if got.DefaultBranchResolved {
		t.Fatal("Scan(defaultBranchRef=\"\").DefaultBranchResolved = true, want false")
	}
	if len(got.PatternA) != 0 || len(got.PatternB) != 0 || len(got.CloseBranches) != 0 ||
		len(got.MergedBranches) != 0 || len(got.Worktrees) != 0 {
		t.Fatalf("Scan(defaultBranchRef=\"\") = %+v, want every field zero (assert nothing)", got)
	}
	if got.Flagged() {
		t.Fatal("Scan(defaultBranchRef=\"\").Flagged() = true, want false")
	}
}

func TestScan_Negative_UnresolvableDefaultBranchRef_IsARealError(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "root",
	}})

	_, err := Scan(context.Background(), repo.Dir, "does-not-exist-anywhere")
	if err == nil {
		t.Fatal("Scan(bogus, non-empty defaultBranchRef): want a real error, got nil (must not silently downgrade to \"unresolved\")")
	}
}

func TestScan_Negative_NotARepo(t *testing.T) {
	if _, err := Scan(context.Background(), t.TempDir(), "main"); err == nil {
		t.Fatal("Scan outside a repo: want error, got nil")
	}
}

func TestResult_Flagged(t *testing.T) {
	cases := []struct {
		name string
		r    Result
		want bool
	}{
		{"empty", Result{}, false},
		{"pattern-a present", Result{PatternA: []PatternA{{SpecName: "x"}}}, true},
		{"pattern-b alone never flags", Result{PatternB: []PatternB{{SpecName: "x"}}}, false},
		{
			"ritual-incomplete close branch flags",
			Result{CloseBranches: []CloseBranch{{Name: "x", Class: RitualIncomplete}}},
			true,
		},
		{
			"superseded-elsewhere alone never flags",
			Result{CloseBranches: []CloseBranch{{Name: "x", Class: SupersededElsewhere}}},
			false,
		},
		{
			"survey alone never flags, even unmerged+dirty+unmanaged",
			Result{
				MergedBranches: []string{"a"},
				Worktrees:      []Worktree{{Path: "/x", Merged: false, Dirty: true, Managed: false}},
			},
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.r.Flagged(); got != c.want {
				t.Errorf("Flagged() = %v, want %v (%+v)", got, c.want, c.r)
			}
		})
	}
}

func TestResult_Flagged_NilReceiver(t *testing.T) {
	var r *Result
	if r.Flagged() {
		t.Fatal("(*Result)(nil).Flagged() = true, want false")
	}
}
