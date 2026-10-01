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
		{"exact ref rejects its own short name", "refs/heads/policy/adopt", "policy/adopt", "", false},
		{"namespace wildcard matches one trailing segment", "refs/heads/design/*", "refs/heads/design/x", "", true},
		{"namespace wildcard matches several trailing segments", "refs/heads/design/*", "refs/heads/design/x/y", "", true},
		{"namespace wildcard rejects the bare namespace itself", "refs/heads/design/*", "refs/heads/design", "", false},
		{"namespace wildcard rejects a same-prefix sibling with no separator", "refs/heads/design/*", "refs/heads/designer", "", false},
		{"namespace wildcard rejects the remote-tracking twin", "refs/heads/design/*", "refs/remotes/origin/design/x", "", false},
		{"any-local-branch wildcard matches any branch", "refs/heads/*", "refs/heads/anything/at/all", "", true},
		{"any-local-branch wildcard rejects a tag", "refs/heads/*", "refs/tags/v1", "", false},
		{"any-local-branch wildcard rejects a remote-tracking ref", "refs/heads/*", "refs/remotes/origin/main", "", false},
		{"any-local-branch wildcard rejects a private namespace", "refs/heads/*", "refs/verdi/private", "", false},
		{"any-local-branch wildcard rejects a short branch name", "refs/heads/*", "main", "", false},
		{"checked-out resolves against the pre-ritual branch", ws.RefCheckedOut, "refs/heads/main", "refs/heads/main", true},
		{"checked-out rejects a different branch", ws.RefCheckedOut, "refs/heads/side", "refs/heads/main", false},
		{"checked-out rejects a tag of the same short name", ws.RefCheckedOut, "refs/tags/main", "refs/heads/main", false},
		{"checked-out rejects a short pre-ritual name", ws.RefCheckedOut, "refs/heads/main", "main", false},
		{"checked-out rejects a pre-ritual HEAD outside refs/heads", ws.RefCheckedOut, "refs/remotes/origin/main", "refs/remotes/origin/main", false},
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
	atRoot := ws.WorktreeSite{RepoRoot: "/repo", StoreRoot: "/repo"}
	inSub := ws.WorktreeSite{RepoRoot: "/repo", StoreRoot: "/repo/store"}
	registered := ws.WorktreeSite{RepoRoot: "/repo", StoreRoot: "/repo", RegisteredBefore: true}
	tests := []struct {
		name    string
		pattern ws.WorktreePattern
		path    string
		site    ws.WorktreeSite
		want    bool
	}{
		{"temp matches a fresh path outside the repository", ws.WorktreeTemp, "/tmp/elsewhere/x", atRoot, true},
		{"temp matches a same-prefix sibling of the repository", ws.WorktreeTemp, "/repository-sibling/x", atRoot, true},
		{"temp rejects a path inside the repository", ws.WorktreeTemp, "/repo/.verdi/data/worktrees/x", atRoot, false},
		{"temp rejects the repository root itself", ws.WorktreeTemp, "/repo", atRoot, false},
		{"temp rejects a path inside the repository but outside a sub-store", ws.WorktreeTemp, "/repo/elsewhere/x", inSub, false},
		{"temp rejects a worktree registered before the ritual, wherever it lies", ws.WorktreeTemp, "/tmp/elsewhere/x", registered, false},
		{"registered matches when already registered before the ritual", ws.WorktreeRegistered, "/repo/.verdi/data/worktrees/x", registered, true},
		{"registered matches a registered worktree outside the repository", ws.WorktreeRegistered, "/tmp/elsewhere/x", registered, true},
		{"registered rejects when not registered before the ritual", ws.WorktreeRegistered, "/repo/.verdi/data/worktrees/x", atRoot, false},
		{"directory pattern matches an immediate child", ".verdi/data/worktrees/*", "/repo/.verdi/data/worktrees/abc", atRoot, true},
		{"directory pattern matches an immediate child of a sub-store", ".verdi/data/worktrees/*", "/repo/store/.verdi/data/worktrees/abc", inSub, true},
		{"directory pattern reads the store root, not the repository root", ".verdi/data/worktrees/*", "/repo/.verdi/data/worktrees/abc", inSub, false},
		{"directory pattern rejects a grandchild", ".verdi/data/worktrees/*", "/repo/.verdi/data/worktrees/abc/nested", atRoot, false},
		{"directory pattern rejects a sibling directory", ".verdi/data/worktrees/*", "/repo/.verdi/data/execution/abc", atRoot, false},
		{"directory pattern rejects a path outside the store", ".verdi/data/worktrees/*", "/tmp/abc", atRoot, false},
		{"directory pattern rejects the directory itself", ".verdi/data/worktrees/*", "/repo/.verdi/data/worktrees", atRoot, false},
		{"a malformed pattern matches nothing", "worktrees", "/repo/worktrees", atRoot, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pattern.Matches(tt.path, tt.site); got != tt.want {
				t.Errorf("WorktreePattern(%q).Matches(%q, %+v) = %v, want %v", tt.pattern, tt.path, tt.site, got, tt.want)
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
		{"exact file rejects a case variant", ".verdi/policy/constitution.md", ".verdi/policy/Constitution.md", false},
		{"single-segment wildcard matches one file", ".verdi/diagrams/*", ".verdi/diagrams/foo.svg", true},
		{"single-segment wildcard matches a non-ASCII name", "owned/*", "owned/café.txt", true},
		{"single-segment wildcard rejects a nested file", ".verdi/diagrams/*", ".verdi/diagrams/sub/foo.svg", false},
		{"single-segment wildcard rejects the directory itself", ".verdi/diagrams/*", ".verdi/diagrams", false},
		{"directory pattern matches the directory path itself", ".verdi/specs/active/*/", ".verdi/specs/active/foo", true},
		{"directory pattern matches a file under it", ".verdi/specs/active/*/", ".verdi/specs/active/foo/spec.md", true},
		{"directory pattern matches a nested file under it", ".verdi/specs/active/*/", ".verdi/specs/active/foo/sub/spec.md", true},
		{"directory pattern rejects a sibling directory", ".verdi/specs/active/*/", ".verdi/specs/archive/foo/spec.md", false},
		{"directory pattern rejects a path above it", ".verdi/specs/active/*/", ".verdi/specs/active", false},
		{"plain directory pattern (no wildcard) matches a nested file", ".verdi/policy/overlays/", ".verdi/policy/overlays/x/y.md", true},
		{"a relative path that climbs out matches no directory pattern", "owned/", "../owned/x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pattern.Matches(tt.path); got != tt.want {
				t.Errorf("PathPattern(%q).Matches(%q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
