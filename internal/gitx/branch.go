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

// HasRemoteTrackingBranch reports whether dir has a LOCAL remote-tracking
// ref named refs/remotes/<remote>/<branch> (`git show-ref --verify --quiet
// refs/remotes/<remote>/<branch>`, exit 0 = present, exit 1 = absent) —
// D6-6's hermetic building block: a freshly checked-out GitHub repository
// (actions/checkout's shallow, specific-ref fetch) populates the remote-
// tracking ref for the branch it fetched WITHOUT ever setting
// refs/remotes/origin/HEAD, so probing the two conventional default-branch
// names as remote-tracking refs directly, entirely from local git
// plumbing, tells the caller what a `git ls-remote` round-trip would have
// — with no network call at all (never `ls-remote`, unlike gitx.DefaultBranch's
// doc comment's "cloning normally from a forge" path). Any exit code other
// than 0 or 1 (e.g. dir is not a repository) is a real error, not a false
// answer — same contract as HasLocalBranch.
func HasRemoteTrackingBranch(ctx context.Context, dir, remote, branch string) (bool, error) {
	ref := "refs/remotes/" + remote + "/" + branch
	if _, err := run(ctx, dir, "show-ref", "--verify", "--quiet", ref); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("gitx: HasRemoteTrackingBranch(%q, %q): %w", remote, branch, err)
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

// CheckoutExisting switches dir to an already-existing ref (a branch short
// name or a commit) — `git checkout <ref>` — WITHOUT gitx.Checkout's board
// branch-switch guard (which refuses any uncommitted working-tree change). It
// exists for the one internal case that guard would wrongly block: unwinding a
// CheckoutNewBranch cut. A verb that cut close/<name> at HEAD and then aborted
// must return to the ref it cut FROM — the exact commit close/<name> still
// points at — even though the aborted step left the (untracked, uncommitted)
// artifacts it was about to freeze in the working tree. Because the target is
// that same commit, git changes no tracked file and carries the untracked ones
// across untouched, so nothing is lost and the board guard's protection does
// not apply. User-initiated branch switches must still go through Checkout.
func CheckoutExisting(ctx context.Context, dir, ref string) error {
	if _, err := run(ctx, dir, "checkout", ref); err != nil {
		return fmt.Errorf("gitx: CheckoutExisting(%q): %w", ref, err)
	}
	return nil
}

// DeleteBranch deletes the local branch name — `git branch -d <name>`, git's
// SAFE delete, which refuses (rather than force-removes) a branch carrying
// commits not merged into HEAD or its upstream. That safety is the point: the
// unwind of a CheckoutNewBranch cut deletes only a close/<name> it has already
// proven still points at its cut commit (so `-d` trivially succeeds — that
// commit is the HEAD it just switched back to), and if some actor put unmerged
// commits there `-d` fails loudly and the caller leaves the branch alone
// rather than discarding work. dir must not have name checked out (git refuses
// to delete the current branch); the unwind switches away first.
func DeleteBranch(ctx context.Context, dir, name string) error {
	if _, err := run(ctx, dir, "branch", "-d", name); err != nil {
		return fmt.Errorf("gitx: DeleteBranch(%q): %w", name, err)
	}
	return nil
}

// DeleteMergedBranch deletes the LOCAL branch name in dir via the existing,
// unchanged DeleteBranch ("git branch -d" — spec/gc-reclaim dc-3: git's own
// merged/not-checked-out-anywhere check is an independent second guard
// beyond a caller's own already-computed Merged fact, and a force-delete
// (-D) would erase that guard entirely; DeleteMergedBranch never uses -D),
// returning name's PRE-DELETE tip commit — resolved via the existing
// RevParse BEFORE the delete, since a successful "-d" removes the ref and
// its tip cannot be read back afterward (AC-2's recovery-affordance
// requirement: "every branch actually deleted prints its pre-delete tip
// commit").
//
// The returned tip is populated whenever name resolved, regardless of
// whether the delete itself then succeeded — a caller disclosing a refused
// deletion may still want to name the branch's own tip. Ledger R4-I-81:
// this is composition, not duplication — the only "git branch -d" call in
// this package remains the one inside DeleteBranch itself; DeleteMergedBranch
// adds only the ordering (resolve-then-delete) and the returned tip.
func DeleteMergedBranch(ctx context.Context, dir, name string) (string, error) {
	tip, err := RevParse(ctx, dir, name)
	if err != nil {
		return "", fmt.Errorf("gitx: DeleteMergedBranch(%q): resolving tip: %w", name, err)
	}
	if err := DeleteBranch(ctx, dir, name); err != nil {
		return tip, err
	}
	return tip, nil
}
