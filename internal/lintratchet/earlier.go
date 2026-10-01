package lintratchet

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// EarlierBaseline is the baseline the growth comparison measures the
// committed one against.
type EarlierBaseline struct {
	// Path is the baseline's repository-relative path.
	Path string
	// Commit is the commit the baseline was read at.
	Commit string
	// Where says what Commit is: the merge base of HEAD with the default
	// branch, or HEAD's first parent when HEAD is on the default branch.
	Where string
	// Present is false when the baseline file does not exist at Commit: the
	// growth comparison then does not apply (ledger SI-311 (1)).
	Present bool
	// Counts is the baseline at Commit, when Present.
	Counts Counts
}

// GitEarlier reads the earlier baseline from git (spec/strict-lint-gate
// dc-2): at the merge base of HEAD with the default branch, resolved as verdi
// resolves it everywhere (specstate.ResolveDefaultBranch: CI_DEFAULT_BRANCH,
// then origin/HEAD, then origin/main or origin/master), or, on the default
// branch itself, where that merge base is HEAD, at HEAD's first parent, the
// default branch before the change. It reads the file with git show.
type GitEarlier struct {
	// Dir is a directory inside the repository.
	Dir string
	// Path is the baseline's repository-relative, slash-separated path.
	Path string
}

// NewGitEarlier returns the GitEarlier for the baseline at baselinePath, a
// path relative to dir, which must stay inside the repository.
func NewGitEarlier(ctx context.Context, dir, baselinePath string) (GitEarlier, error) {
	if baselinePath == "" || filepath.IsAbs(baselinePath) {
		return GitEarlier{}, fmt.Errorf("baseline path %q must be relative to the working directory", baselinePath)
	}
	prefix, err := gitx.RepoPrefix(ctx, dir)
	if err != nil {
		return GitEarlier{}, fmt.Errorf("locating %s in its repository: %w", dir, err)
	}
	rel := path.Clean(path.Join(prefix, filepath.ToSlash(baselinePath)))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return GitEarlier{}, fmt.Errorf("baseline path %q does not name a file inside the repository", baselinePath)
	}
	return GitEarlier{Dir: dir, Path: rel}, nil
}

// ReadEarlierBaseline resolves the earlier commit and reads the baseline
// there. A default branch, merge base, or first parent that cannot be
// resolved, a git failure, and a malformed baseline are errors (exit 2); a
// resolvable commit without the file is a Present=false result.
func (g GitEarlier) ReadEarlierBaseline(ctx context.Context) (EarlierBaseline, error) {
	branch, ok := specstate.ResolveDefaultBranch(ctx, g.Dir)
	if !ok {
		msg := specstate.UnresolvedDefaultBranchMessage(ctx, g.Dir)
		if msg == "" {
			msg = "the default branch did not resolve"
		}
		// vocab:identity — non-vocabulary homograph: git's own merge-base/merge state, never the `merge` lifecycle transition word
		return EarlierBaseline{}, fmt.Errorf("the growth comparison needs the merge base with the default branch: %s", msg)
	}
	head, err := gitx.RevParse(ctx, g.Dir, "HEAD")
	if err != nil {
		return EarlierBaseline{}, fmt.Errorf("resolving HEAD: %w", err)
	}
	base, found, err := gitx.MergeBaseCommit(ctx, g.Dir, "HEAD", branch.Ref)
	if err != nil {
		// vocab:identity — non-vocabulary homograph: git's own merge-base/merge state, never the `merge` lifecycle transition word
		return EarlierBaseline{}, fmt.Errorf("resolving the merge base of HEAD with %s: %w", branch.Ref, err)
	}
	if !found {
		// vocab:identity — non-vocabulary homograph: git's own merge-base/merge state, never the `merge` lifecycle transition word
		return EarlierBaseline{}, fmt.Errorf("HEAD and %s have no merge base in this clone (a shallow clone cuts the history the growth comparison needs), so the baseline cannot be checked for growth", branch.Ref)
	}
	// vocab:identity — non-vocabulary homograph: git's own merge-base/merge state, never the `merge` lifecycle transition word
	commit, where := base, "the merge base of HEAD with "+branch.Ref
	if base == head {
		parent, err := gitx.RevParse(ctx, g.Dir, "HEAD^1")
		if err != nil {
			return EarlierBaseline{}, fmt.Errorf("HEAD is on %s, so the growth comparison reads HEAD's first parent, which does not resolve here (a root commit, or a shallow clone's horizon): %w", branch.Ref, err)
		}
		commit, where = parent, "HEAD's first parent (HEAD is on "+branch.Ref+")"
	}
	present, err := gitx.PathExistsAt(ctx, g.Dir, commit, g.Path)
	if err != nil {
		return EarlierBaseline{}, fmt.Errorf("looking for %s at %s %s: %w", g.Path, where, commit, err)
	}
	earlier := EarlierBaseline{Path: g.Path, Commit: commit, Where: where, Present: present}
	if !present {
		return earlier, nil
	}
	data, err := gitx.Show(ctx, g.Dir, commit, g.Path)
	if err != nil {
		return EarlierBaseline{}, fmt.Errorf("reading %s at %s %s: %w", g.Path, where, commit, err)
	}
	counts, err := ParseBaseline(data)
	if err != nil {
		return EarlierBaseline{}, fmt.Errorf("the %s at %s %s is malformed: %w", g.Path, where, commit, err)
	}
	earlier.Counts = counts
	return earlier, nil
}
