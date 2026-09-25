package gitx

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrShallowHistory is FirstParentPathCommits' refusal in a shallow
// repository: the first-parent chain ends at the shallow boundary rather
// than at the ref's true root, so a listing there cannot prove which commit
// was first (SI-270: no date is invented; the missing witness is disclosed).
var ErrShallowHistory = errors.New("gitx: history is shallow; the first-parent chain below the shallow boundary is missing")

// FirstParentPathCommits lists, oldest first, the commits on ref's
// first-parent chain that change any of paths relative to their first
// parent: `git --literal-pathspecs rev-list --first-parent --reverse ref --
// paths...`. A merge commit that brings a path in through its second parent
// is listed; the side-branch commit that first wrote it is not. Between two
// listed commits every path's content is constant, so a caller looking for
// the earliest first-parent commit whose tree holds a path needs to inspect
// only these commits (SI-270's acceptance point).
//
// Paths match byte for byte, never as a glob. An empty list is not an error.
// ref must name one commit: it is refused when shaped like an option, a
// negation (^ref), a range (a..b, a...b, ref^!, ref^-, ref^@), or a path
// (rev:path), and is then resolved to one full commit id, which is walked
// (`rev-parse --verify <ref>^{commit}`); anything else fails closed. At
// least one non-empty path is required, and a shallow repository is refused
// with ErrShallowHistory.
func FirstParentPathCommits(ctx context.Context, dir, ref string, paths ...string) ([]string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.HasPrefix(ref, "^") || strings.ContainsAny(ref, ":") ||
		strings.Contains(ref, "..") || strings.Contains(ref, "^!") || strings.Contains(ref, "^-") || strings.Contains(ref, "^@") {
		return nil, fmt.Errorf("gitx: FirstParentPathCommits: invalid ref %q", ref)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("gitx: FirstParentPathCommits(%s): no paths given", ref)
	}
	for _, p := range paths {
		if p == "" {
			return nil, fmt.Errorf("gitx: FirstParentPathCommits(%s): empty path", ref)
		}
	}
	shallow, err := IsShallow(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("gitx: FirstParentPathCommits(%s): %w", ref, err)
	}
	if shallow {
		return nil, fmt.Errorf("gitx: FirstParentPathCommits(%s) in %s: %w", ref, dir, ErrShallowHistory)
	}
	out, err := run(ctx, dir, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("gitx: FirstParentPathCommits(%s): %w", ref, err)
	}
	commit := strings.TrimSuffix(string(out), "\n")
	if !isFullObjectID(commit) {
		return nil, fmt.Errorf("gitx: FirstParentPathCommits(%s): resolved to %q, not one full commit id", ref, commit)
	}
	args := append([]string{"--literal-pathspecs", "rev-list", "--first-parent", "--reverse", commit, "--"}, paths...)
	out, err = run(ctx, dir, args...)
	if err != nil {
		return nil, fmt.Errorf("gitx: FirstParentPathCommits(%s -- %v): %w", ref, paths, err)
	}
	commits, err := parseRevList(out)
	if err != nil {
		return nil, fmt.Errorf("gitx: FirstParentPathCommits(%s -- %v): %w", ref, paths, err)
	}
	return commits, nil
}
