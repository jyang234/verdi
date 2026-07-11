package gitx

import (
	"context"
	"strings"
)

// CurrentBranch returns dir's currently checked-out branch's short name
// (e.g. "main"), needed by VL-004's I-14 default-branch scoping. It returns
// ("", nil) — not an error — for a detached HEAD, since that is a normal
// git state (e.g. many CI checkouts), not an operational failure; the
// caller reads an empty CurrentBranch as "unknown, can't prove we're on the
// default branch" (I-14: "otherwise a warning, not a finding"). A dir that
// is not a git repository at all is still an error.
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		if _, repoErr := run(ctx, dir, "rev-parse", "--git-dir"); repoErr == nil {
			return "", nil // detached HEAD in a real repo: not an error
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// DefaultBranch returns dir's configured remote "origin" HEAD branch short
// name (e.g. "main"), resolved via `git symbolic-ref refs/remotes/origin/HEAD`
// — set by `git remote set-head origin -a` or by cloning normally from a
// forge, which both GitLab's and GitHub's standard checkout actions do.
// It returns ("", nil) — not an error — when no such ref is configured
// (e.g. a bare local fixture repo with no "origin" remote at all): I-14's
// local-otherwise-warns posture treats an unknown default branch as "can't
// prove it", not as an operational failure. A dir that is not a git
// repository at all is still an error.
func DefaultBranch(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "symbolic-ref", "--short", "-q", "refs/remotes/origin/HEAD")
	if err != nil {
		if _, repoErr := run(ctx, dir, "rev-parse", "--git-dir"); repoErr == nil {
			return "", nil
		}
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	branch = strings.TrimPrefix(branch, "origin/")
	return branch, nil
}

// MergeBase returns the best common ancestor commit of a and b in dir —
// VL-010's diff base (I-14: "diff base = merge-base(HEAD, default branch)").
func MergeBase(ctx context.Context, dir, a, b string) (string, error) {
	out, err := run(ctx, dir, "merge-base", a, b)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
