package writescope

import (
	"path/filepath"
	"strings"
)

// Matches reports whether ref, a full refname ("refs/heads/design/x"),
// lies within p: an exact ref, a namespace wildcard ("refs/heads/design/*",
// matching every ref one or more segments under the namespace), any local
// branch ("refs/heads/*"), or RefCheckedOut, which matches only the branch
// checked out BEFORE the ritual ran. checkedOutBefore is that branch's
// full refname as `git symbolic-ref -q HEAD` reports it, or "" for a
// detached HEAD; RefCheckedOut never matches a detached HEAD or a HEAD
// attached outside refs/heads, since there is then no checked-out branch
// for a mutation to be read as moving (parent dc-7). Every comparison is of
// full refnames, so a tag or remote-tracking ref sharing a branch's short
// name never matches. checkedOutBefore is read once per ritual run, from
// the harness's "before" snapshot: a ritual that switches branches mid-run
// and then moves the NEW branch does not thereby move "the checked-out
// branch", because that was never the branch the ritual started on.
func (p RefPattern) Matches(ref, checkedOutBefore string) bool {
	if p == RefCheckedOut {
		return strings.HasPrefix(checkedOutBefore, "refs/heads/") && ref == checkedOutBefore
	}
	pattern := string(p)
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(ref, prefix) && len(ref) > len(prefix)
	}
	return ref == pattern
}

// WorktreeSite is where a worktree path is judged: the repository's root
// (its main worktree), the store root its directory patterns are relative
// to, and whether the worktree was already registered before the ritual
// ran. Every path is canonical (absolute, symlinks resolved): Matches
// compares paths lexically and never touches the filesystem.
type WorktreeSite struct {
	RepoRoot         string
	StoreRoot        string
	RegisteredBefore bool
}

// Matches reports whether a worktree at path lies within p: WorktreeTemp, a
// fresh worktree (not registered before the ritual) outside the
// repository; WorktreeRegistered, a worktree already registered before
// the ritual, wherever it lies; or a store-relative directory pattern
// ("dir/*"), matching a path whose immediate parent, read relative to the
// store root, is exactly dir (parent dc-7). A path that cannot be related
// to a root (filepath.Rel fails, e.g. mismatched volumes on Windows) is
// treated as outside it.
func (p WorktreePattern) Matches(path string, site WorktreeSite) bool {
	switch p {
	case WorktreeTemp:
		return !site.RegisteredBefore && !underDir(site.RepoRoot, path)
	case WorktreeRegistered:
		return site.RegisteredBefore
	}
	dir, ok := strings.CutSuffix(string(p), "/*")
	if !ok || !underDir(site.StoreRoot, path) {
		return false
	}
	rel, err := filepath.Rel(site.StoreRoot, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	i := strings.LastIndex(rel, "/")
	return i >= 0 && rel[:i] == dir
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
