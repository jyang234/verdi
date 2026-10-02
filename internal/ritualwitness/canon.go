package ritualwitness

import (
	"path/filepath"
	"strings"
)

// canonicalPath returns p absolute, cleaned, and with every symlink in its
// longest existing prefix resolved, so one location has one spelling: on
// macOS a temporary directory's /var/folders/... and git's
// /private/var/folders/... compare equal, as do a relative and an
// absolute spelling. A path that no longer exists (a removed worktree)
// keeps its missing tail verbatim under its resolved existing ancestor.
// base resolves a relative p; "" means the process's working directory.
func canonicalPath(base, p string) string {
	if !filepath.IsAbs(p) {
		if base == "" {
			abs, err := filepath.Abs(p)
			if err != nil {
				return filepath.Clean(p)
			}
			p = abs
		} else {
			p = filepath.Join(base, p)
		}
	}
	p = filepath.Clean(p)
	var tail []string
	cur := p
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
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
