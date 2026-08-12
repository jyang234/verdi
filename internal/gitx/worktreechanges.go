package gitx

import (
	"bytes"
	"context"
	"fmt"
	"sort"
)

// WorktreeChangedPaths returns the unique, sorted repository-root-relative
// paths that differ from HEAD in either the index or the worktree, plus
// untracked paths — `git status --porcelain=v1 -z --untracked-files=all` —
// without ever reading or digesting the paths' content.
//
// Unlike StagedPaths (index-vs-HEAD only), this reports staged changes,
// unstaged worktree edits, deletions, and untracked files alike: the
// context compiler's worktree-overlay source needs every path that departs
// from the committed HEAD tree, not only what a subsequent commit would
// absorb. -z NUL-delimits entries so no legal path byte (whitespace,
// newlines, non-ASCII) is lost, and BOTH sides of a rename are returned for
// the same reason StagedPaths returns both: `git status`'s human format
// collapses a rename into "old -> new", but the source path's absence and
// the destination path's presence are each their own fact a caller must
// see.
func WorktreeChangedPaths(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, fmt.Errorf("gitx: WorktreeChangedPaths(%s): %w", dir, err)
	}
	paths, err := parseWorktreeStatus(out)
	if err != nil {
		return nil, fmt.Errorf("gitx: WorktreeChangedPaths(%s): %w", dir, err)
	}
	sort.Strings(paths)
	return dedupeSorted(paths), nil
}

// parseWorktreeStatus extracts every changed path from `git status
// --porcelain=v1 -z --untracked-files=all` output.
//
// Each entry is "XY <path>" NUL-terminated, where X is the index status and
// Y the worktree status. Under -z a rename or copy (X or Y of 'R'/'C')
// appends its ORIGINAL path as a second NUL-terminated field immediately
// after, reversing the human format's "from -> to" into "to\0from\0" with
// the arrow dropped. A clean entry (X == ' ' && Y == ' ') cannot occur —
// `git status` never lists an unchanged path — and an ignored entry
// ('!' in both columns, only ever emitted with an explicit --ignored flag
// this call never passes) is skipped defensively rather than assumed
// absent.
func parseWorktreeStatus(out []byte) ([]string, error) {
	fields := bytes.Split(out, []byte{0})
	var paths []string
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) == 0 {
			// The trailing NUL yields one empty final field.
			continue
		}
		if len(entry) < 4 || entry[2] != ' ' {
			return nil, fmt.Errorf("malformed `git status --porcelain=v1 -z` entry %q", entry)
		}
		index, worktree, path := entry[0], entry[1], string(entry[3:])

		orig := ""
		if index == 'R' || index == 'C' || worktree == 'R' {
			i++
			if i >= len(fields) || len(fields[i]) == 0 {
				return nil, fmt.Errorf("`git status --porcelain=v1 -z` entry %q declares a rename/copy with no original-path field", entry)
			}
			orig = string(fields[i])
		}

		if index == '!' && worktree == '!' {
			continue
		}
		paths = append(paths, path)
		// A rename's source departs from HEAD (deleted at its old path) in
		// either the index or worktree column; a copy's source is
		// untouched, so only a rename ('R') contributes its original path.
		if index == 'R' || worktree == 'R' {
			paths = append(paths, orig)
		}
	}
	return paths, nil
}

// dedupeSorted removes adjacent duplicates from an already-sorted slice —
// a status entry can in principle name the same path twice (e.g. as both a
// rename destination and, through a distinct entry, an unrelated
// modification target), and WorktreeChangedPaths promises unique paths.
func dedupeSorted(paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	out := paths[:1]
	for _, p := range paths[1:] {
		if p != out[len(out)-1] {
			out = append(out, p)
		}
	}
	return out
}
