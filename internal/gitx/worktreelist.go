package gitx

import (
	"context"
	"fmt"
	"strings"
)

// WorktreeEntry is one entry from `git worktree list --porcelain`
// (spec/closure-hygiene dc-4): a single worktree registered against the
// repository, primary or linked, with its checked-out HEAD commit and
// (when not detached) branch.
type WorktreeEntry struct {
	// Path is the worktree's absolute filesystem path, exactly as git
	// reports it (already resolved — e.g. a macOS /var/folders temp path
	// comes back /private/var/folders'd).
	Path string
	// Head is the worktree's checked-out HEAD commit sha.
	Head string
	// Branch is the short local branch name checked out at this worktree
	// (e.g. "design/x"), or "" when the worktree's HEAD is detached —
	// AC-3(b): "for a detached HEAD, its commit" is the only name a caller
	// has, never a guessed branch.
	Branch string
	// Bare is true for a bare repository's own worktree entry (git worktree
	// list's "bare" porcelain line) — never true for an ordinary verdi
	// store checkout, but parsed rather than silently dropped so a caller
	// sees a complete, honest inventory.
	Bare bool
	// Prunable is true when git marks this entry prunable (git worktree
	// list's "prunable[ <reason>]" porcelain line) — for instance, a
	// worktree whose directory was deleted without `git worktree remove`,
	// so its gitdir link now points at a non-existent location. PrunableReason
	// carries git's own explanation when it supplies one (the text after
	// "prunable "). A caller surveying worktrees uses this to DISCLOSE why a
	// worktree's live state (clean/dirty) could not be resolved, rather than
	// guess it (spec/closure-hygiene AC-3(b)).
	Prunable       bool
	PrunableReason string
}

// WorktreeList lists every worktree registered against the repository at
// dir — `git worktree list --porcelain`, parsed (spec/closure-hygiene
// dc-4) — in git's own order, which always places the PRIMARY checkout
// first (git's own documented guarantee; a caller that must single it out
// cross-checks that first entry against its .git being a directory rather
// than a linked-worktree .git file, dc-4's two-signal exclusion — this
// function itself performs no such filtering, a pure parse of every entry
// git reports, not limited to worktrees under any particular root). One
// read-only `git worktree list` call — never `add`, `remove`, or `prune`.
func WorktreeList(ctx context.Context, dir string) ([]WorktreeEntry, error) {
	out, err := run(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("gitx: WorktreeList: %w", err)
	}
	return parseWorktreeList(string(out)), nil
}

// parseWorktreeList parses `git worktree list --porcelain`'s own format:
// one block per worktree, separated by a blank line, each block a set of
// "<key>[ <value>]" lines — "worktree <path>", "HEAD <sha>", "branch
// refs/heads/<name>", "detached", "bare", "locked[ <reason>]", "prunable[
// <reason>]". The "prunable" key is captured (a worktree survey needs it to
// disclose why a stale worktree's live state cannot be resolved —
// spec/closure-hygiene AC-3(b)); other unrecognized lines ("locked"'s own
// reason text, or any future porcelain key) are ignored rather than
// rejected, so a newer git adding a field this parser does not need never
// breaks it.
func parseWorktreeList(out string) []WorktreeEntry {
	var entries []WorktreeEntry
	var cur *WorktreeEntry

	flush := func() {
		if cur != nil {
			entries = append(entries, *cur)
			cur = nil
		}
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			flush()
			cur = &WorktreeEntry{Path: value}
		case "HEAD":
			if cur != nil {
				cur.Head = value
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		case "prunable":
			if cur != nil {
				cur.Prunable = true
				cur.PrunableReason = value // "" for a bare "prunable" line
			}
		}
	}
	flush()
	return entries
}
