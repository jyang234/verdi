package gitx

import (
	"context"
	"fmt"
)

// Show returns path's content as it existed at commit, via
// `git show <commit>:<path>` — the mechanism the index uses to resolve a
// pinned ref (kind/name@commit) to historical content that may differ from
// the current working tree. path is repo-relative (forward slashes). Inside
// a read session for dir (WithReadSession) a blob is read from the
// session's one batch process instead — the same bytes `git show` prints
// for a blob, which it streams raw — and every other case still runs `git
// show` itself.
func Show(ctx context.Context, dir, commit, path string) ([]byte, error) {
	if s := sessionFor(ctx, dir); s != nil {
		if data, ok := s.blob(ctx, commit, path); ok {
			return data, nil
		}
	}
	out, err := run(ctx, dir, "show", commit+":"+path)
	if err != nil {
		return nil, fmt.Errorf("gitx: Show(%s:%s): %w", commit, path, err)
	}
	return out, nil
}
