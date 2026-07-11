package lint

// Context carries the git- and CI-derived facts the git-aware rules need
// (I-14). The CLI fills this from git (symbolic-ref/merge-base — see
// internal/gitx's CurrentBranch/DefaultBranch/MergeBase) and, when
// present, generic CI environment variables (see cienv.go); tests
// construct it directly.
type Context struct {
	// DefaultBranch is the store's default branch short name (e.g. "main"),
	// or "" when it cannot be established (no configured git remote HEAD
	// and no CI default-branch variable) — I-14's "otherwise" case.
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

// EnforceDraftGate reports whether VL-004 must be enforced as a finding
// (true) rather than merely warned about (false), per I-14: "VL-004
// enforced when linting the default branch or a change targeting it;
// otherwise a warning, not a finding." An unknown DefaultBranch can never
// enforce — three-valued honesty (constitution 2): lint cannot prove it is
// looking at the default branch, so it does not claim to.
func (c Context) EnforceDraftGate() bool {
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
