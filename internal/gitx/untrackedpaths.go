package gitx

import (
	"context"
	"fmt"
	"strings"
)

// UntrackedPaths lists the paths git does not track at all — `git ls-files
// --others --exclude-standard` — relative to dir with forward slashes,
// respecting .gitignore (`--exclude-standard`). It is the read-only
// primitive the wall-changes classifier (spec/wall-changes ac-1) needs and
// no existing gitx query answers: WorktreeChangedPaths reports every
// changed path (staged, unstaged, AND untracked) but discards which of the
// three a path is, and LsFilesWithUntracked unions tracked with untracked
// rather than isolating the latter. A caller partitions
// WorktreeChangedPaths' entries against this list to tell "a path git has
// never seen" from "a path git already tracks with a pending change" —
// exactly the untracked-file/another-staged-path split ac-1's closed
// reason vocabulary draws. An empty repository (or an empty subdirectory)
// is not an error: it yields a nil slice.
func UntrackedPaths(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("gitx: UntrackedPaths(%q): %w", dir, err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
