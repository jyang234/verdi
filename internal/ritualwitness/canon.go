package ritualwitness

import (
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/pathcanon"
)

// canonicalPath is p in its one spelling (pathcanon.Canonical: absolute,
// cleaned, its longest existing prefix resolved through symbolic links and
// any missing tail kept beneath it), with a relative p resolved against
// base; "" means the process's working directory.
func canonicalPath(base, p string) string {
	if !filepath.IsAbs(p) && base != "" {
		p = filepath.Join(base, p)
	}
	return pathcanon.Canonical(p)
}

// within reports whether path lies at or under dir; both are canonical.
func within(dir, path string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

// storeRelative converts a repository-relative path to a store-relative
// one by removing prefix, reporting false for a path outside the store.
func storeRelative(prefix, repoPath string) (string, bool) {
	if prefix == "" {
		return repoPath, true
	}
	rel, ok := strings.CutPrefix(repoPath, prefix)
	if !ok || rel == "" {
		return "", false
	}
	return rel, true
}
