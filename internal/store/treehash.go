package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OWNER/verdi/internal/gitx"
)

// LayoutVersion is the store layout schema's version segment, embedded in
// cache filenames (D4: "Cache filenames embed layout version and tree
// hash"). It tracks verdi.layout/vN.
const LayoutVersion = "v1"

// verdiDataPrefix is the one committed-zone subtree TreeHash must exclude
// (01 §Zones: "Committed | .verdi/ (minus data/)").
const verdiDataPrefix = ".verdi/data/"

// TreeHash computes the D4/I-15 corpus tree hash: sha256 over the sorted
// (path, git-blob-sha) pairs of the committed zone (.verdi/ minus data/)
// plus every corpus-contributing file discovered in the given services'
// roots (.flowmap.yaml itself, and whichever of boundary-contract.json /
// verdi.bindings.yaml / api/openapi.* each service has). Every blob sha is
// computed from the file's current on-disk content via gitx.HashObject, so
// a dirty (uncommitted) edit to any of these files changes the hash
// immediately — exactly the D4 guarantee ("a boundary-contract or
// obligation change invalidates the cache exactly like a spec change
// does"). Paths are relative to root with forward slashes.
func TreeHash(ctx context.Context, root string, services []Service) (string, error) {
	tracked, err := gitx.LsFiles(ctx, root)
	if err != nil {
		return "", fmt.Errorf("store: TreeHash: %w", err)
	}

	paths := make(map[string]bool)
	for _, p := range tracked {
		if strings.HasPrefix(p, verdiDataPrefix) {
			continue
		}
		if !strings.HasPrefix(p, ".verdi/") {
			continue
		}
		paths[p] = true
	}

	for _, svc := range services {
		for _, abs := range corpusContributingFiles(svc) {
			rel, err := filepath.Rel(root, abs)
			if err != nil {
				return "", fmt.Errorf("store: TreeHash: relativizing %s: %w", abs, err)
			}
			paths[filepath.ToSlash(rel)] = true
		}
	}

	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	h := sha256.New()
	for _, p := range sorted {
		blobSHA, err := gitx.HashObject(ctx, root, p)
		if err != nil {
			return "", fmt.Errorf("store: TreeHash: hashing %s: %w", p, err)
		}
		fmt.Fprintf(h, "%s\x00%s\n", p, blobSHA)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// corpusContributingFiles lists svc's discovered files that feed the tree
// hash: .flowmap.yaml always, plus whichever fixed-path companions were
// found present at discovery time.
func corpusContributingFiles(svc Service) []string {
	files := []string{filepath.Join(svc.Dir, flowmapFile)}
	if svc.BoundaryContractPath != "" {
		files = append(files, svc.BoundaryContractPath)
	}
	if svc.BindingsPath != "" {
		files = append(files, svc.BindingsPath)
	}
	if svc.OpenAPIPath != "" {
		files = append(files, svc.OpenAPIPath)
	}
	return files
}

// CacheKey is the disposable index cache's filename (D4: "Cache filenames
// embed layout version and tree hash"; 01 §Directory layout:
// "cache/index-<layout-version>-<tree-hash>").
func CacheKey(treeHash string) string {
	return fmt.Sprintf("index-%s-%s", LayoutVersion, treeHash)
}
