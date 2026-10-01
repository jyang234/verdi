// Package gitdir holds the shapes the git-directory writer detector must
// tell apart: a write through a helper chain, a ".git" literal path, a
// git-directory value that is never written, an unrelated write, and a
// git-directory path handed to a writer in another package.
package gitdir

import (
	"context"
	"os"
	"path/filepath"

	"example.com/synth/gitx"
	"example.com/synth/other"
)

// Reconcile writes under the git directory through a helper chain.
func Reconcile(ctx context.Context, repo string) error {
	common, err := gitx.CommonDir(ctx, repo)
	if err != nil {
		return err
	}
	return removeEntries(adminDir(common))
}

func adminDir(common string) string { return filepath.Join(common, "worktrees") }

func removeEntries(admin string) error {
	for _, e := range list(admin) {
		if err := removeOne(e); err != nil {
			return err
		}
	}
	return nil
}

func list(dir string) []string { return []string{filepath.Join(dir, "a")} }

func removeOne(entry string) error { return os.RemoveAll(entry) }

// WriteHook writes under a ".git" literal path.
func WriteHook(repo string) error {
	return os.WriteFile(filepath.Join(repo, ".git", "hooks", "x"), nil, 0o600)
}

// CacheKey derives a value from the git directory but never writes it.
func CacheKey(ctx context.Context, repo string) string {
	common, _ := gitx.CommonDir(ctx, repo)
	return common + "\x00"
}

// WriteElsewhere writes a path unrelated to the git directory.
func WriteElsewhere(dir string) error { return os.WriteFile(filepath.Join(dir, "x"), nil, 0o600) }

// ViaOtherPackage hands a git-directory path to a writer in another package.
func ViaOtherPackage(ctx context.Context, repo string) error {
	common, _ := gitx.CommonDir(ctx, repo)
	return other.Write(filepath.Join(common, "x"))
}
