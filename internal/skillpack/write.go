package skillpack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/atomicfile"
)

// FileDigest is one written file's repository-relative slash path and
// content digest.
type FileDigest struct {
	Path   string
	Digest string
}

// Result is what Write wrote, sorted by Path.
type Result struct {
	RenderCommit string
	Files        []FileDigest
}

// Write renders every skill for every host in hosts under root, writing
// each file atomically, and returns the sorted digests. It reads nothing
// from a store; root need only be an existing directory.
func Write(ctx context.Context, root string, hosts []Host) (Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Result{}, fmt.Errorf("skillpack: root: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("skillpack: root %q is not a directory", root)
	}
	res := Result{RenderCommit: RenderCommit(ctx, root)}
	for _, h := range hosts {
		for _, s := range Skills() {
			r, err := Render(h, s, res.RenderCommit)
			if err != nil {
				return Result{}, err
			}
			full := filepath.Join(root, filepath.FromSlash(r.Path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return Result{}, fmt.Errorf("skillpack: %s: %w", r.Path, err)
			}
			if err := atomicfile.Write(full, r.Content, 0o644); err != nil {
				return Result{}, fmt.Errorf("skillpack: %s: %w", r.Path, err)
			}
			res.Files = append(res.Files, FileDigest{Path: r.Path, Digest: r.Digest})
		}
	}
	sort.Slice(res.Files, func(i, j int) bool { return res.Files[i].Path < res.Files[j].Path })
	return res, nil
}
