package lint

import (
	"context"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// Context carries the git- and CI-derived facts the git-aware rules need
// (I-14). The CLI fills this from git (symbolic-ref/merge-base — see
// internal/gitx's CurrentBranch/DefaultBranch/MergeBase) and, when
// present, generic CI environment variables (see cienv.go); tests
// construct it directly.
type Context struct {
	// DefaultBranch is the store's default branch short name (e.g. "main"),
	// or "" when it cannot be established (no CI default-branch variable,
	// no configured git remote HEAD, and — D6-6's hermetic fallback — no
	// single unambiguous local origin/main or origin/master ref either) —
	// I-14's "otherwise" case.
	DefaultBranch string
	// CurrentBranch is the currently checked-out branch's short name, or ""
	// on a detached HEAD.
	CurrentBranch string
	// TargetBranch is the branch an open MR/PR targets, read from CI
	// environment variables only (a local checkout has no reliable way to
	// know this) — "" when not running in an MR/PR pipeline.
	TargetBranch string
	// DiffBase is the commit VL-010 diffs HEAD against — I-14:
	// "merge-base(HEAD, default branch)" — supplied by the caller (the CLI
	// computes it via gitx.MergeBase; tests set it directly to an exact
	// fixture commit).
	DiffBase string
	// InCI reports whether a recognized CI environment was detected.
	InCI bool
}

// TargetsDefaultBoundary reports whether this lint run sits AT the
// default-branch PR boundary — linting the default branch itself, or a
// change/PR targeting it in CI — the single gate every readiness check
// that used to key off a persisted status field now shares (Task 4,
// "Move readiness checks to the PR boundary": VL-020's obligation
// completeness, VL-004's legacy-draft compatibility disclosure). Renamed
// from EnforceDraftGate, whose I-14 meaning it keeps verbatim ("VL-004
// enforced when linting the default branch or a change targeting it;
// otherwise a warning, not a finding") — merge-signaled acceptance widens
// the boundary's readership beyond VL-004 alone, so the name no longer
// names one rule. An unknown DefaultBranch can never report true — three-
// valued honesty (constitution 2): lint cannot prove it is looking at the
// default branch, so it does not claim to.
func (c Context) TargetsDefaultBoundary() bool {
	if c.DefaultBranch == "" {
		return false
	}
	if c.CurrentBranch != "" && c.CurrentBranch == c.DefaultBranch {
		return true
	}
	if c.InCI && c.TargetBranch != "" && c.TargetBranch == c.DefaultBranch {
		return true
	}
	return false
}

// BuildContext derives Context from git and CI environment signals per
// I-14: CurrentBranch via symbolic-ref; DefaultBranch via
// specstate.ResolveDefaultBranch's Branch.Name; DiffBase via
// merge-base(HEAD, Branch.Ref) when the default branch is known — Ref,
// never Name, is what is actually passed to `git merge-base`, since Ref
// is the field specstate guarantees is git-resolvable (a bare short name
// like "master" can fail to resolve locally when only a remote-tracking
// ref exists). Every git/CI lookup failure degrades to "unknown" rather
// than aborting — the git-aware rules already treat an unknown field as
// "can't prove it, don't enforce" (three-valued honesty, constitution 2).
//
// Lifted from cmd/verdi/lint.go's buildLintContext (verbatim behavior) so
// the disclosures-view enumeration (internal/disclosureview,
// spec/disclosures-panel ac-1) runs the SAME context-construction path
// `verdi lint` runs — and so internal/specalign's test no longer needs
// its own documented duplicate of it.
func BuildContext(ctx context.Context, root string) Context {
	env := ReadCIEnv()

	var lctx Context
	lctx.InCI = env.InCI
	lctx.TargetBranch = env.TargetBranch

	if branch, err := gitx.CurrentBranch(ctx, root); err == nil {
		lctx.CurrentBranch = branch
	}

	if branch, ok := specstate.ResolveDefaultBranch(ctx, root); ok {
		lctx.DefaultBranch = branch.Name

		if base, err := gitx.MergeBase(ctx, root, "HEAD", branch.Ref); err == nil {
			lctx.DiffBase = base
		}
	}

	return lctx
}
