package wtmanager

import (
	"path/filepath"
	"testing"
)

// TestWorktreesRoot_MatchesPrivateDefinition proves the exported wrapper
// (spec/closure-hygiene dc-4) computes exactly the same path the package's
// own private worktreesRoot does — the whole point of exporting it being
// that a consumer (internal/residue) shares this ONE definition rather than
// a second hardcoded .verdi/data/worktrees/ literal.
func TestWorktreesRoot_MatchesPrivateDefinition(t *testing.T) {
	for _, root := range []string{"/tmp/store", "relative/store", ""} {
		got := WorktreesRoot(root)
		want := worktreesRoot(root)
		if got != want {
			t.Fatalf("WorktreesRoot(%q) = %q, want %q (private worktreesRoot)", root, got, want)
		}
	}
}

func TestWorktreesRoot_Shape(t *testing.T) {
	got := WorktreesRoot("/store")
	want := filepath.Join("/store", ".verdi", "data", "worktrees")
	if got != want {
		t.Fatalf("WorktreesRoot(/store) = %q, want %q", got, want)
	}
}
