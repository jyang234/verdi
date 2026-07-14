package lint

import (
	"context"
	"os"

	"github.com/jyang234/verdi/internal/gitx"
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

// ResolveDefaultBranch is the "which branch is the default" resolution
// this package's own buildLintContext caller and cmd/verdi/gate.go's
// resolveDefaultBranch both already implemented independently (small,
// same-package duplication each of those doc comments calls out as
// deliberate): CI_DEFAULT_BRANCH env var first, else the configured
// remote's HEAD symbolic ref, else "" (unknown, never guessed). Exported
// (V1-P7) so a THIRD, cross-package copy — internal/mcpserve's review
// population, which needs the same resolution to find a design branch's
// open MR — shares this one definition instead of duplicating it again
// (CLAUDE.md: anything used by two or more packages lives in a shared
// internal/ package).
func ResolveDefaultBranch(ctx context.Context, root string) string {
	if env := ReadCIEnv(); env.DefaultBranch != "" {
		return env.DefaultBranch
	}
	if branch, err := gitx.DefaultBranch(ctx, root); err == nil {
		return branch
	}
	return ""
}
