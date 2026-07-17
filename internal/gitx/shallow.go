package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsShallow reports whether dir is a shallow clone — whether git's own
// shallow-boundary marker file exists. A shallow clone's `git log` silently
// stops at that boundary rather than at the ref's true root, so a caller
// that walks history to exhaustion (cmd/verdi/sync_ancestor.go's fetch
// walk) must disclose the truncation instead of claiming it saw the ref's
// entire history.
//
// The marker's location is resolved via `git rev-parse --git-path shallow`
// rather than a hard-coded dir/.git/shallow, which is worktree-correct: in
// a linked worktree the shallow marker lives in the COMMON git dir (shallow
// is a repository-wide property), and `--git-path` returns that absolute
// path, whereas dir/.git is only a gitdir pointer file there. git prints
// the path relative to dir when it is under dir and absolute otherwise, so
// a relative result is joined onto dir before the existence check. This
// only reads the marker's PRESENCE — never its contents — so it stays a
// cheap, allocation-light probe with no bearing on the walk's semantics.
func IsShallow(ctx context.Context, dir string) (bool, error) {
	out, err := run(ctx, dir, "rev-parse", "--git-path", "shallow")
	if err != nil {
		return false, fmt.Errorf("gitx: IsShallow(%s): %w", dir, err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return false, fmt.Errorf("gitx: IsShallow(%s): git returned an empty shallow-marker path", dir)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	switch _, statErr := os.Stat(path); {
	case statErr == nil:
		return true, nil
	case errors.Is(statErr, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("gitx: IsShallow(%s): stat %s: %w", dir, path, statErr)
	}
}
