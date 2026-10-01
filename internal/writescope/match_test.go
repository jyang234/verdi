package writescope_test

import (
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

func TestRefPatternMatches(t *testing.T) {
	tests := []struct {
		name             string
		pattern          ws.RefPattern
		ref              string
		checkedOutBefore string
		want             bool
	}{
		{"exact ref matches itself", "refs/heads/policy/adopt", "refs/heads/policy/adopt", "", true},
		{"exact ref rejects a different ref", "refs/heads/policy/adopt", "refs/heads/policy/other", "", false},
		{"namespace wildcard matches one trailing segment", "refs/heads/design/*", "refs/heads/design/x", "", true},
		{"namespace wildcard matches several trailing segments", "refs/heads/design/*", "refs/heads/design/x/y", "", true},
		{"namespace wildcard rejects the bare namespace itself", "refs/heads/design/*", "refs/heads/design", "", false},
		{"namespace wildcard rejects a same-prefix sibling with no separator", "refs/heads/design/*", "refs/heads/designer", "", false},
		{"any-local-branch wildcard matches any branch", "refs/heads/*", "refs/heads/anything/at/all", "", true},
		{"any-local-branch wildcard rejects a non-refs/heads ref", "refs/heads/*", "refs/tags/v1", "", false},
		{"checked-out resolves against the pre-ritual branch", ws.RefCheckedOut, "refs/heads/main", "main", true},
		{"checked-out rejects a different branch", ws.RefCheckedOut, "refs/heads/side", "main", false},
		{"checked-out with no pre-ritual branch (detached) never matches", ws.RefCheckedOut, "refs/heads/main", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pattern.Matches(tt.ref, tt.checkedOutBefore); got != tt.want {
				t.Errorf("RefPattern(%q).Matches(%q, %q) = %v, want %v", tt.pattern, tt.ref, tt.checkedOutBefore, got, tt.want)
			}
		})
	}
}

func TestWorktreePatternMatches(t *testing.T) {
	const storeRoot = "/repo"
	tests := []struct {
		name             string
		pattern          ws.WorktreePattern
		path             string
		registeredBefore bool
		want             bool
	}{
		{"temp matches a path outside the store", ws.WorktreeTemp, "/tmp/elsewhere/x", false, true},
		{"temp rejects a path inside the store", ws.WorktreeTemp, "/repo/.verdi/data/worktrees/x", false, false},
		{"temp rejects the store root itself", ws.WorktreeTemp, "/repo", false, false},
		{"registered matches when already registered before the ritual", ws.WorktreeRegistered, "/repo/.verdi/data/worktrees/x", true, true},
		{"registered rejects when not registered before the ritual", ws.WorktreeRegistered, "/repo/.verdi/data/worktrees/x", false, false},
		{"directory pattern matches an immediate child", ".verdi/data/worktrees/*", "/repo/.verdi/data/worktrees/abc", false, true},
		{"directory pattern rejects a grandchild", ".verdi/data/worktrees/*", "/repo/.verdi/data/worktrees/abc/nested", false, false},
		{"directory pattern rejects a sibling directory", ".verdi/data/worktrees/*", "/repo/.verdi/data/execution/abc", false, false},
		{"directory pattern rejects a path outside the store", ".verdi/data/worktrees/*", "/tmp/abc", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pattern.Matches(tt.path, storeRoot, tt.registeredBefore); got != tt.want {
				t.Errorf("WorktreePattern(%q).Matches(%q, %q, %v) = %v, want %v", tt.pattern, tt.path, storeRoot, tt.registeredBefore, got, tt.want)
			}
		})
	}
}

func TestPathPatternMatches(t *testing.T) {
	tests := []struct {
		name    string
		pattern ws.PathPattern
		path    string
		want    bool
	}{
		{"whole tree matches anything", ws.PathWholeTree, "anything/at/all.txt", true},
		{"exact file matches itself", ".verdi/policy/constitution.md", ".verdi/policy/constitution.md", true},
		{"exact file rejects a different file", ".verdi/policy/constitution.md", ".verdi/policy/other.md", false},
		{"single-segment wildcard matches one file", ".verdi/diagrams/*", ".verdi/diagrams/foo.svg", true},
		{"single-segment wildcard rejects a nested file", ".verdi/diagrams/*", ".verdi/diagrams/sub/foo.svg", false},
		{"directory pattern matches the directory path itself", ".verdi/specs/active/*/", ".verdi/specs/active/foo", true},
		{"directory pattern matches a file under it", ".verdi/specs/active/*/", ".verdi/specs/active/foo/spec.md", true},
		{"directory pattern matches a nested file under it", ".verdi/specs/active/*/", ".verdi/specs/active/foo/sub/spec.md", true},
		{"directory pattern rejects a sibling directory", ".verdi/specs/active/*/", ".verdi/specs/archive/foo/spec.md", false},
		{"directory pattern rejects a path above it", ".verdi/specs/active/*/", ".verdi/specs/active", false},
		{"plain directory pattern (no wildcard) matches a nested file", ".verdi/policy/overlays/", ".verdi/policy/overlays/x/y.md", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pattern.Matches(tt.path); got != tt.want {
				t.Errorf("PathPattern(%q).Matches(%q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
