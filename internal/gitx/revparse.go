package gitx

import (
	"context"
	"fmt"
	"strings"
)

// RevParse resolves rev (a ref name, a commit, or "<rev>:<path>") to the
// object id `git rev-parse` would resolve it to, run inside dir. It fails if
// dir is not inside a git repository or rev does not resolve.
func RevParse(ctx context.Context, dir, rev string) (string, error) {
	out, err := run(ctx, dir, "rev-parse", "--verify", rev)
	if err != nil {
		return "", fmt.Errorf("gitx: RevParse(%q): %w", rev, err)
	}
	return strings.TrimSpace(string(out)), nil
}
