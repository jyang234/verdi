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
// remote's HEAD symbolic ref, else (D6-6) the hermetic local
// origin/main-or-master fallback (fallbackDefaultBranch), else ""
// (unknown, never guessed). Exported (V1-P7) so a THIRD, cross-package
// copy — internal/mcpserve's review population, which needs the same
// resolution to find a design branch's open MR — shares this one
// definition instead of duplicating it again (CLAUDE.md: anything used by
// two or more packages lives in a shared internal/ package). D6-6: this
// is the ONE seam every verb that needs "what is the default branch"
// shares (gate, close, gc, lint itself via BuildContext, the mcpserve
// tools, wallbadge via BuildContext) — the fallback added here therefore
// fixes the same "fresh GitHub checkout" friction for all of them at once,
// not gate alone.
func ResolveDefaultBranch(ctx context.Context, root string) string {
	if env := ReadCIEnv(); env.DefaultBranch != "" {
		return env.DefaultBranch
	}
	if branch, err := gitx.DefaultBranch(ctx, root); err == nil && branch != "" {
		return branch
	}
	return fallbackDefaultBranch(ctx, root)
}

// fallbackDefaultBranch is D6-6's hermetic local-plumbing fallback,
// consulted only when neither an explicit CI_DEFAULT_BRANCH env var nor a
// configured origin/HEAD symbolic ref resolves the default branch — the
// common shape of a freshly checked-out GitHub Actions repository: GitHub
// Actions sets no CI_DEFAULT_BRANCH, and actions/checkout's shallow,
// specific-ref fetch never runs `git remote set-head`, so origin/HEAD is
// never configured either. Probes the two conventional default-branch
// names as LOCAL remote-tracking refs only — refs/remotes/origin/main and
// refs/remotes/origin/master — via gitx.HasRemoteTrackingBranch; NO
// network call (never `git ls-remote`). Exactly one present resolves to
// it; both present is ambiguous (refuse rather than guess — a real repo
// should not carry both, and guessing wrong would silently point gate,
// close, etc. at the wrong branch); neither present is unknown. Every one
// of these outcomes but the single-match case returns "" — the same
// "can't prove it" value every other unresolvable case on this seam
// already returns (I-14's fail-closed, never-guess posture).
func fallbackDefaultBranch(ctx context.Context, root string) string {
	hasMain, mainErr := gitx.HasRemoteTrackingBranch(ctx, root, "origin", "main")
	hasMaster, masterErr := gitx.HasRemoteTrackingBranch(ctx, root, "origin", "master")
	if mainErr != nil || masterErr != nil {
		return ""
	}
	switch {
	case hasMain && hasMaster:
		return "" // ambiguous — refuse rather than guess
	case hasMain:
		return "main"
	case hasMaster:
		return "master"
	default:
		return ""
	}
}
