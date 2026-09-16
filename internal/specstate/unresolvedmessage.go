package specstate

import "context"

// UnresolvedDefaultBranchMessage returns the legible, actionable diagnostic
// for the specific cause an operator hits most often when a Git-derived
// verdict comes back unprovable — the default branch could not be resolved
// AT ALL (D6-6) — by independently re-checking ResolveDefaultBranch and, if
// it fails, naming every source the resolution chain tries
// (CI_DEFAULT_BRANCH, configured git remote HEAD, the unambiguous local
// origin/main-or-master fallback) plus the `git remote set-head` remedy.
// Returns "" when the default branch DOES resolve for root — meaning a
// caller's own Unproven verdict (if any) has some OTHER cause.
//
// This is the ONE shared wording every verb that needs it reuses verbatim
// (build start, design start's --kind/--name path, and — dc-7,
// spec/uat-round-1, I-130 — design start --from-stub via
// internal/stubinstantiate), moved here from cmd/verdi/buildstart.go's own
// former private copy so a second package (internal/stubinstantiate) could
// reach it without duplicating the text: cmd/verdi's own
// unresolvableDefaultBranchMessage now delegates to this function verbatim,
// so an operator sees the identical diagnostic for the identical failure
// regardless of which verb or package surfaced it.
func UnresolvedDefaultBranchMessage(ctx context.Context, root string) string {
	if _, ok := ResolveDefaultBranch(ctx, root); ok {
		return ""
	}
	return "cannot determine the default branch (no CI_DEFAULT_BRANCH, no configured git remote HEAD, and no single unambiguous local origin/main or origin/master ref) — failing closed; run `git remote set-head origin <branch>` to configure it"
}
