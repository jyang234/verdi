package lint

import (
	"context"
	"os"

	"github.com/jyang234/verdi/internal/specstate"
)

// CIEnv is the generic CI environment signal this package reads directly:
// GitLab's CI_DEFAULT_BRANCH/CI_MERGE_REQUEST_TARGET_BRANCH_NAME and
// GitHub Actions' GITHUB_BASE_REF, plus each forge's own "am I running in
// CI at all" marker. Kept in this one small file, deliberately not grown
// beyond these variables: the I-22 forge port (another agent's work) will
// absorb CI-context detection properly once it exists; this is the
// generic stopgap phase 4 needs for VL-004's I-14 baseline today.
type CIEnv struct {
	// DefaultBranch is the repository's configured default branch, when a
	// CI job declares it (GitLab: CI_DEFAULT_BRANCH).
	DefaultBranch string
	// TargetBranch is the branch an open MR/PR targets (GitLab:
	// CI_MERGE_REQUEST_TARGET_BRANCH_NAME; GitHub Actions: GITHUB_BASE_REF).
	TargetBranch string
	// InCI reports whether either forge's own "running in CI" marker
	// (GitLab: CI; GitHub Actions: GITHUB_ACTIONS) is set.
	InCI bool
}

// ReadCIEnv reads CIEnv from the process environment.
func ReadCIEnv() CIEnv {
	var e CIEnv
	e.DefaultBranch = os.Getenv("CI_DEFAULT_BRANCH")
	e.TargetBranch = os.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")
	if e.TargetBranch == "" {
		e.TargetBranch = os.Getenv("GITHUB_BASE_REF")
	}
	e.InCI = os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
	return e
}

// ResolveDefaultBranch is the compatibility wrapper over
// internal/specstate.ResolveDefaultBranch, which now owns the "which
// branch is the default" resolution algorithm this function used to
// implement directly (moved there so spec-acceptance state derivation and
// every other consumer of "what is the default branch" — gate, close, gc,
// lint itself via BuildContext, the mcpserve tools, wallbadge — share the
// ONE definition, CLAUDE.md: "anything used by two or more packages lives
// in a shared internal/ package"). It keeps this package's existing
// short-name contract: callers that only ever compared DefaultBranch as a
// short branch-name string (e.g. against CurrentBranch or TargetBranch)
// keep compiling and behaving identically, receiving Branch.Name and
// never Branch.Ref. New callers that need a ref they can pass to git
// directly (merge-base, ls-tree, ...) should call
// specstate.ResolveDefaultBranch themselves for Branch.Ref instead.
func ResolveDefaultBranch(ctx context.Context, root string) string {
	branch, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok {
		return ""
	}
	return branch.Name
}
