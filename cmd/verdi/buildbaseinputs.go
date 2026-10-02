// Build start's governed inputs (ledger SI-334 (2)). build start cuts its
// branch at the resolved default branch's commit (UAT-023), which HEAD's
// checkout need not equal, while its preconditions — the cascade check,
// the obligation-quality check, and the lifecycle conflict gate's adoption
// probe and policy store — read the working tree. So build start judges
// them only when the working tree's content at every path they read equals
// the base commit's tree there, and refuses otherwise, naming the paths: a
// precondition is never judged against content the cut does not carry.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// governedInputs names store-relative slash paths: each tree covers itself
// and everything beneath it, and each glob covers what path.Match matches.
type governedInputs struct {
	trees []string
	globs []string
}

// buildGovernedInputs returns the paths build start's preconditions read
// for the spec specName: its directory (the acceptance check's spec.md
// and the rest of it), its obligations (the obligation-quality check's
// <ac>--<kind>.md files), and the policy store (the conflict gate's
// adoption probe and authority) — and, when the spec implements a feature,
// what the cascade check then reads: every active spec's spec.md (its
// scan for a superseding spec) and the story's re-affirmation records.
// A spec implementing no feature gives the cascade check nothing to read.
func buildGovernedInputs(specName string, spec *artifact.SpecFrontmatter) governedInputs {
	g := governedInputs{trees: []string{
		path.Join(".verdi", "specs", "active", specName),
		path.Join(".verdi", "obligations", specName),
		path.Join(".verdi", "policy"),
	}}
	if len(evidence.ImplementsByFeature(spec.Links)) > 0 {
		g.trees = append(g.trees, path.Join(".verdi", "reaffirmations", store.RefSlug(spec.Story)))
		g.globs = append(g.globs, path.Join(".verdi", "specs", "active", "*", "spec.md"))
	}
	return g
}

// covers reports whether the store-relative slash path rel is governed.
func (g governedInputs) covers(rel string) bool {
	for _, tree := range g.trees {
		if rel == tree || strings.HasPrefix(rel, tree+"/") {
			return true
		}
	}
	for _, glob := range g.globs {
		if ok, _ := path.Match(glob, rel); ok {
			return true
		}
	}
	return false
}

// differingGovernedInputs returns, sorted, every governed path whose
// content in root's working tree differs from base's tree: a file present
// on only one side, or with a different blob. The working tree is read as
// the preconditions read it — every file on disk, ignored or not — and
// hashed as git would store it (gitx.HashObject); base's tree is read
// once (gitx.LsTreeEntries), under the store root's own prefix inside the
// repository. Both calls are read-only.
func differingGovernedInputs(ctx context.Context, root, base string, g governedInputs) ([]string, error) {
	prefix, err := gitx.RepoPrefix(ctx, root)
	if err != nil {
		return nil, err
	}
	entries, err := gitx.LsTreeEntries(ctx, root, base)
	if err != nil {
		return nil, err
	}
	atBase := make(map[string]string)
	for _, e := range entries {
		rel, inStore := strings.CutPrefix(e.Path, prefix)
		if inStore && g.covers(rel) {
			atBase[rel] = e.Object
		}
	}
	onDisk, err := g.worktreeFiles(root)
	if err != nil {
		return nil, err
	}

	var differing []string
	for _, rel := range onDisk {
		want, tracked := atBase[rel]
		delete(atBase, rel)
		if !tracked {
			differing = append(differing, rel)
			continue
		}
		got, err := gitx.HashObject(ctx, root, filepath.FromSlash(rel))
		if err != nil {
			return nil, err
		}
		if got != want {
			differing = append(differing, rel)
		}
	}
	for rel := range atBase {
		differing = append(differing, rel)
	}
	sort.Strings(differing)
	return differing, nil
}

// worktreeFiles lists, sorted and without repeats, every governed file
// under root on disk: every non-directory beneath each tree (a tree that
// is itself a file is one), and every non-directory a glob matches.
func (g governedInputs) worktreeFiles(root string) ([]string, error) {
	seen := make(map[string]bool)
	for _, tree := range g.trees {
		top := filepath.Join(root, filepath.FromSlash(tree))
		if _, err := os.Lstat(top); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("reading %s: %w", tree, err)
		}
		err := filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					return err
				}
				seen[filepath.ToSlash(rel)] = true
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", tree, err)
		}
	}
	for _, glob := range g.globs {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(glob)))
		if err != nil {
			return nil, fmt.Errorf("matching %s: %w", glob, err)
		}
		for _, m := range matches {
			info, err := os.Lstat(m)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", m, err)
			}
			if info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(root, m)
			if err != nil {
				return nil, err
			}
			seen[filepath.ToSlash(rel)] = true
		}
	}
	files := make([]string, 0, len(seen))
	for rel := range seen {
		files = append(files, rel)
	}
	sort.Strings(files)
	return files, nil
}
