package gitx

import (
	"bytes"
	"context"
	"fmt"
)

// UntrackedPaths lists the paths git does not track at all — `git ls-files
// -z --others --exclude-standard` — relative to dir with forward slashes,
// respecting .gitignore (`--exclude-standard`). Run from the repository's
// top level, the paths are repository-root-relative and cover the whole
// working tree; run from a subdirectory, git limits them to that
// subdirectory and makes them relative to it.
//
// -z is load-bearing: without it git quotes any name holding a non-ASCII
// byte, a double quote, or a control character (core.quotePath), and a
// newline inside a name would split one path into two. NUL cannot occur in
// a path, so splitting on it returns every name byte-exact.
//
// It is the read-only primitive the wall-changes classifier (spec/
// wall-changes ac-1) needs and no existing gitx query answers:
// LsFilesWithUntracked unions tracked with untracked rather than isolating
// the latter. A caller partitions a changed-path listing against this one
// to tell "a path git has never seen" from "a path git already tracks with
// a pending change" — the untracked-file/another-staged-path split ac-1's
// reason vocabulary draws. An empty repository (or an empty subdirectory)
// is not an error: it yields a nil slice.
func UntrackedPaths(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("gitx: UntrackedPaths(%q): %w", dir, err)
	}
	var paths []string
	for _, field := range bytes.Split(out, []byte{0}) {
		if len(field) > 0 {
			paths = append(paths, string(field))
		}
	}
	return paths, nil
}
