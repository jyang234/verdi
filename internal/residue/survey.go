package residue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// scanMergedBranches is AC-3(a): every local branch, other than
// defaultBranch itself, whose tip is an ancestor of defaultTip — counted
// and named, sorted. Read-only: gitx.LocalBranches and gitx.IsAncestor
// only (ac-3's static obligation).
func scanMergedBranches(ctx context.Context, root, defaultBranch, defaultTip string) ([]string, error) {
	branches, err := gitx.LocalBranches(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("residue: listing local branches: %w", err)
	}

	var merged []string
	for _, b := range branches {
		if b == defaultBranch {
			continue
		}
		tip, err := gitx.RevParse(ctx, root, b)
		if err != nil {
			return nil, fmt.Errorf("residue: resolving %s: %w", b, err)
		}
		ok, err := gitx.IsAncestor(ctx, root, tip, defaultTip)
		if err != nil {
			return nil, fmt.Errorf("residue: checking %s merged: %w", b, err)
		}
		if ok {
			merged = append(merged, b)
		}
	}
	sort.Strings(merged)
	return merged, nil
}

// Worktree is one AC-3(b) entry: a registered, non-primary git worktree.
type Worktree struct {
	Path    string
	Branch  string // "" for a detached HEAD (dc-4: disclosed, never guessed)
	Commit  string // HEAD commit sha, always populated
	Managed bool
	Merged  bool
	Dirty   bool
}

// scanWorktrees is AC-3(b): every git worktree registered against root
// (gitx.WorktreeList, not limited to managed worktrees — dc-4), excluding
// the primary checkout, each named with its branch (or, for a detached
// HEAD, its commit alone) and whether it is merged, clean, and managed.
//
// Merged is resolved at the COMMIT level uniformly (gitx.IsAncestor
// against defaultTip, using the worktree's own reported HEAD commit) for
// every entry, branched or detached alike — never a guessed branch-level
// property a detached worktree does not have (dc-4/ac-3's own disclosure
// requirement is satisfied by construction: the same primitive resolves
// both cases identically, so there is no separate "unknown" case to
// reach). Managed is decided against wtmanager.WorktreesRoot (dc-4's
// shared definition, not a second hardcoded literal). Zero git-mutating
// calls anywhere in this path (ac-3's static obligation) — worktree list,
// merge-base/rev-parse, and status checks only.
func scanWorktrees(ctx context.Context, root, defaultTip string) ([]Worktree, error) {
	entries, err := gitx.WorktreeList(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("residue: listing worktrees: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	primary := entries[0]
	if !looksLikePrimaryWorktree(primary) {
		return nil, fmt.Errorf("residue: internal error: git worktree list's first entry %q does not look like the primary checkout (dc-4's two-signal cross-check failed: its .git is not a directory)", primary.Path)
	}

	resolvedRoot, rerr := filepath.EvalSymlinks(root)
	if rerr != nil {
		resolvedRoot = root // best effort; git's own reported paths are already resolved either way
	}
	managedRoot := filepath.Clean(wtmanager.WorktreesRoot(resolvedRoot))

	var out []Worktree
	for _, e := range entries[1:] {
		merged, err := gitx.IsAncestor(ctx, root, e.Head, defaultTip)
		if err != nil {
			return nil, fmt.Errorf("residue: checking worktree %s merged: %w", e.Path, err)
		}
		dirty, err := gitx.StatusDirty(ctx, e.Path)
		if err != nil {
			return nil, fmt.Errorf("residue: checking worktree %s dirty: %w", e.Path, err)
		}

		out = append(out, Worktree{
			Path:    e.Path,
			Branch:  e.Branch,
			Commit:  e.Head,
			Managed: isUnderRoot(filepath.Clean(e.Path), managedRoot),
			Merged:  merged,
			Dirty:   dirty,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// looksLikePrimaryWorktree is dc-4's second, independent signal cross-
// checking git's own first-entry-is-primary ordering: the primary
// checkout's .git is a directory, never a linked-worktree .git FILE
// (which instead contains a "gitdir: <path>" pointer).
func looksLikePrimaryWorktree(e gitx.WorktreeEntry) bool {
	info, err := os.Stat(filepath.Join(e.Path, ".git"))
	return err == nil && info.IsDir()
}

// isUnderRoot reports whether path is a proper descendant of root — both
// arguments already filepath.Clean'd by the caller, and (in production)
// resolved through the SAME symlink-resolution pass, so a host where the
// store root itself sits behind a symlink (e.g. a t.TempDir() on macOS)
// cannot spuriously misclassify a managed worktree as unmanaged.
func isUnderRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
