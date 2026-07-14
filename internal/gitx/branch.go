package gitx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
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

// HasLocalBranch reports whether dir has a LOCAL ref named
// refs/heads/<name> — never a remote-tracking one (`git show-ref --verify
// --quiet refs/heads/<name>`, exit 0 = present, exit 1 = absent). This is
// spec/worktree-manager ac-2's gate: a remote-tracking-only branch or a
// name that resolves nowhere at all must both read as "no local ref"
// here, before any `git worktree add` is attempted, so a caller never
// relies on git's own worktree-add DWIM behavior (which would otherwise
// silently mint a new local branch tracking a same-named remote one —
// exactly what dc-1/dc-5 forbid). Any exit code other than 0 or 1 (e.g.
// dir is not a repository) is a real error, not a false answer.
func HasLocalBranch(ctx context.Context, dir, name string) (bool, error) {
	if _, err := run(ctx, dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("gitx: HasLocalBranch(%q): %w", name, err)
	}
	return true, nil
}

// CheckoutNewBranch creates a new branch named name at dir's current HEAD
// and checks it out — `git checkout -b <name>` (PLAN.md Phase 7's branch-
// cutting ritual for `design start`'s design/<name> and `feature start`'s
// feature/<name>; 01 §Temporal classes: "a transition is always a
// ritual"). It fails — rather than silently reusing the existing branch —
// if name already exists, matching D3's one-writer, no-clobber posture.
func CheckoutNewBranch(ctx context.Context, dir, name string) error {
	if _, err := run(ctx, dir, "checkout", "-b", name); err != nil {
		return fmt.Errorf("gitx: CheckoutNewBranch(%q): %w", name, err)
	}
	return nil
}
