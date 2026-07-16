// Nearest-ancestor bundle resolution (spec/sync-local-flow ac-2). The
// fold's own ancestor rule (internal/evidence.LoadRecordsWithSources,
// gitx.IsAncestor — 03 §The fold: "current ... whose commit is an
// ancestor of C") already accepts any record whose commit is an ancestor
// of the one being evaluated. This file makes sync's forge FETCH honor
// the identical rule, verbatim, rather than demanding a HEAD-exact bundle
// the fold itself would not require (dc-1; the D6-32 asymmetry closed in
// full). Split out of sync.go as its own topic — ancestor enumeration and
// the fetch walk built over it — distinct from bundle materialization/
// evaluation (CLAUDE.md: one file ~= one topic).
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/gitx"
)

// candidateAncestorCommits returns commit itself followed by its
// ancestors, nearest first, unbounded (dc-1) — the exact set and order
// gitx.Log(ctx, root, commit) already produces: `git log <rev>` lists rev
// itself first (a commit is its own ancestor — gitx.IsAncestor's own
// documented self-inclusive semantics), then its reachable history,
// most-recent-first, with no depth limit and no path filter. This is the
// SAME primitive internal/evidence's own fold-reader ancestry check
// (gitx.IsAncestor, wrapping `git merge-base --is-ancestor`) is a thin
// wrapper over — both are built on git's single reachability concept, so
// there is no second, possibly-disagreeing notion of "ancestor" anywhere
// in the tree. No hand-rolled parent walk.
func candidateAncestorCommits(ctx context.Context, root, commit string) ([]string, error) {
	commits, err := gitx.Log(ctx, root, commit)
	if err != nil {
		return nil, err
	}
	shas := make([]string, len(commits))
	for i, c := range commits {
		shas[i] = c.SHA
	}
	return shas, nil
}

// fetchAncestorBundle applies ac-2/dc-1's ancestor rule to sync's bundle
// fetch. It tries commit itself first — dc-1: "a commit is its own
// ancestor ... the walk starts there" — without requiring root to resolve
// any git history at all, so the common case (a bundle already sits at
// exactly the evaluated commit) never pays an ancestry-enumeration cost
// it doesn't need. Only when commit itself has no bundle (ErrNoBundle)
// does it enumerate and walk commit's further ancestors via
// candidateAncestorCommits, nearest first, with no depth bound short of
// exhausting the walked ref's entire reachable history.
//
// acceptedCommit/distance disclose which commit's bundle was accepted and
// how many commits back it sits from commit (0 = commit itself) — ac-2's
// legibility requirement.
//
// A transport failure at any candidate (an error NOT wrapping
// forge.ErrNoBundle) is returned immediately, unwalked further — dc-1: a
// rate limit or network error is an existing, generic operational
// failure, never a signal to try the next ancestor (cost safety against a
// pathological walk is an operational property to observe, never a
// designed narrower cutoff this contract authorizes).
//
// If commit's further ancestry cannot even be enumerated (root's git
// history for commit cannot be read — e.g. a non-git root, or a commit
// unresolvable in it), that is disclosed alongside the commit-itself miss
// in the returned error, wrapping the SAME forge.ErrNoBundle the
// commit-itself attempt already produced: never a different error class
// invented, and never a claim that a deeper walk happened when it
// structurally could not.
func fetchAncestorBundle(ctx context.Context, root string, f forge.Forge, ref, commit string) (tree forge.DerivedTree, acceptedCommit string, distance int, err error) {
	tree, headErr := f.FetchEvidenceBundle(ctx, ref, commit)
	if headErr == nil {
		return tree, commit, 0, nil
	}
	if !errors.Is(headErr, forge.ErrNoBundle) {
		return nil, "", 0, headErr
	}

	rest, logErr := candidateAncestorCommits(ctx, root, commit)
	if logErr != nil {
		return nil, "", 0, fmt.Errorf("no evidence bundle for commit %s, and its further ancestor history could not be walked (%v): %w", commit, logErr, headErr)
	}

	// rest[0] is commit itself (candidateAncestorCommits/gitx.Log's own
	// documented ordering) — already tried above, so start from index 1;
	// index i directly IS the disclosed "commits back" distance.
	lastErr := headErr
	for i := 1; i < len(rest); i++ {
		candidate := rest[i]
		t, err := f.FetchEvidenceBundle(ctx, ref, candidate)
		switch {
		case err == nil:
			return t, candidate, i, nil
		case errors.Is(err, forge.ErrNoBundle):
			lastErr = err
			continue
		default:
			return nil, "", 0, err
		}
	}

	oldest := commit
	if len(rest) > 0 {
		oldest = rest[len(rest)-1]
	}
	return nil, "", 0, fmt.Errorf("no evidence bundle found for ref %q anywhere in %d commit(s) walked (%s..%s): %w", ref, len(rest), oldest, commit, lastErr)
}
