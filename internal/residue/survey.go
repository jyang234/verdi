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
//
// Merged and Dirty are asserted facts ONLY when their paired *Unresolved
// flag is false. A per-worktree git failure (e.g. `git status` cannot run
// in a worktree whose directory was deleted without `git worktree remove`)
// sets the flag and leaves the paired field at its zero value — AC-3(b)'s
// posture of a state "disclosed rather than guessed" when it cannot be
// resolved. The zero Worktree is therefore a fully-resolved one, and a
// stale registration never aborts the survey; it is disclosed in place.
type Worktree struct {
	Path    string
	Branch  string // "" for a detached HEAD (dc-4: disclosed, never guessed)
	Commit  string // HEAD commit sha, always populated
	Managed bool
	Merged  bool
	Dirty   bool
	// MergedUnresolved / DirtyUnresolved report that this worktree's merge
	// state / clean state could not be resolved; when true, the paired
	// Merged / Dirty field is not an assertion and must not be read as one.
	MergedUnresolved bool
	DirtyUnresolved  bool
	// Reason names why an aspect was unresolvable — git's own prunable
	// reason where it supplies one (a worktree directory deleted without
	// `git worktree remove` is marked prunable), else the failing command's
	// error. "" when everything about this worktree resolved.
	Reason string
}

// scanWorktrees is AC-3(b): every git worktree registered against root
// (gitx.WorktreeList, not limited to managed worktrees — dc-4), excluding
// the primary checkout, each named with its branch (or, for a detached
// HEAD, its commit alone) and whether it is merged, clean, and managed.
//
// Merged is resolved at the COMMIT level uniformly (gitx.IsAncestor
// against defaultTip, using the worktree's own reported HEAD commit) for
// every entry, branched or detached alike — never a guessed branch-level
// property a detached worktree does not have. Managed is decided against
// wtmanager.WorktreesRoot (dc-4's shared definition, not a second hardcoded
// literal). Zero git-mutating calls anywhere in this path (ac-3's static
// obligation) — worktree list, merge-base/rev-parse, and status checks
// only.
//
// A per-worktree git failure (the survey's motivating population is 31
// long-lived registrations, so a stale one — a directory deleted without
// `git worktree remove`, which git still lists, marked prunable — is an
// expected input, not an exceptional one) is DISCLOSED on that worktree's
// own entry, never propagated as an operational error that would abort the
// whole audit: AC-3(b) requires such a state "disclosed rather than
// guessed" when it cannot be resolved. Only a failure to enumerate
// worktrees at all, or a primary checkout that fails dc-4's cross-check,
// is still a hard error — those are not per-worktree resolution gaps.
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
		wt := Worktree{
			Path:    e.Path,
			Branch:  e.Branch,
			Commit:  e.Head,
			Managed: isUnderRoot(filepath.Clean(e.Path), managedRoot),
		}

		// Merge state is resolved in root against the porcelain-reported HEAD
		// sha, so a deleted worktree directory does not by itself break it —
		// but any failure is disclosed on this entry, never propagated.
		if merged, err := gitx.IsAncestor(ctx, root, e.Head, defaultTip); err != nil {
			wt.MergedUnresolved = true
			wt.Reason = worktreeUnresolvedReason(e, fmt.Sprintf("merge state: %v", err))
		} else {
			wt.Merged = merged
		}

		// Clean state runs `git status` INSIDE the worktree directory, so a
		// worktree whose directory was deleted without `git worktree remove`
		// cannot be resolved — disclosed here, never an abort.
		if dirty, err := gitx.StatusDirty(ctx, e.Path); err != nil {
			wt.DirtyUnresolved = true
			wt.Reason = worktreeUnresolvedReason(e, fmt.Sprintf("clean state: %v", err))
		} else {
			wt.Dirty = dirty
		}

		out = append(out, wt)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// worktreeUnresolvedReason is the disclosure text for a worktree whose
// live state could not be resolved: git's own prunable reason where it
// supplies one (the honest "why" the entry is stale — a directory deleted
// without `git worktree remove` is marked prunable, with its gitdir-link
// explanation), else the failing command's own error.
func worktreeUnresolvedReason(e gitx.WorktreeEntry, fallback string) string {
	if e.Prunable {
		if e.PrunableReason != "" {
			return "prunable: " + e.PrunableReason
		}
		return "prunable"
	}
	return fallback
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
