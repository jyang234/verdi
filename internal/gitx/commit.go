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
