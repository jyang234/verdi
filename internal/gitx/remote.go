package gitx

import (
	"context"
	"fmt"
	"strings"
)

// RemoteURL returns the URL configured for remote (typically "origin") in
// dir's git repository (`git remote get-url <name>`) — used by phase 5's
// `sync` to auto-detect the forge kind (gitlab/github) when verdi.yaml
// carries no explicit `forge:` key (I-22).
func RemoteURL(ctx context.Context, dir, name string) (string, error) {
	out, err := run(ctx, dir, "remote", "get-url", name)
	if err != nil {
		return "", fmt.Errorf("gitx: RemoteURL(%q): %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}
