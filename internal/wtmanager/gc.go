package wtmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/gitx"
)

// worktreeRemove is gitx.WorktreeRemove behind a package-level var, the
// same test seam worktreeAdd (ensure.go) uses.
var worktreeRemove = gitx.WorktreeRemove

// Decision is GC's reclaim verdict for one managed worktree — a total,
// four-outcome map (ac-4's static obligation) with no silent fifth path.
type Decision int

const (
	// KeepNotEligible: branch is neither merged nor (locally) deleted —
	// still active work, kept regardless of any other state.
	KeepNotEligible Decision = iota
	// KeepDirty: eligible, but carries uncommitted changes — feature
	// dc-4's one named exception, never force-removed.
	KeepDirty
	// KeepLocked: eligible and clean, but its lockfile is currently held
	// by a live process — dc-4's own added exception, a narrow,
	// self-resolving race window (dc-2: the lock is held only for the
	// duration of a single git-worktree-mutating call).
	KeepLocked
	// Reclaim: eligible, clean, and unlocked — the only outcome GC
	// actually removes anything for.
	Reclaim
)

// String names d for logging/disclosure.
func (d Decision) String() string {
	switch d {
	case KeepNotEligible:
		return "keep-not-eligible"
	case KeepDirty:
		return "keep-dirty"
	case KeepLocked:
		return "keep-locked"
	case Reclaim:
		return "reclaim"
	default:
		return "unknown"
	}
}

// decideReclaim is GC's reclaim decision, in full: a single, total
// function over (eligible, dirty, unlocked) with no unreachable
// combination silently falling through to removal. eligible is dc-3's
// merged-or-locally-deleted signal; dirty and locked are only meaningful
// when eligible (an ineligible worktree is kept regardless of either).
func decideReclaim(eligible, dirty, locked bool) Decision {
	switch {
	case !eligible:
		return KeepNotEligible
	case dirty:
		return KeepDirty
	case locked:
		return KeepLocked
	default:
		return Reclaim
	}
}

// Result is GC's one-line-per-worktree report (dc-4: "every skip and
// every reclaim is printed... no removal a human running it cannot see
// named in its own output").
type Result struct {
	Name     string // the managed worktree's directory name
	Branch   string // the design branch it was cut for
	Path     string // its full path under root
	Decision Decision
	Detail   string // e.g. the live holder's pid, for KeepLocked
}

// Line renders r as gc's disclosed report line — a distinct message per
// keep-reason (never one undifferentiated "kept" message), per the ac-4
// behavioral obligation.
func (r Result) Line() string {
	switch r.Decision {
	case Reclaim:
		return fmt.Sprintf("reclaimed: %s (branch %s)", r.Name, r.Branch)
	case KeepDirty:
		return fmt.Sprintf("kept: uncommitted changes (%s, branch %s)", r.Name, r.Branch)
	case KeepLocked:
		return fmt.Sprintf("kept: in use by pid %s (%s, branch %s)", r.Detail, r.Name, r.Branch)
	default:
		return fmt.Sprintf("kept: not eligible (%s, branch %s not merged or deleted)", r.Name, r.Branch)
	}
}

// GC scans root's managed worktrees (.verdi/data/worktrees/) and reclaims
// each one whose branch is merged (gitx.IsAncestor against
// defaultBranchRef's tip) or locally deleted (dc-3) — unless it carries
// uncommitted changes (gitx.StatusDirty) or its lockfile is currently
// held by a live process (internal/filelock.Peek), both of which are
// disclosed and kept instead (dc-4). Reads never delete: this function
// is the only path in this package that ever calls `git worktree
// remove`, and only WITHOUT --force. Results are sorted by worktree name
// for a deterministic report.
//
// A root with no .verdi/data/worktrees/ directory at all (nothing ever
// cut yet) returns (nil, nil) — not an error.
func GC(ctx context.Context, root, defaultBranchRef string) ([]Result, error) {
	wtRoot := worktreesRoot(root)
	entries, err := os.ReadDir(wtRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("wtmanager: GC: reading %s: %w", wtRoot, err)
	}

	var results []Result
	for _, e := range entries {
		if !e.IsDir() {
			continue // lockfiles and any stray file are not managed worktrees
		}
		r, err := gcOne(ctx, root, wtRoot, e.Name(), defaultBranchRef)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// gcOne computes and, if warranted, executes the reclaim decision for one
// managed worktree.
func gcOne(ctx context.Context, root, wtRoot, name, defaultBranchRef string) (Result, error) {
	branch := branchForWorktreeName(name)
	path := filepath.Join(wtRoot, name)
	lp := path + ".lock"
	res := Result{Name: name, Branch: branch, Path: path}

	eligible, err := reclaimEligible(ctx, root, branch, defaultBranchRef)
	if err != nil {
		return Result{}, fmt.Errorf("wtmanager: GC: checking %s eligibility: %w", name, err)
	}

	var dirty, locked bool
	var lockedPID int
	if eligible {
		dirty, err = gitx.StatusDirty(ctx, path)
		if err != nil {
			return Result{}, fmt.Errorf("wtmanager: GC: checking %s dirty: %w", name, err)
		}
		if !dirty {
			info, held, perr := filelock.Peek(lp)
			if perr != nil {
				return Result{}, fmt.Errorf("wtmanager: GC: peeking lock for %s: %w", name, perr)
			}
			locked, lockedPID = held, info.PID
		}
	}

	res.Decision = decideReclaim(eligible, dirty, locked)
	if res.Decision == KeepLocked {
		res.Detail = strconv.Itoa(lockedPID)
	}
	if res.Decision != Reclaim {
		return res, nil
	}

	// Reclaim: acquire the lock only for this single mutating call
	// (dc-2), which also closes the race between the Peek above and
	// here — a concurrent owner that grabbed the lock in between is
	// still never removed out from under it.
	f, aerr := filelock.Acquire(lp)
	if aerr != nil {
		var held *filelock.ErrHeld
		if errors.As(aerr, &held) {
			res.Decision = KeepLocked
			res.Detail = strconv.Itoa(held.Info.PID)
			return res, nil
		}
		return Result{}, fmt.Errorf("wtmanager: GC: acquiring lock for %s: %w", name, aerr)
	}
	removeErr := worktreeRemove(ctx, root, path)
	_ = filelock.Release(f, lp)
	if removeErr != nil {
		return Result{}, fmt.Errorf("wtmanager: GC: removing worktree %s: %w", name, removeErr)
	}
	return res, nil
}

// reclaimEligible computes dc-3's merged-or-deleted signal for branch.
// Deleted (LOCAL-ONLY, dc-3 — deliberately narrower than
// verdi-store-layout's general fetch-and-prune reading): branch no
// longer resolves under refs/heads/design/* at all. Merged: reuses
// gitx.IsAncestor, cross-referencing branch's own tip against
// defaultBranchRef's — skipped (not an error, not eligible) when
// defaultBranchRef can't be resolved, since a locally-deleted branch is
// independently reclaim-eligible either way.
func reclaimEligible(ctx context.Context, root, branch, defaultBranchRef string) (bool, error) {
	local, err := gitx.HasLocalBranch(ctx, root, branch)
	if err != nil {
		return false, err
	}
	if !local {
		return true, nil // deleted signal (dc-3)
	}
	if defaultBranchRef == "" {
		return false, nil
	}
	tip, err := gitx.RevParse(ctx, root, branch)
	if err != nil {
		return false, err
	}
	defTip, err := gitx.RevParse(ctx, root, defaultBranchRef)
	if err != nil {
		return false, err
	}
	return gitx.IsAncestor(ctx, root, tip, defTip)
}
