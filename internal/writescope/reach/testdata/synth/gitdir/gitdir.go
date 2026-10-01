// Package gitdir holds the shapes the git-directory writer detector must
// tell apart: a write through a helper chain, a ".git" literal path, a
// git-directory value that is never written, an unrelated write, a
// git-directory path handed to a writer in another package, a walk of the
// git directory whose callback writes the paths it is handed (a literal or
// a named function), a write through an os.Root opened on the git
// directory, and a tree copied into it.
package gitdir

import (
	"context"
	"io/fs"
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

// PruneByWalk deletes what a walk of the git directory hands its callback.
func PruneByWalk(ctx context.Context, repo string) error {
	common, _ := gitx.CommonDir(ctx, repo)
	return filepath.WalkDir(filepath.Join(common, "worktrees"), func(path string, d fs.DirEntry, err error) error {
		return os.RemoveAll(path)
	})
}

func removeVisited(path string, d fs.DirEntry, err error) error { return os.RemoveAll(path) }

// PruneByNamedWalk hands the walk a named function that deletes each path.
func PruneByNamedWalk(ctx context.Context, repo string) error {
	common, _ := gitx.CommonDir(ctx, repo)
	return filepath.WalkDir(filepath.Join(common, "worktrees"), removeVisited)
}

// WalkElsewhere deletes what a walk of an unrelated directory finds.
func WalkElsewhere(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error { return os.Remove(path) })
}

// PruneInRoot deletes an administrative entry through an os.Root opened on
// the git directory.
func PruneInRoot(ctx context.Context, repo string) error {
	common, _ := gitx.CommonDir(ctx, repo)
	root, err := os.OpenRoot(common)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll("worktrees/x")
}

// CopyInto copies a tree into the git directory.
func CopyInto(ctx context.Context, repo string) error {
	common, _ := gitx.CommonDir(ctx, repo)
	return os.CopyFS(filepath.Join(common, "x"), os.DirFS("."))
}
