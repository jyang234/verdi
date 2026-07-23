package gitx

import (
	"bytes"
	"context"
	"fmt"
	"sort"
)

// StagedPaths returns the repository-relative paths whose index entries
// differ from HEAD. Git's NUL-delimited output preserves every legal path
// byte, including whitespace and newlines; the returned paths are sorted so
// callers can report them deterministically.
func StagedPaths(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "diff", "--cached", "--name-only", "-z", "--")
	if err != nil {
		return nil, fmt.Errorf("gitx: StagedPaths(%s): %w", dir, err)
	}

	fields := bytes.Split(out, []byte{0})
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) > 0 {
			paths = append(paths, string(field))
		}
	}
	sort.Strings(paths)
	return paths, nil
}
