package gitx

import (
	"context"
	"fmt"
	"strings"
)

// AddAll stages every change under dir — `git add -A` — the write half of
// PLAN.md Phase 7's design/accept/feature ritual commits (01 §D3: "atomic
// writes"; a transition is committed in one shot, never left half-staged).
func AddAll(ctx context.Context, dir string) error {
	if _, err := run(ctx, dir, "add", "-A"); err != nil {
		return fmt.Errorf("gitx: AddAll(%s): %w", dir, err)
	}
	return nil
}

// CreateCommit creates a commit in dir with message, using the checkout's
// ambient git identity (unlike fixturegit's test-only fixed identity — a
// real `design start`/`accept`/`feature start` commit is the developer's
// or CI's own, never fixture-pinned), and returns the new commit's full
// SHA. Named CreateCommit, not Commit, because Commit already names this
// package's `git log` record type (log.go).
func CreateCommit(ctx context.Context, dir, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("gitx: CreateCommit(%s): message must not be empty", dir)
	}
	if _, err := run(ctx, dir, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("gitx: CreateCommit(%s): %w", dir, err)
	}
	return RevParse(ctx, dir, "HEAD")
}

// CreateCommitPaths creates a commit in dir recording exactly the given
// paths — `git commit -m <message> -- <paths...>` — and returns its full SHA.
// It is CreateCommit's scoped sibling, standing to it as AddPaths stands to
// AddAll, and exists because staging scope alone does not bound commit scope:
// AddPaths stages only what a ritual wrote, but a pathspec-less `git commit`
// then records the WHOLE index, so anything the operator had staged
// beforehand rides into the ritual's commit (proven in
// TestCreateCommit_RecordsTheWholeIndex). The pathspec form records only
// these paths and leaves every other index entry staged and uncommitted —
// which is what lets `verdi policy adopt --starter` keep the promise ac-10,
// the CLI and the workbench's policy setup guide all make in the word
// "exactly", on a branch whose whole purpose is to isolate the adoption for
// review.
//
// The pathspec form reads the WORKING TREE, not the index: git commits each
// path's on-disk content (and restages it), so a caller that staged one
// version and then changed the file on disk commits the newer one. Callers
// here write, then stage, then commit, with nothing in between, so the two
// coincide; a caller that needs index content committed instead must use
// CreateCommit. Each path must already be known to git — a never-added path
// is a fatal, commits-nothing `git commit`.
//
// Pathspecs resolve against DIR, exactly as AddPaths' own documented caveat
// and TestAddPaths_PathspecsResolveAgainstDir describe, never against the
// repository root; pass the same paths given to AddPaths.
//
// Refuses a blank message and an empty paths slice before running git
// (AddPaths' posture: a scoped call site with nothing to name is a caller
// bug, and here it would silently degrade into CreateCommit's whole-index
// commit — the exact defect this function exists to prevent).
func CreateCommitPaths(ctx context.Context, dir, message string, paths ...string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("gitx: CreateCommitPaths(%s): message must not be empty", dir)
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("gitx: CreateCommitPaths(%s): no paths given", dir)
	}
	args := append([]string{"commit", "-m", message, "--"}, paths...)
	if _, err := run(ctx, dir, args...); err != nil {
		return "", fmt.Errorf("gitx: CreateCommitPaths(%s, %v): %w", dir, paths, err)
	}
	return RevParse(ctx, dir, "HEAD")
}
