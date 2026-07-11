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

// LsFilesWithUntracked lists every path under dir that git either tracks or
// sees as an untracked, non-ignored file — `git ls-files --cached --others
// --exclude-standard` — relative to dir with forward slashes. Unlike LsFiles
// (tracked only), it surfaces brand-new untracked files, so a caller
// enumerating a corpus catches additions git has not yet been told about
// (D4: "staleness is detected, never guessed"). `--exclude-standard` keeps
// .gitignore'd paths out — e.g. the store's `.verdi/data/`, covered by the
// committed `.verdi/.gitignore`. A tracked file deleted from the working
// tree stays listed (it still lives in the index): a caller that hashes
// on-disk content must treat a listed-but-absent path as deleted. It fails
// if dir is not inside a git repository; an empty result is a nil slice, not
// an error.
func LsFilesWithUntracked(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "ls-files", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("gitx: LsFilesWithUntracked(%q): %w", dir, err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
