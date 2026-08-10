package gitx

import (
	"context"
	"fmt"
)

// ApplyPatch applies patch — unified-diff bytes, supplied on stdin — to
// dir's working tree via `git apply` (spec/execution-workspace §Exact
// workspace materialization, shape (b): "patch application is new code (no
// git apply wrapper exists anywhere in internal/gitx)").
//
// Any git-apply failure — a malformed patch, or a genuine conflict against
// dir's current content — is returned as an error carrying git's own
// stderr text (via runStdin's existing wrap), so a caller can disclose the
// concrete reason rather than a bare non-zero exit.
func ApplyPatch(ctx context.Context, dir string, patch []byte) error {
	if _, err := runStdin(ctx, dir, nil, patch, "apply"); err != nil {
		return fmt.Errorf("gitx: ApplyPatch: %w", err)
	}
	return nil
}
