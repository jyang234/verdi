package gitx

import (
	"context"
	"fmt"
)

// Show returns path's content as it existed at commit, via
// `git show <commit>:<path>` — the mechanism the index uses to resolve a
// pinned ref (kind/name@commit) to historical content that may differ from
// the current working tree. path is repo-relative (forward slashes).
func Show(ctx context.Context, dir, commit, path string) ([]byte, error) {
	out, err := run(ctx, dir, "show", commit+":"+path)
	if err != nil {
		return nil, fmt.Errorf("gitx: Show(%s:%s): %w", commit, path, err)
	}
	return out, nil
}
