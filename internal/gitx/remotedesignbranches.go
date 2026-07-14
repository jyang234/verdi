package gitx

import (
	"context"
	"strings"
)

// RemoteDesignBranches lists dir's remote-tracking `refs/remotes/origin/design/*`
// branch short names, "origin/" stripped so a name is directly comparable
// to LocalBranches's own short names (e.g. both return "design/foo") — the
// remote-tracking half of spec/ref-index ac-2's enumeration (dc-5's oq-2
// resolution: local and remote-tracking design refs alike join the index).
// Sorted by git's default refname order (deterministic), mirroring
// LocalBranches. The remote name is hardcoded to "origin", matching every
// other single-remote assumption already load-bearing in this store
// (gitx.DefaultBranch, gitx.Push, gitx.HasRemote's callers — spec/ref-index
// dc-2).
func RemoteDesignBranches(ctx context.Context, dir string) ([]string, error) {
	out, err := run(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin/design")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		branches = append(branches, strings.TrimPrefix(line, "origin/"))
	}
	return branches, nil
}
