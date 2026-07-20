package residue

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/gitx"
)

// Result is internal/residue's whole scan: every AC-1/AC-2/AC-3 finding
// from one run — or a disclosed "the default branch could not be
// resolved" outcome asserting NONE of them (the story's own three-valued
// posture: "where a default branch cannot be resolved: assert nothing").
type Result struct {
	// DefaultBranchResolved is false when defaultBranchRef was empty (the
	// same "" = unknown convention gitx.DefaultBranch/lint.
	// ResolveDefaultBranch already use throughout this codebase) — every
	// other field is then zero, and the caller must disclose that rather
	// than render an empty-but-clean report (a clean report still asserts
	// something; this asserts nothing).
	DefaultBranchResolved bool
	DefaultBranch         string

	PatternA []PatternA
	PatternB []PatternB
	// CloseBranches holds every unmerged close/<name> branch, both
	// classifications together (AC-2's own report scope), sorted by name.
	CloseBranches []CloseBranch

	MergedBranches []string // AC-3(a), sorted

	Worktrees []Worktree // AC-3(b), sorted by path, primary excluded
}

// Flagged reports whether r contains an exit-1-worthy finding (dc-3): any
// AC-1 pattern (a) finding, or any AC-2 ritual-incomplete classification.
// Pattern (b), superseded-elsewhere, and the whole of AC-3's survey never
// flag — an unresolved default branch never flags either (it asserts
// nothing, which is not itself a finding).
func (r *Result) Flagged() bool {
	if r == nil {
		return false
	}
	if len(r.PatternA) > 0 {
		return true
	}
	for _, cb := range r.CloseBranches {
		if cb.Class == RitualIncomplete {
			return true
		}
	}
	return false
}

// Scan runs spec/closure-hygiene's whole closure-hygiene scan (dc-1)
// against root, at defaultBranchRef — already resolved by the caller
// (e.g. internal/lint.ResolveDefaultBranch), mirroring internal/wtmanager.
// GC's own caller-resolves convention rather than re-resolving it here.
//
// An empty defaultBranchRef yields a Result with DefaultBranchResolved
// false and every other field zero — never a guess. Any OTHER failure to
// resolve it (a non-empty ref that does not exist) is a genuine
// operational error, not a soft disclosure — mirroring internal/wtmanager.
// reclaimEligible's own precedent of propagating a real RevParse failure
// rather than silently downgrading it.
func Scan(ctx context.Context, root, defaultBranchRef string) (*Result, error) {
	if defaultBranchRef == "" {
		return &Result{}, nil
	}
	defaultTip, err := gitx.RevParse(ctx, root, defaultBranchRef)
	if err != nil {
		return nil, fmt.Errorf("residue: resolving default branch %q: %w", defaultBranchRef, err)
	}

	specs, err := walkActiveSpecs(root)
	if err != nil {
		return nil, err
	}
	// dc-2: status: superseded is excluded BEFORE either AC-1 pattern's
	// logic runs — a check that happens first, not a state that merely
	// happens never to match either pattern's own conditions — and,
	// per dc-2's own "AC-1/AC-2" grouping, before AC-2's own close/*
	// branch classification runs too (scanCloseBranches's supersededNames
	// argument): a leftover close/<name> branch for a name that has since
	// become superseded (a route that never archives at all) is stale,
	// not an actionable ritual-incomplete/superseded-elsewhere finding.
	nonSuperseded := excludeSuperseded(specs)

	closeBranches, err := scanCloseBranches(ctx, root, defaultTip, supersededNames(specs))
	if err != nil {
		return nil, err
	}
	patternA := findPatternA(closeBranches, activeStatusByName(nonSuperseded), activeClassByName(nonSuperseded))
	patternB, err := findPatternB(root, nonSuperseded)
	if err != nil {
		return nil, err
	}

	merged, err := scanMergedBranches(ctx, root, defaultBranchRef, defaultTip)
	if err != nil {
		return nil, err
	}
	worktrees, err := scanWorktrees(ctx, root, defaultTip)
	if err != nil {
		return nil, err
	}

	return &Result{
		DefaultBranchResolved: true,
		DefaultBranch:         defaultBranchRef,
		PatternA:              patternA,
		PatternB:              patternB,
		CloseBranches:         closeBranches,
		MergedBranches:        merged,
		Worktrees:             worktrees,
	}, nil
}
