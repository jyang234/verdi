package specstate

import (
	"context"
	"os"

	"github.com/jyang234/verdi/internal/gitx"
)

// ResolveDefaultBranch resolves the repository's configured default
// branch, in the same precedence internal/lint's cienv.go established
// (moved here so it becomes the ONE default-branch resolution every
// consumer shares, not a copy each package reimplements):
//
//  1. the CI_DEFAULT_BRANCH environment variable, when set;
//  2. otherwise the configured remote's origin/HEAD symbolic ref
//     (gitx.DefaultBranch);
//  3. otherwise the hermetic local fallback: exactly one of the
//     remote-tracking refs refs/remotes/origin/main or
//     refs/remotes/origin/master present (both present is ambiguous and
//     refuses rather than guesses; neither present is unknown) — the
//     fresh-GitHub-checkout shape actions/checkout leaves behind, with NO
//     network call;
//  4. otherwise unresolved.
//
// A resolved NAME is not enough on its own: the design's "If default-
// branch identity or ancestry is unavailable, acceptance is disclosed as
// unproven" rule means every caller of this package needs a ref it can
// actually pass to git, not just a string a caller might guess is a
// branch. So once a name is chosen, ResolveDefaultBranch additionally
// requires it to resolve to a real ref before reporting success: a local
// branch of that name if one is checked out, otherwise its
// "origin/<name>" remote-tracking ref, otherwise ("", false, unresolved)
// — never a bare name a later git command might fail to resolve. Local
// convenience never substitutes a branch named "main" unless that
// resolution is the explicit, visible D6-6 fallback above; there is no
// silent secondary guess.
func ResolveDefaultBranch(ctx context.Context, root string) (Branch, bool) {
	name, ok := resolveDefaultBranchName(ctx, root)
	if !ok {
		return Branch{}, false
	}
	ref, ok := resolveBranchRef(ctx, root, name)
	if !ok {
		return Branch{}, false
	}
	return Branch{Name: name, Ref: ref}, true
}

// resolveDefaultBranchName runs the name-only precedence chain: env, then
// origin/HEAD, then the D6-6 hermetic local fallback.
func resolveDefaultBranchName(ctx context.Context, root string) (string, bool) {
	if name := os.Getenv("CI_DEFAULT_BRANCH"); name != "" {
		return name, true
	}
	if branch, err := gitx.DefaultBranch(ctx, root); err == nil && branch != "" {
		return branch, true
	}
	return fallbackDefaultBranchName(ctx, root)
}

// fallbackDefaultBranchName is the D6-6 hermetic local-plumbing fallback,
// consulted only when neither CI_DEFAULT_BRANCH nor a configured
// origin/HEAD symbolic ref resolves a name — the common shape of a
// freshly checked-out GitHub Actions repository: GitHub Actions sets no
// CI_DEFAULT_BRANCH, and actions/checkout's shallow, specific-ref fetch
// never runs `git remote set-head`, so origin/HEAD is never configured
// either. Probes the two conventional default-branch names as LOCAL
// remote-tracking refs only — refs/remotes/origin/main and
// refs/remotes/origin/master — via gitx.HasRemoteTrackingBranch; NO
// network call. Exactly one present resolves to it; both present is
// ambiguous (refuse rather than guess); neither present is unknown.
func fallbackDefaultBranchName(ctx context.Context, root string) (string, bool) {
	hasMain, mainErr := gitx.HasRemoteTrackingBranch(ctx, root, "origin", "main")
	hasMaster, masterErr := gitx.HasRemoteTrackingBranch(ctx, root, "origin", "master")
	if mainErr != nil || masterErr != nil {
		return "", false
	}
	switch {
	case hasMain && hasMaster:
		return "", false // ambiguous — refuse rather than guess
	case hasMain:
		return "main", true
	case hasMaster:
		return "master", true
	default:
		return "", false
	}
}

// resolveBranchRef picks a git-resolvable ref for name: the local branch
// of that name if one is checked out, otherwise its "origin/<name>"
// remote-tracking ref, otherwise unresolved — a named branch (however it
// was discovered) that resolves nowhere at all is never reported as
// success.
func resolveBranchRef(ctx context.Context, root, name string) (string, bool) {
	hasLocal, err := gitx.HasLocalBranch(ctx, root, name)
	if err != nil {
		return "", false
	}
	if hasLocal {
		return name, true
	}

	hasRemote, err := gitx.HasRemoteTrackingBranch(ctx, root, "origin", name)
	if err != nil {
		return "", false
	}
	if hasRemote {
		return "origin/" + name, true
	}

	return "", false
}
