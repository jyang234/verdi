// Build start's governed inputs (ledger SI-334 (2), as amended). build
// start cuts its branch at the resolved default branch's commit (UAT-023),
// which HEAD's checkout need not equal, while its preconditions read the
// working tree (the cascade check, the obligation-quality check, the
// conflict gate's adoption probe, policy store, store manifest, and
// instruction-projection check) and HEAD's commit tree (the conflict
// gate's context compile, through `git show HEAD:<path>`). So build start
// judges them only when the content at every governed path equals the
// base commit's tree, both in the working tree and in HEAD's commit tree,
// and refuses otherwise, naming the paths: a precondition is never judged
// against content the cut does not carry.
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
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/store"
)

// governedInputs names store-relative slash paths: each tree covers itself
// and everything beneath it, and each glob covers what path.Match matches.
type governedInputs struct {
	trees []string
	globs []string
}

// buildGovernedInputs returns the paths build start's preconditions read
// for the spec specName, apart from the instruction-projection files
// (projectionInputs): its directory (the acceptance check's spec.md, and
// the context compile's spec at HEAD), its obligations (the
// obligation-quality check's <ac>--<kind>.md files, and the compile's
// bound obligations at HEAD), the policy store (the conflict gate's
// adoption probe and authority), and the store manifest and model (the
// conflict gate's provider loads both through store.Open,
// internal/readinessload.NewConflictProvider; ledger SI-336) — and, when the spec implements a feature, what the cascade
// check then reads: every active spec's spec.md (its scan for a
// superseding spec, which also holds the compile's parent features) and
// the story's re-affirmation records. A spec implementing no feature gives
// the cascade check nothing to read.
func buildGovernedInputs(specName string, spec *artifact.SpecFrontmatter) governedInputs {
	g := governedInputs{trees: []string{
		path.Join(".verdi", "specs", "active", specName),
		path.Join(".verdi", "obligations", specName),
		path.Join(".verdi", "policy"),
		path.Join(".verdi", "verdi.yaml"),
		path.Join(".verdi", "model.yaml"),
	}}
	if len(evidence.ImplementsByFeature(spec.Links)) > 0 {
		g.trees = append(g.trees, path.Join(".verdi", "reaffirmations", store.RefSlug(spec.Story)))
		g.globs = append(g.globs, path.Join(".verdi", "specs", "active", "*", "spec.md"))
	}
	return g
}

// projectionInputs returns the instruction-projection files the conflict
// gate's projection check (internal/instructionprojection.Verify, the
// context compile's stage 5) reads: every adapter's managed path, from
// root's policy store. Its projection manifests lie under .verdi/policy/,
// already governed. A store that has adopted no constitution has no
// projection to read. Build start asks only once the policy store is known
// to agree with the base, so the adapters read here are the base's too.
func projectionInputs(root string) (governedInputs, error) {
	policyStore, err := policyauthority.Load(root)
	if errors.Is(err, policyauthority.ErrNotAdopted) {
		return governedInputs{}, nil
	}
	if err != nil {
		return governedInputs{}, err
	}
	managed, err := instructionprojection.ManagedPaths(policyStore.Constitution.Adapters)
	if err != nil {
		return governedInputs{}, err
	}
	if len(managed) == 0 {
		return governedInputs{}, nil
	}
	return governedInputs{trees: managed}, nil
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

// governedDiff names, sorted, the governed paths whose content differs
// from the base's tree in the working tree and in HEAD's commit tree.
type governedDiff struct {
	worktree []string
	head     []string
}

// empty reports whether no governed path differs on either side.
func (d governedDiff) empty() bool { return len(d.worktree) == 0 && len(d.head) == 0 }

// String renders the differing paths side by side, for a refusal.
func (d governedDiff) String() string {
	var parts []string
	if len(d.worktree) > 0 {
		parts = append(parts, "in the working tree at "+strings.Join(d.worktree, ", "))
	}
	if len(d.head) > 0 {
		parts = append(parts, "in HEAD's commit at "+strings.Join(d.head, ", "))
	}
	return strings.Join(parts, "; ")
}

// differingBuildInputs is build start's whole governed-input comparison
// for specName: buildGovernedInputs first, then, once those agree on both
// sides (so the policy store is the base's), the instruction-projection
// files projectionInputs names.
func differingBuildInputs(ctx context.Context, root, base, specName string, spec *artifact.SpecFrontmatter) (governedDiff, error) {
	d, err := differingGovernedInputs(ctx, root, base, buildGovernedInputs(specName, spec))
	if err != nil || !d.empty() {
		return d, err
	}
	projection, err := projectionInputs(root)
	if err != nil {
		return governedDiff{}, err
	}
	return differingGovernedInputs(ctx, root, base, projection)
}

// differingGovernedInputs compares every governed path's content with
// base's tree on two sides: root's working tree, read as the
// preconditions read it — every file on disk, ignored or not — and hashed
// as git would store it (gitx.HashObject); and HEAD's commit tree, the
// tree the context compile reads with `git show HEAD:<path>`. A path
// differs on a side when it is present on only one of the two, or with a
// different blob. Trees are listed with gitx.LsTreeEntries under the store
// root's own prefix inside the repository; every call is read-only.
func differingGovernedInputs(ctx context.Context, root, base string, g governedInputs) (governedDiff, error) {
	prefix, err := gitx.RepoPrefix(ctx, root)
	if err != nil {
		return governedDiff{}, err
	}
	atBase, err := governedTree(ctx, root, base, prefix, g)
	if err != nil {
		return governedDiff{}, err
	}
	atHead, err := governedTree(ctx, root, "HEAD", prefix, g)
	if err != nil {
		return governedDiff{}, err
	}
	onDisk, err := g.worktreeFiles(root)
	if err != nil {
		return governedDiff{}, err
	}

	var d governedDiff
	inWorktree := make(map[string]bool, len(onDisk))
	for _, rel := range onDisk {
		inWorktree[rel] = true
		want, tracked := atBase[rel]
		if !tracked {
			d.worktree = append(d.worktree, rel)
			continue
		}
		got, err := gitx.HashObject(ctx, root, filepath.FromSlash(rel))
		if err != nil {
			return governedDiff{}, err
		}
		if got != want {
			d.worktree = append(d.worktree, rel)
		}
	}
	for rel := range atBase {
		if !inWorktree[rel] {
			d.worktree = append(d.worktree, rel)
		}
	}
	d.head = differingTrees(atBase, atHead)
	sort.Strings(d.worktree)
	return d, nil
}

// governedTree maps every governed store-relative path in ref's tree to
// its object id.
func governedTree(ctx context.Context, root, ref, prefix string, g governedInputs) (map[string]string, error) {
	entries, err := gitx.LsTreeEntries(ctx, root, ref)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, e := range entries {
		rel, inStore := strings.CutPrefix(e.Path, prefix)
		if inStore && g.covers(rel) {
			out[rel] = e.Object
		}
	}
	return out, nil
}

// differingTrees returns, sorted, every path present in only one of a and
// b, or present in both with a different object id.
func differingTrees(a, b map[string]string) []string {
	var out []string
	for rel, oid := range a {
		if other, ok := b[rel]; !ok || other != oid {
			out = append(out, rel)
		}
	}
	for rel := range b {
		if _, ok := a[rel]; !ok {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
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
	// The glob is matched within root (os.DirFS), so a metacharacter in
	// root's own path is never read as part of the pattern (re-review
	// RR-A3). Its matches are slash paths relative to root.
	for _, glob := range g.globs {
		matches, err := fs.Glob(os.DirFS(root), glob)
		if err != nil {
			return nil, fmt.Errorf("matching %s: %w", glob, err)
		}
		for _, rel := range matches {
			info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", rel, err)
			}
			if info.IsDir() {
				continue
			}
			seen[rel] = true
		}
	}
	files := make([]string, 0, len(seen))
	for rel := range seen {
		files = append(files, rel)
	}
	sort.Strings(files)
	return files, nil
}
