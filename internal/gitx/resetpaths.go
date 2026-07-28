package gitx

import (
	"context"
	"fmt"
)

// ResetPaths unstages exactly the given paths — `git reset -q HEAD -- <paths>`
// — restoring each path's index entry to its HEAD state while leaving the
// working tree untouched. It is the index-only inverse of AddPaths: a ritual
// that staged paths with AddPaths and must then abandon the whole change
// (spec/obligation-seam ac-3's post-flip rollback) resets exactly those index
// entries, so a subsequently-restored working tree and a HEAD-matching index
// leave `git status` clean. paths may be absolute or dir-relative; git
// resolves either against the repository root. Resetting a path that is not
// staged is a harmless no-op, so an over-broad reset (every path the ritual
// might have staged) is safe. Returns an error if paths is empty — a caller
// with nothing to unstage should not call this (mirrors AddPaths).
func ResetPaths(ctx context.Context, dir string, paths ...string) error {
	if len(paths) == 0 {
		return fmt.Errorf("gitx: ResetPaths(%s): no paths given", dir)
	}
	args := append([]string{"reset", "-q", "HEAD", "--"}, paths...)
	if _, err := run(ctx, dir, args...); err != nil {
		return fmt.Errorf("gitx: ResetPaths(%s, %v): %w", dir, paths, err)
	}
	return nil
}
