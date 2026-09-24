package gitx

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// AncestryPathChanges lists the commits on the full-history ancestry path
// from..to that change path, in git's order (newest first):
// `git --literal-pathspecs rev-list --ancestry-path --full-history from..to -- path`.
// path is repo-relative.
//
// The ancestry path is every commit that descends from `from` and is `to`
// or an ancestor of `to`; `from` itself is excluded. --full-history walks
// every parent of every merge: default history simplification follows only
// a parent the merge leaves path unchanged against, and so can hide a
// change made on the merge's other side (a merge that restores a stale
// copy hides the commit that changed it). A merge is listed when path
// differs from at least one of its parents that is `from` or on the path,
// so a commit on the path that is not listed holds, at path, exactly the
// content of each such parent. --literal-pathspecs matches path byte for
// byte, never as a glob or pathspec magic.
//
// An empty list is not an error. A revision that does not resolve, and
// output that is not one full object id per line, are errors.
func AncestryPathChanges(ctx context.Context, dir, from, to, path string) ([]string, error) {
	for _, rev := range []string{from, to} {
		if rev == "" || strings.HasPrefix(rev, "-") || strings.Contains(rev, "..") {
			return nil, fmt.Errorf("gitx: AncestryPathChanges: invalid rev %q", rev)
		}
	}
	if path == "" {
		return nil, fmt.Errorf("gitx: AncestryPathChanges: empty path")
	}
	out, err := run(ctx, dir, "--literal-pathspecs", "rev-list", "--ancestry-path", "--full-history", from+".."+to, "--", path)
	if err != nil {
		return nil, fmt.Errorf("gitx: AncestryPathChanges(%s..%s -- %s): %w", from, to, path, err)
	}
	commits, err := parseRevList(out)
	if err != nil {
		return nil, fmt.Errorf("gitx: AncestryPathChanges(%s..%s -- %s): %w", from, to, path, err)
	}
	return commits, nil
}

// parseRevList parses rev-list output: one full object id per line, each
// line newline-terminated.
func parseRevList(out []byte) ([]string, error) {
	commits := []string{}
	if len(out) == 0 {
		return commits, nil
	}
	if !bytes.HasSuffix(out, []byte("\n")) {
		return nil, fmt.Errorf("rev-list output does not end with a newline")
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if !fullObjectIDRe.MatchString(line) {
			return nil, fmt.Errorf("malformed rev-list line %q", line)
		}
		commits = append(commits, line)
	}
	return commits, nil
}
