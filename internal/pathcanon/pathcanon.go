// Package pathcanon gives a filesystem location one spelling, so two
// spellings of the same path compare equal.
package pathcanon

import "path/filepath"

// Canonical returns p absolute (a relative p resolves against the process's
// working directory), cleaned, and with every symbolic link in its longest
// existing prefix resolved. On macOS a temporary directory's
// /var/folders/... and git's /private/var/folders/... then compare equal,
// as do a relative and an absolute spelling. A path that no longer exists,
// such as a removed worktree, keeps its missing tail verbatim under its
// resolved existing ancestor, so the answer does not depend on how that
// ancestor is spelled. When the working directory cannot be read, a
// relative p is returned cleaned.
func Canonical(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	var tail []string
	for cur := abs; ; {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}
