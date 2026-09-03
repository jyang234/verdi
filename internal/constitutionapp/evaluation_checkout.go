package constitutionapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// evaluationCheckout is one private detached worktree at the exact proposed
// commit. It exists only while the existing compiler and conflict evaluator
// consume filesystem-backed authority and projection bytes.
type evaluationCheckout struct {
	git        GitReader
	repoRoot   string
	parentPath string
	root       string
}

func (s Service) materializeEvaluationCheckout(ctx context.Context, repoRoot, commit string) (*evaluationCheckout, error) {
	parentPath, err := os.MkdirTemp("", "verdi-constitution-impact-")
	if err != nil {
		return nil, fmt.Errorf("constitutionapp: create exact evaluation checkout parent: %w", err)
	}
	root := filepath.Join(parentPath, "tree")
	if err := s.Git.WorktreeAddDetached(ctx, repoRoot, root, commit); err != nil {
		var cleanupErr error
		if _, statErr := os.Lstat(root); statErr == nil {
			cleanupErr = s.Git.WorktreeRemove(context.WithoutCancel(ctx), repoRoot, root)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			cleanupErr = statErr
		}
		if cleanupErr == nil {
			cleanupErr = os.Remove(parentPath)
		}
		return nil, fmt.Errorf("constitutionapp: materialize exact evaluation checkout: %w", errors.Join(err, cleanupErr))
	}
	return &evaluationCheckout{git: s.Git, repoRoot: repoRoot, parentPath: parentPath, root: root}, nil
}

// Close removes the detached worktree through gitx's non-forcing owner seam
// before removing the empty private parent. Cleanup outlives request
// cancellation so a canceled evaluation cannot strand registered worktree
// custody. A dirty or otherwise unremovable worktree is preserved and exposed
// as an operational failure; it is never force-deleted.
func (c *evaluationCheckout) Close(ctx context.Context) error {
	cleanupCtx := context.WithoutCancel(ctx)
	if err := c.git.WorktreeRemove(cleanupCtx, c.repoRoot, c.root); err != nil {
		return fmt.Errorf("constitutionapp: remove exact evaluation checkout: %w", err)
	}
	if err := os.Remove(c.parentPath); err != nil {
		return fmt.Errorf("constitutionapp: remove exact evaluation checkout parent: %w", err)
	}
	return nil
}
