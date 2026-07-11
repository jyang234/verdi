package gitx

import (
	"context"
	"fmt"
	"strings"
)

// LsFiles lists every path git tracks under dir (respecting .gitignore),
// relative to dir with forward slashes — the store's committed-zone
// enumeration (D4). It fails if dir is not inside a git repository. An
// empty repository (or an empty subdirectory) is not an error: it yields a
// nil slice.
func LsFiles(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "ls-files")
	if err != nil {
		return nil, fmt.Errorf("gitx: LsFiles(%q): %w", dir, err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
