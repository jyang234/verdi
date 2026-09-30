package gitx

import (
	"context"
	"fmt"
	"strings"
)

// Location is where a directory sits in its repository: the top level
// every repository-root-relative path resolves against, and the
// directory's own path below it.
type Location struct {
	// TopLevel is the repository's working-tree root, as `git rev-parse
	// --show-toplevel` prints it (an absolute path; symlinks resolved).
	TopLevel string
	// Prefix is the directory's path relative to TopLevel, a slash path
	// ending in "/", or "" when the directory IS the top level (the same
	// answer RepoPrefix gives).
	Prefix string
}

// Locate answers both halves of Location in one `git rev-parse
// --show-toplevel --show-prefix` call. A caller that mixes git queries
// whose paths mean different things — `git status` always answers
// repository-root-relative, while `git ls-files` and `git ls-tree` answer
// relative to the directory they run in — runs every query from TopLevel
// and names its own paths as Prefix plus the store-relative path, so all
// of them speak one vocabulary even when the store sits below the git root.
//
// git prints one line per answer, so a path holding a newline cannot be
// carried unambiguously; that case is an error, never a guess.
func Locate(ctx context.Context, dir string) (Location, error) {
	out, err := run(ctx, dir, "rev-parse", "--show-toplevel", "--show-prefix")
	if err != nil {
		return Location{}, fmt.Errorf("gitx: Locate(%s): %w", dir, err)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != 2 || lines[0] == "" {
		return Location{}, fmt.Errorf("gitx: Locate(%s): `git rev-parse --show-toplevel --show-prefix` printed %d lines, want 2 (a path holding a newline cannot be located unambiguously)", dir, len(lines))
	}
	return Location{TopLevel: lines[0], Prefix: lines[1]}, nil
}
