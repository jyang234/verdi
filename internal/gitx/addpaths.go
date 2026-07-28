package gitx

import (
	"context"
	"fmt"
)

// AddPaths stages exactly the given paths — `git add -- <paths...>` — for
// rituals that must commit only the files they themselves modified, never
// the rest of the working tree. Unlike AddAll's `git add -A`, a sibling
// untracked or modified file elsewhere in dir is never picked up (D6-33: an
// acceptance ritual that writes a frozen stamp swept an unrelated untracked
// scratch binary into its commit via AddAll — this is the fix).
//
// paths may be absolute, or relative to DIR: `git add`'s pathspecs resolve
// against the process's working directory — which is dir — never against the
// repository root. A repository-root-relative path passed with a dir deeper
// than the repository root is a fatal, stages-nothing `git add` ("fatal:
// pathspec '<path>' did not match any files"), proven in
// TestAddPaths_PathspecsResolveAgainstDir. Every caller today passes dir =
// the store root with store-relative paths, where the two readings coincide,
// so this is a caveat for the next caller rather than a live defect.
//
// Returns an error if paths is empty — a scoped-add call site with nothing to
// stage is a caller bug, not a legal no-op: a ritual about to commit always
// has at least one path it owns.
func AddPaths(ctx context.Context, dir string, paths ...string) error {
	if len(paths) == 0 {
		return fmt.Errorf("gitx: AddPaths(%s): no paths given", dir)
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := run(ctx, dir, args...); err != nil {
		return fmt.Errorf("gitx: AddPaths(%s, %v): %w", dir, paths, err)
	}
	return nil
}
