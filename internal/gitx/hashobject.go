package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HashObject computes the git blob SHA-1 of path's current on-disk content
// — exactly what `git hash-object` would compute, and exactly what `git
// add` would store, regardless of whether path is tracked, staged, or
// merely sitting dirty in the working tree (I-15: "dirty working files
// hashed as git would hash the blob"). path may be relative to dir or
// absolute; dir need not itself be inside a git repository — blob hashing
// is a pure function of file bytes, not repository state.
func HashObject(ctx context.Context, dir, path string) (string, error) {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(dir, path)
	}
	if _, err := os.Stat(full); err != nil {
		return "", fmt.Errorf("gitx: HashObject(%q): %w", path, err)
	}

	out, err := run(ctx, dir, "hash-object", full)
	if err != nil {
		return "", fmt.Errorf("gitx: HashObject(%q): %w", path, err)
	}
	return strings.TrimSpace(string(out)), nil
}
