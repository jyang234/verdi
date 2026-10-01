package writescope

import (
	"path/filepath"
	"strings"
)

// Matches reports whether ref (a full "refs/heads/..." refname) lies
// within p: an exact ref, a namespace wildcard ("refs/heads/design/*",
// matching every ref one or more segments under the namespace), any local
// branch ("refs/heads/*"), or RefCheckedOut, which matches only the branch
// checked out BEFORE the ritual ran — checkedOutBefore, a short branch name
// as gitx.CurrentBranch returns it — and never matches a detached HEAD
// (checkedOutBefore == ""), since there is then no checked-out branch for a
// mutation to be read as moving (parent dc-7). checkedOutBefore is read
// exactly once per ritual run, at the moment the harness takes its "before"
// sensor snapshot: a ritual that switches branches mid-run and then moves
// the NEW branch does not thereby move "the checked-out branch" in this
// sense, because that was never the branch the ritual started on.
func (p RefPattern) Matches(ref, checkedOutBefore string) bool {
	if p == RefCheckedOut {
		return checkedOutBefore != "" && ref == "refs/heads/"+checkedOutBefore
	}
	pattern := string(p)
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(ref, prefix) && len(ref) > len(prefix)
	}
	return ref == pattern
}

// Matches reports whether a worktree at path lies within p: WorktreeTemp,
// a path git does not consider part of storeRoot's own tree at all;
// WorktreeRegistered, a path the harness's "before" snapshot had already
// registered (registeredBefore, computed by the caller from a `git
// worktree list --porcelain` taken before the ritual ran); or a
// store-relative directory pattern ("dir/*"), matching a path whose
// immediate parent, read relative to storeRoot, is exactly dir (parent
// dc-7). Paths are compared with filepath.Rel, so storeRoot and path may
// use either OS path separator; a path that cannot be related to storeRoot
// at all (filepath.Rel error, e.g. mismatched volumes on Windows) matches
// neither a directory pattern nor WorktreeRegistered's implied containment,
// and is treated as outside the store for WorktreeTemp.
func (p WorktreePattern) Matches(path, storeRoot string, registeredBefore bool) bool {
	switch p {
	case WorktreeTemp:
		return !underDir(storeRoot, path)
	case WorktreeRegistered:
		return registeredBefore
	}
	dir, ok := strings.CutSuffix(string(p), "/*")
	if !ok {
		return false
	}
	rel, err := filepath.Rel(storeRoot, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	parent := ""
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		parent = rel[:i]
	}
	return parent == dir
}

// underDir reports whether path lies at or under root.
func underDir(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

// Matches reports whether path, a store-relative forward-slash path, lies
// within p: PathWholeTree matches every path; a pattern ending in "/"
// matches that directory and everything under it; a "*" segment matches
// exactly one whole path segment, wherever it appears; any other pattern
// matches path exactly (parent dc-7: an untracked file inside a declared
// path belongs to that path, which this directory-prefix matching gives
// for free).
func (p PathPattern) Matches(path string) bool {
	if p == PathWholeTree {
		return true
	}
	pattern := string(p)
	dirMode := strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/")
	patSegs := strings.Split(pattern, "/")
	pathSegs := strings.Split(path, "/")

	if dirMode {
		if len(pathSegs) < len(patSegs) {
			return false
		}
		return segmentsMatch(patSegs, pathSegs[:len(patSegs)])
	}
	if len(pathSegs) != len(patSegs) {
		return false
	}
	return segmentsMatch(patSegs, pathSegs)
}

// segmentsMatch reports whether path's segments equal pattern's, treating
// a "*" pattern segment as matching any single segment. Both slices must
// already be the same length.
func segmentsMatch(pattern, path []string) bool {
	for i, seg := range pattern {
		if seg == "*" {
			continue
		}
		if seg != path[i] {
			return false
		}
	}
	return true
}
